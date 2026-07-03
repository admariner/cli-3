package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/printer"
)

func TestModifyCursorConfigUsesHostedMCP(t *testing.T) {
	settings := map[string]any{}

	if err := modifyCursorConfig(settings); err != nil {
		t.Fatalf("modify cursor config: %v", err)
	}

	servers, ok := settings["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers = %#v", settings["mcpServers"])
	}
	planetscale, ok := servers["planetscale"].(map[string]any)
	if !ok {
		t.Fatalf("planetscale server = %#v", servers["planetscale"])
	}
	if got := planetscale["url"]; got != hostedMCPURL {
		t.Fatalf("url = %v, want %s", got, hostedMCPURL)
	}
	if _, ok := planetscale["command"]; ok {
		t.Fatalf("hosted config should not use local command: %#v", planetscale)
	}
}

func TestInstallMCPServerReturnsBackupPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(configPath, []byte(`{"mcpServers":{}}`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	backupPath, err := installMCPServer(configPath, "cursor", modifyCursorConfig)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if backupPath == "" {
		t.Fatal("expected backup path")
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup stat: %v", err)
	}
}

func TestInstallCmdClaudeCodeJSON(t *testing.T) {
	format := printer.JSON
	var out bytes.Buffer
	ch := &cmdutil.Helper{
		Printer: printer.NewPrinter(&format),
	}
	ch.Printer.SetResourceOutput(&out)

	cmd := InstallCmd(ch)
	cmd.SetArgs([]string{"--target", "claude-code"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var resp installResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp.Status != "action_required" {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Command != claudeCodeCmd {
		t.Fatalf("command = %q", resp.Command)
	}
	if resp.HostedMCPURL != hostedMCPURL {
		t.Fatalf("hosted MCP URL = %q", resp.HostedMCPURL)
	}
}
