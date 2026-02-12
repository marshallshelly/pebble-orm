package schema

import (
	"maps"
	"reflect"
	"testing"
)

// TestGlobalTableNameRegistry tests the RegisterTableName functionality
func TestGlobalTableNameRegistry(t *testing.T) {
	// Save original state
	originalNames := make(map[string]string)
	maps.Copy(originalNames, customTableNames)

	// Clear registry for clean test
	customTableNames = make(map[string]string)
	defer func() {
		customTableNames = originalNames
	}()

	// Register custom table names (simulating generated code)
	RegisterTableName("TestModel", "test_models")
	RegisterTableName("AnotherModel", "custom_another")

	// Verify registration
	if customTableNames["TestModel"] != "test_models" {
		t.Errorf("Expected 'test_models', got '%s'", customTableNames["TestModel"])
	}

	if customTableNames["AnotherModel"] != "custom_another" {
		t.Errorf("Expected 'custom_another', got '%s'", customTableNames["AnotherModel"])
	}
}

// TestExtractTableNameWithRegistry tests that registered names take priority
func TestExtractTableNameWithRegistry(t *testing.T) {
	// Save and clear registry
	originalNames := make(map[string]string)
	maps.Copy(originalNames, customTableNames)
	customTableNames = make(map[string]string)
	defer func() {
		customTableNames = originalNames
	}()

	// Define a test model
	type RegisteredModel struct {
		ID int `po:"id,primaryKey,serial"`
	}

	type UnregisteredModel struct {
		ID int `po:"id,primaryKey,serial"`
	}

	// Register one model
	RegisterTableName("RegisteredModel", "registered_table")

	parser := NewParser()

	// Test registered model
	t.Run("Registered model uses custom name", func(t *testing.T) {
		table, err := parser.Parse(reflect.TypeFor[RegisteredModel]())
		if err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		if table.Name != "registered_table" {
			t.Errorf("Expected 'registered_table', got '%s'", table.Name)
		}
	})

	// Test unregistered model
	t.Run("Unregistered model uses snake_case", func(t *testing.T) {
		table, err := parser.Parse(reflect.TypeFor[UnregisteredModel]())
		if err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		expected := "unregistered_model"
		if table.Name != expected {
			t.Errorf("Expected '%s', got '%s'", expected, table.Name)
		}
	})
}

// TestGeneratedCodeScenario simulates the workflow of generated code
func TestGeneratedCodeScenario(t *testing.T) {
	// Save and clear registry
	originalNames := make(map[string]string)
	maps.Copy(originalNames, customTableNames)
	customTableNames = make(map[string]string)
	defer func() {
		customTableNames = originalNames
	}()

	// Simulate generated code from `pebble generate metadata`
	// This would be in table_names.gen.go:
	//   func init() {
	//       schema.RegisterTableName("Tenant", "tenants")
	//       schema.RegisterTableName("TenantUser", "tenant_users")
	//   }
	RegisterTableName("Tenant", "tenants")
	RegisterTableName("TenantUser", "tenant_users")

	// Define models (without comment directives)
	type Tenant struct {
		ID int `po:"id,primaryKey,serial"`
	}

	type TenantUser struct {
		ID int `po:"id,primaryKey,serial"`
	}

	parser := NewParser()

	// Test that registered names are used
	t.Run("Tenant uses registered name", func(t *testing.T) {
		table, err := parser.Parse(reflect.TypeFor[Tenant]())
		if err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		if table.Name != "tenants" {
			t.Errorf("Expected 'tenants' from registry, got '%s'", table.Name)
			t.Error("❌ Production build would create wrong table names!")
		} else {
			t.Log("✅ Registered table name works (production-safe)")
		}
	})

	t.Run("TenantUser uses registered name", func(t *testing.T) {
		table, err := parser.Parse(reflect.TypeFor[TenantUser]())
		if err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		if table.Name != "tenant_users" {
			t.Errorf("Expected 'tenant_users' from registry, got '%s'", table.Name)
		} else {
			t.Log("✅ Registered table name works (production-safe)")
		}
	})
}
