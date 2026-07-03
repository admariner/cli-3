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
	start := 0
	quote := byte(0)
	for i := 0; i < len(query); i++ {
		c := query[i]
		if quote != 0 {
			if c == '\\' && quote == '\'' && i+1 < len(query) {
				i++
				continue
			}
			if c == quote {
				if i+1 < len(query) && query[i+1] == quote {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case ';':
			if trimmed := strings.TrimSpace(query[start:i]); trimmed != "" {
				out = append(out, trimmed)
			}
			start = i + 1
		}
	}
	if trimmed := strings.TrimSpace(query[start:]); trimmed != "" {
		out = append(out, trimmed)
	}
	return out
}

func isDestructiveStatement(stmt string) bool {
	upper := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(strings.ToUpper(stripQuotedSQL(stmt)))
	return slices.ContainsFunc(destructiveWords, func(word string) bool {
		return containsWord(upper, word)
	})
}

func stripQuotedSQL(stmt string) string {
	var out strings.Builder
	out.Grow(len(stmt))
	quote := byte(0)
	for i := 0; i < len(stmt); i++ {
		c := stmt[i]
		if quote != 0 {
			out.WriteByte(' ')
			if c == '\\' && quote == '\'' && i+1 < len(stmt) {
				i++
				out.WriteByte(' ')
				continue
			}
			if c == quote {
				if i+1 < len(stmt) && stmt[i+1] == quote {
					i++
					out.WriteByte(' ')
					continue
				}
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
			out.WriteByte(' ')
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

func containsWord(q, word string) bool {
	return strings.Contains(" "+q+" ", " "+word+" ")
}
