package sql

import (
	"errors"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/printer"
	"github.com/planetscale/cli/internal/sqlquery"
)

func reportJSON(ch *cmdutil.Helper, v any, exitCode int) error {
	if err := ch.Printer.PrintJSON(v); err != nil {
		return err
	}
	return cmdutil.JSONReportedError(exitCode)
}

func handleExecuteError(ch *cmdutil.Helper, err error, database, branch string) error {
	if ch.Printer.Format() != printer.JSON {
		return err
	}

	if _, ok := errors.AsType[*sqlquery.DestructiveQueryError](err); ok {
		return reportJSON(ch, map[string]any{
			"status":     "action_required",
			"query_kind": "destructive",
			"message":    err.Error(),
			"issues": []map[string]string{
				{
					"code":        "DESTRUCTIVE_SQL",
					"message":     "Query would delete or drop data or schema objects",
					"remediation": "Ask the user to approve, then re-run the same command with --force",
				},
			},
			"next_steps": []string{
				"Ask the user to approve this destructive query",
				cmdutil.AgentSQLCmd(ch.Config.Organization, database, branch, true),
			},
		}, cmdutil.ActionRequestedExitCode)
	}

	return reportJSON(ch, map[string]any{
		"status": "error",
		"error":  err.Error(),
		"next_steps": []string{
			cmdutil.AgentAuthCheckCmd(),
			cmdutil.AgentSQLCmd(ch.Config.Organization, database, branch, false),
		},
	}, cmdutil.FatalErrExitCode)
}
