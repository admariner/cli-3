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
