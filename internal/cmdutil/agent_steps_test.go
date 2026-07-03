package cmdutil

import (
	"errors"
	"testing"

	"github.com/planetscale/cli/internal/config"
	"github.com/spf13/cobra"
)

func TestAgentStepsFlagOrder(t *testing.T) {
	tests := []struct {
		name string
		got  string
	}{
		{name: "database list", got: AgentDatabaseListCmd("bb")},
		{name: "branch list", got: AgentBranchListCmd("bb", "mydb")},
		{name: "sql", got: AgentSQLCmd("bb", "mydb", "main", false)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if containsBeforeSubcommand(tt.got, "--org") {
				t.Fatalf("bad flag order: %q", tt.got)
			}
		})
	}
}

func containsBeforeSubcommand(cmd, flag string) bool {
	// Reject "pscale --org" (root-level org); want "pscale <subcommand> ... --org".
	const bad = "pscale --org"
	return len(cmd) >= len(bad) && cmd[:len(bad)] == bad
}

func TestCheckAuthenticationJSONHandledError(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("format", "json", "")

	err := CheckAuthentication(&config.Config{})(cmd, nil)
	if err == nil {
		t.Fatal("expected auth error")
	}
	var cmdErr *Error
	if !errors.As(err, &cmdErr) || !cmdErr.Handled {
		t.Fatalf("expected handled JSON error, got %v", err)
	}
}
