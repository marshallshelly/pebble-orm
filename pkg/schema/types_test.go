package schema

import (
	"database/sql"
	"reflect"
	"testing"
	"time"
)

func TestTypeMapper_GoTypeToPostgreSQL(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name     string
		goType   reflect.Type
		expected string
	}{
		// Basic types
		{"bool", reflect.TypeFor[bool](), "boolean"},
		{"int8", reflect.TypeFor[int8](), "smallint"},
		{"int16", reflect.TypeFor[int16](), "smallint"},
		{"int32", reflect.TypeFor[int32](), "integer"},
		{"int", reflect.TypeFor[int](), "integer"},
		{"int64", reflect.TypeFor[int64](), "bigint"},
		{"uint8", reflect.TypeFor[uint8](), "smallint"},
		{"uint16", reflect.TypeFor[uint16](), "integer"},
		{"uint32", reflect.TypeFor[uint32](), "bigint"},
		{"uint64", reflect.TypeFor[uint64](), "bigint"},
		{"float32", reflect.TypeFor[float32](), "real"},
		{"float64", reflect.TypeFor[float64](), "double precision"},
		{"string", reflect.TypeFor[string](), "text"},

		// Special types
		{"time.Time", reflect.TypeFor[time.Time](), "timestamp with time zone"},
		{"[]byte", reflect.TypeFor[[]byte](), "bytea"},

		// Nullable types
		{"sql.NullString", reflect.TypeFor[sql.NullString](), "text"},
		{"sql.NullInt64", reflect.TypeFor[sql.NullInt64](), "bigint"},
		{"sql.NullInt32", reflect.TypeFor[sql.NullInt32](), "integer"},
		{"sql.NullFloat64", reflect.TypeFor[sql.NullFloat64](), "double precision"},
		{"sql.NullBool", reflect.TypeFor[sql.NullBool](), "boolean"},
		{"sql.NullTime", reflect.TypeFor[sql.NullTime](), "timestamp with time zone"},

		// Pointer types (should dereference)
		{"*string", reflect.TypeFor[*string](), "text"},
		{"*int", reflect.TypeFor[*int](), "integer"},
		{"*bool", reflect.TypeFor[*bool](), "boolean"},

		// Array types
		{"[]string", reflect.TypeFor[[]string](), "text[]"},
		{"[]int", reflect.TypeFor[[]int](), "integer[]"},
		{"[]bool", reflect.TypeFor[[]bool](), "boolean[]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.GoTypeToPostgreSQL(tt.goType)
			if result != tt.expected {
				t.Errorf("GoTypeToPostgreSQL(%s) = %s, want %s", tt.name, result, tt.expected)
			}
		})
	}
}

func TestTypeMapper_CustomMappings(t *testing.T) {
	tm := NewTypeMapper()

	type CustomID string
	customType := reflect.TypeFor[CustomID]()

	// Register custom mapping
	tm.RegisterType(customType, "uuid")

	result := tm.GoTypeToPostgreSQL(customType)
	if result != "uuid" {
		t.Errorf("expected custom mapping to return 'uuid', got '%s'", result)
	}
}

func TestIsNullable(t *testing.T) {
	tests := []struct {
		name     string
		goType   reflect.Type
		expected bool
	}{
		{"string", reflect.TypeFor[string](), false},
		{"int", reflect.TypeFor[int](), false},
		{"bool", reflect.TypeFor[bool](), false},

		// Pointer types are nullable
		{"*string", reflect.TypeFor[*string](), true},
		{"*int", reflect.TypeFor[*int](), true},
		{"*bool", reflect.TypeFor[*bool](), true},

		// sql.Null* types are nullable
		{"sql.NullString", reflect.TypeFor[sql.NullString](), true},
		{"sql.NullInt64", reflect.TypeFor[sql.NullInt64](), true},
		{"sql.NullInt32", reflect.TypeFor[sql.NullInt32](), true},
		{"sql.NullFloat64", reflect.TypeFor[sql.NullFloat64](), true},
		{"sql.NullBool", reflect.TypeFor[sql.NullBool](), true},
		{"sql.NullTime", reflect.TypeFor[sql.NullTime](), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNullable(tt.goType)
			if result != tt.expected {
				t.Errorf("IsNullable(%s) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestParsePostgreSQLType(t *testing.T) {
	tests := []struct {
		name     string
		typeStr  string
		expected PostgreSQLType
	}{
		{
			name:    "simple type",
			typeStr: "text",
			expected: PostgreSQLType{
				Name: "text",
			},
		},
		{
			name:    "array type",
			typeStr: "text[]",
			expected: PostgreSQLType{
				Name:     "text[]",
				IsArray:  true,
				BaseType: "text",
			},
		},
		{
			name:    "integer array",
			typeStr: "integer[]",
			expected: PostgreSQLType{
				Name:     "integer[]",
				IsArray:  true,
				BaseType: "integer",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParsePostgreSQLType(tt.typeStr)
			if result.Name != tt.expected.Name {
				t.Errorf("Name: expected %s, got %s", tt.expected.Name, result.Name)
			}
			if result.IsArray != tt.expected.IsArray {
				t.Errorf("IsArray: expected %v, got %v", tt.expected.IsArray, result.IsArray)
			}
			if result.BaseType != tt.expected.BaseType {
				t.Errorf("BaseType: expected %s, got %s", tt.expected.BaseType, result.BaseType)
			}
		})
	}
}
