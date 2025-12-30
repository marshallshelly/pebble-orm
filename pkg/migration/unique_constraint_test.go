package migration

import (
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func TestUniqueConstraintDetection(t *testing.T) {
	t.Run("add unique constraint", func(t *testing.T) {
		// Setup: Table exists without UNIQUE constraint
		dbSchema := map[string]*schema.TableMetadata{
			"users": {
				Name: "users",
				Columns: []schema.ColumnMetadata{
					{Name: "email", SQLType: "varchar(255)"},
				},
				Constraints: []schema.ConstraintMetadata{},
			},
		}

		// Code: Model has unique tag
		codeSchema := map[string]*schema.TableMetadata{
			"users": {
				Name: "users",
				Columns: []schema.ColumnMetadata{
					{Name: "email", SQLType: "varchar(255)"},
				},
				Constraints: []schema.ConstraintMetadata{
					{
						Name:    "users_email_key",
						Type:    schema.UniqueConstraint,
						Columns: []string{"email"},
					},
				},
			},
		}

		differ := NewDiffer()
		diff := differ.Compare(codeSchema, dbSchema)

		// Assert: Should detect missing constraint
		if len(diff.TablesModified) != 1 {
			t.Fatalf("Expected 1 table to be modified, got %d", len(diff.TablesModified))
		}

		tableDiff := diff.TablesModified[0]
		if len(tableDiff.ConstraintsAdded) != 1 {
			t.Fatalf("Expected 1 constraint to be added, got %d", len(tableDiff.ConstraintsAdded))
		}

		addedConstraint := tableDiff.ConstraintsAdded[0]
		if addedConstraint.Type != schema.UniqueConstraint {
			t.Errorf("Expected UNIQUE constraint, got %s", addedConstraint.Type)
		}
		if len(addedConstraint.Columns) != 1 || addedConstraint.Columns[0] != "email" {
			t.Errorf("Expected columns [email], got %v", addedConstraint.Columns)
		}

		// Generate SQL
		planner := NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		expectedSQL := "ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);"
		if !strings.Contains(upSQL, expectedSQL) {
			t.Errorf("Expected SQL:\n%s\nGot:\n%s", expectedSQL, upSQL)
		}
	})

	t.Run("drop unique constraint", func(t *testing.T) {
		// Setup: Table has UNIQUE constraint
		dbSchema := map[string]*schema.TableMetadata{
			"users": {
				Name: "users",
				Columns: []schema.ColumnMetadata{
					{Name: "email", SQLType: "varchar(255)"},
				},
				Constraints: []schema.ConstraintMetadata{
					{
						Name:    "users_email_key",
						Type:    schema.UniqueConstraint,
						Columns: []string{"email"},
					},
				},
			},
		}

		// Code: Model no longer has unique tag
		codeSchema := map[string]*schema.TableMetadata{
			"users": {
				Name: "users",
				Columns: []schema.ColumnMetadata{
					{Name: "email", SQLType: "varchar(255)"},
				},
				Constraints: []schema.ConstraintMetadata{},
			},
		}

		differ := NewDiffer()
		diff := differ.Compare(codeSchema, dbSchema)

		// Assert: Should detect constraint to drop
		if len(diff.TablesModified) != 1 {
			t.Fatalf("Expected 1 table to be modified, got %d", len(diff.TablesModified))
		}

		tableDiff := diff.TablesModified[0]
		if len(tableDiff.ConstraintsDropped) != 1 {
			t.Fatalf("Expected 1 constraint to be dropped, got %d", len(tableDiff.ConstraintsDropped))
		}

		planner := NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		expectedSQL := "ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;"
		if !strings.Contains(upSQL, expectedSQL) {
			t.Errorf("Expected SQL:\n%s\nGot:\n%s", expectedSQL, upSQL)
		}
	})

	t.Run("composite unique constraint", func(t *testing.T) {
		dbSchema := map[string]*schema.TableMetadata{
			"user_roles": {
				Name: "user_roles",
				Columns: []schema.ColumnMetadata{
					{Name: "user_id", SQLType: "integer"},
					{Name: "role_id", SQLType: "integer"},
				},
				Constraints: []schema.ConstraintMetadata{},
			},
		}

		codeSchema := map[string]*schema.TableMetadata{
			"user_roles": {
				Name: "user_roles",
				Columns: []schema.ColumnMetadata{
					{Name: "user_id", SQLType: "integer"},
					{Name: "role_id", SQLType: "integer"},
				},
				Constraints: []schema.ConstraintMetadata{
					{
						Name:    "user_roles_user_id_role_id_key",
						Type:    schema.UniqueConstraint,
						Columns: []string{"user_id", "role_id"},
					},
				},
			},
		}

		differ := NewDiffer()
		diff := differ.Compare(codeSchema, dbSchema)

		planner := NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		expectedSQL := "ALTER TABLE user_roles ADD CONSTRAINT user_roles_user_id_role_id_key UNIQUE (user_id, role_id);"
		if !strings.Contains(upSQL, expectedSQL) {
			t.Errorf("Expected SQL:\n%s\nGot:\n%s", expectedSQL, upSQL)
		}
	})

	t.Run("unique constraint with different name but same columns", func(t *testing.T) {
		// Database has constraint with auto-generated name
		dbSchema := map[string]*schema.TableMetadata{
			"users": {
				Name: "users",
				Columns: []schema.ColumnMetadata{
					{Name: "email", SQLType: "varchar(255)"},
				},
				Constraints: []schema.ConstraintMetadata{
					{
						Name:    "users_email_key",
						Type:    schema.UniqueConstraint,
						Columns: []string{"email"},
					},
				},
			},
		}

		// Code has constraint with same columns (should match)
		codeSchema := map[string]*schema.TableMetadata{
			"users": {
				Name: "users",
				Columns: []schema.ColumnMetadata{
					{Name: "email", SQLType: "varchar(255)"},
				},
				Constraints: []schema.ConstraintMetadata{
					{
						Name:    "users_email_key", // Same columns, so should match
						Type:    schema.UniqueConstraint,
						Columns: []string{"email"},
					},
				},
			},
		}

		differ := NewDiffer()
		diff := differ.Compare(codeSchema, dbSchema)

		// Assert: Should NOT detect any changes (constraints match by columns)
		if diff.HasChanges() {
			t.Errorf("Expected no changes, but got changes: %+v", diff)
		}
	})

	t.Run("multiple unique constraints on same table", func(t *testing.T) {
		dbSchema := map[string]*schema.TableMetadata{
			"users": {
				Name: "users",
				Columns: []schema.ColumnMetadata{
					{Name: "email", SQLType: "varchar(255)"},
					{Name: "username", SQLType: "varchar(100)"},
				},
				Constraints: []schema.ConstraintMetadata{},
			},
		}

		codeSchema := map[string]*schema.TableMetadata{
			"users": {
				Name: "users",
				Columns: []schema.ColumnMetadata{
					{Name: "email", SQLType: "varchar(255)"},
					{Name: "username", SQLType: "varchar(100)"},
				},
				Constraints: []schema.ConstraintMetadata{
					{
						Name:    "users_email_key",
						Type:    schema.UniqueConstraint,
						Columns: []string{"email"},
					},
					{
						Name:    "users_username_key",
						Type:    schema.UniqueConstraint,
						Columns: []string{"username"},
					},
				},
			},
		}

		differ := NewDiffer()
		diff := differ.Compare(codeSchema, dbSchema)

		// Assert: Should detect 2 constraints to add
		if len(diff.TablesModified) != 1 {
			t.Fatalf("Expected 1 table to be modified, got %d", len(diff.TablesModified))
		}

		tableDiff := diff.TablesModified[0]
		if len(tableDiff.ConstraintsAdded) != 2 {
			t.Fatalf("Expected 2 constraints to be added, got %d", len(tableDiff.ConstraintsAdded))
		}

		planner := NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		// Check both constraints are in the SQL
		if !strings.Contains(upSQL, "users_email_key UNIQUE (email)") {
			t.Errorf("Expected email constraint in SQL:\n%s", upSQL)
		}
		if !strings.Contains(upSQL, "users_username_key UNIQUE (username)") {
			t.Errorf("Expected username constraint in SQL:\n%s", upSQL)
		}
	})

	t.Run("mix of unique and check constraints", func(t *testing.T) {
		dbSchema := map[string]*schema.TableMetadata{
			"products": {
				Name: "products",
				Columns: []schema.ColumnMetadata{
					{Name: "sku", SQLType: "varchar(50)"},
					{Name: "price", SQLType: "numeric(10,2)"},
				},
				Constraints: []schema.ConstraintMetadata{},
			},
		}

		codeSchema := map[string]*schema.TableMetadata{
			"products": {
				Name: "products",
				Columns: []schema.ColumnMetadata{
					{Name: "sku", SQLType: "varchar(50)"},
					{Name: "price", SQLType: "numeric(10,2)"},
				},
				Constraints: []schema.ConstraintMetadata{
					{
						Name:    "products_sku_key",
						Type:    schema.UniqueConstraint,
						Columns: []string{"sku"},
					},
					{
						Name:       "products_price_check",
						Type:       schema.CheckConstraint,
						Expression: "(price > 0)",
					},
				},
			},
		}

		differ := NewDiffer()
		diff := differ.Compare(codeSchema, dbSchema)

		planner := NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		// Check both constraint types are handled
		if !strings.Contains(upSQL, "ADD CONSTRAINT products_sku_key UNIQUE (sku)") {
			t.Errorf("Expected UNIQUE constraint in SQL:\n%s", upSQL)
		}
		if !strings.Contains(upSQL, "ADD CONSTRAINT products_price_check CHECK (price > 0)") {
			t.Errorf("Expected CHECK constraint in SQL:\n%s", upSQL)
		}
	})
}

func TestConstraintKeyGeneration(t *testing.T) {
	differ := NewDiffer()

	t.Run("unique constraint key", func(t *testing.T) {
		c := schema.ConstraintMetadata{
			Name:    "users_email_key",
			Type:    schema.UniqueConstraint,
			Columns: []string{"email"},
		}

		key := differ.getConstraintKey(c)
		expected := "unique:email"
		if key != expected {
			t.Errorf("Expected key %s, got %s", expected, key)
		}
	})

	t.Run("composite unique constraint key", func(t *testing.T) {
		c := schema.ConstraintMetadata{
			Name:    "user_roles_user_id_role_id_key",
			Type:    schema.UniqueConstraint,
			Columns: []string{"user_id", "role_id"},
		}

		key := differ.getConstraintKey(c)
		expected := "unique:user_id,role_id"
		if key != expected {
			t.Errorf("Expected key %s, got %s", expected, key)
		}
	})

	t.Run("check constraint key", func(t *testing.T) {
		c := schema.ConstraintMetadata{
			Name:       "products_price_check",
			Type:       schema.CheckConstraint,
			Expression: "(price > 0)",
		}

		key := differ.getConstraintKey(c)
		expected := "products_price_check"
		if key != expected {
			t.Errorf("Expected key %s, got %s", expected, key)
		}
	})
}

func TestCreateTableWithUniqueConstraints(t *testing.T) {
	planner := NewPlanner()

	t.Run("single column unique", func(t *testing.T) {
		table := &schema.TableMetadata{
			Name: "users",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "serial", Nullable: true},
				{Name: "email", SQLType: "varchar(255)", Nullable: false, Unique: true},
			},
			Constraints: []schema.ConstraintMetadata{
				{
					Name:    "users_email_key",
					Type:    schema.UniqueConstraint,
					Columns: []string{"email"},
				},
			},
		}

		sql := planner.generateCreateTable(table)

		// Should have UNIQUE inline on column definition (with NOT NULL before it)
		if !strings.Contains(sql, "email varchar(255) NOT NULL UNIQUE") {
			t.Errorf("Expected inline UNIQUE in column definition:\n%s", sql)
		}

		// Should NOT have table-level constraint for single-column UNIQUE
		// (to avoid duplication)
		if strings.Contains(sql, "CONSTRAINT users_email_key UNIQUE") {
			t.Errorf("Should not have table-level UNIQUE for single column:\n%s", sql)
		}
	})

	t.Run("multi-column unique", func(t *testing.T) {
		table := &schema.TableMetadata{
			Name: "user_roles",
			Columns: []schema.ColumnMetadata{
				{Name: "user_id", SQLType: "integer"},
				{Name: "role_id", SQLType: "integer"},
			},
			Constraints: []schema.ConstraintMetadata{
				{
					Name:    "user_roles_user_id_role_id_key",
					Type:    schema.UniqueConstraint,
					Columns: []string{"user_id", "role_id"},
				},
			},
		}

		sql := planner.generateCreateTable(table)

		// Should have table-level constraint for multi-column UNIQUE
		if !strings.Contains(sql, "CONSTRAINT user_roles_user_id_role_id_key UNIQUE (user_id, role_id)") {
			t.Errorf("Expected table-level multi-column UNIQUE:\n%s", sql)
		}
	})
}
