// Package migration provides database migration functionality.
package migration

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
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
	TablesAdded       []schema.TableMetadata // Tables to create
	TablesDropped     []schema.TableMetadata // Tables to drop (full metadata for down migration)
	TablesModified    []TableDiff            // Tables with changes
	EnumTypesAdded    []schema.EnumType      // Enum types to create
	EnumTypesDropped  []schema.EnumType      // Enum types to drop (full metadata for down migration)
	EnumTypesModified []EnumTypeDiff         // Enum types with new values
	DomainsAdded      []schema.DomainType    // Domain types to create
	DomainsDropped    []schema.DomainType    // Domain types to drop (full metadata for down migration)
	ExtensionsAdded   []string               // PostgreSQL extensions to create
}

// TableDiff represents changes to a single table.
type TableDiff struct {
	TableName          string                      // Name of the table
	ColumnsAdded       []schema.ColumnMetadata     // Columns to add
	ColumnsDropped     []schema.ColumnMetadata     // Columns to drop (full metadata for down migration)
	ColumnsModified    []ColumnDiff                // Columns with changes
	IndexesAdded       []schema.IndexMetadata      // Indexes to create
	IndexesDropped     []schema.IndexMetadata      // Indexes to drop (full metadata for down migration)
	ForeignKeysAdded   []schema.ForeignKeyMetadata // Foreign keys to add
	ForeignKeysDropped []schema.ForeignKeyMetadata // Foreign keys to drop (full metadata for down migration)
	ConstraintsAdded   []schema.ConstraintMetadata // Constraints to add
	ConstraintsDropped []schema.ConstraintMetadata // Constraints to drop (full metadata for down migration)
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
	Name      string   // Enum type name
	OldValues []string // Existing values in database
	NewValues []string // New values to add
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
		len(d.EnumTypesModified) > 0 ||
		len(d.DomainsAdded) > 0 ||
		len(d.DomainsDropped) > 0 ||
		len(d.ExtensionsAdded) > 0
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

// VersionScheme selects how new migration versions are numbered.
type VersionScheme string

const (
	// TimestampVersions numbers migrations as UTC timestamps (20060102150405).
	// Two branches never collide, which is why it is the default.
	TimestampVersions VersionScheme = "timestamp"

	// SequentialVersions numbers migrations as zero-padded counters (000, 001, 002).
	// Readable, but two branches that each add a migration produce the same
	// number and collide on merge.
	SequentialVersions VersionScheme = "sequential"
)

const defaultSequentialWidth = 3

var reTimestampVersion = regexp.MustCompile(`^\d{14}$`)
var reSequentialVersion = regexp.MustCompile(`^\d{1,13}$`)

// DetectVersionScheme reports the scheme already in use in a migrations
// directory. found is false when the directory holds no migrations yet.
// A directory mixing both schemes is an error: versions are ordered
// lexicographically, so a sequential version sorts ahead of every timestamp
// and would silently reorder applied history.
func DetectVersionScheme(migrationsDir string) (scheme VersionScheme, found bool, err error) {
	versions, err := existingVersions(migrationsDir)
	if err != nil {
		return "", false, err
	}
	if len(versions) == 0 {
		return "", false, nil
	}

	var timestamps, sequentials int
	for _, v := range versions {
		switch {
		case reTimestampVersion.MatchString(v):
			timestamps++
		case reSequentialVersion.MatchString(v):
			sequentials++
		}
	}

	switch {
	case timestamps > 0 && sequentials > 0:
		return "", true, fmt.Errorf(
			"migrations directory %s mixes timestamp and sequential versions; "+
				"versions sort lexicographically, so a sequential version would run before every timestamped one",
			migrationsDir)
	case sequentials > 0:
		return SequentialVersions, true, nil
	case timestamps > 0:
		return TimestampVersions, true, nil
	default:
		return "", false, nil
	}
}

// NextVersion returns the version string for the next migration in
// migrationsDir. The scheme already present in the directory always wins, so a
// project never mixes the two; preferred applies only to the first migration.
func NextVersion(migrationsDir string, preferred VersionScheme) (string, error) {
	scheme, found, err := DetectVersionScheme(migrationsDir)
	if err != nil {
		return "", err
	}
	if !found {
		scheme = preferred
		if scheme == "" {
			scheme = TimestampVersions
		}
	}

	if scheme == TimestampVersions {
		return GenerateVersion(), nil
	}

	versions, err := existingVersions(migrationsDir)
	if err != nil {
		return "", err
	}

	next, width := 0, defaultSequentialWidth
	for _, v := range versions {
		if len(v) > width {
			width = len(v)
		}
		n, convErr := strconv.Atoi(v)
		if convErr != nil {
			continue
		}
		if n >= next {
			next = n + 1
		}
	}

	if len(strconv.Itoa(next)) > width {
		return "", fmt.Errorf(
			"sequential migration %d does not fit the %d-digit numbering in %s; "+
				"versions sort lexicographically, so %d would run before %s. "+
				"Renumber the existing files to a wider padding first",
			next, width, migrationsDir, next, strings.Repeat("9", width))
	}

	return fmt.Sprintf("%0*d", width, next), nil
}

func existingVersions(migrationsDir string) ([]string, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	seen := make(map[string]struct{})
	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") && !strings.HasSuffix(name, ".down.sql") {
			continue
		}
		version, _, ok := strings.Cut(name, "_")
		if !ok {
			continue
		}
		if _, dup := seen[version]; dup {
			continue
		}
		seen[version] = struct{}{}
		versions = append(versions, version)
	}
	return versions, nil
}
