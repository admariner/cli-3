package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/printer"
)

func TestHandleWorkflowErrorMissingFlagsJSON(t *testing.T) {
	format := printer.JSON
	var out bytes.Buffer
	ch := &cmdutil.Helper{
		Printer: printer.NewPrinter(&format),
	}
	ch.Printer.SetResourceOutput(&out)

	err := reportMissingCreateFlags(ch, "bb", "mydb", "main", errors.New("missing required flags: --source-keyspace, --tables"))
	if err == nil {
		t.Fatal("expected handled JSON error")
	}
	if cmdErr, ok := errors.AsType[*cmdutil.Error](err); !ok || !cmdErr.Handled {
		t.Fatalf("expected handled JSON error, got %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp["status"] != "action_required" {
		t.Fatalf("status = %v", resp["status"])
	}
	nextSteps, ok := resp["next_steps"].([]any)
	if !ok || len(nextSteps) < 2 {
		t.Fatalf("next_steps = %v", resp["next_steps"])
	}
}

func TestHandleWorkflowErrorNotFoundJSON(t *testing.T) {
	format := printer.JSON
	var out bytes.Buffer
	ch := &cmdutil.Helper{
		Printer: printer.NewPrinter(&format),
	}
	ch.Printer.SetResourceOutput(&out)

	ctx := errorContext{Org: "bb", Database: "mydb", Branch: "main"}
	err := ctx.handle(ch, errors.New("database mydb does not exist in organization bb"))
	if err == nil {
		t.Fatal("expected handled JSON error")
	}

	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp["status"] != "error" {
		t.Fatalf("status = %v", resp["status"])
	}
}

func TestReportConfirmationRequiredJSON(t *testing.T) {
	format := printer.JSON
	var out bytes.Buffer
	ch := &cmdutil.Helper{
		Printer: printer.NewPrinter(&format),
	}
	ch.Printer.SetResourceOutput(&out)

	ctx := errorContext{Org: "bb", Database: "mydb", Number: "1"}
	err := ctx.reportConfirmationRequired(ch, "CUTOVER_CONFIRMATION_REQUIRED",
		"Cutover deletes moved tables from the source keyspace and ends replication",
		cmdutil.AgentWorkflowActionCmd("bb", "mydb", "1", "cutover", "--force"))
	if err == nil {
		t.Fatal("expected handled JSON error")
	}

	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp["status"] != "action_required" {
		t.Fatalf("status = %v", resp["status"])
	}
}

func TestHandleWorkflowErrorAuthFailureJSON(t *testing.T) {
	format := printer.JSON
	var out bytes.Buffer
	ch := &cmdutil.Helper{
		Printer: printer.NewPrinter(&format),
	}
	ch.Printer.SetResourceOutput(&out)

	ctx := errorContext{Org: "bb", Database: "mydb", Number: "1"}
	err := ctx.handle(ch, errors.New("the access token has expired. Please run 'pscale auth login'"))
	if err == nil {
		t.Fatal("expected handled JSON error")
	}
	if cmdErr, ok := errors.AsType[*cmdutil.Error](err); !ok || !cmdErr.Handled || cmdErr.ExitCode != cmdutil.ActionRequestedExitCode {
		t.Fatalf("expected handled action_required error, got %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp["status"] != "action_required" {
		t.Fatalf("status = %v", resp["status"])
	}
	issues, ok := resp["issues"].([]any)
	if !ok || len(issues) == 0 {
		t.Fatalf("issues = %v", resp["issues"])
	}
	issue, ok := issues[0].(map[string]any)
	if !ok || issue["code"] != "NO_AUTH" {
		t.Fatalf("issue = %v", issues[0])
	}
	nextSteps, ok := resp["next_steps"].([]any)
	if !ok || len(nextSteps) == 0 || nextSteps[0] != cmdutil.AgentAuthLoginCmd() {
		t.Fatalf("next_steps = %v", resp["next_steps"])
	}
}

func TestHandleWorkflowErrorHumanModePassthrough(t *testing.T) {
	format := printer.Human
	ch := &cmdutil.Helper{
		Printer: printer.NewPrinter(&format),
	}

	orig := errors.New("database mydb does not exist in organization bb")
	err := errorContext{Org: "bb", Database: "mydb"}.handle(ch, orig)
	if !errors.Is(err, orig) {
		t.Fatalf("expected original error, got %v", err)
	}
}
