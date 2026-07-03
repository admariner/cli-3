package sqlquery

import (
	"context"
	"testing"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"
)

func TestIsReadQuery(t *testing.T) {
	if !isReadQuery("SELECT 1") {
		t.Fatal("SELECT should be read")
	}
	if !isReadQuery("  with x as (select 1) select * from x") {
		t.Fatal("WITH should be read")
	}
	if isReadQuery("INSERT INTO t VALUES (1)") {
		t.Fatal("INSERT should not be read")
	}
	if isReadQuery("UPDATE t SET x = 1") {
		t.Fatal("UPDATE should not be read")
	}
}

func TestMySQLDSNDatabase(t *testing.T) {
	if got := mysqlDSNDatabase(Options{}); got != "@primary" {
		t.Fatalf("default = %q, want @primary", got)
	}
	if got := mysqlDSNDatabase(Options{Replica: true}); got != "" {
		t.Fatalf("replica = %q, want empty", got)
	}
	if got := mysqlDSNDatabase(Options{Keyspace: "commerce"}); got != "commerce" {
		t.Fatalf("explicit = %q", got)
	}
}

func TestExecuteValidation(t *testing.T) {
	ch := &cmdutil.Helper{
		Config: &config.Config{Organization: "bb"},
	}

	tests := []struct {
		name    string
		opts    Options
		wantErr string
	}{
		{
			name:    "missing query",
			opts:    Options{Organization: "bb", Database: "db", Branch: "main"},
			wantErr: "query is required",
		},
		{
			name:    "missing org",
			opts:    Options{Query: "SELECT 1", Database: "db", Branch: "main"},
			wantErr: "organization is required (use --org or set org in pscale.yml)",
		},
		{
			name:    "missing database",
			opts:    Options{Organization: "bb", Query: "SELECT 1", Branch: "main"},
			wantErr: "database and branch are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Execute(context.Background(), ch, tt.opts)
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}
