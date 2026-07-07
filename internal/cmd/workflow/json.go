package workflow

import (
	"strings"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/printer"
	ps "github.com/planetscale/planetscale-go/planetscale"
)

// errorContext carries workflow command context so JSON error responses can
// include remediation commands with real values instead of placeholders.
type errorContext struct {
	Org      string
	Database string
	Branch   string
	Number   string
}

// handle converts err into a structured JSON error when the printer is in
// JSON mode; in human mode it returns err unchanged.
func (c errorContext) handle(ch *cmdutil.Helper, err error) error {
	if err == nil || ch.Printer.Format() != printer.JSON {
		return err
	}
	if cmdutil.ErrCode(err) == ps.ErrNotFound || strings.Contains(err.Error(), "does not exist") {
		return c.report(ch, "error", "NOT_FOUND", err.Error(), c.notFoundNextSteps())
	}
	// Failures that are not workflow-specific (auth, network, usage) get the
	// same code and remediation the global classifier would give on any other
	// command, so an expired token reports NO_AUTH with login steps rather
	// than WORKFLOW_ERROR.
	if resp := cmdutil.GlobalJSONError(err); resp.Code != "COMMAND_FAILED" {
		return c.report(ch, resp.Status, resp.Code, resp.Error, resp.NextSteps)
	}
	return c.report(ch, "error", "WORKFLOW_ERROR", err.Error(), c.defaultNextSteps())
}

// reportConfirmationRequired reports a confirmation-gated action in JSON mode.
// Callers gate on JSON format before calling.
func (c errorContext) reportConfirmationRequired(ch *cmdutil.Helper, code, message, retryCmd string) error {
	return c.report(ch, "action_required", code, message, []string{
		"Ask the user to approve this workflow action",
		retryCmd,
	})
}

func reportMissingCreateFlags(ch *cmdutil.Helper, org, database, branch string, err error) error {
	if ch.Printer.Format() != printer.JSON {
		return err
	}
	c := errorContext{Org: org, Database: database, Branch: branch}
	return c.report(ch, "action_required", "MISSING_FLAGS", err.Error(), []string{
		cmdutil.AgentKeyspaceListCmd(org, database, branch),
		cmdutil.AgentWorkflowCreateCmd(org, database, branch),
	})
}

func (c errorContext) report(ch *cmdutil.Helper, status, code, message string, nextSteps []string) error {
	payload := map[string]any{
		"status": status,
		"error":  message,
		"issues": []map[string]string{
			{
				"code":    code,
				"message": message,
			},
		},
		"next_steps": nextSteps,
	}
	if c.Database != "" {
		payload["database"] = c.Database
	}
	if c.Branch != "" {
		payload["branch"] = c.Branch
	}
	if c.Number != "" {
		payload["workflow_number"] = c.Number
	}

	if err := ch.Printer.PrintJSON(payload); err != nil {
		return err
	}
	exitCode := cmdutil.FatalErrExitCode
	if status == "action_required" {
		exitCode = cmdutil.ActionRequestedExitCode
	}
	return cmdutil.JSONReportedError(exitCode)
}

func (c errorContext) notFoundNextSteps() []string {
	steps := []string{
		cmdutil.AgentDatabaseListCmd(c.Org),
		cmdutil.AgentWorkflowListCmd(c.Org, c.Database),
	}
	if c.Branch != "" {
		steps = append(steps, cmdutil.AgentKeyspaceListCmd(c.Org, c.Database, c.Branch))
	}
	if c.Number != "" {
		steps = append(steps, cmdutil.AgentWorkflowShowCmd(c.Org, c.Database, c.Number))
	}
	return steps
}

func (c errorContext) defaultNextSteps() []string {
	steps := []string{cmdutil.AgentAuthCheckCmd()}
	if c.Database != "" {
		steps = append(steps, cmdutil.AgentWorkflowListCmd(c.Org, c.Database))
		if c.Number != "" {
			steps = append(steps, cmdutil.AgentWorkflowShowCmd(c.Org, c.Database, c.Number))
		}
	}
	return steps
}
