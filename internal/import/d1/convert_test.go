package d1

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeDump writes sql to a temp file and returns its path, for tests that need to run the
// full ParseDump -> BuildTypeCoercionContext -> convert pipeline.
func writeDump(t *testing.T, sql string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "dump.sql")
	if err := os.WriteFile(path, []byte(sql), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func convertTablesDDL(t *testing.T, sql string) string {
	t.Helper()
	parts, _, err := ConvertSchemaParts(writeDump(t, sql))
	if err != nil {
		t.Fatalf("ConvertSchemaParts: %v", err)
	}
	return parts.Tables
}

// pgTestDBName is the scratch database used to verify generated DDL against a real
// Postgres instance when one is reachable (see assertValidPostgresDDL).
func pgTestDBName() string {
	if db := os.Getenv("D1_TEST_PG_DB"); db != "" {
		return db
	}
	return "d1_fix_test"
}

// postgresTestConn probes for a locally reachable Postgres, trying a direct connection
// first and falling back to `sudo -u postgres psql` (a common local dev setup), and returns
// the command + prefix args to reach it, or ok=false if neither works.
func postgresTestConn() (cmdName string, prefixArgs []string, ok bool) {
	psqlPath, err := exec.LookPath("psql")
	if err != nil {
		return "", nil, false
	}
	db := pgTestDBName()
	if runPsqlCheck(psqlPath, nil, db) {
		return psqlPath, nil, true
	}
	if _, err := exec.LookPath("sudo"); err == nil {
		prefix := []string{"-n", "-u", "postgres", psqlPath}
		if runPsqlCheck("sudo", prefix, db) {
			return "sudo", prefix, true
		}
	}
	return "", nil, false
}

func runPsqlCheck(cmdName string, prefixArgs []string, db string) bool {
	args := append(append([]string{}, prefixArgs...), "-d", db, "-c", "SELECT 1")
	return exec.Command(cmdName, args...).Run() == nil
}

// assertValidPostgresDDL executes ddl against a real local Postgres instance inside a
// transaction that is always rolled back, to catch the psql-level syntax/semantic failures
// unit tests based on string matching alone could miss. Skips gracefully when no local
// Postgres is reachable (e.g. in CI), so it never fails a build for environment reasons.
func assertValidPostgresDDL(t *testing.T, ddl string) {
	t.Helper()
	cmdName, prefixArgs, ok := postgresTestConn()
	if !ok {
		t.Skip("no local Postgres reachable for DDL verification")
	}
	args := append(append([]string{}, prefixArgs...), "-v", "ON_ERROR_STOP=1", "-d", pgTestDBName(), "-c", "BEGIN; "+ddl+" ROLLBACK;")
	out, err := exec.Command(cmdName, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("generated DDL failed against Postgres:\n%s\n--- psql output ---\n%s", ddl, out)
	}
}

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

// Bug 1: DEFAULT (time('now')) was not recognized and corrupted into invalid SQL.
func TestConvertDefaultTimeNow(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE events (id INTEGER PRIMARY KEY, event_time TEXT DEFAULT (time('now')));`)
	if strings.Contains(ddl, `'(time(`) {
		t.Fatalf("time('now') default was not mapped, still contains broken literal:\n%s", ddl)
	}
	if !strings.Contains(ddl, "to_char(now(), 'HH24:MI:SS')") {
		t.Fatalf("expected mapped time('now') default:\n%s", ddl)
	}
}

