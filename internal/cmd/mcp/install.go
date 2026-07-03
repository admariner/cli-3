package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/printer"
	"github.com/spf13/cobra"
	"github.com/tidwall/jsonc"
)

const (
	hostedMCPURL  = "https://mcp.pscale.dev/mcp/planetscale"
	mcpDocsURL    = "https://planetscale.com/docs/connect/mcp"
	claudeCodeCmd = `claude mcp add --transport http "planetscale" https://mcp.pscale.dev/mcp/planetscale`
)

type installResponse struct {
	Status       string   `json:"status"`
	Target       string   `json:"target,omitempty"`
	ConfigPath   string   `json:"config_path,omitempty"`
	BackupPath   string   `json:"backup_path,omitempty"`
	HostedMCPURL string   `json:"hosted_mcp_url"`
	DocsURL      string   `json:"docs_url"`
	Command      string   `json:"command,omitempty"`
	Message      string   `json:"message"`
	NextSteps    []string `json:"next_steps,omitempty"`
}

// InstallCmd returns a new cobra.Command for the mcp install command.
func InstallCmd(ch *cmdutil.Helper) *cobra.Command {
	var target string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the hosted MCP server",
		Long: `Install the hosted PlanetScale model context protocol (MCP) server.

For clients this command cannot safely configure, it prints the current hosted
MCP setup instructions instead of installing the deprecated local stdio server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var configPath string
			var err error

			switch target {
			case "cursor":
				configPath, err = getCursorConfigPath()
				if err != nil {
					return fmt.Errorf("failed to determine Cursor config path: %w", err)
				}
				backupPath, err := installMCPServer(configPath, target, modifyCursorConfig)
				if err != nil {
					return err
				}
				return printInstallResponse(ch, installResponse{
					Status:       "ok",
					Target:       target,
					ConfigPath:   configPath,
					BackupPath:   backupPath,
					HostedMCPURL: hostedMCPURL,
					DocsURL:      mcpDocsURL,
					Message:      fmt.Sprintf("Hosted MCP server configured for %s", target),
				})
			case "claude", "claude-code":
				return printInstallResponse(ch, installResponse{
					Status:       "action_required",
					Target:       "claude-code",
					HostedMCPURL: hostedMCPURL,
					DocsURL:      mcpDocsURL,
					Command:      claudeCodeCmd,
					Message:      "Claude Code uses the hosted MCP server. Run the command in next_steps to install it.",
					NextSteps: []string{
						claudeCodeCmd,
					},
				})
			case "zed":
				return printInstallResponse(ch, installResponse{
					Status:       "action_required",
					Target:       target,
					HostedMCPURL: hostedMCPURL,
					DocsURL:      mcpDocsURL,
					Message:      "Zed hosted MCP setup is documented in the current PlanetScale MCP docs.",
					NextSteps: []string{
						mcpDocsURL,
					},
				})
			default:
				return fmt.Errorf("invalid target vendor: %s (supported values: cursor, claude-code, zed)", target)
			}
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Target vendor for MCP installation (required). Possible values: [cursor, claude-code, zed]")
	cmd.MarkFlagRequired("target")
	cmd.RegisterFlagCompletionFunc("target", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"cursor", "claude-code", "zed"}, cobra.ShellCompDirectiveDefault
	})

	return cmd
}

func printInstallResponse(ch *cmdutil.Helper, resp installResponse) error {
	if ch.Printer.Format() == printer.JSON {
		return ch.Printer.PrintJSON(resp)
	}

	switch resp.Status {
	case "ok":
		fmt.Printf("%s\n", resp.Message)
		if resp.ConfigPath != "" {
			fmt.Printf("Config path: %s\n", resp.ConfigPath)
		}
		if resp.BackupPath != "" {
			fmt.Printf("Created backup at %s\n", resp.BackupPath)
		}
	default:
		fmt.Printf("%s\n", resp.Message)
		if resp.Command != "" {
			fmt.Printf("\n  %s\n", resp.Command)
		}
		fmt.Printf("\nSee %s\n", resp.DocsURL)
	}
	return nil
}

// installMCPServer handles common file I/O for all editors
func installMCPServer(configPath string, target string, modifyConfig func(map[string]any) error) (string, error) {
	// Check if config directory exists
	configDir := filepath.Dir(configPath)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return "", fmt.Errorf("no %s installation: path %s not found", target, configDir)
	}

	// Read existing config or create empty settings
	var fullSettings map[string]any
	if fileData, err := os.ReadFile(configPath); err == nil {
		cleanJSON := jsonc.ToJSON(fileData)
		if err := json.Unmarshal(cleanJSON, &fullSettings); err != nil {
			return "", fmt.Errorf("failed to parse %s config file: %w", target, err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to read %s config file: %w", target, err)
	} else {
		fullSettings = make(map[string]any)
	}

	// Let the editor-specific function modify the config
	if err := modifyConfig(fullSettings); err != nil {
		return "", err
	}

	// Marshal updated config
	updatedData, err := json.MarshalIndent(fullSettings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal %s config: %w", target, err)
	}

	// Backup existing file before writing
	var backupPath string
	if _, err := os.Stat(configPath); err == nil {
		backupPath = configPath + "~"
		if err := os.Rename(configPath, backupPath); err != nil {
			return "", fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Write updated config
	if err := os.WriteFile(configPath, updatedData, 0644); err != nil {
		return "", fmt.Errorf("failed to write %s config file: %w", target, err)
	}

	return backupPath, nil
}

// modifyCursorConfig adds the hosted PlanetScale MCP server to Cursor.
func modifyCursorConfig(settings map[string]any) error {
	var mcpServers map[string]any
	if existingServers, ok := settings["mcpServers"].(map[string]any); ok {
		mcpServers = existingServers
	} else {
		mcpServers = make(map[string]any)
	}

	mcpServers["planetscale"] = map[string]any{
		"url": hostedMCPURL,
	}

	settings["mcpServers"] = mcpServers
	return nil
}

// getCursorConfigPath returns the path to the Cursor MCP config file
func getCursorConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine user home directory: %w", err)
	}

	// Cursor uses ~/.cursor/mcp.json for its MCP configuration
	return filepath.Join(homeDir, ".cursor", "mcp.json"), nil
}
