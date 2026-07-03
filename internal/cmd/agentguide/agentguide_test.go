package agentguide

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/printer"
)

func TestAgentGuideJSONBootstrap(t *testing.T) {
	format := printer.JSON
	var out bytes.Buffer
	ch := &cmdutil.Helper{
		Printer: printer.NewPrinter(&format),
	}
	ch.Printer.SetResourceOutput(&out)

	cmd := AgentGuideCmd(ch)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var resp response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.FirstCommand != cmdutil.AgentAuthCheckCmd() {
		t.Fatalf("first command = %q", resp.FirstCommand)
	}
	if resp.HostedMCPURL != HostedMCPURL {
		t.Fatalf("hosted MCP URL = %q", resp.HostedMCPURL)
	}
	if resp.Guide == "" {
		t.Fatal("expected embedded guide")
	}
}
