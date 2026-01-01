package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func TestGenerateVersion(t *testing.T) {
	version := GenerateVersion()

	// Should be 14 characters (YYYYMMDDHHmmss)
	if len(version) != 14 {
		t.Errorf("Expected version length 14, got %d", len(version))
	}

	// Should be numeric
	for _, c := range version {
		if c < '0' || c > '9' {
			t.Errorf("Expected numeric version, got %s", version)
			break
		}
	}
}

func TestGenerateFileName(t *testing.T) {
	tests := []struct {
		version   string
		name      string
		direction string
		expected  string
	}{
		{"20240101120000", "create_users", "up", "20240101120000_create_users.up.sql"},
		{"20240101120000", "create_users", "down", "20240101120000_create_users.down.sql"},
		{"20240215153045", "add_email_index", "up", "20240215153045_add_email_index.up.sql"},
	}

	for _, test := range tests {
		result := GenerateFileName(test.version, test.name, test.direction)
		if result != test.expected {
			t.Errorf("GenerateFileName(%s, %s, %s) = %s, expected %s",
				test.version, test.name, test.direction, result, test.expected)
		}
	}
}

func TestGeneratorGenerate(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "pebble-migrations-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	generator := NewGenerator(tmpDir)

	// Create a simple schema diff
	diff := &SchemaDiff{
		TablesAdded: []schema.TableMetadata{
			{
				Name: "users",
				Columns: []schema.ColumnMetadata{
					{Name: "id", SQLType: "serial", Nullable: false},
					{Name: "email", SQLType: "varchar(255)", Nullable: false},
				},
				PrimaryKey: &schema.PrimaryKeyMetadata{
					Name:    "users_pkey",
					Columns: []string{"id"},
				},
			},
		},
	}

	// Generate migration
	migrationFile, err := generator.Generate("create_users", diff)
	if err != nil {
		t.Fatalf("Failed to generate migration: %v", err)
	}

	// Check migration file structure
	if migrationFile.Name != "create_users" {
		t.Errorf("Expected name 'create_users', got %s", migrationFile.Name)
	}
	if len(migrationFile.Version) != 14 {
		t.Errorf("Expected version length 14, got %d", len(migrationFile.Version))
	}

	// Check up file exists
	if _, err := os.Stat(migrationFile.UpPath); os.IsNotExist(err) {
		t.Errorf("Up migration file does not exist: %s", migrationFile.UpPath)
	}

	// Check down file exists
	if _, err := os.Stat(migrationFile.DownPath); os.IsNotExist(err) {
		t.Errorf("Down migration file does not exist: %s", migrationFile.DownPath)
	}

	// Read up migration content
	upContent, err := os.ReadFile(migrationFile.UpPath)
	if err != nil {
		t.Fatalf("Failed to read up migration: %v", err)
	}

	upSQL := string(upContent)
	if !strings.Contains(upSQL, "CREATE TABLE IF NOT EXISTS users") {
		t.Errorf("Expected CREATE TABLE in up migration, got: %s", upSQL)
	}

	// Read down migration content
	downContent, err := os.ReadFile(migrationFile.DownPath)
	if err != nil {
		t.Fatalf("Failed to read down migration: %v", err)
	}

	downSQL := string(downContent)
	if !strings.Contains(downSQL, `DROP TABLE IF EXISTS "users"`) {
		t.Errorf("Expected DROP TABLE in down migration, got: %s", downSQL)
	}
}

func TestGeneratorGenerateEmpty(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "pebble-migrations-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	generator := NewGenerator(tmpDir)

	// Generate empty migration
	migrationFile, err := generator.GenerateEmpty("add_custom_logic")
	if err != nil {
		t.Fatalf("Failed to generate empty migration: %v", err)
	}

	// Check migration file structure
	if migrationFile.Name != "add_custom_logic" {
		t.Errorf("Expected name 'add_custom_logic', got %s", migrationFile.Name)
	}

	// Read up migration content
	upContent, err := os.ReadFile(migrationFile.UpPath)
	if err != nil {
		t.Fatalf("Failed to read up migration: %v", err)
	}

	upSQL := string(upContent)
	if !strings.Contains(upSQL, "-- Migration: add_custom_logic") {
		t.Errorf("Expected migration comment, got: %s", upSQL)
	}
	if !strings.Contains(upSQL, "-- Write your UP migration here") {
		t.Errorf("Expected placeholder comment, got: %s", upSQL)
	}

	// Read down migration content
	downContent, err := os.ReadFile(migrationFile.DownPath)
	if err != nil {
		t.Fatalf("Failed to read down migration: %v", err)
	}

	downSQL := string(downContent)
	if !strings.Contains(downSQL, "-- Write your DOWN migration here") {
		t.Errorf("Expected placeholder comment, got: %s", downSQL)
	}
}

