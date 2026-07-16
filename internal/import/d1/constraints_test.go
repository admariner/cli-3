package d1

import (
	"strings"
	"testing"
)

func TestParseTableLevelForeignKey(t *testing.T) {
	cols, refs := parseTableLevelForeignKey(`FOREIGN KEY (entity_id) REFERENCES external_entities(id)`)
	if len(cols) != 1 || cols[0] != "entity_id" {
		t.Fatalf("unexpected columns: %#v", cols)
	}
	refTable, refCol := parseReferencesTarget(refs)
	if refTable != "external_entities" || refCol != "id" {
		t.Fatalf("unexpected ref target: %s.%s", refTable, refCol)
	}
}

func TestColumnFKTargetUsesTableConstraint(t *testing.T) {
	table := TableSchema{
		Name: "entity_links",
		Columns: []ColumnSchema{
			{Name: "entity_id", Type: "TEXT", NotNull: true},
			{Name: "post_id", Type: "INTEGER", NotNull: true},
		},
		Constraints: []string{
			`PRIMARY KEY (entity_id, post_id)`,
			`FOREIGN KEY (entity_id) REFERENCES external_entities(id)`,
			`FOREIGN KEY (post_id) REFERENCES posts(id)`,
		},
	}
	col := table.Columns[0]

	refTable, refCol := columnFKTarget(col, table)
	if refTable != "external_entities" || refCol != "id" {
		t.Fatalf("got %s.%s", refTable, refCol)
	}
}

