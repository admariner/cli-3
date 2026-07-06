package cmdutil

import "fmt"

// Agent command strings for next_steps — flags go after the subcommand (not on pscale root).

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
	if org == "" {
		return "pscale database list --org <org> --format json"
	}
	return fmt.Sprintf("pscale database list --org %s --format json", org)
}

func AgentBranchListCmd(org, database string) string {
	if database == "" {
		database = "<database>"
	}
	if org == "" {
		return fmt.Sprintf("pscale branch list %s --org <org> --format json", database)
	}
	return fmt.Sprintf("pscale branch list %s --org %s --format json", database, org)
}

func AgentSQLCmd(org, database, branch string, force bool) string {
	if database == "" {
		database = "<database>"
	}
	if branch == "" {
		branch = "<branch>"
	}
	query := "SELECT 1"
	forceFlag := ""
	if force {
		forceFlag = " --force"
		query = "<query>"
	}
	if org == "" {
		return fmt.Sprintf("pscale sql %s %s --org <org> --format json%s --query \"%s\"", database, branch, forceFlag, query)
	}
	return fmt.Sprintf("pscale sql %s %s --org %s --format json%s --query \"%s\"", database, branch, org, forceFlag, query)
}
