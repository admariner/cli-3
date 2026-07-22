package d1

import (
	"regexp"
	"strings"

	"github.com/planetscale/cli/internal/postgres"
)

var (
	referencesClauseRe     = regexp.MustCompile(`(?is)^REFERENCES\s+(?:"([^"]+)"|'([^']+)'|` + "`" + `([^` + "`" + `]+)` + "`" + `|([a-zA-Z_][\w]*))\s*\(\s*([^)]+)\)\s*(.*)$`)
	foreignKeyConstraintRe = regexp.MustCompile(`(?is)^FOREIGN\s+KEY\s*\(\s*([^)]+)\)\s*(REFERENCES\s+.+)$`)
	primaryKeyConstraintRe = regexp.MustCompile(`(?is)^PRIMARY\s+KEY\s*\(\s*([^)]+)\)\s*(?:ON\s+CONFLICT\s+\w+)?\s*$`)
	uniqueConstraintRe     = regexp.MustCompile(`(?is)^UNIQUE\s*\(\s*([^)]+)\)\s*(?:ON\s+CONFLICT\s+\w+)?\s*$`)
	createIndexRe          = regexp.MustCompile(`(?is)^CREATE\s+(UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"([^"]+)"|'([^']+)'|` + "`" + `([^` + "`" + `]+)` + "`" + `|([a-zA-Z_][\w]*))\s+ON\s+(?:"([^"]+)"|'([^']+)'|` + "`" + `([^` + "`" + `]+)` + "`" + `|([a-zA-Z_][\w]*))\s*\(\s*([^)]+)\)\s*(?:WHERE\b.+)?;?\s*$`)
	partialIndexRe         = regexp.MustCompile(`(?is)\)\s*WHERE\b`)
)

func isPartialIndexDDL(raw string) bool {
	return partialIndexRe.MatchString(raw)
}

// IndexSchema holds a parsed CREATE INDEX statement from a dump.
type IndexSchema struct {
	Name    string
	Table   string
	Unique  bool
	Columns string
	RawDDL  string
}

func isTableConstraint(part string) bool {
	upper := strings.ToUpper(strings.TrimSpace(part))
	return strings.HasPrefix(upper, "PRIMARY KEY") ||
		strings.HasPrefix(upper, "FOREIGN KEY") ||
		strings.HasPrefix(upper, "UNIQUE(") ||
		strings.HasPrefix(upper, "UNIQUE (") ||
		strings.HasPrefix(upper, "CHECK(") ||
		strings.HasPrefix(upper, "CHECK (") ||
		strings.HasPrefix(upper, "CONSTRAINT ")
}

func referencesClause(colDef string) string {
	idx := indexOfIgnoreCase(colDef, "REFERENCES")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(colDef[idx:])
}

func convertTableConstraint(clause string, table TableSchema, all []TableSchema, ctx *TypeCoercionContext) string {
	clause = strings.TrimSpace(clause)
	clause = strings.TrimSuffix(clause, ",")
	if clause == "" {
		return ""
	}

	upper := strings.ToUpper(clause)
	switch {
	case strings.HasPrefix(upper, "CONSTRAINT "):
		return convertNamedConstraint(clause, table, all, ctx)
	case strings.HasPrefix(upper, "FOREIGN KEY"):
		return convertForeignKeyConstraint(clause, table, all)
	case strings.HasPrefix(upper, "PRIMARY KEY"):
		return convertPrimaryKeyConstraint(clause, table)
	case strings.HasPrefix(upper, "UNIQUE"):
		return convertUniqueConstraint(clause, table)
	case strings.HasPrefix(upper, "CHECK"):
		return convertCheckConstraint(clause, table, ctx)
	default:
		return clause
	}
}

