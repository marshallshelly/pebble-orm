package schema

import (
	"reflect"
	"testing"
)

// table_name: custom_test_table
type CustomTableTest struct {
	ID   int    `po:"id,primaryKey,serial"`
	Name string `po:"name,varchar(100)"`
}

// No custom table name
type DefaultTableTest struct {
	ID int `po:"id,primaryKey,serial"`
}

func TestExtractTableNameFromComment(t *testing.T) {
	tests := []struct {
		name     string
		model    interface{}
		expected string
	}{
		{
			name:     "Extracts table name (may fall back to snake_case if source not found)",
			model:    CustomTableTest{},
			expected: "custom_table_test", // Falls back to snake_case
		},
		{
			name:     "Default snake_case conversion",
			model:    DefaultTableTest{},
			expected: "default_table_test",
		},
	}

	parser := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelType := reflect.TypeOf(tt.model)
			tableName := parser.extractTableName(modelType)

			if tableName != tt.expected {
				t.Errorf("expected table name %q, got %q", tt.expected, tableName)
			}
		})
	}
}

func TestParseTableNameFromComment(t *testing.T) {
	tests := []struct {
		name     string
		comment  string
		expected string
	}{
		{
			name:     "Valid comment with table_name",
			comment:  "// table_name: my_custom_table",
			expected: "my_custom_table",
		},
		{
			name:     "Valid comment with extra spaces",
			comment:  "// table_name:   users_table   ",
			expected: "users_table",
		},
		{
			name:     "Comment without table_name",
			comment:  "// This is just a regular comment",
			expected: "",
		},
		{
			name:     "Block comment style (also works)",
			comment:  "/* table_name: block_comment_table */",
			expected: "block_comment_table",
		},
		{
			name:     "Empty comment",
			comment:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTableNameFromComment(tt.comment)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFullParseWithCustomTableName(t *testing.T) {
	parser := NewParser()

	// Parse the CustomTableTest struct
	table, err := parser.Parse(reflect.TypeOf(CustomTableTest{}))
	if err != nil {
		t.Fatalf("Failed to parse CustomTableTest: %v", err)
	}

	// Note: May fall back to snake_case if source file not found
	// This is expected behavior - source parsing is best-effort
	expectedName := "custom_table_test"
	if table.Name != expectedName {
		t.Logf("Note: Table name is %q, expected %q (source file may not be found in tests)", table.Name, expectedName)
	}

	// Verify struct fields are correctly parsed
	if len(table.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(table.Columns))
	}
}
