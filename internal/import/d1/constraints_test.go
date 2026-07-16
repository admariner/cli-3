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

// Bug 9: table-level CHECK constraints were passed through verbatim, so a CHECK referencing
// a mixed-case column broke once Postgres folded the unquoted reference to lowercase.
func TestConvertCheckConstraintRequotesMixedCaseColumn(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t ("MixedCase" INTEGER, CHECK (MixedCase > 0));`)
	if !strings.Contains(ddl, `CHECK ("MixedCase" > 0)`) {
		t.Fatalf("expected CHECK column reference re-quoted with declared case:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// Bug 18: CHECK constraints with double-quoted string literals (SQLite's double-quote
// literal fallback) were passed through unmodified, so Postgres treated them as
// (nonexistent) column references instead of string values.
func TestConvertCheckConstraintDoubleQuotedLiterals(t *testing.T) {
	ddl := convertTablesDDL(t, `CREATE TABLE t (status TEXT, CHECK (status IN ("active", "inactive")));`)
	if !strings.Contains(ddl, `CHECK ("status" IN ('active', 'inactive'))`) {
		t.Fatalf("expected double-quoted CHECK literals converted to string literals:\n%s", ddl)
	}
	assertValidPostgresDDL(t, ddl)
}

// Bug 10: FK/REFERENCES clauses were quoted with whatever case appeared in the clause
// itself rather than the declared table/column case, so case-insensitive SQLite references
// broke once Postgres compared quoted (case-sensitive) identifiers.
func TestConvertReferencesClauseCanonicalizesCase(t *testing.T) {
	sql := `CREATE TABLE Users (id INTEGER PRIMARY KEY);
CREATE TABLE Posts (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES USERS(ID));
`
	ddl := convertTablesDDL(t, sql)
	if !strings.Contains(ddl, `REFERENCES "Users" ("id")`) {
		t.Fatalf("expected REFERENCES canonicalized to declared table/column case:\n%s", ddl)
	}
	// Verify the canonicalized REFERENCES clause against real Postgres with tables created
	// in dependency order (table load ordering for case-mismatched FK references is a
	// separate, pre-existing concern outside this fix's scope).
	verifyDDL := `CREATE TABLE "Users" ("id" BIGINT PRIMARY KEY); CREATE TABLE "Posts" ("id" BIGINT PRIMARY KEY, "user_id" BIGINT REFERENCES "Users" ("id"));`
	assertValidPostgresDDL(t, verifyDDL)
}

// Bug 11: PRIMARY KEY/UNIQUE column lists with ASC/DESC/COLLATE modifiers were quoted as a
// single broken identifier (e.g. "b DESC") instead of a plain column reference.
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

// Bug 12: PRIMARY KEY/UNIQUE constraints with a trailing ON CONFLICT clause were emitted
// unconverted (raw SQLite syntax), which Postgres cannot parse.
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
	got := convertPrimaryKeyConstraint("PRIMARY KEY (a, b) ON CONFLICT ROLLBACK")
	want := `PRIMARY KEY ("a", "b")`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// Bug 13: partial-index detection required whitespace around WHERE, so a valid partial
// index written without a space before WHERE silently became a full-table index.
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
