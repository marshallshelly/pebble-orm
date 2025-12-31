// Package migration provides database migration functionality.
package migration

import (
	"time"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// Migration represents a database migration.
type Migration struct {
	Version   string    // Version/timestamp (e.g., "20240101120000")
	Name      string    // Migration name (e.g., "create_users_table")
	UpSQL     string    // SQL for applying the migration
	DownSQL   string    // SQL for rolling back the migration
	AppliedAt time.Time // When the migration was applied
}

// MigrationFile represents a migration file on disk.
type MigrationFile struct {
	Version  string // Version/timestamp
	Name     string // Migration name
	UpPath   string // Path to .up.sql file
	DownPath string // Path to .down.sql file
}

// SchemaDiff represents differences between two schemas.
type SchemaDiff struct {
	TablesAdded    []schema.TableMetadata // Tables to create
	TablesDropped  []string               // Table names to drop
	TablesModified []TableDiff            // Tables with changes
	EnumTypesAdded []schema.EnumType      // Enum types to create
	EnumTypesDropped []string             // Enum type names to drop
	EnumTypesModified []EnumTypeDiff      // Enum types with new values
}

// TableDiff represents changes to a single table.
type TableDiff struct {
	TableName          string                      // Name of the table
	ColumnsAdded       []schema.ColumnMetadata     // Columns to add
	ColumnsDropped     []string                    // Column names to drop
	ColumnsModified    []ColumnDiff                // Columns with changes
	IndexesAdded       []schema.IndexMetadata      // Indexes to create
	IndexesDropped     []string                    // Index names to drop
	ForeignKeysAdded   []schema.ForeignKeyMetadata // Foreign keys to add
	ForeignKeysDropped []string                    // Foreign key names to drop
	ConstraintsAdded   []schema.ConstraintMetadata // Constraints to add
	ConstraintsDropped []string                    // Constraint names to drop
	PrimaryKeyChanged  *PrimaryKeyChange           // Primary key modification
}

// ColumnDiff represents changes to a single column.
type ColumnDiff struct {
	ColumnName     string // Name of the column
	OldColumn      schema.ColumnMetadata
	NewColumn      schema.ColumnMetadata
	TypeChanged    bool // SQL type changed
	NullChanged    bool // Nullability changed
	DefaultChanged bool // Default value changed
}

// PrimaryKeyChange represents a change to the primary key.
type PrimaryKeyChange struct {
	Old *schema.PrimaryKeyMetadata
	New *schema.PrimaryKeyMetadata
}

// EnumTypeDiff represents changes to an enum type.
type EnumTypeDiff struct {
	Name        string   // Enum type name
	OldValues   []string // Existing values in database
	NewValues   []string // New values to add
}

// MigrationStatus represents the status of a migration.
type MigrationStatus string

const (
	// StatusPending means the migration has not been applied.
	StatusPending MigrationStatus = "pending"
	// StatusApplied means the migration has been applied.
	StatusApplied MigrationStatus = "applied"
	// StatusFailed means the migration failed to apply.
	StatusFailed MigrationStatus = "failed"
)

// MigrationRecord represents a migration in the tracking table.
type MigrationRecord struct {
	Version   string          // Migration version
	Name      string          // Migration name
	Status    MigrationStatus // Current status
	AppliedAt *time.Time      // When applied (nil if not applied)
	Error     *string         // Error message if failed
}

// MigrationPlan represents a plan for applying migrations.
type MigrationPlan struct {
	Migrations []Migration // Migrations to apply in order
	DryRun     bool        // Whether this is a dry run
}

// HasChanges returns true if there are any schema differences.
func (d *SchemaDiff) HasChanges() bool {
	return len(d.TablesAdded) > 0 ||
		len(d.TablesDropped) > 0 ||
		len(d.TablesModified) > 0 ||
		len(d.EnumTypesAdded) > 0 ||
		len(d.EnumTypesDropped) > 0 ||
		len(d.EnumTypesModified) > 0
}

// HasChanges returns true if the table has any changes.
func (t *TableDiff) HasChanges() bool {
	return len(t.ColumnsAdded) > 0 ||
		len(t.ColumnsDropped) > 0 ||
		len(t.ColumnsModified) > 0 ||
		len(t.IndexesAdded) > 0 ||
		len(t.IndexesDropped) > 0 ||
		len(t.ForeignKeysAdded) > 0 ||
		len(t.ForeignKeysDropped) > 0 ||
		len(t.ConstraintsAdded) > 0 ||
		len(t.ConstraintsDropped) > 0 ||
		t.PrimaryKeyChanged != nil
}

// GenerateVersion generates a timestamp-based version string.
// Format: YYYYMMDDHHmmss (e.g., "20240101120000")
func GenerateVersion() string {
	return time.Now().Format("20060102150405")
}

// GenerateFileName generates a migration filename.
// Format: {version}_{name}.{up|down}.sql
func GenerateFileName(version, name, direction string) string {
	return version + "_" + name + "." + direction + ".sql"
}
