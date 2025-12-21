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
		{"bool", reflect.TypeOf(true), "boolean"},
		{"int8", reflect.TypeOf(int8(0)), "smallint"},
		{"int16", reflect.TypeOf(int16(0)), "smallint"},
		{"int32", reflect.TypeOf(int32(0)), "integer"},
		{"int", reflect.TypeOf(int(0)), "integer"},
		{"int64", reflect.TypeOf(int64(0)), "bigint"},
		{"uint8", reflect.TypeOf(uint8(0)), "smallint"},
		{"uint16", reflect.TypeOf(uint16(0)), "integer"},
		{"uint32", reflect.TypeOf(uint32(0)), "bigint"},
		{"uint64", reflect.TypeOf(uint64(0)), "bigint"},
		{"float32", reflect.TypeOf(float32(0)), "real"},
		{"float64", reflect.TypeOf(float64(0)), "double precision"},
		{"string", reflect.TypeOf(""), "text"},

		// Special types
		{"time.Time", reflect.TypeOf(time.Time{}), "timestamp with time zone"},
		{"[]byte", reflect.TypeOf([]byte{}), "bytea"},

		// Nullable types
		{"sql.NullString", reflect.TypeOf(sql.NullString{}), "text"},
		{"sql.NullInt64", reflect.TypeOf(sql.NullInt64{}), "bigint"},
		{"sql.NullInt32", reflect.TypeOf(sql.NullInt32{}), "integer"},
		{"sql.NullFloat64", reflect.TypeOf(sql.NullFloat64{}), "double precision"},
		{"sql.NullBool", reflect.TypeOf(sql.NullBool{}), "boolean"},
		{"sql.NullTime", reflect.TypeOf(sql.NullTime{}), "timestamp with time zone"},

		// Pointer types (should dereference)
		{"*string", reflect.TypeOf(new(string)), "text"},
		{"*int", reflect.TypeOf(new(int)), "integer"},
		{"*bool", reflect.TypeOf(new(bool)), "boolean"},

		// Array types
		{"[]string", reflect.TypeOf([]string{}), "text[]"},
		{"[]int", reflect.TypeOf([]int{}), "integer[]"},
		{"[]bool", reflect.TypeOf([]bool{}), "boolean[]"},
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
	customType := reflect.TypeOf(CustomID(""))

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
		{"string", reflect.TypeOf(""), false},
		{"int", reflect.TypeOf(0), false},
		{"bool", reflect.TypeOf(false), false},

		// Pointer types are nullable
		{"*string", reflect.TypeOf(new(string)), true},
		{"*int", reflect.TypeOf(new(int)), true},
		{"*bool", reflect.TypeOf(new(bool)), true},

		// sql.Null* types are nullable
		{"sql.NullString", reflect.TypeOf(sql.NullString{}), true},
		{"sql.NullInt64", reflect.TypeOf(sql.NullInt64{}), true},
		{"sql.NullInt32", reflect.TypeOf(sql.NullInt32{}), true},
		{"sql.NullFloat64", reflect.TypeOf(sql.NullFloat64{}), true},
		{"sql.NullBool", reflect.TypeOf(sql.NullBool{}), true},
		{"sql.NullTime", reflect.TypeOf(sql.NullTime{}), true},
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
