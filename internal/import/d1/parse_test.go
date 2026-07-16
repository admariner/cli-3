package d1

import (
	"strings"
	"testing"
)

func TestParseColumnDefaultBeforeNotNull(t *testing.T) {
	col := parseColumn("active INTEGER DEFAULT 1 NOT NULL")
	if col.DefaultValue != "1" {
		t.Fatalf("default = %q, want 1", col.DefaultValue)
	}
	if !col.NotNull {
		t.Fatal("expected NOT NULL")
	}
}

func TestParseColumnDefaultStringNotNullNotConstraint(t *testing.T) {
	col := parseColumn("status TEXT DEFAULT 'value NOT NULL'")
	if col.DefaultValue != "'value NOT NULL'" {
		t.Fatalf("default = %q, want quoted literal", col.DefaultValue)
	}
	if col.NotNull {
		t.Fatal("NOT NULL inside default string must not set column constraint")
	}
}

func TestParseColumnCheckNotNullNotConstraint(t *testing.T) {
	col := parseColumn("status TEXT CHECK (status IS NOT NULL)")
	if col.NotNull {
		t.Fatal("NOT NULL inside CHECK must not set column constraint")
	}
}

func TestTrimDefaultClause(t *testing.T) {
	cases := map[string]string{
		"1 NOT NULL":              "1",
		"'draft' NOT NULL UNIQUE": "'draft'",
		"CURRENT_TIMESTAMP":       "CURRENT_TIMESTAMP",
	}
	for in, want := range cases {
		if got := trimDefaultClause(in); got != want {
			t.Fatalf("trimDefaultClause(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseColumnUniqueConstraint(t *testing.T) {
	col := parseColumn("email TEXT NOT NULL UNIQUE")
	if !col.Unique {
		t.Fatal("expected column-level UNIQUE constraint")
	}

	col = parseColumn("unique_token TEXT NOT NULL")
	if col.Unique {
		t.Fatalf("identifier unique_token should not be treated as UNIQUE constraint")
	}

	col = parseColumn("unique_id INTEGER PRIMARY KEY")
	if col.Unique {
		t.Fatalf("identifier unique_id should not be treated as UNIQUE constraint")
	}
}

// Bug 17: column-level CHECK constraints were silently discarded during parsing (stripped
// purely to avoid keyword confusion, never captured anywhere).
func TestParseColumnCapturesCheckExpression(t *testing.T) {
	col := parseColumn("age INTEGER CHECK (age >= 0)")
	if len(col.CheckExprs) != 1 || col.CheckExprs[0] != "age >= 0" {
		t.Fatalf("CheckExprs = %#v, want [\"age >= 0\"]", col.CheckExprs)
	}
	if col.NotNull {
		t.Fatal("CHECK clause must not set NOT NULL")
	}
}

func TestParseColumnCheckThenNotNull(t *testing.T) {
	col := parseColumn("age INTEGER CHECK (age >= 0) NOT NULL")
	if len(col.CheckExprs) != 1 || col.CheckExprs[0] != "age >= 0" {
		t.Fatalf("CheckExprs = %#v", col.CheckExprs)
	}
	if !col.NotNull {
		t.Fatal("expected NOT NULL after CHECK clause to still be detected")
	}
}

// Bug 14: GENERATED ALWAYS AS (...) STORED computed columns had no parsing support at all.
func TestParseColumnGeneratedStored(t *testing.T) {
	col := parseColumn("total REAL GENERATED ALWAYS AS (price * qty) STORED")
	if col.Type != "REAL" {
		t.Fatalf("Type = %q, want REAL", col.Type)
	}
	if col.GeneratedExpr != "price * qty" {
		t.Fatalf("GeneratedExpr = %q, want %q", col.GeneratedExpr, "price * qty")
	}
	if col.GeneratedMode != "STORED" {
		t.Fatalf("GeneratedMode = %q, want STORED", col.GeneratedMode)
	}
	if col.DefaultValue != "" {
		t.Fatalf("generated column must not also parse a DEFAULT, got %q", col.DefaultValue)
	}
}

func TestParseColumnGeneratedVirtualDefaultMode(t *testing.T) {
	col := parseColumn("total REAL AS (price * qty)")
	if col.GeneratedExpr != "price * qty" {
		t.Fatalf("GeneratedExpr = %q, want %q", col.GeneratedExpr, "price * qty")
	}
	if col.GeneratedMode != "VIRTUAL" {
		t.Fatalf("GeneratedMode = %q, want VIRTUAL (SQLite's default when omitted)", col.GeneratedMode)
	}
}

func TestParseColumnGeneratedThenNotNull(t *testing.T) {
	col := parseColumn("total REAL GENERATED ALWAYS AS (price * qty) STORED NOT NULL")
	if col.GeneratedExpr != "price * qty" {
		t.Fatalf("GeneratedExpr = %q", col.GeneratedExpr)
	}
	if !col.NotNull {
		t.Fatal("expected NOT NULL after generated clause to still be detected")
	}
}

// Bug 16 (parsing half): NUMERIC/DECIMAL precision and scale must survive column parsing so
// the Postgres type mapping can preserve it (DECIMAL(10,2) -> NUMERIC(10,2)).
func TestParseColumnPreservesDecimalPrecision(t *testing.T) {
	col := parseColumn("amount DECIMAL(10,2) NOT NULL")
	if col.Type != "DECIMAL(10,2)" {
		t.Fatalf("Type = %q, want DECIMAL(10,2)", col.Type)
	}
	if !col.NotNull {
		t.Fatal("expected NOT NULL detected after DECIMAL(10,2)")
	}
}

func TestExtractGeneratedClauseNoMatch(t *testing.T) {
	cleaned, expr, mode := extractGeneratedClause("NOT NULL UNIQUE")
	if expr != "" || mode != "" || cleaned != "NOT NULL UNIQUE" {
		t.Fatalf("got cleaned=%q expr=%q mode=%q", cleaned, expr, mode)
	}
}

func TestExtractCheckClausesMultiple(t *testing.T) {
	cleaned, checks := extractCheckClauses("CHECK (a > 0) NOT NULL")
	if len(checks) != 1 || checks[0] != "a > 0" {
		t.Fatalf("checks = %#v", checks)
	}
	if strings.Contains(cleaned, "CHECK") {
		t.Fatalf("cleaned still contains CHECK: %q", cleaned)
	}
}