func TestConvertCheckConstraintRequotesMixedCaseColumn(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t ("MixedCase" INTEGER, CHECK (MixedCase > 0));`)
	if !strings.Contains(ddl, `CHECK ("MixedCase" > 0)`) {
		t.Fatalf("expected CHECK column reference re-quoted with declared case:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// TestConvertCheckConstraintFunctionCallNotQuotedAsColumn guards against a bare
// identifier that happens to share its name with a table column being quoted as a
// column reference when it's actually a function call (e.g. a column named "length"
// alongside a CHECK using the length(...) scalar function). Quoting the function name
// produces invalid Postgres DDL like "length"(name) instead of length(name). (length is
// used here rather than count because Postgres disallows aggregate functions like count()
// inside CHECK constraints.)
func TestConvertCheckConstraintFunctionCallNotQuotedAsColumn(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t (name TEXT, length INTEGER, CHECK (length(name) > 0));`)
	if strings.Contains(ddl, `"length"(`) {
		t.Fatalf("function call length(name) must not be quoted as a column reference:\n%s", ddl)
	}
	if !strings.Contains(ddl, `CHECK (length("name") > 0)`) {
		t.Fatalf(`expected length(...) function call preserved as-is (not quoted) in CHECK:%s`, "\n"+ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

func TestConvertCheckConstraintDoubleQuotedLiterals(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t (status TEXT, CHECK (status IN ("active", "inactive")));`)
	if !strings.Contains(ddl, `CHECK ("status" IN ('active', 'inactive'))`) {
		t.Fatalf("expected double-quoted CHECK literals converted to string literals:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

func TestConvertReferencesClauseCanonicalizesCase(t *testing.T) {
	sql := `CREATE TABLE Users (id INTEGER PRIMARY KEY);
CREATE TABLE Posts (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES USERS(ID));
`
	ddl := convertTablesDDL(t, sql)
	if !strings.Contains(ddl, `REFERENCES "Users" ("id")`) {
		t.Fatalf("expected REFERENCES canonicalized to declared table/column case:\n%s", ddl)
	}
	// Verify the canonicalized REFERENCES clause against real Postgres with tables created
	// in dependency order; table load ordering for case-mismatched FK references is a
	// separate, pre-existing concern not covered by this test.
	verifyDDL := `CREATE TABLE "Users" ("id" BIGINT PRIMARY KEY); CREATE TABLE "Posts" ("id" BIGINT PRIMARY KEY, "user_id" BIGINT REFERENCES "Users" ("id"));`
	assertValidPostgresDDL(t, verifyDDL)
}

func TestQuoteColumnListStripsIndexedColumnModifiers(t *testing.T) {
	got := quoteColumnList("a, b DESC")
	want := `"a", "b"`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	ddl := convertTablesDDL(t, `CREATE TABLE t (a INTEGER, b INTEGER, PRIMARY KEY (a, b DESC));`)
	if !strings.Contains(ddl, `PRIMARY KEY ("a", "b")`) {
		t.Fatalf("expected PRIMARY KEY column list without indexed-column modifiers:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

func TestConvertUniqueConstraintDropsOnConflictClause(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t (a INTEGER, b INTEGER, UNIQUE (a, b) ON CONFLICT REPLACE);`)
	if strings.Contains(strings.ToUpper(ddl), "ON CONFLICT") {
		t.Fatalf("ON CONFLICT clause must not appear in Postgres DDL:\n%s", ddl)
	}
	if !strings.Contains(ddl, `UNIQUE ("a", "b")`) {
		t.Fatalf("expected UNIQUE constraint converted:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

func TestConvertPrimaryKeyConstraintDropsOnConflictClause(t *testing.T) {
	got := convertPrimaryKeyConstraint("PRIMARY KEY (a, b) ON CONFLICT ROLLBACK", TableSchema{})
	want := `PRIMARY KEY ("a", "b")`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// SQLite resolves column names in table constraints case-insensitively, so the local
// column list of PRIMARY KEY / UNIQUE / FOREIGN KEY constraints must be canonicalized to
// the declared case, exactly like the referenced side of REFERENCES already is.
func TestConvertTableConstraintCanonicalizesLocalColumnCase(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t (id INTEGER, PRIMARY KEY (ID));`)
	if !strings.Contains(ddl, `PRIMARY KEY ("id")`) {
		t.Fatalf("expected PRIMARY KEY column canonicalized to declared case:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)

	ddl = convertTablesDDL(t, `CREATE TABLE t (a INTEGER, b INTEGER, UNIQUE (A, B));`)
	if !strings.Contains(ddl, `UNIQUE ("a", "b")`) {
		t.Fatalf("expected UNIQUE columns canonicalized to declared case:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)

	sql := `CREATE TABLE users (id INTEGER PRIMARY KEY);
CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER, FOREIGN KEY (USER_ID) REFERENCES users(id));
`
	ddl = convertTablesDDL(t, sql)
	if !strings.Contains(ddl, `FOREIGN KEY ("user_id") REFERENCES "users" ("id")`) {
		t.Fatalf("expected FOREIGN KEY local columns canonicalized to declared case:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// Named constraints (CONSTRAINT <name> <body>) must get the same conversion as unnamed
// ones instead of being passed through verbatim.
func TestConvertNamedConstraint(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t ("MixedCase" INTEGER, CONSTRAINT chk_mixed CHECK (MixedCase > 0));`)
	if !strings.Contains(ddl, `CONSTRAINT "chk_mixed" CHECK ("MixedCase" > 0)`) {
		t.Fatalf("expected named CHECK constraint converted:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)

	sql := `CREATE TABLE Users (id INTEGER PRIMARY KEY);
CREATE TABLE Posts (id INTEGER PRIMARY KEY, user_id INTEGER, CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES USERS(ID));
`
	ddl = convertTablesDDL(t, sql)
	if !strings.Contains(ddl, `CONSTRAINT "fk_user" FOREIGN KEY ("user_id") REFERENCES "Users" ("id")`) {
		t.Fatalf("expected named FOREIGN KEY constraint converted:\n%s", ddl)
	}

	ddl = convertTablesDDL(t, `CREATE TABLE t (a INTEGER, b INTEGER, CONSTRAINT uq_ab UNIQUE (a, b) ON CONFLICT REPLACE);`)
	if !strings.Contains(ddl, `CONSTRAINT "uq_ab" UNIQUE ("a", "b")`) {
		t.Fatalf("expected named UNIQUE constraint converted with ON CONFLICT dropped:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// SQL keywords inside a CHECK expression must never be quoted as column references, even
// when the table has a column with the same name.
func TestConvertCheckExprKeywordsNotQuotedAsColumns(t *testing.T) {
	table := TableSchema{Name: "t", Columns: []ColumnSchema{
		{Name: "a", Type: "INTEGER"},
		{Name: "b", Type: "INTEGER"},
		{Name: "and", Type: "INTEGER"},
		{Name: "end", Type: "INTEGER"},
	}}
	got := convertCheckExpr("a > 0 AND b < 5", table)
	want := `"a" > 0 AND "b" < 5`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// A keyword-named column can still be referenced when quoted in the source DDL.
	got = convertCheckExpr(`"end" > 0`, table)
	want = `"end" > 0`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// Bracket- and backtick-quoted identifiers are valid SQLite quoting that Postgres
// rejects; both must be converted to double-quoted identifiers with declared case.
func TestConvertCheckExprBracketAndBacktickIdentifiers(t *testing.T) {
	table := TableSchema{Name: "t", Columns: []ColumnSchema{{Name: "Age", Type: "INTEGER"}}}
	cases := map[string]string{
		"[Age] > 0":     `"Age" > 0`,
		"[age] > 0":     `"Age" > 0`,
		"`Age` > 0":     `"Age" > 0`,
		"`age` > 0":     `"Age" > 0`,
		"[unknown] > 0": `"unknown" > 0`,
	}
	for expr, want := range cases {
		if got := convertCheckExpr(expr, table); got != want {
			t.Fatalf("convertCheckExpr(%q) = %q, want %q", expr, got, want)
		}
	}
	ddl := convertTablesDDL(t, "CREATE TABLE t (\"Age\" INTEGER, CHECK ([Age] >= 0));")
	if !strings.Contains(ddl, `CHECK ("Age" >= 0)`) {
		t.Fatalf("expected bracket identifier converted in CHECK:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

func TestIsPartialIndexDDLNoWhitespaceBeforeWhere(t *testing.T) {
	if !isPartialIndexDDL(`CREATE UNIQUE INDEX idx ON t (a)WHERE deleted_at IS NULL;`) {
		t.Fatal("expected partial index to be detected without whitespace before WHERE")
	}
}

func TestConvertIndexDDLSkipsPartialIndexNoWhitespace(t *testing.T) {
	got := convertIndexDDL(`CREATE UNIQUE INDEX idx ON t (a)WHERE deleted_at IS NULL;`)
	if got != "" {
		t.Fatalf("convertIndexDDL = %q, want empty (partial index should be skipped, not silently made non-partial)", got)
	}
}