func TestGeneratorListMigrations(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "pebble-migrations-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	generator := NewGenerator(tmpDir)

	// Create some migration files
	migrations := []struct {
		version string
		name    string
	}{
		{"20240101120000", "create_users"},
		{"20240102140000", "create_posts"},
		{"20240103160000", "add_indexes"},
	}

	for _, m := range migrations {
		upFile := filepath.Join(tmpDir, GenerateFileName(m.version, m.name, "up"))
		downFile := filepath.Join(tmpDir, GenerateFileName(m.version, m.name, "down"))

		if err := os.WriteFile(upFile, []byte("-- up"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(downFile, []byte("-- down"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// List migrations
	listed, err := generator.ListMigrations()
	if err != nil {
		t.Fatalf("Failed to list migrations: %v", err)
	}

	// Check count
	if len(listed) != 3 {
		t.Errorf("Expected 3 migrations, got %d", len(listed))
	}

	// Check order (should be sorted by version)
	if len(listed) >= 3 {
		if listed[0].Version != "20240101120000" {
			t.Errorf("Expected first migration version 20240101120000, got %s", listed[0].Version)
		}
		if listed[1].Version != "20240102140000" {
			t.Errorf("Expected second migration version 20240102140000, got %s", listed[1].Version)
		}
		if listed[2].Version != "20240103160000" {
			t.Errorf("Expected third migration version 20240103160000, got %s", listed[2].Version)
		}
	}

	// Check names
	if len(listed) >= 1 && listed[0].Name != "create_users" {
		t.Errorf("Expected name 'create_users', got %s", listed[0].Name)
	}
}

func TestGeneratorListMigrationsEmptyDir(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "pebble-migrations-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	generator := NewGenerator(tmpDir)

	// List migrations from empty directory
	listed, err := generator.ListMigrations()
	if err != nil {
		t.Fatalf("Failed to list migrations: %v", err)
	}

	if len(listed) != 0 {
		t.Errorf("Expected 0 migrations, got %d", len(listed))
	}
}

func TestGeneratorListMigrationsNonExistentDir(t *testing.T) {
	generator := NewGenerator("/nonexistent/directory")

	// List migrations from non-existent directory
	listed, err := generator.ListMigrations()
	if err != nil {
		t.Fatalf("Expected no error for non-existent directory, got: %v", err)
	}

	if len(listed) != 0 {
		t.Errorf("Expected 0 migrations, got %d", len(listed))
	}
}

func TestGeneratorListMigrationsIncompletePair(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "pebble-migrations-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	generator := NewGenerator(tmpDir)

	// Create only up file (missing down file)
	upFile := filepath.Join(tmpDir, "20240101120000_incomplete.up.sql")
	if err := os.WriteFile(upFile, []byte("-- up"), 0644); err != nil {
		t.Fatal(err)
	}

	// List migrations
	listed, err := generator.ListMigrations()
	if err != nil {
		t.Fatalf("Failed to list migrations: %v", err)
	}

	// Should not include incomplete migration
	if len(listed) != 0 {
		t.Errorf("Expected 0 migrations (incomplete pair), got %d", len(listed))
	}
}

func TestGeneratorReadMigration(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "pebble-migrations-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	generator := NewGenerator(tmpDir)

	// Create migration files
	version := "20240101120000"
	name := "test_migration"
	upContent := "CREATE TABLE test (id serial);"
	downContent := "DROP TABLE test;"

	upFile := filepath.Join(tmpDir, GenerateFileName(version, name, "up"))
	downFile := filepath.Join(tmpDir, GenerateFileName(version, name, "down"))

	if err := os.WriteFile(upFile, []byte(upContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(downFile, []byte(downContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Read migration
	migrationFile := MigrationFile{
		Version:  version,
		Name:     name,
		UpPath:   upFile,
		DownPath: downFile,
	}

	migration, err := generator.ReadMigration(migrationFile)
	if err != nil {
		t.Fatalf("Failed to read migration: %v", err)
	}

	// Check content
	if migration.Version != version {
		t.Errorf("Expected version %s, got %s", version, migration.Version)
	}
	if migration.Name != name {
		t.Errorf("Expected name %s, got %s", name, migration.Name)
	}
	if migration.UpSQL != upContent {
		t.Errorf("Expected up SQL %s, got %s", upContent, migration.UpSQL)
	}
	if migration.DownSQL != downContent {
		t.Errorf("Expected down SQL %s, got %s", downContent, migration.DownSQL)
	}
}