// convertNamedConstraint converts a `CONSTRAINT <name> <body>` clause by re-quoting the
// constraint name and running the body through the same conversion as unnamed constraints,
// so named constraints get identical quoting/canonicalization fixes.
func convertNamedConstraint(clause string, table TableSchema, all []TableSchema, ctx *TypeCoercionContext) string {
	rest := strings.TrimSpace(clause[len("CONSTRAINT"):])
	name, body := parseColumnNameAndRest(rest)
	if name == "" || body == "" {
		return clause
	}
	converted := convertTableConstraint(body, table, all, ctx)
	if converted == "" {
		return clause
	}
	return "CONSTRAINT " + postgres.QuoteIdentifier(name) + " " + converted
}

// convertCheckConstraint converts a table-level `CHECK (expr)` clause, re-quoting any
// identifiers inside expr that reference the table's columns (so mixed-case column names
// survive Postgres's case-folding of unquoted identifiers) and converting SQLite's
// double-quoted string-literal fallback into proper single-quoted literals. ctx (when
// non-nil) is used to detect columns that will be coerced to Postgres BOOLEAN, so literal
// 0/1 comparisons against them can be rewritten to false/true (see convertCheckExpr).
func convertCheckConstraint(clause string, table TableSchema, ctx *TypeCoercionContext) string {
	clause = strings.TrimSpace(clause)
	clause = strings.TrimSuffix(clause, ",")

	upper := strings.ToUpper(clause)
	if !strings.HasPrefix(upper, "CHECK") {
		return clause
	}
	rest := strings.TrimSpace(clause[len("CHECK"):])
	if !strings.HasPrefix(rest, "(") {
		return clause
	}
	end, ok := matchingParenEnd(rest, 0)
	if !ok {
		return clause
	}
	expr := rest[1:end]
	return "CHECK (" + convertCheckExpr(expr, table, ctx) + ")"
}

// checkExprKeywords are SQL keywords that can legitimately appear as bare words inside a
// CHECK/GENERATED expression. They are never treated as column references, even when the
// table has a column with the same name, since quoting them would corrupt the expression
// (e.g. `a > 0 AND b < 5` must not become `"a" > 0 "and" "b" < 5`). The tradeoff is that a
// column named after one of these keywords won't be case-canonicalized when referenced
// bare — such references must already be quoted in the source DDL to be unambiguous.
var checkExprKeywords = map[string]struct{}{
	"and": {}, "or": {}, "not": {}, "in": {}, "is": {}, "null": {},
	"like": {}, "glob": {}, "regexp": {}, "match": {}, "escape": {},
	"between": {}, "case": {}, "when": {}, "then": {}, "else": {}, "end": {},
	"cast": {}, "as": {}, "exists": {}, "distinct": {}, "collate": {},
	"isnull": {}, "notnull": {}, "true": {}, "false": {},
	"current_time": {}, "current_date": {}, "current_timestamp": {},
}