// Bug 2: fallback DEFAULT string-quoting did not escape embedded quotes, producing broken SQL.
func TestConvertDefaultEscapesEmbeddedQuotes(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE logs (id INTEGER PRIMARY KEY, ts TEXT DEFAULT (strftime('%Y-%m-%d','now')));`)
	if !strings.Contains(ddl, `'(strftime(''%Y-%m-%d'',''now''))'`) {
		t.Fatalf("expected properly escaped literal default:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// Bug 3: DEFAULT (random()) on an INTEGER/BIGINT column silently yielded near-constant 0/1.
func TestConvertDefaultRandomInteger(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE tokens (id INTEGER PRIMARY KEY, nonce INTEGER DEFAULT (random()));`)
	if !strings.Contains(ddl, "floor(random() * 9223372036854775807)") {
		t.Fatalf("expected genuine random bigint default:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// Bug 4: julianday(...)/randomblob(...) defaults referenced nonexistent Postgres functions.
func TestConvertDefaultJuliandayNow(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t (id INTEGER PRIMARY KEY, jd REAL DEFAULT (julianday('now')));`)
	if strings.Contains(strings.ToUpper(ddl), "JULIANDAY(") {
		t.Fatalf("julianday() must not appear in Postgres DDL:\n%s", ddl)
	}
	if !strings.Contains(ddl, "2440587.5") {
		t.Fatalf("expected julian day epoch conversion:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

func TestConvertDefaultRandomblob(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t (id INTEGER PRIMARY KEY, blob_val BLOB DEFAULT (randomblob(16)));`)
	if strings.Contains(strings.ToUpper(ddl), "RANDOMBLOB(") {
		t.Fatalf("randomblob() must not appear in Postgres DDL:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// Bug 6: UUID-inferred TEXT column with randomblob()/hex() DEFAULT was wrapped as an invalid literal.
func TestConvertDefaultUUIDGeneratorExpr(t *testing.T) {
	sql := `CREATE TABLE items (
  id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || lower(hex(randomblob(2))) || '-' || lower(hex(randomblob(2))) || '-' || lower(hex(randomblob(6))))
);
INSERT INTO items (id) VALUES ('11111111-1111-4111-8111-111111111111');
`
	ddl := convertTablesDDL(t, sql)
	if !strings.Contains(ddl, "UUID") {
		t.Fatalf("expected id to be inferred as UUID:\n%s", ddl)
	}
	if !strings.Contains(ddl, "gen_random_uuid()") {
		t.Fatalf("expected gen_random_uuid() default:\n%s", ddl)
	}
	if strings.Contains(strings.ToUpper(ddl), "RANDOMBLOB") {
		t.Fatalf("randomblob() must not leak into Postgres DDL:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// Bug 5: the boolean-like INTEGER heuristic ignored foreign keys, so an FK column could
// become BOOLEAN while its referenced primary key stayed BIGINT.
func TestBooleanHeuristicIgnoresForeignKeys(t *testing.T) {
	sql := `CREATE TABLE roles (id INTEGER PRIMARY KEY);
CREATE TABLE users (id INTEGER PRIMARY KEY, role_id INTEGER REFERENCES roles(id));
INSERT INTO roles (id) VALUES (0), (1), (2);
INSERT INTO users (id, role_id) VALUES (1, 0), (2, 1);
`
	ddl := convertTablesDDL(t, sql)
	if !strings.Contains(ddl, `"role_id" BIGINT REFERENCES "roles" ("id")`) {
		t.Fatalf("expected FK column to stay BIGINT despite 0/1-only sampled values:\n%s", ddl)
	}
	if strings.Contains(ddl, `"role_id" BOOLEAN`) {
		t.Fatalf("FK column must not be coerced to BOOLEAN:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// Bug 7: BOOLEAN-mapped column DEFAULT only special-cased literal "0"/"1".
func TestConvertDefaultBooleanNonZeroOneLiteral(t *testing.T) {
	sql := `CREATE TABLE flags (
  id INTEGER PRIMARY KEY,
  rollout_state INTEGER DEFAULT 2
);
INSERT INTO flags (id, rollout_state) VALUES (1, 0);
INSERT INTO flags (id, rollout_state) VALUES (2, 1);
`
	ddl := convertTablesDDL(t, sql)
	if !strings.Contains(ddl, `"rollout_state" BOOLEAN DEFAULT TRUE`) {
		t.Fatalf("expected boolean column with TRUE default for non-zero literal:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// Bug 8: SQLite's double-quoted string DEFAULT literal fallback was emitted as a Postgres
// identifier, which always fails since Postgres never treats double quotes as literals.
func TestConvertDefaultDoubleQuotedLiteral(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t (id INTEGER PRIMARY KEY, status TEXT DEFAULT "active");`)
	if !strings.Contains(ddl, `DEFAULT 'active'`) {
		t.Fatalf("expected double-quoted default converted to a string literal:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// Bug 15: unmapped quote-free function DEFAULTs like CAST(...) became literal strings
// instead of the computed value they were meant to represent.
func TestConvertDefaultCastUnixepoch(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t (id INTEGER PRIMARY KEY, cst TEXT DEFAULT (CAST(unixepoch() AS TEXT)));`)
	if strings.Contains(ddl, "'CAST(") || strings.Contains(ddl, "CAST(unixepoch") {
		t.Fatalf("CAST(unixepoch() AS TEXT) default must not become a literal string:\n%s", ddl)
	}
	if !strings.Contains(ddl, "extract(epoch from now())::bigint)::text") {
		t.Fatalf("expected epoch-as-text default:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// Bug 16: NUMERIC/DECIMAL-affinity columns were mapped to TEXT, losing numeric semantics.
func TestConvertNumericDecimalTypes(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t (id INTEGER PRIMARY KEY, amount DECIMAL(10,2) NOT NULL, tax NUMERIC NOT NULL);`)
	if !strings.Contains(ddl, `"amount" NUMERIC(10,2) NOT NULL`) {
		t.Fatalf("expected DECIMAL(10,2) to map to NUMERIC(10,2):\n%s", ddl)
	}
	if !strings.Contains(ddl, `"tax" NUMERIC NOT NULL`) {
		t.Fatalf("expected bare NUMERIC to map to NUMERIC:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// Bug 14: GENERATED ALWAYS AS (...) STORED computed columns were silently stripped of their
// generation expression, becoming ordinary writable columns.
func TestConvertGeneratedStoredColumn(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE line_items (id INTEGER PRIMARY KEY, price REAL NOT NULL, qty REAL NOT NULL, total REAL GENERATED ALWAYS AS (price * qty) STORED);`)
	if !strings.Contains(ddl, `GENERATED ALWAYS AS ("price" * "qty") STORED`) {
		t.Fatalf("expected generated column expression preserved:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

func TestConvertGeneratedVirtualColumnFallsBackToStored(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE line_items (id INTEGER PRIMARY KEY, price REAL NOT NULL, qty REAL NOT NULL, total REAL GENERATED ALWAYS AS (price * qty) VIRTUAL);`)
	if !strings.Contains(ddl, `GENERATED ALWAYS AS ("price" * "qty") STORED`) {
		t.Fatalf("expected VIRTUAL generated column to fall back to STORED (only mode Postgres 16 supports):\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

func TestConvertGeneratedColumnHasNoDefault(t *testing.T) {
	col := ColumnSchema{Name: "total", Type: "REAL", GeneratedExpr: "price * qty", GeneratedMode: "STORED", DefaultValue: "0"}
	table := TableSchema{Name: "line_items", Columns: []ColumnSchema{
		{Name: "price", Type: "REAL"},
		{Name: "qty", Type: "REAL"},
		col,
	}}
	got := convertColumn(col, table, []TableSchema{table}, nil)
	if strings.Contains(got, "DEFAULT") {
		t.Fatalf("generated column must not also emit DEFAULT: %q", got)
	}
}
