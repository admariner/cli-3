package mcp

import (
	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

// McpCmd returns a new cobra.Command for the mcp command.
func McpCmd(ch *cmdutil.Helper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp <command>",
		Short: "Manage PlanetScale MCP configuration",
		Long: `Manage PlanetScale model context protocol (MCP) configuration.

PlanetScale recommends the hosted MCP server:

  https://mcp.pscale.dev/mcp/planetscale

See https://planetscale.com/docs/connect/mcp for current setup instructions.`,
	}

	cmd.AddCommand(InstallCmd(ch))
	cmd.AddCommand(ServerCmd(ch))

	return cmd
}
