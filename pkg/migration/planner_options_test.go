package migration

import (
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func TestPlannerOptions_IfNotExists(t *testing.T) {
	table := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "serial", Nullable: false},
			{Name: "email", SQLType: "varchar(255)", Nullable: false},
		},
		PrimaryKey: &schema.PrimaryKeyMetadata{
			Name:    "users_pkey",
			Columns: []string{"id"},
		},
		Indexes: []schema.IndexMetadata{
			{Name: "idx_users_email", Columns: []string{"email"}, Unique: true, Type: "btree"},
		},
	}

	t.Run("Default (IF NOT EXISTS enabled)", func(t *testing.T) {
		planner := NewPlanner()
		sql := planner.generateCreateTable(table)

		if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS users") {
			t.Errorf("Expected CREATE TABLE IF NOT EXISTS, got: %s", sql)
		}

		if !strings.Contains(sql, "CREATE UNIQUE INDEX IF NOT EXISTS") {
			t.Errorf("Expected CREATE UNIQUE INDEX IF NOT EXISTS, got: %s", sql)
		}
	})

	t.Run("IF NOT EXISTS disabled", func(t *testing.T) {
		planner := NewPlannerWithOptions(PlannerOptions{
			IfNotExists: false,
		})
		sql := planner.generateCreateTable(table)

		if !strings.Contains(sql, "CREATE TABLE users") {
			t.Errorf("Expected CREATE TABLE (without IF NOT EXISTS), got: %s", sql)
		}

		if strings.Contains(sql, "IF NOT EXISTS") {
			t.Errorf("Expected no IF NOT EXISTS, got: %s", sql)
		}
	})

	t.Run("Idempotent migrations", func(t *testing.T) {
		planner := NewPlanner()
		diff := &SchemaDiff{
			TablesAdded: []schema.TableMetadata{*table},
		}

		upSQL, _ := planner.GenerateMigration(diff)

		// Should be safe to run multiple times
		if !strings.Contains(upSQL, "IF NOT EXISTS") {
			t.Errorf("Expected idempotent SQL with IF NOT EXISTS, got: %s", upSQL)
		}
	})
}
