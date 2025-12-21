package builder

import "fmt"

// PostgreSQL-specific functions and operators

// JSONBOperators provides JSONB-specific query building methods

// JSONBContains checks if left JSONB contains right JSONB
func JSONBContains(column string, value interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: "@>",
		Value:    value,
	}
}

// JSONBContainedBy checks if left JSONB is contained by right JSONB
func JSONBContainedBy(column string, value interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: "<@",
		Value:    value,
	}
}

// JSONBHasKey checks if JSONB contains a specific key
func JSONBHasKey(column string, key string) Condition {
	return Condition{
		Column:   column,
		Operator: "?",
		Value:    key,
	}
}

// JSONBHasAnyKey checks if JSONB contains any of the specified keys
func JSONBHasAnyKey(column string, keys []string) Condition {
	return Condition{
		Column:   column,
		Operator: "?|",
		Value:    keys,
	}
}

// JSONBHasAllKeys checks if JSONB contains all of the specified keys
func JSONBHasAllKeys(column string, keys []string) Condition {
	return Condition{
		Column:   column,
		Operator: "?&",
		Value:    keys,
	}
}

// JSONBPath extracts value at specified path
// Usage: JSONBPath("data", "user", "name") -> data->'user'->'name'
func JSONBPath(column string, path ...string) string {
	result := column
	for _, p := range path {
		result += fmt.Sprintf("->'%s'", p)
	}
	return result
}

// JSONBPathText extracts value at specified path as text
// Usage: JSONBPathText("data", "user", "name") -> data->'user'->>'name'
func JSONBPathText(column string, path ...string) string {
	if len(path) == 0 {
		return column
	}
	result := column
	for i, p := range path {
		if i == len(path)-1 {
			result += fmt.Sprintf("->>'%s'", p)
		} else {
			result += fmt.Sprintf("->'%s'", p)
		}
	}
	return result
}

// Array Operators

// ArrayContains checks if array contains value
func ArrayContains(column string, value interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: "@>",
		Value:    value,
	}
}

// ArrayContainedBy checks if array is contained by value
func ArrayContainedBy(column string, value interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: "<@",
		Value:    value,
	}
}

// ArrayOverlap checks if arrays have common elements
func ArrayOverlap(column string, value interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: "&&",
		Value:    value,
	}
}

// ArrayLength returns the length of an array
func ArrayLength(column string, dimension int) string {
	return fmt.Sprintf("array_length(%s, %d)", column, dimension)
}

// PostgreSQL String Functions

// ILike is case-insensitive LIKE (already defined in where.go, but documented here)
// Usage: ILike("name", "%john%")

// RegexpMatch checks if string matches regex pattern
func RegexpMatch(column string, pattern string) Condition {
	return Condition{
		Column:   column,
		Operator: "~",
		Value:    pattern,
	}
}

// RegexpMatchInsensitive checks if string matches regex pattern (case-insensitive)
func RegexpMatchInsensitive(column string, pattern string) Condition {
	return Condition{
		Column:   column,
		Operator: "~*",
		Value:    pattern,
	}
}

// RegexpNotMatch checks if string doesn't match regex pattern
func RegexpNotMatch(column string, pattern string) Condition {
	return Condition{
		Column:   column,
		Operator: "!~",
		Value:    pattern,
	}
}

// PostgreSQL Full-Text Search

// ToTSVector converts text to tsvector for full-text search
func ToTSVector(column string) string {
	return fmt.Sprintf("to_tsvector(%s)", column)
}

// ToTSQuery converts text to tsquery for full-text search
func ToTSQuery(query string) string {
	return fmt.Sprintf("to_tsquery('%s')", query)
}

// TSMatch performs full-text search match
func TSMatch(column string, query string) Condition {
	return Condition{
		Column:   fmt.Sprintf("to_tsvector(%s)", column),
		Operator: "@@",
		Value:    fmt.Sprintf("to_tsquery('%s')", query),
		Raw:      true, // Don't parameterize the value
	}
}

// PostgreSQL Aggregate Functions (for use in SELECT)

// JSONBAgg aggregates values into a JSONB array
func JSONBAgg(column string) string {
	return fmt.Sprintf("jsonb_agg(%s)", column)
}

// JSONBObjectAgg aggregates key-value pairs into a JSONB object
func JSONBObjectAgg(keyColumn, valueColumn string) string {
	return fmt.Sprintf("jsonb_object_agg(%s, %s)", keyColumn, valueColumn)
}

// ArrayAgg aggregates values into an array
func ArrayAgg(column string) string {
	return fmt.Sprintf("array_agg(%s)", column)
}

// StringAgg concatenates strings with a delimiter
func StringAgg(column, delimiter string) string {
	return fmt.Sprintf("string_agg(%s, '%s')", column, delimiter)
}

// PostgreSQL Date/Time Functions

// DateTrunc truncates timestamp to specified precision
func DateTrunc(precision, column string) string {
	return fmt.Sprintf("date_trunc('%s', %s)", precision, column)
}

// Age calculates age between timestamps
func Age(timestamp1, timestamp2 string) string {
	return fmt.Sprintf("age(%s, %s)", timestamp1, timestamp2)
}

// Extract extracts field from timestamp
func Extract(field, column string) string {
	return fmt.Sprintf("extract(%s from %s)", field, column)
}

// PostgreSQL Math Functions

// Ceiling returns ceiling of a number
func Ceiling(column string) string {
	return fmt.Sprintf("ceiling(%s)", column)
}

// Floor returns floor of a number
func Floor(column string) string {
	return fmt.Sprintf("floor(%s)", column)
}

// Round rounds a number to specified decimal places
func Round(column string, decimals int) string {
	return fmt.Sprintf("round(%s, %d)", column, decimals)
}

// Abs returns absolute value
func Abs(column string) string {
	return fmt.Sprintf("abs(%s)", column)
}
