package d1

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/planetscale/cli/internal/printer"
)

// PrintHumanResponse writes a human-readable success response via the shared printer.
func PrintHumanResponse(p *printer.Printer, resp Response) {
	p.Printf("Status: %s", resp.Status)
	if resp.Phase != "" {
		p.Printf(" (%s)", resp.Phase)
	} else if resp.Command != "" {
		p.Printf(" (%s)", resp.Command)
	}
	p.Println()

	if resp.MigrationID != "" {
		p.Printf("Migration ID: %s\n", resp.MigrationID)
	}

	printHumanData(p, resp.Command, resp.Data)

	if resp.Error != nil {
		p.Printf("\nError [%s]: %s\n", resp.Error.Code, resp.Error.Message)
		if resp.Error.Remediation != "" {
			p.Printf("%s\n", resp.Error.Remediation)
		}
	}

	if len(resp.Issues) > 0 {
		p.Printf("\nIssues (%d):\n", len(resp.Issues))
		printIssuesGrouped(p, resp.Issues, "  ")
	}

	if len(resp.NextSteps) > 0 {
		p.Println("\nNext steps:")
		for _, step := range resp.NextSteps {
			if step.Command != "" {
				p.Printf("  - %s (%s)\n", step.Command, step.Reason)
			} else if step.Tool != "" {
				p.Printf("  - %s: %s\n", step.Tool, step.Reason)
			} else {
				p.Printf("  - %s\n", step.Reason)
			}
		}
	}
}

type issueGroup struct {
	severity  string
	code      string
	text      string
	locations []string
}

// printIssuesGrouped renders issues ordered by severity (errors first) and
// collapses issues sharing a code and message onto one line listing every
// affected table/column, so a finding that hits many columns reads as one
// entry instead of a wall of near-identical lines.
func printIssuesGrouped(p *printer.Printer, issues []Issue, indent string) {
	var groups []issueGroup
	index := map[string]int{}
	for _, issue := range issues {
		text := issue.Remediation
		if text == "" {
			text = issue.Message
		}
		key := issue.Severity + "\x00" + issue.Code + "\x00" + text
		i, ok := index[key]
		if !ok {
			i = len(groups)
			index[key] = i
			groups = append(groups, issueGroup{severity: issue.Severity, code: issue.Code, text: text})
		}
		loc := issue.Table
		if issue.Column != "" {
			loc += "." + issue.Column
		}
		if loc != "" {
			groups[i].locations = append(groups[i].locations, loc)
		}
	}

	sort.SliceStable(groups, func(a, b int) bool {
		return severityRank(groups[a].severity) < severityRank(groups[b].severity)
	})

	lastSeverity := ""
	for _, g := range groups {
		if g.severity != lastSeverity {
			p.Printf("%s%s:\n", indent, severityLabel(g.severity))
			lastSeverity = g.severity
		}
		switch len(g.locations) {
		case 0:
			p.Printf("%s  %s: %s\n", indent, g.code, g.text)
		case 1:
			p.Printf("%s  %s %s: %s\n", indent, g.code, g.locations[0], g.text)
		default:
			p.Printf("%s  %s (%d): %s\n", indent, g.code, len(g.locations), g.text)
			for _, line := range wrapList(g.locations, locationLineWidth-len(indent)-4) {
				p.Printf("%s    %s\n", indent, line)
			}
		}
	}
}

const locationLineWidth = 96