// convertCheckExpr rewrites identifiers and string literals inside a CHECK/GENERATED
// expression for Postgres:
//   - bare or double-quoted tokens that match one of the table's column names (case
//     insensitively) are re-quoted using the column's declared case, so Postgres's
//     case-folding of unquoted identifiers can't cause a "column does not exist" error.
//     Bare tokens that are SQL keywords (see checkExprKeywords) are never treated as
//     column references.
//   - double-quoted tokens that do NOT match a column name are treated as SQLite's
//     double-quoted string-literal fallback and converted to proper single-quoted
//     string literals (Postgres always treats double quotes as identifiers).
//   - bracket-quoted ([col]) and backtick-quoted identifiers — both valid SQLite quoting
//     that Postgres rejects — are converted to double-quoted identifiers, canonicalized
//     to the column's declared case when they match one.
//   - single-quoted string literals and everything else are passed through unchanged.
//
// When ctx is non-nil, a second pass (see rewriteBooleanCheckLiterals) rewrites literal 0/1
// comparisons against columns that will be coerced to Postgres BOOLEAN (e.g. `is_active = 1`
// or `is_active IN (0,1)`), since SQLite's INTEGER 0/1 booleans are never valid literals
// against a BOOLEAN column in Postgres.
func convertCheckExpr(expr string, table TableSchema, ctx *TypeCoercionContext) string {
	colMap := make(map[string]string, len(table.Columns))
	for _, col := range table.Columns {
		colMap[strings.ToLower(col.Name)] = col.Name
	}

	var out strings.Builder
	n := len(expr)
	for i := 0; i < n; {
		c := expr[i]
		switch {
		case c == '\'':
			j := i + 1
			for j < n {
				if expr[j] == '\'' {
					if j+1 < n && expr[j+1] == '\'' {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			out.WriteString(expr[i:j])
			i = j
		case c == '"':
			j := i + 1
			var raw strings.Builder
			for j < n {
				if expr[j] == '"' {
					if j+1 < n && expr[j+1] == '"' {
						raw.WriteByte('"')
						j += 2
						continue
					}
					j++
					break
				}
				raw.WriteByte(expr[j])
				j++
			}
			inner := raw.String()
			if actual, ok := colMap[strings.ToLower(inner)]; ok {
				out.WriteString(postgres.QuoteIdentifier(actual))
			} else {
				out.WriteString(quotePostgresLiteral(inner))
			}
			i = j
		case c == '[':
			end := strings.IndexByte(expr[i+1:], ']')
			if end < 0 {
				out.WriteString(expr[i:])
				i = n
				break
			}
			inner := expr[i+1 : i+1+end]
			out.WriteString(postgres.QuoteIdentifier(canonicalIdent(inner, colMap)))
			i = i + 1 + end + 1
		case c == '`':
			j := i + 1
			var raw strings.Builder
			for j < n {
				if expr[j] == '`' {
					if j+1 < n && expr[j+1] == '`' {
						raw.WriteByte('`')
						j += 2
						continue
					}
					j++
					break
				}
				raw.WriteByte(expr[j])
				j++
			}
			out.WriteString(postgres.QuoteIdentifier(canonicalIdent(raw.String(), colMap)))
			i = j
		case isIdentStartByte(c):
			j := i + 1
			for j < n && isSQLIdentChar(expr[j]) {
				j++
			}
			word := expr[i:j]
			k := j
			for k < n && (expr[k] == ' ' || expr[k] == '\t') {
				k++
			}
			isFunctionCall := k < n && expr[k] == '('
			_, isKeyword := checkExprKeywords[strings.ToLower(word)]
			if actual, ok := colMap[strings.ToLower(word)]; ok && !isFunctionCall && !isKeyword {
				out.WriteString(postgres.QuoteIdentifier(actual))
			} else {
				out.WriteString(word)
			}
			i = j
		default:
			out.WriteByte(c)
			i++
		}
	}
	result := out.String()
	if boolCols := booleanCoercedColumnNames(table, ctx); len(boolCols) > 0 {
		result = rewriteBooleanCheckLiterals(result, boolCols)
	}
	return result
}

func isIdentStartByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

// canonicalIdent returns the declared-case column name for ident when it matches one of
// the table's columns (colMap keys are lower-cased declared names), or ident unchanged.
func canonicalIdent(ident string, colMap map[string]string) string {
	if actual, ok := colMap[strings.ToLower(ident)]; ok {
		return actual
	}
	return ident
}

// booleanCoercedColumnNames returns the lower-cased names of table's columns that
// sqliteTypeToPostgres/isBooleanLikeColumn will coerce to Postgres BOOLEAN. Returns nil
// (rather than an empty, non-nil map) when ctx is nil or no column qualifies, so callers can
// use a plain length check to skip the boolean CHECK-literal rewrite pass entirely.
func booleanCoercedColumnNames(table TableSchema, ctx *TypeCoercionContext) map[string]struct{} {
	if ctx == nil {
		return nil
	}
	var cols map[string]struct{}
	for _, col := range table.Columns {
		if isBooleanLikeColumn(col, table, ctx) {
			if cols == nil {
				cols = make(map[string]struct{}, len(table.Columns))
			}
			cols[strings.ToLower(col.Name)] = struct{}{}
		}
	}
	return cols
}

// ceTokKind classifies a token produced by tokenizeConvertedExpr, the minimal tokenizer used
// by rewriteBooleanCheckLiterals to locate literal 0/1 operands next to a boolean-coerced
// column reference in an already-converted CHECK expression (column references are always
// double-quoted at this point, see convertCheckExpr).
type ceTokKind int

const (
	ceTokOther ceTokKind = iota
	ceTokWS
	ceTokString // '...'
	ceTokQIdent // "..." (a quoted identifier - always a column reference post-conversion)
	ceTokNumber // bare digit run, e.g. "0", "1", "42"
	ceTokWord   // bare word, e.g. IN, NOT, AND, a function name
	ceTokOp     // "=", "<>", "!="
	ceTokComma  // ","
	ceTokLParen // "("
	ceTokRParen // ")"
)

type ceTok struct {
	kind ceTokKind
	text string
}

func isDigitByte(c byte) bool {
	return c >= '0' && c <= '9'
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// tokenizeConvertedExpr tokenizes an already-converted CHECK expression (as produced by the
// main convertCheckExpr scan) just enough to let rewriteBooleanCheckLiterals find literal
// 0/1 operands next to quoted column references. It does not need to handle SQLite's raw
// quoting variants (brackets, backticks, double-quoted string fallback) since convertCheckExpr
// has already normalized those away.
func tokenizeConvertedExpr(s string) []ceTok {
	var toks []ceTok
	n := len(s)
	for i := 0; i < n; {
		c := s[i]
		switch {
		case isSpaceByte(c):
			j := i + 1
			for j < n && isSpaceByte(s[j]) {
				j++
			}
			toks = append(toks, ceTok{ceTokWS, s[i:j]})
			i = j
		case c == '\'':
			j := i + 1
			for j < n {
				if s[j] == '\'' {
					if j+1 < n && s[j+1] == '\'' {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			toks = append(toks, ceTok{ceTokString, s[i:j]})
			i = j
		case c == '"':
			j := i + 1
			for j < n {
				if s[j] == '"' {
					if j+1 < n && s[j+1] == '"' {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			toks = append(toks, ceTok{ceTokQIdent, s[i:j]})
			i = j
		case isDigitByte(c):
			j := i + 1
			for j < n && isDigitByte(s[j]) {
				j++
			}
			toks = append(toks, ceTok{ceTokNumber, s[i:j]})
			i = j
		case isIdentStartByte(c):
			j := i + 1
			for j < n && isSQLIdentChar(s[j]) {
				j++
			}
			toks = append(toks, ceTok{ceTokWord, s[i:j]})
			i = j
		case c == '<' && i+1 < n && s[i+1] == '>':
			toks = append(toks, ceTok{ceTokOp, "<>"})
			i += 2
		case c == '!' && i+1 < n && s[i+1] == '=':
			toks = append(toks, ceTok{ceTokOp, "!="})
			i += 2
		case c == '=':
			toks = append(toks, ceTok{ceTokOp, "="})
			i++
		case c == ',':
			toks = append(toks, ceTok{ceTokComma, ","})
			i++
		case c == '(':
			toks = append(toks, ceTok{ceTokLParen, "("})
			i++
		case c == ')':
			toks = append(toks, ceTok{ceTokRParen, ")"})
			i++
		default:
			toks = append(toks, ceTok{ceTokOther, s[i : i+1]})
			i++
		}
	}
	return toks
}

// unquoteQIdent strips the surrounding double quotes from a ceTokQIdent's text and
// un-escapes doubled internal quotes, e.g. `"is_active"` -> `is_active`.
func unquoteQIdent(text string) string {
	if len(text) >= 2 && strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`) {
		return strings.ReplaceAll(text[1:len(text)-1], `""`, `"`)
	}
	return text
}

// boolLiteralFor maps a SQLite 0/1 integer literal token to the Postgres boolean literal it
// represents, or ("", false) for any other value (only exactly "0" or "1" - a boolean-coerced
// column never legitimately compares against any other integer).
func boolLiteralFor(tok ceTok) (string, bool) {
	if tok.kind != ceTokNumber {
		return "", false
	}
	switch tok.text {
	case "0":
		return "false", true
	case "1":
		return "true", true
	}
	return "", false
}

// rewriteBooleanCheckLiterals rewrites literal SQLite 0/1 integer comparisons against
// boolCols (lower-cased column names already known to be coerced to Postgres BOOLEAN) into
// proper Postgres boolean literals, e.g.:
//
//	"is_active" = 1        -> "is_active" = true
//	"is_active" <> 0        -> "is_active" <> false
//	0 = "is_active"         -> false = "is_active"
//	"is_active" IN (0, 1)   -> "is_active" IN (false, true)
//
// Only literal 0/1 operands directly adjacent (across whitespace) to a boolCols reference via
// a comparison operator or an IN(...) list are rewritten; everything else - including
// comparisons on any other column - passes through unchanged.
func rewriteBooleanCheckLiterals(expr string, boolCols map[string]struct{}) string {
	toks := tokenizeConvertedExpr(expr)
	n := len(toks)

	nextNonWS := func(i int) int {
		for j := i + 1; j < n; j++ {
			if toks[j].kind != ceTokWS {
				return j
			}
		}
		return -1
	}

	isBoolColumn := func(tok ceTok) bool {
		if tok.kind != ceTokQIdent {
			return false
		}
		_, ok := boolCols[strings.ToLower(unquoteQIdent(tok.text))]
		return ok
	}

	for i := 0; i < n; i++ {
		if isBoolColumn(toks[i]) {
			j := nextNonWS(i)
			if j < 0 {
				continue
			}
			switch {
			case toks[j].kind == ceTokOp:
				if k := nextNonWS(j); k >= 0 {
					if repl, ok := boolLiteralFor(toks[k]); ok {
						toks[k].text = repl
					}
				}
			case toks[j].kind == ceTokWord && strings.EqualFold(toks[j].text, "IN"):
				if k := nextNonWS(j); k >= 0 && toks[k].kind == ceTokLParen {
					depth := 1
					for m := k + 1; m < n && depth > 0; m++ {
						switch toks[m].kind {
						case ceTokLParen:
							depth++
						case ceTokRParen:
							depth--
						case ceTokNumber:
							if depth == 1 {
								if repl, ok := boolLiteralFor(toks[m]); ok {
									toks[m].text = repl
								}
							}
						}
					}
				}
			}
			continue
		}

		// Reversed operand order: <literal> <op> <bool column>.
		if repl, ok := boolLiteralFor(toks[i]); ok {
			j := nextNonWS(i)
			if j >= 0 && toks[j].kind == ceTokOp {
				if k := nextNonWS(j); k >= 0 && isBoolColumn(toks[k]) {
					toks[i].text = repl
				}
			}
		}
	}

	var out strings.Builder
	for _, t := range toks {
		out.WriteString(t.text)
	}
	return out.String()
}

func convertForeignKeyConstraint(clause string, table TableSchema, all []TableSchema) string {
	m := foreignKeyConstraintRe.FindStringSubmatch(clause)
	if m == nil {
		return clause
	}
	cols := quoteColumnListFor(m[1], &table)
	refs := convertReferencesClause(strings.TrimSpace(m[2]), all)
	return "FOREIGN KEY (" + cols + ") " + refs
}

func convertPrimaryKeyConstraint(clause string, table TableSchema) string {
	m := primaryKeyConstraintRe.FindStringSubmatch(clause)
	if m == nil {
		return clause
	}
	return "PRIMARY KEY (" + quoteColumnListFor(m[1], &table) + ")"
}

func convertUniqueConstraint(clause string, table TableSchema) string {
	m := uniqueConstraintRe.FindStringSubmatch(clause)
	if m == nil {
		return clause
	}
	return "UNIQUE (" + quoteColumnListFor(m[1], &table) + ")"
}

// convertReferencesClause converts a `REFERENCES table(col, ...) [tail]` clause. SQLite
// resolves table/column names in REFERENCES case-insensitively, but Postgres compares
// quoted identifiers case-sensitively, so the referenced table/columns are canonicalized
// to the actual declared case from all (the full set of parsed tables) rather than quoting
// whatever case happens to appear in this clause.
func convertReferencesClause(refs string, all []TableSchema) string {
	m := referencesClauseRe.FindStringSubmatch(refs)
	if m == nil {
		return refs
	}
	rawTable := firstNonEmpty(m[1], m[2], m[3], m[4])
	refTable := tableByName(all, rawTable)
	tableName := rawTable
	if refTable != nil {
		tableName = refTable.Name
	}
	table := postgres.QuoteIdentifier(tableName)
	refCols := quoteColumnListFor(m[5], refTable)
	tail := strings.TrimSpace(m[6])
	if tail != "" {
		return "REFERENCES " + table + " (" + refCols + ") " + tail
	}
	return "REFERENCES " + table + " (" + refCols + ")"
}

// quoteColumnList quotes a comma-separated column list, stripping SQLite indexed-column
// modifiers (COLLATE, ASC, DESC) that cannot appear in this position in Postgres.
func quoteColumnList(list string) string {
	return quoteColumnListFor(list, nil)
}

// quoteColumnListFor is like quoteColumnList but additionally canonicalizes each column's
// case against table's declared columns when table is non-nil.
func quoteColumnListFor(list string, table *TableSchema) string {
	parts := splitCommaList(list)
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		name := cleanIndexedColumnName(part)
		if name == "" {
			continue
		}
		quoted = append(quoted, postgres.QuoteIdentifier(canonicalColumnName(name, table)))
	}
	return strings.Join(quoted, ", ")
}

// cleanIndexedColumnName extracts the bare column name from a column-list entry that may
// carry SQLite indexed-column modifiers, e.g. "b DESC" -> "b", `"MixedCase" COLLATE NOCASE
// ASC` -> "MixedCase".
func cleanIndexedColumnName(part string) string {
	part = strings.TrimSpace(part)
	if part == "" {
		return ""
	}
	name, _ := parseColumnNameAndRest(part)
	if name == "" {
		return strings.Trim(part, "`\"'")
	}
	return name
}

// canonicalColumnName looks up name (case-insensitively) among table's declared columns
// and returns the declared case, or name unchanged if table is nil or has no match.
func canonicalColumnName(name string, table *TableSchema) string {
	if table == nil {
		return name
	}
	for _, col := range table.Columns {
		if strings.EqualFold(col.Name, name) {
			return col.Name
		}
	}
	return name
}

func splitCommaList(list string) []string {
	var parts []string
	var current strings.Builder
	depth := 0
	inSingle := false
	inDouble := false

	for _, r := range list {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
			current.WriteRune(r)
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
			current.WriteRune(r)
		case '(':
			if !inSingle && !inDouble {
				depth++
			}
			current.WriteRune(r)
		case ')':
			if !inSingle && !inDouble {
				depth--
			}
			current.WriteRune(r)
		case ',':
			if depth == 0 && !inSingle && !inDouble {
				parts = append(parts, current.String())
				current.Reset()
				continue
			}
			current.WriteRune(r)
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func convertIndexDDL(raw string) string {
	if isPartialIndexDDL(raw) {
		return ""
	}
	m := createIndexRe.FindStringSubmatch(raw)
	if m == nil {
		return ""
	}
	if indexColumnsLookExpression(m[10]) {
		return ""
	}
	unique := strings.TrimSpace(m[1]) != ""
	name := postgres.QuoteIdentifier(firstNonEmpty(m[2], m[3], m[4], m[5]))
	table := postgres.QuoteIdentifier(firstNonEmpty(m[6], m[7], m[8], m[9]))
	cols := quoteColumnList(m[10])
	prefix := "CREATE INDEX IF NOT EXISTS "
	if unique {
		prefix = "CREATE UNIQUE INDEX IF NOT EXISTS "
	}
	return prefix + name + " ON " + table + " (" + cols + ");"
}

func isUUIDColumn(col ColumnSchema, table TableSchema, all []TableSchema, ctx *TypeCoercionContext) bool {
	if isExplicitUUIDColumn(col) {
		if ctx == nil {
			return false
		}
		return samplesAllowUUID(table.Name, col.Name, ctx)
	}
	return columnReferencesUUIDKey(col, table, all, ctx)
}

const maxUUIDFKDepth = 32

func columnReferencesUUIDKey(col ColumnSchema, table TableSchema, all []TableSchema, ctx *TypeCoercionContext) bool {
	visited := make(map[string]struct{})
	return columnReferencesUUIDKeyVisited(col, table, all, ctx, visited, 0)
}

func columnReferencesUUIDKeyVisited(col ColumnSchema, table TableSchema, all []TableSchema, ctx *TypeCoercionContext, visited map[string]struct{}, depth int) bool {
	if depth >= maxUUIDFKDepth {
		return false
	}
	key := table.Name + "." + col.Name
	if _, seen := visited[key]; seen {
		return false
	}
	visited[key] = struct{}{}

	refTable, refCol := columnFKTarget(col, table)
	if refTable == "" {
		return false
	}
	ref := tableByName(all, refTable)
	if ref == nil {
		return false
	}
	refColSchema := columnByName(*ref, refCol)
	if isExplicitUUIDColumn(refColSchema) {
		if ctx == nil {
			return false
		}
		return samplesAllowUUID(ref.Name, refColSchema.Name, ctx)
	}
	return columnReferencesUUIDKeyVisited(refColSchema, *ref, all, ctx, visited, depth+1)
}

func isExplicitUUIDColumn(col ColumnSchema) bool {
	name := strings.ToLower(col.Name)
	t := strings.ToUpper(col.Type)

	if !isTextLikeType(t) {
		return false
	}

	if col.PrimaryKey && (name == "id" || name == "uuid") {
		return true
	}
	if strings.HasSuffix(name, "_uuid") {
		return true
	}
	return false
}

func columnFKTarget(col ColumnSchema, table TableSchema) (string, string) {
	if col.ForeignKey != "" {
		return parseReferencesTarget(col.ForeignKey)
	}
	for _, constraint := range table.Constraints {
		cols, refs := parseTableLevelForeignKey(constraint)
		for _, name := range cols {
			if name == col.Name {
				return parseReferencesTarget(refs)
			}
		}
	}
	return "", ""
}

func parseTableLevelForeignKey(constraint string) ([]string, string) {
	m := foreignKeyConstraintRe.FindStringSubmatch(constraint)
	if m == nil {
		return nil, ""
	}
	cols := make([]string, 0)
	for _, part := range splitCommaList(m[1]) {
		part = strings.Trim(strings.TrimSpace(part), "`\"'")
		if part != "" {
			cols = append(cols, part)
		}
	}
	return cols, strings.TrimSpace(m[2])
}

func parseReferencesTarget(refs string) (string, string) {
	m := referencesClauseRe.FindStringSubmatch(strings.TrimSpace(refs))
	if m == nil {
		return "", ""
	}
	table := firstNonEmpty(m[1], m[2], m[3], m[4])
	refCols := splitCommaList(m[5])
	refCol := ""
	if len(refCols) > 0 {
		refCol = strings.Trim(strings.TrimSpace(refCols[0]), "`\"'")
	}
	return table, refCol
}

func tableByName(all []TableSchema, name string) *TableSchema {
	lower := strings.ToLower(name)
	for i := range all {
		if strings.ToLower(all[i].Name) == lower {
			return &all[i]
		}
	}
	return nil
}

func columnByName(table TableSchema, name string) ColumnSchema {
	lower := strings.ToLower(name)
	for _, col := range table.Columns {
		if strings.ToLower(col.Name) == lower {
			return col
		}
	}
	return ColumnSchema{}
}

func isTextLikeType(t string) bool {
	return t == "" || strings.Contains(t, "CHAR") || strings.Contains(t, "CLOB") || strings.Contains(t, "TEXT")
}
