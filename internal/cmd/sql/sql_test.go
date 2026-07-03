package sql

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/spf13/cobra"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"
	"github.com/planetscale/cli/internal/printer"
)

func TestSQLCmdHasOrgPersistentFlag(t *testing.T) {
	format := printer.Human
	ch := &cmdutil.Helper{
		Printer: printer.NewPrinter(&format),
		Config:  &config.Config{},
	}
	cmd := SQLCmd(ch)
	if cmd.PersistentFlags().Lookup("org") == nil {
		t.Fatal("expected --org persistent flag on sql subcommand")
	}
}

func TestSQLCmdParsesPositionalsBeforeOrgFlag(t *testing.T) {
	format := printer.JSON
	ch := &cmdutil.Helper{
		Printer: printer.NewPrinter(&format),
		Config:  &config.Config{AccessToken: "token"},
	}
	cmd := SQLCmd(ch)
	cmd.RunE = func(_ *cobra.Command, args []string) error {
		if ch.Config.Organization != "acme" {
			t.Fatalf("org = %q, want acme", ch.Config.Organization)
		}
		if len(args) != 2 || args[0] != "mydb" || args[1] != "main" {
			t.Fatalf("args = %v, want [mydb main]", args)
		}
		return nil
	}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"mydb", "main", "--org", "acme", "--query", "SELECT 1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestSQLCmdDestructiveQueryReturnsActionRequiredJSON(t *testing.T) {
	format := printer.JSON
	var out bytes.Buffer
	ch := &cmdutil.Helper{
		Printer: printer.NewPrinter(&format),
		Config:  &config.Config{Organization: "acme", AccessToken: "token"},
	}
	ch.Printer.SetResourceOutput(&out)
	cmd := SQLCmd(ch)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	ch.Printer.SetResourceOutput(&out)
	cmd.SetArgs([]string{"mydb", "main", "--org", "acme", "--query", "DELETE FROM users"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	var cmdErr *cmdutil.Error
	if !errors.As(err, &cmdErr) || !cmdErr.Handled {
		t.Fatalf("expected handled JSONReportedError, got %v", err)
	}
	if cmdErr.ExitCode != cmdutil.ActionRequestedExitCode {
		t.Fatalf("exit code = %d", cmdErr.ExitCode)
	}

	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp["status"] != "action_required" {
		t.Fatalf("status = %v", resp["status"])
	}
	if resp["query_kind"] != "destructive" {
		t.Fatalf("query_kind = %v", resp["query_kind"])
	}
}

func TestHandleExecuteErrorReturnsFatalExitForJSONError(t *testing.T) {
	format := printer.JSON
	var out bytes.Buffer
	ch := &cmdutil.Helper{
		Printer: printer.NewPrinter(&format),
		Config:  &config.Config{Organization: "acme"},
	}
	ch.Printer.SetResourceOutput(&out)

	err := handleExecuteError(ch, errors.New("query failed"), "mydb", "main")
	if err == nil {
		t.Fatal("expected error")
	}
	var cmdErr *cmdutil.Error
	if !errors.As(err, &cmdErr) || !cmdErr.Handled {
		t.Fatalf("expected handled JSONReportedError, got %v", err)
	}
	if cmdErr.ExitCode != cmdutil.FatalErrExitCode {
		t.Fatalf("exit code = %d", cmdErr.ExitCode)
	}

	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp["status"] != "error" {
		t.Fatalf("status = %v", resp["status"])
	}
}