// wrapList joins items with commas, breaking into multiple lines so no line
// exceeds width (except when a single item is longer than width).
func wrapList(items []string, width int) []string {
	var lines []string
	var line strings.Builder
	for _, item := range items {
		if line.Len() == 0 {
			line.WriteString(item)
			continue
		}
		if line.Len()+2+len(item) > width {
			lines = append(lines, line.String()+",")
			line.Reset()
			line.WriteString(item)
			continue
		}
		line.WriteString(", ")
		line.WriteString(item)
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return lines
}

func severityRank(severity string) int {
	switch severity {
	case SeverityError:
		return 0
	case SeverityWarning:
		return 1
	case SeverityInfo:
		return 2
	}
	return 3
}

func severityLabel(severity string) string {
	switch severity {
	case SeverityError:
		return "Errors"
	case SeverityWarning:
		return "Warnings"
	case SeverityInfo:
		return "Info"
	}
	return severity
}

func printVerifyResultHuman(p *printer.Printer, r VerifyResult) {
	matched := "no"
	if r.Matched {
		matched = "yes"
	}
	p.Printf("\nMatched: %s\n", matched)

	for _, table := range r.Tables {
		if table.Match {
			continue
		}
		p.Printf("  row count mismatch %s: sqlite=%d postgres=%d\n", table.Table, table.SourceRows, table.DestRows)
	}
	for _, check := range r.Checks {
		if check.Matched {
			continue
		}
		label := check.Name
		if check.Table != "" {
			label = check.Table
			if check.Column != "" {
				label += "." + check.Column
			}
		}
		if check.Message != "" {
			p.Printf("  check failed %s: %s\n", label, check.Message)
		} else {
			p.Printf("  check failed %s\n", label)
		}
	}
}

func printMigrationStateHuman(p *printer.Printer, r MigrationState) {
	if r.Method != "" {
		p.Printf("Method: %s\n", r.Method)
	}
	if len(r.LoadedTables) > 0 {
		p.Printf("Tables loaded: %d\n", len(r.LoadedTables))
	}
	if r.InputPath != "" {
		p.Printf("Input: %s\n", r.InputPath)
	}
	if !r.UpdatedAt.IsZero() {
		p.Printf("Updated: %s\n", r.UpdatedAt.Format(time.RFC3339))
	}
}

func printImportResultHuman(p *printer.Printer, r ImportResult) {
	p.Printf("\nMethod: %s", r.Method)
	if r.DryRun {
		p.Print(" (dry run)")
	}
	p.Println()
	if r.Plan != nil {
		sizeMB := float64(r.Plan.EstimatedSizeBytes) / (1024 * 1024)
		p.Printf("Plan: %d tables, %.1f MB estimated\n", len(r.Plan.Tables), sizeMB)
	}
	if r.TablesLoaded > 0 {
		p.Printf("Tables loaded: %d\n", r.TablesLoaded)
	}
	if r.Timings != nil && r.Timings.TotalMs > 0 {
		p.Printf("Total time: %.1fs\n", float64(r.Timings.TotalMs)/1000)
	}
}

func printDoctorResultHuman(p *printer.Printer, r DoctorResult) {
	p.Println("\nChecks:")
	for _, c := range r.Checks {
		line := fmt.Sprintf("  %s: %s", c.Name, c.Status)
		if c.Version != "" {
			line += fmt.Sprintf(" (%s)", c.Version)
		}
		p.Println(line)
	}
	p.Printf("Ready: %v\n", r.Ready)
}

func printHumanData(p *printer.Printer, command string, data any) {
	if data == nil {
		return
	}

	switch command {
	case "doctor":
		switch r := data.(type) {
		case DoctorResult:
			printDoctorResultHuman(p, r)
		case *DoctorResult:
			if r != nil {
				printDoctorResultHuman(p, *r)
			}
		}
	case "lint":
		switch r := data.(type) {
		case LintResult:
			p.Printf("\nTables: %d | Errors: %d | Warnings: %d\n", r.TableCount, r.ErrorCount, r.WarningCount)
		case *LintResult:
			if r != nil {
				p.Printf("\nTables: %d | Errors: %d | Warnings: %d\n", r.TableCount, r.ErrorCount, r.WarningCount)
			}
		}
	case "start":
		switch r := data.(type) {
		case ImportResult:
			printImportResultHuman(p, r)
		case *ImportResult:
			if r != nil {
				printImportResultHuman(p, *r)
			}
		}
	case "verify":
		switch r := data.(type) {
		case VerifyResult:
			printVerifyResultHuman(p, r)
		case *VerifyResult:
			if r != nil {
				printVerifyResultHuman(p, *r)
			}
		}
	case "status":
		switch r := data.(type) {
		case MigrationState:
			printMigrationStateHuman(p, r)
		case *MigrationState:
			if r != nil {
				printMigrationStateHuman(p, *r)
			}
		}
	case "convert-schema":
		if m, ok := data.(map[string]any); ok {
			p.Println()
			p.Printf("  Input: %v\n", m["input"])
			p.Printf("  Output: %v\n", m["output"])
			p.Printf("  Tables: %v\n", m["table_count"])
		}
	case "complete":
		switch r := data.(type) {
		case CompleteResult:
			p.Println()
			p.Printf("  Migration ID: %s\n", r.MigrationID)
			p.Printf("  Status: %s\n", r.Status)
			printCompleteReminderHuman(p, r)
		case *CompleteResult:
			if r != nil {
				p.Println()
				p.Printf("  Migration ID: %s\n", r.MigrationID)
				p.Printf("  Status: %s\n", r.Status)
				printCompleteReminderHuman(p, *r)
			}
		case map[string]string:
			p.Println()
			p.Printf("  Migration ID: %s\n", r["migration_id"])
			p.Printf("  Status: %s\n", r["status"])
		}
	}
}

// StatusResponse builds the status command envelope.
func StatusResponse(state *MigrationState) Response {
	var next []NextStep
	if state != nil {
		next = StatusNextSteps(state)
	}
	resp := OKResponse("status", state, next)
	if state != nil {
		resp.MigrationID = state.MigrationID
		resp.Phase = state.Phase
	}
	return resp
}

// OKResponse builds a success response.
func OKResponse(command string, data any, next []NextStep) Response {
	return Response{
		Status:    "ok",
		Command:   command,
		Data:      data,
		NextSteps: next,
	}
}

// ErrorResponse builds an error response from an error.
func ErrorResponse(command string, err error) Response {
	resp := Response{
		Status:  "error",
		Command: command,
	}
	if me, ok := migrationErr(err); ok {
		resp.Error = &me.Info
	} else {
		resp.Error = &ErrorInfo{
			Code:    ErrCodeImportFailed,
			Message: err.Error(),
		}
	}
	return resp
}

// DoctorResponse builds the doctor command envelope, including check details when not ready.
func DoctorResponse(result *DoctorResult) Response {
	resp := OKResponse("doctor", result, DoctorNextSteps(result))
	if result != nil && !result.Ready {
		resp.Status = "error"
		if err := DoctorReadinessError(result); err != nil {
			if me, ok := migrationErr(err); ok {
				resp.Error = &me.Info
			} else {
				resp.Error = &ErrorInfo{
					Code:    ErrCodePrereqFailed,
					Message: err.Error(),
				}
			}
		}
	}
	return resp
}

// LintResponse builds the lint command envelope with status derived from issue severity.
func LintResponse(result *LintResult) Response {
	resp := OKResponse("lint", result, LintNextSteps(result))
	resp.Issues = result.Issues
	if result.ErrorCount > 0 {
		resp.Status = "error"
		resp.Error = &ErrorInfo{
			Code:        ErrCodeLintBlocked,
			Message:     lintBlockedReason(result.ErrorCount),
			Remediation: lintBlockedRemediation,
		}
	} else if result.WarningCount > 0 {
		resp.Status = "warning"
	}
	return resp
}
