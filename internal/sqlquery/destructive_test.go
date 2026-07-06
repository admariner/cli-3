package sqlquery

import (
	"errors"
	"testing"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"
)

func TestIsDestructiveQuery(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{query: "SELECT 1", want: false},
		{query: "INSERT INTO t VALUES (1)", want: false},
		{query: "UPDATE t SET x = 1", want: false},
		{query: "DELETE FROM users WHERE id = 1", want: true},
		{query: "drop table users", want: true},
		{query: "TRUNCATE TABLE users", want: true},
		{query: "ALTER TABLE users DROP COLUMN email", want: true},
		{query: "SELECT 1; DELETE FROM users", want: true},
		{query: "  delete from users", want: true},
		{query: "SELECT 1\nDELETE FROM users", want: true},
		{query: "WITH c AS (SELECT 1) DELETE FROM users", want: true},
		{query: "SELECT '; DELETE FROM users' AS sample", want: false},
		{query: "SELECT 'DROP TABLE users' AS sample", want: false},
		{query: "SELECT \"TRUNCATE\" FROM users", want: false},
		{query: "INSERT INTO logs VALUES ('created; still same statement')", want: false},
		{query: "SELECT 1 -- DELETE FROM users", want: false},
		{query: "SELECT 1 # DELETE FROM users", want: false},
		{query: "SELECT 1 /* DROP TABLE users */", want: false},
		{query: "SELECT 1 -- ; DELETE FROM users\nSELECT 2", want: false},
		{query: "SELECT 1 # ; DELETE FROM users\nSELECT 2", want: false},
		{query: "SELECT 1 /* ; TRUNCATE users */; SELECT 2", want: false},
		{query: "SELECT 1; /* comment */ DELETE FROM users", want: true},
		{query: "SELECT deleted_at FROM users", want: false},
		{query: "SELECT is_deleted FROM users", want: false},
		{query: "SELECT * FROM delete_queue", want: false},
		{query: "SELECT drop FROM t", want: false},
		{query: "SELECT 1 AS drop", want: false},
		{query: "SELECT truncate FROM t", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			if got := IsDestructiveQuery(tt.query); got != tt.want {
				t.Fatalf("IsDestructiveQuery(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestExecuteBlocksDestructiveWithoutForce(t *testing.T) {
	ch := &cmdutil.Helper{
		Config: &config.Config{Organization: "bb"},
	}

	_, err := Execute(t.Context(), ch, Options{
		Organization: "bb",
		Database:     "db",
		Branch:       "main",
		Query:        "DELETE FROM users",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := errors.AsType[*DestructiveQueryError](err); !ok {
		t.Fatalf("error = %T (%v), want *DestructiveQueryError", err, err)
	}
}
