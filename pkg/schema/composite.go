package schema

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// CompositeType represents a PostgreSQL composite type
// Example: CREATE TYPE address AS (street text, city text, zipcode text);
type CompositeType interface {
	// SQLValue returns the SQL representation of the composite type
	// Format: (value1, value2, value3)
	SQLValue() string

	// Scan scans a PostgreSQL composite type into the Go struct
	Scan(value interface{}) error
}

// BaseComposite provides helper methods for composite types
type BaseComposite struct{}

// ParseComposite parses a PostgreSQL composite type string
// Format: "(value1,value2,value3)"
func ParseComposite(s string) []string {
	// Remove parentheses
	s = strings.TrimPrefix(s, "(")
	s = strings.TrimSuffix(s, ")")

	// Handle empty composite
	if s == "" {
		return []string{}
	}

	// Split by comma, handling quoted values
	var result []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for _, r := range s {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			inQuotes = !inQuotes
		case r == ',' && !inQuotes:
			result = append(result, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// FormatComposite formats values into a PostgreSQL composite type string
func FormatComposite(values ...interface{}) string {
	parts := make([]string, len(values))
	for i, v := range values {
		if v == nil {
			parts[i] = ""
			continue
		}

		switch val := v.(type) {
		case string:
			// Escape quotes and backslashes
			escaped := strings.ReplaceAll(val, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
			parts[i] = fmt.Sprintf("\"%s\"", escaped)
		default:
			parts[i] = fmt.Sprintf("%v", val)
		}
	}

	return fmt.Sprintf("(%s)", strings.Join(parts, ","))
}

// Example composite type implementation:
//
// type Address struct {
//     Street  string
//     City    string
//     ZipCode string
// }
//
// func (a Address) SQLValue() string {
//     return FormatComposite(a.Street, a.City, a.ZipCode)
// }
//
// func (a Address) Value() (driver.Value, error) {
//     return a.SQLValue(), nil
// }
//
// func (a *Address) Scan(value interface{}) error {
//     if value == nil {
//         return nil
//     }
//
//     str, ok := value.(string)
//     if !ok {
//         bytes, ok := value.([]byte)
//         if !ok {
//             return fmt.Errorf("failed to scan Address: expected string or []byte")
//         }
//         str = string(bytes)
//     }
//
//     parts := ParseComposite(str)
//     if len(parts) >= 1 {
//         a.Street = parts[0]
//     }
//     if len(parts) >= 2 {
//         a.City = parts[1]
//     }
//     if len(parts) >= 3 {
//         a.ZipCode = parts[2]
//     }
//
//     return nil
// }

// Point represents a PostgreSQL point type (x, y)
type Point struct {
	X float64
	Y float64
}

// SQLValue returns the SQL representation
func (p Point) SQLValue() string {
	return FormatComposite(p.X, p.Y)
}

// Value implements driver.Valuer
func (p Point) Value() (driver.Value, error) {
	return p.SQLValue(), nil
}

// Scan implements sql.Scanner
func (p *Point) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan Point: expected string or []byte")
		}
		str = string(bytes)
	}

	parts := ParseComposite(str)
	if len(parts) >= 2 {
		fmt.Sscanf(parts[0], "%f", &p.X)
		fmt.Sscanf(parts[1], "%f", &p.Y)
	}

	return nil
}

// Circle represents a PostgreSQL circle type (center point, radius)
type Circle struct {
	Center Point
	Radius float64
}

// SQLValue returns the SQL representation
func (c Circle) SQLValue() string {
	return fmt.Sprintf("<(%f,%f),%f>", c.Center.X, c.Center.Y, c.Radius)
}

// Value implements driver.Valuer
func (c Circle) Value() (driver.Value, error) {
	return c.SQLValue(), nil
}

// Scan implements sql.Scanner
func (c *Circle) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	str, ok := value.(string)
	if !ok {
		bytes, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("failed to scan Circle: expected string or []byte")
		}
		str = string(bytes)
	}

	// Parse format: <(x,y),r>
	fmt.Sscanf(str, "<(%f,%f),%f>", &c.Center.X, &c.Center.Y, &c.Radius)
	return nil
}
