package agentguide

import (
	"github.com/spf13/cobra"

	clicontent "github.com/planetscale/cli"
	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/printer"
)

const (
	HostedMCPURL = "https://mcp.pscale.dev/mcp/planetscale"
	MCPDocsURL   = "https://planetscale.com/docs/connect/mcp"
)

type response struct {
	Status            string   `json:"status"`
	Guide             string   `json:"guide"`
	FirstCommand      string   `json:"first_command"`
	AgentGuideCommand string   `json:"agent_guide_command"`
	HostedMCPURL      string   `json:"hosted_mcp_url"`
	MCPDocsURL        string   `json:"mcp_docs_url"`
	SupportedEngines  []string `json:"supported_engines"`
	Conventions       []string `json:"conventions"`
	NextSteps         []string `json:"next_steps"`
}

// AgentGuideCmd prints the embedded guide for agents and automation.
func AgentGuideCmd(ch *cmdutil.Helper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-guide",
		Short: "Show guidance for AI agents and automation",
		Long: `Show guidance for AI agents and automation using pscale.

Use --format json for a machine-readable bootstrap response with first commands,
supported engines, and hosted MCP details.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if ch.Printer.Format() == printer.JSON {
				return ch.Printer.PrintJSON(response{
					Status:            "ok",
					Guide:             clicontent.AgentGuide,
					FirstCommand:      cmdutil.AgentAuthCheckCmd(),
					AgentGuideCommand: cmdutil.AgentGuideCmd(),
					HostedMCPURL:      HostedMCPURL,
					MCPDocsURL:        MCPDocsURL,
					SupportedEngines:  []string{"mysql", "postgresql"},
					Conventions: []string{
						"Always pass --format json for automation",
						"Put --org on resource subcommands, not on pscale root",
						"Put positional arguments before flags for commands like sql and branch list",
						"Use hosted MCP for MCP clients; direct CLI automation should start with auth check",
					},
					NextSteps: []string{
						cmdutil.AgentAuthCheckCmd(),
					},
				})
			}

			ch.Printer.Print(clicontent.AgentGuide)
			return nil
		},
	}

	return cmd
}
