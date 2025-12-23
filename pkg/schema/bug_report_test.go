package schema

import (
	"reflect"
	"testing"
)

// Test models for bug report validation
// These MUST be at package level for comment directives to be accessible

// table_name: tenants
type BugReportTenant struct {
	ID   int    `po:"id,primaryKey,serial"`
	Name string `po:"name,varchar(255),unique,notNull"`
}

// table_name: tenant_users
type BugReportTenantUser struct {
	ID       int `po:"id,primaryKey,serial"`
	TenantID int `po:"tenant_id,integer,notNull"`
}

// TestBugReport_TableNameCommentDirective validates the fix for the reported bug
// where table_name comments were not being parsed, causing migrations to create
// singular table names instead of custom plural names.
func TestBugReport_TableNameCommentDirective(t *testing.T) {
	parser := NewParser()

	//Test 1: Parse BugReportTenant - should get "tenants" from comment, not "bug_report_tenant"
	t.Run("BugReportTenant should parse to 'tenants' table", func(t *testing.T) {
		table, err := parser.Parse(reflect.TypeOf(BugReportTenant{}))
		if err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		expected := "tenants"
		if table.Name != expected {
			t.Errorf("Expected table name '%s', got '%s' (bug is NOT fixed)", expected, table.Name)
		} else {
			t.Logf("✅ Custom table name works: BugReportTenant → %s", table.Name)
		}

		// Make sure it's NOT using the fallback snake_case name
		if table.Name == "bug_report_tenant" {
			t.Error("Table name is using fallback snake_case - comment directive was ignored!")
		}
	})

	// Test 2: Parse BugReportTenantUser - should get "tenant_users" from comment
	t.Run("BugReportTenantUser should parse to 'tenant_users' table", func(t *testing.T) {
		table, err := parser.Parse(reflect.TypeOf(BugReportTenantUser{}))
		if err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		expected := "tenant_users"
		if table.Name != expected {
			t.Errorf("Expected table name '%s', got '%s' (bug is NOT fixed)", expected, table.Name)
		} else {
			t.Logf("✅ Custom table name works: BugReportTenantUser → %s", table.Name)
		}

		// Make sure it's NOT using the fallback snake_case name
		if table.Name == "bug_report_tenant_user" {
			t.Error("Table name is using fallback snake_case - comment directive was ignored!")
		}
	})

	// Test 3: Verify column parsing still works
	t.Run("Columns should be parsed correctly", func(t *testing.T) {
		table, _ := parser.Parse(reflect.TypeOf(BugReportTenant{}))

		if len(table.Columns) != 2 {
			t.Errorf("Expected 2 columns, got %d", len(table.Columns))
		}

		// Check if primary key is set
		if table.PrimaryKey == nil {
			t.Error("Primary key not set")
		}
	})
}

// Helper function to get table names for debugging
func getTableNames(tables map[string]interface{}) []string {
	names := make([]string, 0, len(tables))
	for name := range tables {
		names = append(names, name)
	}
	return names
}

// TestTableNameCommentParsing validates the comment parsing regex
func TestTableNameCommentParsing(t *testing.T) {
	tests := []struct {
		name     string
		comment  string
		expected string
	}{
		{
			name:     "Standard format",
			comment:  "// table_name: tenants",
			expected: "tenants",
		},
		{
			name:     "Extra spaces",
			comment:  "// table_name:   tenant_users   ",
			expected: "tenant_users",
		},
		{
			name:     "Block comment",
			comment:  "/* table_name: custom_table */",
			expected: "custom_table",
		},
		{
			name:     "No directive",
			comment:  "// This is just a regular comment",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTableNameFromComment(tt.comment)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
