package sqlquery

import (
	"slices"
	"strings"
)

var destructiveWords = []string{"DELETE", "DROP", "TRUNCATE"}

// DestructiveQueryError is returned when a query would delete or drop data or
// schema objects and --force was not passed.
type DestructiveQueryError struct{}

func (e *DestructiveQueryError) Error() string {
	return "destructive SQL requires explicit user approval (ask the user, then re-run with --force)"
}

// IsDestructiveQuery reports whether the query deletes or drops resources.
// This is a best-effort guardrail for agents, not a SQL parser. Any statement
// containing the words DELETE, DROP, or TRUNCATE requires approval.
func IsDestructiveQuery(query string) bool {
	return slices.ContainsFunc(splitSQLStatements(query), isDestructiveStatement)
}

func splitSQLStatements(query string) []string {
	out := make([]string, 0, strings.Count(query, ";")+1)
	for part := range strings.SplitSeq(query, ";") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func isDestructiveStatement(stmt string) bool {
	upper := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(strings.ToUpper(stmt))
	return slices.ContainsFunc(destructiveWords, func(word string) bool {
		return containsWord(upper, word)
	})
}

func containsWord(q, word string) bool {
	return strings.Contains(" "+q+" ", " "+word+" ")
}
