package schema

import (
	"bytes"
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// StringArray is a custom type for PostgreSQL text[] arrays that handles
// both binary and text format scanning. This is particularly useful when
// using simple_protocol mode (required for PgBouncer transaction pooling),
// where PostgreSQL returns arrays as text format strings like {a,b,c}.
//
// Usage:
//
//	type Schedule struct {
//	    ID   int                `po:"id,primaryKey,serial"`
//	    Days schema.StringArray `po:"days,text[]"`
//	}
type StringArray []string

// Value implements driver.Valuer for database writes.
// Formats the array as a PostgreSQL array literal: {value1,value2,value3}
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return formatPostgresArray(a), nil
}

// Scan implements sql.Scanner for database reads.
// Handles both text format (simple_protocol) and native pgx array scanning.
func (a *StringArray) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}

	switch v := src.(type) {
	case []byte:
		return a.scanString(string(v))
	case string:
		return a.scanString(v)
	case []string:
		// pgx native array scanning in extended protocol mode
		*a = v
		return nil
	default:
		return fmt.Errorf("StringArray.Scan: cannot scan %T into StringArray", src)
	}
}

func (a *StringArray) scanString(s string) error {
	elements, err := parsePostgresArray(s)
	if err != nil {
		return fmt.Errorf("StringArray.Scan: %w", err)
	}
	*a = elements
	return nil
}

// Int32Array is a custom type for PostgreSQL integer[] arrays.
type Int32Array []int32

// Value implements driver.Valuer for database writes.
func (a Int32Array) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	strs := make([]string, len(a))
	for i, v := range a {
		strs[i] = strconv.FormatInt(int64(v), 10)
	}
	return formatPostgresArray(strs), nil
}

// Scan implements sql.Scanner for database reads.
func (a *Int32Array) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}

	switch v := src.(type) {
	case []byte:
		return a.scanString(string(v))
	case string:
		return a.scanString(v)
	case []int32:
		*a = v
		return nil
	default:
		return fmt.Errorf("Int32Array.Scan: cannot scan %T into Int32Array", src)
	}
}

func (a *Int32Array) scanString(s string) error {
	elements, err := parsePostgresArray(s)
	if err != nil {
		return fmt.Errorf("Int32Array.Scan: %w", err)
	}

	result := make([]int32, len(elements))
	for i, elem := range elements {
		val, err := strconv.ParseInt(elem, 10, 32)
		if err != nil {
			return fmt.Errorf("Int32Array.Scan: invalid integer %q: %w", elem, err)
		}
		result[i] = int32(val)
	}
	*a = result
	return nil
}

// Int64Array is a custom type for PostgreSQL bigint[] arrays.
type Int64Array []int64

// Value implements driver.Valuer for database writes.
func (a Int64Array) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	strs := make([]string, len(a))
	for i, v := range a {
		strs[i] = strconv.FormatInt(v, 10)
	}
	return formatPostgresArray(strs), nil
}

// Scan implements sql.Scanner for database reads.
func (a *Int64Array) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}

	switch v := src.(type) {
	case []byte:
		return a.scanString(string(v))
	case string:
		return a.scanString(v)
	case []int64:
		*a = v
		return nil
	default:
		return fmt.Errorf("Int64Array.Scan: cannot scan %T into Int64Array", src)
	}
}

func (a *Int64Array) scanString(s string) error {
	elements, err := parsePostgresArray(s)
	if err != nil {
		return fmt.Errorf("Int64Array.Scan: %w", err)
	}

	result := make([]int64, len(elements))
	for i, elem := range elements {
		val, err := strconv.ParseInt(elem, 10, 64)
		if err != nil {
			return fmt.Errorf("Int64Array.Scan: invalid integer %q: %w", elem, err)
		}
		result[i] = val
	}
	*a = result
	return nil
}

// Float64Array is a custom type for PostgreSQL double precision[] arrays.
type Float64Array []float64

// Value implements driver.Valuer for database writes.
func (a Float64Array) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	strs := make([]string, len(a))
	for i, v := range a {
		strs[i] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return formatPostgresArray(strs), nil
}

// Scan implements sql.Scanner for database reads.
func (a *Float64Array) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}

	switch v := src.(type) {
	case []byte:
		return a.scanString(string(v))
	case string:
		return a.scanString(v)
	case []float64:
		*a = v
		return nil
	default:
		return fmt.Errorf("Float64Array.Scan: cannot scan %T into Float64Array", src)
	}
}

