package d1

import "testing"

func TestLooksLikeTimestampColumnName(t *testing.T) {
	cases := map[string]bool{
		"created_at":    true,
		"updated_at":    true,
		"event_date":    true,
		"date_of_birth": true,
		"date":          true,
		"timestamp_raw": true,
		"candidate":     false,
		"mandate":       false,
		"metadata":      false,
	}
	for name, want := range cases {
		if got := looksLikeTimestampColumnName(name); got != want {
			t.Fatalf("looksLikeTimestampColumnName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsTimestampTextIgnoresFalsePositiveNames(t *testing.T) {
	for _, name := range []string{"candidate", "mandate"} {
		col := ColumnSchema{Name: name, Type: "TEXT"}
		if isTimestampText(col) {
			t.Fatalf("isTimestampText(%q) = true, want false", name)
		}
	}
}

func TestMapSQLiteDefaultFunctionUnixEpoch(t *testing.T) {
	cases := map[string]struct {
		def    string
		pgType string
		want   string
	}{
		"unixepoch('now') timestamptz":    {"unixepoch('now')", "TIMESTAMPTZ", "now()"},
		"UNIXEPOCH('now') timestamptz":    {"UNIXEPOCH('now')", "TIMESTAMPTZ", "now()"},
		"UnixEpoch('now') timestamptz":    {"UnixEpoch('now')", "TIMESTAMPTZ", "now()"},
		"(UNIXEPOCH('now')) timestamptz":  {"(UNIXEPOCH('now'))", "TIMESTAMPTZ", "now()"},
		"UNIXEPOCH('subsec') timestamptz": {"UNIXEPOCH('subsec')", "TIMESTAMPTZ", "clock_timestamp()"},
		"unixepoch() timestamptz":         {"unixepoch()", "TIMESTAMPTZ", "now()"},
		"unixepoch('now') bigint":         {"unixepoch('now')", "BIGINT", "extract(epoch from now())::bigint"},
		"unixepoch numeric timestamptz":   {"UNIXEPOCH(1700000000)", "TIMESTAMPTZ", "to_timestamp(1700000000)"},
		"CURRENT_TIMESTAMP":               {"CURRENT_TIMESTAMP", "TIMESTAMPTZ", "CURRENT_TIMESTAMP"},
		"datetime('now')":                 {"datetime('now')", "TIMESTAMPTZ", "now()"},
		"date('now') timestamptz":         {"date('now')", "TIMESTAMPTZ", "CURRENT_DATE"},
		"(date('now')) timestamptz":       {"(date('now'))", "TIMESTAMPTZ", "CURRENT_DATE"},
		"date('now') text":                {"date('now')", "TEXT", "CURRENT_DATE"},
	}
	for name, tc := range cases {
		got := mapSQLiteDefaultFunction(tc.def, tc.pgType)
		if got != tc.want {
			t.Fatalf("%s: mapSQLiteDefaultFunction(%q, %q) = %q, want %q", name, tc.def, tc.pgType, got, tc.want)
		}
	}
}

// TestConvertDefaultDateExpressionNotQuotedAsString guards against regressing
// to treating a SQLite expression default such as (date('now')) as a plain
// string literal. Quoting the whole expression produces invalid Postgres DDL
// (a syntax/type error at import time) instead of a valid expression default.
func TestConvertDefaultDateExpressionNotQuotedAsString(t *testing.T) {
	cases := map[string]struct {
		def    string
		pgType string
		want   string
	}{
		"parenthesized date('now') on TIMESTAMPTZ": {"(date('now'))", "TIMESTAMPTZ", "CURRENT_DATE"},
		"bare date('now') on TIMESTAMPTZ":          {"date('now')", "TIMESTAMPTZ", "CURRENT_DATE"},
		"date('now') on TEXT":                      {"date('now')", "TEXT", "CURRENT_DATE"},
	}
	for name, tc := range cases {
		got := convertDefault(tc.def, tc.pgType)
		if got != tc.want {
			t.Fatalf("%s: convertDefault(%q, %q) = %q, want %q", name, tc.def, tc.pgType, got, tc.want)
		}
		if len(got) > 0 && got[0] == '\'' {
			t.Fatalf("%s: convertDefault(%q, %q) = %q, must not be a quoted string literal", name, tc.def, tc.pgType, got)
		}
	}
}

// TestConvertDefaultStringLiteralsStillQuoted ensures genuine SQLite string
// defaults are still emitted as Postgres string literals; the date()
// expression fix must not change this behavior.
func TestConvertDefaultStringLiteralsStillQuoted(t *testing.T) {
	cases := map[string]struct {
		def    string
		pgType string
		want   string
	}{
		"already-quoted string default": {"'foo'", "TEXT", "'foo'"},
		"bare text default gets quoted": {"foo", "TEXT", "'foo'"},
	}
	for name, tc := range cases {
		got := convertDefault(tc.def, tc.pgType)
		if got != tc.want {
			t.Fatalf("%s: convertDefault(%q, %q) = %q, want %q", name, tc.def, tc.pgType, got, tc.want)
		}
	}
}
