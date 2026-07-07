package cmdutil

import (
	"fmt"
	"strings"
)

// Agent command strings for next_steps — flags go after the subcommand (not on
// pscale root). Empty values render as <placeholder> for the agent to fill in.

func AgentGuideCmd() string {
	return "pscale agent-guide --format json"
}

func AgentAuthCheckCmd() string {
	return "pscale auth check --format json"
}

func AgentAuthLoginCmd() string {
	return "pscale auth login --format json"
}

func AgentOrgListCmd() string {
	return "pscale org list --format json"
}

func AgentDatabaseListCmd(org string) string {
	return fmt.Sprintf("pscale database list --org %s --format json", orPlaceholder(org, "<org>"))
}

func AgentBranchListCmd(org, database string) string {
	return fmt.Sprintf("pscale branch list %s --org %s --format json",
		orPlaceholder(database, "<database>"), orPlaceholder(org, "<org>"))
}

func AgentSQLCmd(org, database, branch string, force bool) string {
	forceFlag, query := "", "SELECT 1"
	if force {
		forceFlag, query = " --force", "<query>"
	}
	return fmt.Sprintf("pscale sql %s %s --org %s --format json%s --query %q",
		orPlaceholder(database, "<database>"), orPlaceholder(branch, "<branch>"),
		orPlaceholder(org, "<org>"), forceFlag, query)
}

func AgentWorkflowListCmd(org, database string) string {
	return fmt.Sprintf("pscale workflow list %s --org %s --format json",
		orPlaceholder(database, "<database>"), orPlaceholder(org, "<org>"))
}

func AgentWorkflowShowCmd(org, database, number string) string {
	return fmt.Sprintf("pscale workflow show %s %s --org %s --format json",
		orPlaceholder(database, "<database>"), orPlaceholder(number, "<number>"),
		orPlaceholder(org, "<org>"))
}

func AgentWorkflowCreateCmd(org, database, branch string) string {
	return fmt.Sprintf("pscale workflow create %s %s --org %s --format json --source-keyspace <source> --target-keyspace <target> --tables <table>",
		orPlaceholder(database, "<database>"), orPlaceholder(branch, "<branch>"),
		orPlaceholder(org, "<org>"))
}

func AgentKeyspaceListCmd(org, database, branch string) string {
	return fmt.Sprintf("pscale keyspace list %s %s --org %s --format json",
		orPlaceholder(database, "<database>"), orPlaceholder(branch, "<branch>"),
		orPlaceholder(org, "<org>"))
}

// AgentWorkflowActionCmd renders a workflow subcommand like cutover or cancel,
// with extraFlags appended after --format json.
func AgentWorkflowActionCmd(org, database, number, action string, extraFlags ...string) string {
	flags := ""
	if len(extraFlags) > 0 {
		flags = " " + strings.Join(extraFlags, " ")
	}
	return fmt.Sprintf("pscale workflow %s %s %s --org %s --format json%s",
		action, orPlaceholder(database, "<database>"), orPlaceholder(number, "<number>"),
		orPlaceholder(org, "<org>"), flags)
}

func orPlaceholder(s, placeholder string) string {
	if s == "" {
		return placeholder
	}
	return s
}