func (a *Float64Array) scanString(s string) error {
	elements, err := parsePostgresArray(s)
	if err != nil {
		return fmt.Errorf("Float64Array.Scan: %w", err)
	}

	result := make([]float64, len(elements))
	for i, elem := range elements {
		val, err := strconv.ParseFloat(elem, 64)
		if err != nil {
			return fmt.Errorf("Float64Array.Scan: invalid float %q: %w", elem, err)
		}
		result[i] = val
	}
	*a = result
	return nil
}

// BoolArray is a custom type for PostgreSQL boolean[] arrays.
type BoolArray []bool

// Value implements driver.Valuer for database writes.
func (a BoolArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	strs := make([]string, len(a))
	for i, v := range a {
		if v {
			strs[i] = "t"
		} else {
			strs[i] = "f"
		}
	}
	return formatPostgresArray(strs), nil
}

// Scan implements sql.Scanner for database reads.
func (a *BoolArray) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}

	switch v := src.(type) {
	case []byte:
		return a.scanString(string(v))
	case string:
		return a.scanString(v)
	case []bool:
		*a = v
		return nil
	default:
		return fmt.Errorf("BoolArray.Scan: cannot scan %T into BoolArray", src)
	}
}

func (a *BoolArray) scanString(s string) error {
	elements, err := parsePostgresArray(s)
	if err != nil {
		return fmt.Errorf("BoolArray.Scan: %w", err)
	}

	result := make([]bool, len(elements))
	for i, elem := range elements {
		switch strings.ToLower(elem) {
		case "t", "true", "1", "yes", "on":
			result[i] = true
		case "f", "false", "0", "no", "off":
			result[i] = false
		default:
			return fmt.Errorf("BoolArray.Scan: invalid boolean %q", elem)
		}
	}
	*a = result
	return nil
}

// parsePostgresArray parses a PostgreSQL array text representation.
// Format: {value1,value2,value3} with support for:
// - Quoted elements: {"hello world","foo"}
// - Escaped characters: {"he said \"hi\""}
// - NULL values (unquoted NULL)
// - Empty arrays: {}
//
// This implementation is based on pgx's parseUntypedTextArray but simplified
// for single-dimensional arrays which covers most use cases.
func parsePostgresArray(s string) ([]string, error) {
	s = strings.TrimSpace(s)

	// Handle empty input
	if s == "" {
		return nil, nil
	}

	// Must start and end with braces
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, fmt.Errorf("invalid array format: must be enclosed in braces")
	}

	// Remove outer braces
	s = s[1 : len(s)-1]

	// Handle empty array
	if s == "" {
		return []string{}, nil
	}

	var elements []string
	var current bytes.Buffer
	inQuotes := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}

		switch ch {
		case '\\':
			escaped = true
		case '"':
			inQuotes = !inQuotes
		case ',':
			if inQuotes {
				current.WriteByte(ch)
			} else {
				elem := current.String()
				// Handle NULL (unquoted NULL is SQL NULL, but we treat as empty string for simplicity)
				if elem == "NULL" {
					elem = ""
				}
				elements = append(elements, elem)
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	// Don't forget the last element
	elem := current.String()
	if elem == "NULL" {
		elem = ""
	}
	elements = append(elements, elem)

	return elements, nil
}

// formatPostgresArray formats a string slice as a PostgreSQL array literal.
// Values are quoted if they contain special characters.
func formatPostgresArray(elements []string) string {
	if len(elements) == 0 {
		return "{}"
	}

	var buf bytes.Buffer
	buf.WriteByte('{')

	for i, elem := range elements {
		if i > 0 {
			buf.WriteByte(',')
		}

		// Quote if contains special characters or is empty
		needsQuote := elem == "" ||
			strings.ContainsAny(elem, `{},"\`) ||
			strings.EqualFold(elem, "null") ||
			containsWhitespace(elem)

		if needsQuote {
			buf.WriteByte('"')
			// Escape backslashes and quotes
			for j := 0; j < len(elem); j++ {
				ch := elem[j]
				if ch == '\\' || ch == '"' {
					buf.WriteByte('\\')
				}
				buf.WriteByte(ch)
			}
			buf.WriteByte('"')
		} else {
			buf.WriteString(elem)
		}
	}

	buf.WriteByte('}')
	return buf.String()
}

func containsWhitespace(s string) bool {
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f' {
			return true
		}
	}
	return false
}
