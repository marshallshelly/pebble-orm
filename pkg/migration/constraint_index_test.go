package migration

import (
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// TestConstraintBackedIndexHandling verifies that indexes backing constraints
// are not treated as standalone indexes that can be dropped with DROP INDEX.
//
// Issue: When a UNIQUE constraint creates an index, trying to DROP INDEX fails with:
// ERROR: cannot drop index users_email_key because constraint users_email_key on table users requires it
func TestConstraintBackedIndexHandling(t *testing.T) {
	planner := NewPlanner()

	// Scenario: Table with UNIQUE constraint (which creates an implicit index)
	dbSchema := map[string]*schema.TableMetadata{
		"users": {
			Name: "users",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "uuid", Nullable: false},
				{Name: "email", SQLType: "varchar(255)", Nullable: false, Unique: true},
			},
			PrimaryKey: &schema.PrimaryKeyMetadata{
				Name:    "users_pkey",
				Columns: []string{"id"},
			},
			// In production, introspector should NOT return this index
			// because it backs the UNIQUE constraint
			Indexes: []schema.IndexMetadata{},
		},
	}

	codeSchema := map[string]*schema.TableMetadata{
		"users": {
			Name: "users",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "uuid", Nullable: false},
				{Name: "email", SQLType: "varchar(255)", Nullable: false, Unique: true},
			},
			PrimaryKey: &schema.PrimaryKeyMetadata{
				Name:    "users_pkey",
				Columns: []string{"id"},
			},
			Indexes: []schema.IndexMetadata{},
		},
	}

	differ := NewDiffer()
	diff := differ.Compare(codeSchema, dbSchema)

	// Generate migration
	upSQL, downSQL := planner.GenerateMigration(diff)

	// Should NOT generate any DROP INDEX statements
	if strings.Contains(upSQL, "DROP INDEX") {
		t.Errorf("Migration should not drop constraint-backed indexes\nGot UP SQL:\n%s", upSQL)
	}

	if strings.Contains(downSQL, "DROP INDEX") {
		t.Errorf("Migration should not drop constraint-backed indexes\nGot DOWN SQL:\n%s", downSQL)
	}

	// Should not have any index changes
	if len(diff.TablesModified) > 0 {
		for _, tm := range diff.TablesModified {
			if len(tm.IndexesAdded) > 0 || len(tm.IndexesDropped) > 0 {
				t.Errorf("Should not detect index changes for constraint-backed indexes\nIndexes added: %v\nIndexes dropped: %v",
					tm.IndexesAdded, tm.IndexesDropped)
			}
		}
	}
}

// TestStandaloneIndexStillDetected verifies that truly standalone indexes
// (not backing constraints) are still properly handled by the differ.
func TestStandaloneIndexStillDetected(t *testing.T) {
	planner := NewPlanner()

	// Database has a standalone index on email
	dbSchema := map[string]*schema.TableMetadata{
		"users": {
			Name: "users",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "uuid", Nullable: false},
				{Name: "email", SQLType: "varchar(255)", Nullable: false},
			},
			PrimaryKey: &schema.PrimaryKeyMetadata{
				Name:    "users_pkey",
				Columns: []string{"id"},
			},
			Indexes: []schema.IndexMetadata{
				{Name: "idx_users_email", Columns: []string{"email"}, Unique: false, Type: "btree"},
			},
		},
	}

	// Code schema doesn't have the index
	codeSchema := map[string]*schema.TableMetadata{
		"users": {
			Name: "users",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "uuid", Nullable: false},
				{Name: "email", SQLType: "varchar(255)", Nullable: false},
			},
			PrimaryKey: &schema.PrimaryKeyMetadata{
				Name:    "users_pkey",
				Columns: []string{"id"},
			},
			Indexes: []schema.IndexMetadata{},
		},
	}

	differ := NewDiffer()
	diff := differ.Compare(codeSchema, dbSchema)

	// Generate migration
	upSQL, _ := planner.GenerateMigration(diff)

	// SHOULD generate DROP INDEX for standalone index
	if !strings.Contains(upSQL, "DROP INDEX IF EXISTS idx_users_email") {
		t.Errorf("Should drop standalone index\nGot UP SQL:\n%s", upSQL)
	}
}
