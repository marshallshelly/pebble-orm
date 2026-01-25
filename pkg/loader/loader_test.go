package loader

import (
	"testing"
)

func TestGetSQLTypeFromOptions_JSONB(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "jsonb type",
			tag:      "settings,jsonb",
			expected: "jsonb",
		},
		{
			name:     "json type",
			tag:      "data,json",
			expected: "json",
		},
		{
			name:     "jsonb with notNull",
			tag:      "metadata,jsonb,notNull",
			expected: "jsonb",
		},
		{
			name:     "json with notNull",
			tag:      "metadata,json,notNull",
			expected: "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := parseTag(tt.tag)
			if opts == nil {
				t.Fatalf("parseTag returned nil for tag %q", tt.tag)
			}

			got := getSQLTypeFromOptions(opts)
			if got != tt.expected {
				t.Errorf("getSQLTypeFromOptions() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGetSQLTypeFromOptions_TimestampTZ(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "timestamptz type",
			tag:      "created_at,timestamptz",
			expected: "timestamptz",
		},
		{
			name:     "timestamp type",
			tag:      "created_at,timestamp",
			expected: "timestamp",
		},
		{
			name:     "timestamptz with default",
			tag:      "created_at,timestamptz,default(NOW())",
			expected: "timestamptz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := parseTag(tt.tag)
			if opts == nil {
				t.Fatalf("parseTag returned nil for tag %q", tt.tag)
			}

			got := getSQLTypeFromOptions(opts)
			if got != tt.expected {
				t.Errorf("getSQLTypeFromOptions() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGetSQLTypeFromOptions_Serial(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "bigserial type",
			tag:      "id,bigserial,primaryKey",
			expected: "bigserial",
		},
		{
			name:     "serial type",
			tag:      "id,serial,primaryKey",
			expected: "serial",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := parseTag(tt.tag)
			if opts == nil {
				t.Fatalf("parseTag returned nil for tag %q", tt.tag)
			}

			got := getSQLTypeFromOptions(opts)
			if got != tt.expected {
				t.Errorf("getSQLTypeFromOptions() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGetSQLTypeFromOptions_AllTypes(t *testing.T) {
	// Test all supported types to ensure they work correctly
	types := []string{
		"uuid", "varchar", "text", "char",
		"smallint", "integer", "bigint", "bigserial", "serial",
		"numeric", "decimal", "real", "double precision",
		"boolean", "bool",
		"timestamptz", "timestamp", "date", "time", "interval",
		"jsonb", "json",
		"bytea",
	}

	for _, sqlType := range types {
		t.Run(sqlType, func(t *testing.T) {
			tag := "column," + sqlType
			opts := parseTag(tag)
			if opts == nil {
				t.Fatalf("parseTag returned nil for tag %q", tag)
			}

			got := getSQLTypeFromOptions(opts)
			if got != sqlType {
				t.Errorf("getSQLTypeFromOptions() = %q, want %q", got, sqlType)
			}
		})
	}
}

func TestGetSQLTypeFromOptions_WithSize(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "varchar with size",
			tag:      "email,varchar(255),notNull",
			expected: "varchar(255)",
		},
		{
			name:     "numeric with precision and scale",
			tag:      "amount,numeric(10,2)",
			expected: "numeric(10,2)",
		},
		{
			name:     "char with size",
			tag:      "code,char(3)",
			expected: "char(3)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := parseTag(tt.tag)
			if opts == nil {
				t.Fatalf("parseTag returned nil for tag %q", tt.tag)
			}

			got := getSQLTypeFromOptions(opts)
			if got != tt.expected {
				t.Errorf("getSQLTypeFromOptions() = %q, want %q", got, tt.expected)
			}
		})
	}
}
