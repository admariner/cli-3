package d1

import (
	"regexp"
	"strings"
)

var (
	createViewRe    = regexp.MustCompile(`(?is)^CREATE\s+(?:TEMP(?:ORARY)?\s+)?VIEW\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"([^"]+)"|'([^']+)'|` + "`" + `([^` + "`" + `]+)` + "`" + `|([a-zA-Z_][\w]*))`)
	createTriggerRe = regexp.MustCompile(`(?is)^CREATE\s+(?:TEMP(?:ORARY)?\s+)?TRIGGER\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"([^"]+)"|'([^']+)'|` + "`" + `([^` + "`" + `]+)` + "`" + `|([a-zA-Z_][\w]*))`)
)

func lintDumpDDL(inputPath string) ([]Issue, error) {
	var issues []Issue
	err := foreachDumpStatement(inputPath, func(stmt string) error {
		issues = append(issues, lintDDLStatement(stmt)...)
		return nil
	})
	return issues, err
}

func lintDDLStatement(stmt string) []Issue {
	trimmed := strings.TrimSpace(stmt)
	upper := strings.ToUpper(trimmed)

	switch {
	case isCreateViewPrefix(upper):
		return []Issue{viewNotMigratedIssue(trimmed)}
	case isCreateTriggerPrefix(upper):
		return []Issue{triggerNotMigratedIssue(trimmed)}
	case strings.HasPrefix(upper, "CREATE") && strings.Contains(upper, " INDEX "):
		return lintIndexStatement(trimmed)
	default:
		return nil
	}
}

func viewNotMigratedIssue(stmt string) Issue {
	name := ddlObjectName(createViewRe, stmt)
	return Issue{
		Code:        "VIEW_NOT_MIGRATED",
		Severity:    SeverityWarning,
		Table:       name,
		Message:     "SQLite view will not be imported into Postgres",
		Remediation: "Recreate needed views manually in Postgres after import (SQLite view SQL is not auto-translated)",
	}
}

func triggerNotMigratedIssue(stmt string) Issue {
	name := ddlObjectName(createTriggerRe, stmt)
	return Issue{
		Code:        "TRIGGER_NOT_MIGRATED",
		Severity:    SeverityWarning,
		Table:       name,
		Message:     "SQLite trigger will not be imported into Postgres",
		Remediation: "Recreate trigger logic in Postgres or application code after import",
	}
}

func lintIndexStatement(stmt string) []Issue {
	m := createIndexRe.FindStringSubmatch(stmt)
	if m == nil {
		if isPartialIndexDDL(stmt) {
			return []Issue{{
				Code:        "PARTIAL_INDEX",
				Severity:    SeverityWarning,
				Message:     "Partial index (WHERE clause) will not be preserved",
				Remediation: "Recreate the partial index in Postgres with the same WHERE predicate after import",
			}}
		}
		return []Issue{{
			Code:        "EXPRESSION_INDEX",
			Severity:    SeverityWarning,
			Message:     "Non-simple CREATE INDEX statement will not be migrated",
			Remediation: "Recreate expression or complex indexes manually in Postgres after import",
		}}
	}

	name := firstNonEmpty(m[2], m[3], m[4], m[5])
	table := firstNonEmpty(m[6], m[7], m[8], m[9])
	columns := m[10]

	var issues []Issue
	if isPartialIndexDDL(stmt) {
		issues = append(issues, Issue{
			Code:        "PARTIAL_INDEX",
			Severity:    SeverityWarning,
			Table:       table,
			Message:     "Partial index " + name + " (WHERE clause) will not be preserved",
			Remediation: "Recreate the partial index in Postgres with the same WHERE predicate after import",
		})
	}
	if indexColumnsLookExpression(columns) {
		issues = append(issues, Issue{
			Code:        "EXPRESSION_INDEX",
			Severity:    SeverityWarning,
			Table:       table,
			Message:     "Expression index " + name + " will not be migrated correctly",
			Remediation: "Recreate expression indexes manually in Postgres after import",
		})
	}
	return issues
}

func isCreateViewPrefix(upper string) bool {
	return strings.HasPrefix(upper, "CREATE VIEW") ||
		strings.HasPrefix(upper, "CREATE TEMP VIEW") ||
		strings.HasPrefix(upper, "CREATE TEMPORARY VIEW")
}

func isCreateTriggerPrefix(upper string) bool {
	return strings.HasPrefix(upper, "CREATE TRIGGER") ||
		strings.HasPrefix(upper, "CREATE TEMP TRIGGER") ||
		strings.HasPrefix(upper, "CREATE TEMPORARY TRIGGER")
}

func ddlObjectName(re *regexp.Regexp, stmt string) string {
	m := re.FindStringSubmatch(stmt)
	if m == nil {
		return ""
	}
	return firstNonEmpty(m[1], m[2], m[3], m[4])
}

func indexColumnsLookExpression(columns string) bool {
	columns = strings.TrimSpace(columns)
	if columns == "" {
		return false
	}
	if strings.ContainsAny(columns, "()") {
		return true
	}
	for _, part := range splitCommaList(columns) {
		if indexedColumnPartLooksLikeExpression(part) {
			return true
		}
		name := cleanIndexedColumnName(part)
		if name == "" {
			continue
		}
		if !simpleIndexColumnName(name) {
			return true
		}
	}
	return false
}

// indexedColumnPartLooksLikeExpression reports whether an indexed-column list entry carries
// operator or concatenation tokens (e.g. "email || domain") after its leading identifier,
// marking it as an expression rather than a plain column reference with optional
// COLLATE/ASC/DESC modifiers. Unlike expressions wrapped in parens, these have no "()" for
// the caller's cheap containsAny check to catch, so the leftover text after the identifier
// must be inspected directly.
func indexedColumnPartLooksLikeExpression(part string) bool {
	part = strings.TrimSpace(part)
	if part == "" {
		return false
	}
	_, rest := parseColumnNameAndRest(part)
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return false
	}
	return strings.ContainsAny(rest, "|+-*/%<>=!~&^")
}

func simpleIndexColumnName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if i == 0 {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && c != '_' {
				return false
			}
			continue
		}
		if !isSQLIdentChar(c) {
			return false
		}
	}
	return true
}
