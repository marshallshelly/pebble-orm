package migration

import (
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func TestEnumTypeDetection(t *testing.T) {
	t.Run("add enum type", func(t *testing.T) {
		// Setup: Database has no enum types
		dbSchema := map[string]*schema.TableMetadata{
			"orders": {
				Name: "orders",
				Columns: []schema.ColumnMetadata{
					{Name: "id", SQLType: "serial"},
					{Name: "status", SQLType: "text"},
				},
				EnumTypes: []schema.EnumType{},
			},
		}

		// Code: Model uses enum type
		codeSchema := map[string]*schema.TableMetadata{
			"orders": {
				Name: "orders",
				Columns: []schema.ColumnMetadata{
					{Name: "id", SQLType: "serial"},
					{Name: "status", SQLType: "order_status", EnumType: "order_status"},
				},
				EnumTypes: []schema.EnumType{
					{
						Name:   "order_status",
						Values: []string{"pending", "active", "completed"},
					},
				},
			},
		}

		differ := NewDiffer()
		diff := differ.Compare(codeSchema, dbSchema)

		// Assert: Should detect new enum type
		if len(diff.EnumTypesAdded) != 1 {
			t.Fatalf("Expected 1 enum type to be added, got %d", len(diff.EnumTypesAdded))
		}

		addedEnum := diff.EnumTypesAdded[0]
		if addedEnum.Name != "order_status" {
			t.Errorf("Expected enum name 'order_status', got '%s'", addedEnum.Name)
		}

		expectedValues := []string{"pending", "active", "completed"}
		if len(addedEnum.Values) != len(expectedValues) {
			t.Errorf("Expected %d values, got %d", len(expectedValues), len(addedEnum.Values))
		}
		for i, val := range expectedValues {
			if addedEnum.Values[i] != val {
				t.Errorf("Expected value '%s' at index %d, got '%s'", val, i, addedEnum.Values[i])
			}
		}

		// Generate SQL
		planner := NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		expectedSQL := "CREATE TYPE order_status AS ENUM ('pending', 'active', 'completed');"
		if !strings.Contains(upSQL, expectedSQL) {
			t.Errorf("Expected SQL:\n%s\nGot:\n%s", expectedSQL, upSQL)
		}
	})

	t.Run("drop enum type", func(t *testing.T) {
		// Setup: Database has enum type
		dbSchema := map[string]*schema.TableMetadata{
			"orders": {
				Name: "orders",
				Columns: []schema.ColumnMetadata{
					{Name: "id", SQLType: "serial"},
					{Name: "status", SQLType: "order_status", EnumType: "order_status"},
				},
				EnumTypes: []schema.EnumType{
					{
						Name:   "order_status",
						Values: []string{"pending", "active"},
					},
				},
			},
		}

		// Code: Model no longer uses enum (changed to text)
		codeSchema := map[string]*schema.TableMetadata{
			"orders": {
				Name: "orders",
				Columns: []schema.ColumnMetadata{
					{Name: "id", SQLType: "serial"},
					{Name: "status", SQLType: "text"},
				},
				EnumTypes: []schema.EnumType{},
			},
		}

		differ := NewDiffer()
		diff := differ.Compare(codeSchema, dbSchema)

		// Assert: Should detect dropped enum type
		if len(diff.EnumTypesDropped) != 1 {
			t.Fatalf("Expected 1 enum type to be dropped, got %d", len(diff.EnumTypesDropped))
		}

		if diff.EnumTypesDropped[0] != "order_status" {
			t.Errorf("Expected 'order_status' to be dropped, got '%s'", diff.EnumTypesDropped[0])
		}

		planner := NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		expectedSQL := "DROP TYPE IF EXISTS order_status;"
		if !strings.Contains(upSQL, expectedSQL) {
			t.Errorf("Expected SQL:\n%s\nGot:\n%s", expectedSQL, upSQL)
		}
	})

	t.Run("modify enum type - add values", func(t *testing.T) {
		// Setup: Database has enum with 2 values
		dbSchema := map[string]*schema.TableMetadata{
			"orders": {
				Name: "orders",
				Columns: []schema.ColumnMetadata{
					{Name: "status", SQLType: "order_status", EnumType: "order_status"},
				},
				EnumTypes: []schema.EnumType{
					{
						Name:   "order_status",
						Values: []string{"pending", "active"},
					},
				},
			},
		}

		// Code: Model has enum with 3 values (added "completed")
		codeSchema := map[string]*schema.TableMetadata{
			"orders": {
				Name: "orders",
				Columns: []schema.ColumnMetadata{
					{Name: "status", SQLType: "order_status", EnumType: "order_status"},
				},
				EnumTypes: []schema.EnumType{
					{
						Name:   "order_status",
						Values: []string{"pending", "active", "completed"},
					},
				},
			},
		}

		differ := NewDiffer()
		diff := differ.Compare(codeSchema, dbSchema)

		// Assert: Should detect modified enum type
		if len(diff.EnumTypesModified) != 1 {
			t.Fatalf("Expected 1 enum type to be modified, got %d", len(diff.EnumTypesModified))
		}

		enumDiff := diff.EnumTypesModified[0]
		if enumDiff.Name != "order_status" {
			t.Errorf("Expected enum name 'order_status', got '%s'", enumDiff.Name)
		}

		if len(enumDiff.NewValues) != 1 || enumDiff.NewValues[0] != "completed" {
			t.Errorf("Expected new values ['completed'], got %v", enumDiff.NewValues)
		}

		planner := NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		expectedSQL := "ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'completed';"
		if !strings.Contains(upSQL, expectedSQL) {
			t.Errorf("Expected SQL:\n%s\nGot:\n%s", expectedSQL, upSQL)
		}
	})

	t.Run("enum type unchanged", func(t *testing.T) {
		// Both have same enum type with same values
		enumType := schema.EnumType{
			Name:   "order_status",
			Values: []string{"pending", "active", "completed"},
		}

		dbSchema := map[string]*schema.TableMetadata{
			"orders": {
				Name: "orders",
				EnumTypes: []schema.EnumType{enumType},
			},
		}

		codeSchema := map[string]*schema.TableMetadata{
			"orders": {
				Name: "orders",
				EnumTypes: []schema.EnumType{enumType},
			},
		}

		differ := NewDiffer()
		diff := differ.Compare(codeSchema, dbSchema)

		// Assert: Should not detect any enum changes
		if len(diff.EnumTypesAdded) > 0 {
			t.Errorf("Expected no enums added, got %d", len(diff.EnumTypesAdded))
		}
		if len(diff.EnumTypesDropped) > 0 {
			t.Errorf("Expected no enums dropped, got %d", len(diff.EnumTypesDropped))
		}
		if len(diff.EnumTypesModified) > 0 {
			t.Errorf("Expected no enums modified, got %d", len(diff.EnumTypesModified))
		}
	})

	t.Run("multiple enum types", func(t *testing.T) {
		// Database has one enum
		dbSchema := map[string]*schema.TableMetadata{
			"orders": {
				Name: "orders",
				EnumTypes: []schema.EnumType{
					{Name: "order_status", Values: []string{"pending", "active"}},
				},
			},
		}

		// Code has two enums (order_status expanded, priority added)
		codeSchema := map[string]*schema.TableMetadata{
			"orders": {
				Name: "orders",
				EnumTypes: []schema.EnumType{
					{Name: "order_status", Values: []string{"pending", "active", "completed"}},
					{Name: "order_priority", Values: []string{"low", "medium", "high"}},
				},
			},
		}

		differ := NewDiffer()
		diff := differ.Compare(codeSchema, dbSchema)

		// Should detect: 1 added, 0 dropped, 1 modified
		if len(diff.EnumTypesAdded) != 1 {
			t.Errorf("Expected 1 enum added, got %d", len(diff.EnumTypesAdded))
		}
		if len(diff.EnumTypesModified) != 1 {
			t.Errorf("Expected 1 enum modified, got %d", len(diff.EnumTypesModified))
		}

		planner := NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		// Should have both CREATE TYPE and ALTER TYPE
		if !strings.Contains(upSQL, "CREATE TYPE order_priority") {
			t.Errorf("Expected CREATE TYPE for order_priority in:\n%s", upSQL)
		}
		if !strings.Contains(upSQL, "ALTER TYPE order_status ADD VALUE") {
			t.Errorf("Expected ALTER TYPE for order_status in:\n%s", upSQL)
		}
	})

	t.Run("enum used by multiple tables", func(t *testing.T) {
		// Database: enum doesn't exist
		dbSchema := map[string]*schema.TableMetadata{
			"orders": {
				Name:      "orders",
				EnumTypes: []schema.EnumType{},
			},
			"shipments": {
				Name:      "shipments",
				EnumTypes: []schema.EnumType{},
			},
		}

		// Code: both tables use the same enum
		sharedEnum := schema.EnumType{
			Name:   "status",
			Values: []string{"pending", "active", "completed"},
		}

		codeSchema := map[string]*schema.TableMetadata{
			"orders": {
				Name:      "orders",
				EnumTypes: []schema.EnumType{sharedEnum},
			},
			"shipments": {
				Name:      "shipments",
				EnumTypes: []schema.EnumType{sharedEnum},
			},
		}

		differ := NewDiffer()
		diff := differ.Compare(codeSchema, dbSchema)

		// Should detect enum only once (deduplicated)
		if len(diff.EnumTypesAdded) != 1 {
			t.Fatalf("Expected 1 enum type to be added (deduplicated), got %d", len(diff.EnumTypesAdded))
		}

		if diff.EnumTypesAdded[0].Name != "status" {
			t.Errorf("Expected enum name 'status', got '%s'", diff.EnumTypesAdded[0].Name)
		}

		planner := NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		// Should only create the enum once
		count := strings.Count(upSQL, "CREATE TYPE status AS ENUM")
		if count != 1 {
			t.Errorf("Expected CREATE TYPE to appear once, got %d times", count)
		}
	})
}

func TestEnumSQLGeneration(t *testing.T) {
	planner := NewPlanner()

	t.Run("CREATE TYPE statement", func(t *testing.T) {
		enumType := schema.EnumType{
			Name:   "order_status",
			Values: []string{"pending", "active", "completed"},
		}

		sql := planner.generateCreateEnumType(enumType)
		expected := "CREATE TYPE order_status AS ENUM ('pending', 'active', 'completed');"

		if sql != expected {
			t.Errorf("Expected:\n%s\nGot:\n%s", expected, sql)
		}
	})

	t.Run("DROP TYPE statement", func(t *testing.T) {
		sql := planner.generateDropEnumType("order_status")
		expected := "DROP TYPE IF EXISTS order_status;"

		if sql != expected {
			t.Errorf("Expected:\n%s\nGot:\n%s", expected, sql)
		}
	})

	t.Run("ALTER TYPE ADD VALUE statements", func(t *testing.T) {
		enumDiff := EnumTypeDiff{
			Name:      "order_status",
			OldValues: []string{"pending", "active"},
			NewValues: []string{"completed", "cancelled"},
		}

		statements := planner.generateAlterEnumType(enumDiff)

		if len(statements) != 2 {
			t.Fatalf("Expected 2 statements, got %d", len(statements))
		}

		expected1 := "ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'completed';"
		if statements[0] != expected1 {
			t.Errorf("Expected:\n%s\nGot:\n%s", expected1, statements[0])
		}

		expected2 := "ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'cancelled';"
		if statements[1] != expected2 {
			t.Errorf("Expected:\n%s\nGot:\n%s", expected2, statements[1])
		}
	})
}

func TestEnumMigrationOrdering(t *testing.T) {
	t.Run("enums created before tables", func(t *testing.T) {
		diff := &SchemaDiff{
			EnumTypesAdded: []schema.EnumType{
				{Name: "order_status", Values: []string{"pending"}},
			},
			TablesAdded: []schema.TableMetadata{
				{
					Name: "orders",
					Columns: []schema.ColumnMetadata{
						{Name: "status", SQLType: "order_status", EnumType: "order_status"},
					},
				},
			},
		}

		planner := NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		// Find positions of CREATE TYPE and CREATE TABLE
		createTypePos := strings.Index(upSQL, "CREATE TYPE")
		createTablePos := strings.Index(upSQL, "CREATE TABLE")

		if createTypePos == -1 || createTablePos == -1 {
			t.Fatalf("Missing CREATE TYPE or CREATE TABLE in SQL:\n%s", upSQL)
		}

		// CREATE TYPE must come before CREATE TABLE
		if createTypePos > createTablePos {
			t.Errorf("CREATE TYPE must come before CREATE TABLE, but found:\n%s", upSQL)
		}
	})

	t.Run("enums dropped after tables", func(t *testing.T) {
		diff := &SchemaDiff{
			TablesDropped: []string{"orders"},
			EnumTypesDropped: []string{"order_status"},
		}

		planner := NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		// Find positions of DROP TABLE and DROP TYPE
		dropTablePos := strings.Index(upSQL, "DROP TABLE")
		dropTypePos := strings.Index(upSQL, "DROP TYPE")

		if dropTablePos == -1 || dropTypePos == -1 {
			t.Fatalf("Missing DROP TABLE or DROP TYPE in SQL:\n%s", upSQL)
		}

		// DROP TABLE must come before DROP TYPE
		if dropTablePos > dropTypePos {
			t.Errorf("DROP TABLE must come before DROP TYPE, but found:\n%s", upSQL)
		}
	})
}
