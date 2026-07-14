package schema

import "strings"

// reservedIdents is the set of PostgreSQL reserved keywords (including those
// reserved except as function or type names) that cannot be used as table or
// column names without quoting. Source: PostgreSQL documentation, Appendix C.
var reservedIdents = map[string]bool{
	"all": true, "analyse": true, "analyze": true, "and": true, "any": true,
	"array": true, "as": true, "asc": true, "asymmetric": true,
	"authorization": true, "binary": true, "both": true, "case": true,
	"cast": true, "check": true, "collate": true, "collation": true,
	"column": true, "concurrently": true, "constraint": true, "create": true,
	"cross": true, "current_catalog": true, "current_date": true,
	"current_role": true, "current_schema": true, "current_time": true,
	"current_timestamp": true, "current_user": true, "default": true,
	"deferrable": true, "desc": true, "distinct": true, "do": true,
	"else": true, "end": true, "except": true, "false": true, "fetch": true,
	"for": true, "foreign": true, "freeze": true, "from": true, "full": true,
	"grant": true, "group": true, "having": true, "ilike": true, "in": true,
	"initially": true, "inner": true, "intersect": true, "into": true,
	"is": true, "isnull": true, "join": true, "lateral": true,
	"leading": true, "left": true, "like": true, "limit": true,
	"localtime": true, "localtimestamp": true, "natural": true, "not": true,
	"notnull": true, "null": true, "offset": true, "on": true, "only": true,
	"or": true, "order": true, "outer": true, "overlaps": true,
	"placing": true, "primary": true, "references": true, "returning": true,
	"right": true, "select": true, "session_user": true, "similar": true,
	"some": true, "symmetric": true, "table": true, "tablesample": true,
	"then": true, "to": true, "trailing": true, "true": true, "union": true,
	"unique": true, "user": true, "using": true, "variadic": true,
	"verbose": true, "when": true, "where": true, "window": true, "with": true,
}

// QuoteReservedIdent wraps name in double quotes if it is a PostgreSQL
// reserved keyword, so tables and columns like "user" or "order" work in
// generated SQL. Non-reserved names are returned unchanged, which preserves
// PostgreSQL's case-folding semantics for everything that already works.
func QuoteReservedIdent(name string) string {
	if reservedIdents[strings.ToLower(name)] {
		return `"` + name + `"`
	}
	return name
}

// QuoteReservedIdents applies QuoteReservedIdent to every name in the slice,
// returning a new slice.
func QuoteReservedIdents(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = QuoteReservedIdent(n)
	}
	return out
}
