package schema

import (
	"database/sql"
	"reflect"
	"time"
)

// TypeMapper handles mapping between Go types and PostgreSQL types.
type TypeMapper struct {
	customMappings map[reflect.Type]string
}

// NewTypeMapper creates a new TypeMapper instance.
func NewTypeMapper() *TypeMapper {
	return &TypeMapper{
		customMappings: make(map[reflect.Type]string),
	}
}

// RegisterType registers a custom type mapping.
func (tm *TypeMapper) RegisterType(goType reflect.Type, pgType string) {
	tm.customMappings[goType] = pgType
}

// GoTypeToPostgreSQL maps a Go type to its PostgreSQL equivalent.
// Returns empty string if custom type mapping should be used via tags.
func (tm *TypeMapper) GoTypeToPostgreSQL(t reflect.Type) string {
	// Check custom mappings first
	if pgType, ok := tm.customMappings[t]; ok {
		return pgType
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Standard type mappings
	switch t.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int8:
		return "smallint"
	case reflect.Int16:
		return "smallint"
	case reflect.Int32, reflect.Int:
		return "integer"
	case reflect.Int64:
		return "bigint"
	case reflect.Uint8:
		return "smallint"
	case reflect.Uint16:
		return "integer"
	case reflect.Uint32:
		return "bigint"
	case reflect.Uint64:
		return "bigint"
	case reflect.Float32:
		return "real"
	case reflect.Float64:
		return "double precision"
	case reflect.String:
		return "text"
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			// []byte
			return "bytea"
		}
		// Array types
		elemType := tm.GoTypeToPostgreSQL(t.Elem())
		if elemType != "" {
			return elemType + "[]"
		}
	case reflect.Map:
		// map[string]interface{} is commonly used for JSONB
		if t.Key().Kind() == reflect.String && t.Elem().Kind() == reflect.Interface {
			return "jsonb"
		}
	}

	// Handle special types
	switch t {
	case reflect.TypeOf(time.Time{}):
		return "timestamp with time zone"
	case reflect.TypeOf(JSONB{}):
		return "jsonb"
	case reflect.TypeOf(JSONBArray{}):
		return "jsonb"
	case reflect.TypeOf(sql.NullString{}):
		return "text"
	case reflect.TypeOf(sql.NullInt64{}):
		return "bigint"
	case reflect.TypeOf(sql.NullInt32{}):
		return "integer"
	case reflect.TypeOf(sql.NullFloat64{}):
		return "double precision"
	case reflect.TypeOf(sql.NullBool{}):
		return "boolean"
	case reflect.TypeOf(sql.NullTime{}):
		return "timestamp with time zone"
	}

	// Default for unknown types
	return ""
}

// IsNullable checks if a Go type is nullable.
func IsNullable(t reflect.Type) bool {
	// Pointer types are nullable
	if t.Kind() == reflect.Ptr {
		return true
	}

	// sql.Null* types are nullable
	switch t {
	case reflect.TypeOf(sql.NullString{}),
		reflect.TypeOf(sql.NullInt64{}),
		reflect.TypeOf(sql.NullInt32{}),
		reflect.TypeOf(sql.NullFloat64{}),
		reflect.TypeOf(sql.NullBool{}),
		reflect.TypeOf(sql.NullTime{}):
		return true
	}

	return false
}

// PostgreSQLType represents a PostgreSQL data type with its properties.
type PostgreSQLType struct {
	Name     string
	Size     int // For varchar(n), numeric(p,s) precision
	Scale    int // For numeric(p,s) scale
	IsArray  bool
	BaseType string // For array types
}

// ParsePostgreSQLType parses a PostgreSQL type string into structured data.
func ParsePostgreSQLType(typeStr string) PostgreSQLType {
	// This is a simplified parser. A full implementation would use regex or a proper parser.
	pgType := PostgreSQLType{
		Name: typeStr,
	}

	// Handle array types
	if len(typeStr) > 2 && typeStr[len(typeStr)-2:] == "[]" {
		pgType.IsArray = true
		pgType.BaseType = typeStr[:len(typeStr)-2]
	}

	return pgType
}

// DefaultTypeMapper is the global type mapper instance.
var DefaultTypeMapper = NewTypeMapper()
