package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Generator generates migration files.
type Generator struct {
	migrationsDir string
}

// NewGenerator creates a new migration file generator.
func NewGenerator(migrationsDir string) *Generator {
	return &Generator{
		migrationsDir: migrationsDir,
	}
}

// Generate creates migration files from a schema diff.
func (g *Generator) Generate(name string, diff *SchemaDiff) (*MigrationFile, error) {
	// Ensure migrations directory exists
	if err := os.MkdirAll(g.migrationsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Generate version
	version := GenerateVersion()

	// Generate SQL
	planner := NewPlanner()
	upSQL, downSQL := planner.GenerateMigration(diff)

	// Create migration file
	migrationFile := &MigrationFile{
		Version:  version,
		Name:     name,
		UpPath:   filepath.Join(g.migrationsDir, GenerateFileName(version, name, "up")),
		DownPath: filepath.Join(g.migrationsDir, GenerateFileName(version, name, "down")),
	}

	// Write up migration
	if err := g.writeFile(migrationFile.UpPath, upSQL); err != nil {
		return nil, fmt.Errorf("failed to write up migration: %w", err)
	}

	// Write down migration
	if err := g.writeFile(migrationFile.DownPath, downSQL); err != nil {
		return nil, fmt.Errorf("failed to write down migration: %w", err)
	}

	return migrationFile, nil
}

// GenerateEmpty creates empty migration files for manual editing.
func (g *Generator) GenerateEmpty(name string) (*MigrationFile, error) {
	// Ensure migrations directory exists
	if err := os.MkdirAll(g.migrationsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create migrations directory: %w", err)
	}

	version := GenerateVersion()

	migrationFile := &MigrationFile{
		Version:  version,
		Name:     name,
		UpPath:   filepath.Join(g.migrationsDir, GenerateFileName(version, name, "up")),
		DownPath: filepath.Join(g.migrationsDir, GenerateFileName(version, name, "down")),
	}

	// Write empty files with comments
	upSQL := fmt.Sprintf("-- Migration: %s\n-- Created at: %s\n\n-- Write your UP migration here\n", name, version)
	downSQL := fmt.Sprintf("-- Migration: %s\n-- Created at: %s\n\n-- Write your DOWN migration here\n", name, version)

	if err := g.writeFile(migrationFile.UpPath, upSQL); err != nil {
		return nil, fmt.Errorf("failed to write up migration: %w", err)
	}

	if err := g.writeFile(migrationFile.DownPath, downSQL); err != nil {
		return nil, fmt.Errorf("failed to write down migration: %w", err)
	}

	return migrationFile, nil
}

// ListMigrations lists all migration files in the migrations directory.
func (g *Generator) ListMigrations() ([]MigrationFile, error) {
	// Read directory
	entries, err := os.ReadDir(g.migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []MigrationFile{}, nil
		}
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Group files by version
	fileMap := make(map[string]*MigrationFile)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()

		// Parse filename: {version}_{name}.{direction}.sql
		parts := strings.SplitN(fileName, "_", 2)
		if len(parts) != 2 {
			continue
		}

		version := parts[0]
		rest := parts[1]

		// Extract name and direction
		if before, ok := strings.CutSuffix(rest, ".up.sql"); ok {
			name := before
			if _, exists := fileMap[version]; !exists {
				fileMap[version] = &MigrationFile{Version: version, Name: name}
			}
			fileMap[version].UpPath = filepath.Join(g.migrationsDir, fileName)
		} else if before, ok := strings.CutSuffix(rest, ".down.sql"); ok {
			name := before
			if _, exists := fileMap[version]; !exists {
				fileMap[version] = &MigrationFile{Version: version, Name: name}
			}
			fileMap[version].DownPath = filepath.Join(g.migrationsDir, fileName)
		}
	}

	// Convert map to sorted slice
	var migrations []MigrationFile
	for _, mf := range fileMap {
		// Only include migrations that have both up and down files
		if mf.UpPath != "" && mf.DownPath != "" {
			migrations = append(migrations, *mf)
		}
	}

	// Sort by version
	for i := 0; i < len(migrations); i++ {
		for j := i + 1; j < len(migrations); j++ {
			if migrations[i].Version > migrations[j].Version {
				migrations[i], migrations[j] = migrations[j], migrations[i]
			}
		}
	}

	return migrations, nil
}

// ReadMigration reads the SQL content from a migration file.
func (g *Generator) ReadMigration(file MigrationFile) (*Migration, error) {
	upSQL, err := g.readFile(file.UpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read up migration: %w", err)
	}

	downSQL, err := g.readFile(file.DownPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read down migration: %w", err)
	}

	return &Migration{
		Version: file.Version,
		Name:    file.Name,
		UpSQL:   upSQL,
		DownSQL: downSQL,
	}, nil
}

// writeFile writes content to a file.
func (g *Generator) writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// readFile reads content from a file.
func (g *Generator) readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
