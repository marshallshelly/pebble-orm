package loader

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func TestBuildTableMetadataFromAST_UniqueConstraints(t *testing.T) {
	// Build a struct AST with a unique column: `po:"email,varchar(320),unique,notNull"`
	fields := &ast.FieldList{
		List: []*ast.Field{
			{
				Names: []*ast.Ident{{Name: "ID"}},
				Type:  &ast.Ident{Name: "string"},
				Tag:   &ast.BasicLit{Kind: token.STRING, Value: "`po:\"id,primaryKey,uuid\"`"},
			},
			{
				Names: []*ast.Ident{{Name: "Email"}},
				Type:  &ast.Ident{Name: "string"},
				Tag:   &ast.BasicLit{Kind: token.STRING, Value: "`po:\"email,varchar(320),unique,notNull\"`"},
			},
			{
				Names: []*ast.Ident{{Name: "Name"}},
				Type:  &ast.Ident{Name: "string"},
				Tag:   &ast.BasicLit{Kind: token.STRING, Value: "`po:\"name,varchar(255),notNull\"`"},
			},
		},
	}
	structType := &ast.StructType{Fields: fields}

	table := buildTableMetadataFromAST("users", structType)

	// Verify the email column is marked unique
	var emailCol *schema.ColumnMetadata
	for i, col := range table.Columns {
		if col.Name == "email" {
			emailCol = &table.Columns[i]
			break
		}
	}
	if emailCol == nil {
		t.Fatal("email column not found")
	}
	if !emailCol.Unique {
		t.Error("email column should be marked unique")
	}

	// Verify a UNIQUE constraint was created
	if len(table.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(table.Constraints))
	}

	c := table.Constraints[0]
	if c.Name != "users_email_key" {
		t.Errorf("constraint name = %q, want %q", c.Name, "users_email_key")
	}
	if c.Type != schema.UniqueConstraint {
		t.Errorf("constraint type = %q, want %q", c.Type, schema.UniqueConstraint)
	}
	if len(c.Columns) != 1 || c.Columns[0] != "email" {
		t.Errorf("constraint columns = %v, want [email]", c.Columns)
	}
}

func TestBuildTableMetadataFromAST_NoUniqueNoConstraint(t *testing.T) {
	// A struct with no unique columns should produce no constraints
	fields := &ast.FieldList{
		List: []*ast.Field{
			{
				Names: []*ast.Ident{{Name: "ID"}},
				Type:  &ast.Ident{Name: "int"},
				Tag:   &ast.BasicLit{Kind: token.STRING, Value: "`po:\"id,primaryKey,serial\"`"},
			},
			{
				Names: []*ast.Ident{{Name: "Name"}},
				Type:  &ast.Ident{Name: "string"},
				Tag:   &ast.BasicLit{Kind: token.STRING, Value: "`po:\"name,varchar(255),notNull\"`"},
			},
		},
	}
	structType := &ast.StructType{Fields: fields}

	table := buildTableMetadataFromAST("items", structType)

	if len(table.Constraints) != 0 {
		t.Errorf("expected 0 constraints, got %d", len(table.Constraints))
	}
}

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
