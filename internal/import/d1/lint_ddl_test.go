package d1

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLintDumpDDL(t *testing.T) {
	path := filepath.Join("testdata", "ddl_lint_export.sql")
	result, err := Lint(path)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}

	wantCodes := map[string]bool{
		"VIEW_NOT_MIGRATED":    false,
		"TRIGGER_NOT_MIGRATED": false,
		"PARTIAL_INDEX":        false,
		"EXPRESSION_INDEX":     false,
	}
	for _, issue := range result.Issues {
		if _, ok := wantCodes[issue.Code]; ok {
			wantCodes[issue.Code] = true
			if issue.Severity != SeverityWarning {
				t.Fatalf("%s severity = %q, want warning", issue.Code, issue.Severity)
			}
		}
	}
	for code, found := range wantCodes {
		if !found {
			t.Fatalf("expected lint issue %s", code)
		}
	}
}

func TestLintDDLStatementPartialIndex(t *testing.T) {
	issues := lintDDLStatement(`CREATE INDEX idx ON items(email) WHERE deleted_at IS NULL;`)
	if len(issues) != 1 || issues[0].Code != "PARTIAL_INDEX" {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestLintDDLStatementExpressionIndexUnparsed(t *testing.T) {
	issues := lintDDLStatement(`CREATE INDEX idx ON items(lower(email));`)
	if len(issues) != 1 || issues[0].Code != "EXPRESSION_INDEX" {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestLintDDLStatementTemporaryView(t *testing.T) {
	issues := lintDDLStatement(`CREATE TEMPORARY VIEW active_items AS SELECT 1;`)
	if len(issues) != 1 || issues[0].Code != "VIEW_NOT_MIGRATED" {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestConvertIndexDDLSkipsPartialIndex(t *testing.T) {
	got := convertIndexDDL(`CREATE INDEX idx ON items(email) WHERE deleted_at IS NULL;`)
	if got != "" {
		t.Fatalf("convertIndexDDL = %q, want empty", got)
	}
}

func TestIndexColumnsLookExpressionToleratesSortModifiers(t *testing.T) {
	cases := map[string]bool{
		"email":                    false,
		"email DESC":               false,
		"email ASC":                false,
		`"MixedCase" DESC`:         false,
		"email COLLATE NOCASE ASC": false,
		"a, b DESC":                false,
		"lower(email)":             true,
		"email, lower(name)":       true,
	}
	for cols, want := range cases {
		if got := indexColumnsLookExpression(cols); got != want {
			t.Fatalf("indexColumnsLookExpression(%q) = %v, want %v", cols, got, want)
		}
	}
}

func TestIndexColumnsLookExpressionDetectsUnparenthesizedOperators(t *testing.T) {
	cases := map[string]bool{
		"email || domain":                true,
		"email||domain":                  true,
		"a, email || domain":             true,
		"first_name || ' ' || last_name": true,
	}
	for cols, want := range cases {
		if got := indexColumnsLookExpression(cols); got != want {
			t.Fatalf("indexColumnsLookExpression(%q) = %v, want %v", cols, got, want)
		}
	}
}

// TestIndexColumnsLookExpressionToleratesHyphenatedCollationName guards against a
// hyphenated COLLATE locale name (e.g. "en-US") being misclassified as an expression index
// just because it contains a '-'. The hyphen there is part of a locale identifier, not an
// arithmetic operator, and must not cause the index to be skipped.
func TestIndexColumnsLookExpressionToleratesHyphenatedCollationName(t *testing.T) {
	cases := map[string]bool{
		"email COLLATE en-US":       false,
		"email COLLATE en-US ASC":   false,
		"email COLLATE en_US-x-icu": false,
		"email - 1":                 true,
		"email-1":                   true,
	}
	for cols, want := range cases {
		if got := indexColumnsLookExpression(cols); got != want {
			t.Fatalf("indexColumnsLookExpression(%q) = %v, want %v", cols, got, want)
		}
	}
}

func TestConvertIndexDDLSkipsUnparenthesizedExpressionIndex(t *testing.T) {
	got := convertIndexDDL(`CREATE INDEX idx ON items(email || domain);`)
	if got != "" {
		t.Fatalf("convertIndexDDL = %q, want empty", got)
	}
}

func TestLintDDLStatementFlagsUnparenthesizedExpressionIndex(t *testing.T) {
	issues := lintDDLStatement(`CREATE INDEX idx ON items(email || domain);`)
	if len(issues) != 1 || issues[0].Code != "EXPRESSION_INDEX" {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestConvertIndexDDLPreservesUniqueWithSortModifier(t *testing.T) {
	got := convertIndexDDL(`CREATE UNIQUE INDEX idx_users_email ON users (email DESC);`)
	want := `CREATE UNIQUE INDEX IF NOT EXISTS "idx_users_email" ON "users" ("email");`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLintIndexStatementDoesNotFlagSortModifierAsExpression(t *testing.T) {
	issues := lintDDLStatement(`CREATE UNIQUE INDEX idx_users_email ON users (email DESC);`)
	for _, issue := range issues {
		if issue.Code == "EXPRESSION_INDEX" {
			t.Fatalf("sort modifier must not be misclassified as an expression index: %#v", issues)
		}
	}
}

func TestConvertSchemaPartsSkipsPartialIndex(t *testing.T) {
	path := filepath.Join("testdata", "ddl_lint_export.sql")
	parts, _, err := ConvertSchemaParts(path)
	if err != nil {
		t.Fatalf("ConvertSchemaParts: %v", err)
	}
	if strings.Contains(parts.Indexes, "idx_items_active_email") {
		t.Fatalf("partial index should be omitted:\n%s", parts.Indexes)
	}
	if !strings.Contains(parts.Indexes, `idx_items_email`) {
		t.Fatalf("expected simple index preserved:\n%s", parts.Indexes)
	}
	if strings.Contains(parts.Indexes, `idx_items_lower_email`) {
		t.Fatalf("expression index should be omitted:\n%s", parts.Indexes)
	}
}
