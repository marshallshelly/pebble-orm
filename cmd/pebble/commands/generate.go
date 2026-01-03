package commands

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marshallshelly/pebble-orm/cmd/pebble/output"
	"github.com/marshallshelly/pebble-orm/pkg/loader"
	"github.com/marshallshelly/pebble-orm/pkg/migration"
	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
	"github.com/spf13/cobra"
)

var (
	// Generate flags
	migrationName string
	empty         bool
	modelsPath    string
)

// globalRegistryWrapper wraps the global registry to implement loader.ModelRegistrar
type globalRegistryWrapper struct{}

func (globalRegistryWrapper) RegisterMetadata(table *schema.TableMetadata) error {
	return registry.RegisterMetadata(table)
}

// generateCmd generates migration files
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate migration files",
	Long: `Generate migration files by comparing Go struct definitions with the database schema.

The command introspects the database, compares it with registered models,
and generates timestamped up/down SQL migration files.

Examples:
  # Generate initial migration from models (no database required)
  pebble generate --name initial_schema --models ./internal/models

  # Generate migration by comparing with existing database
  pebble generate --name add_users_table --db "postgres://..." --models ./internal/models
  
  # Generate empty migration for manual SQL
  pebble generate --name custom_sql --empty`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenerate()
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringVarP(&migrationName, "name", "n", "", "Migration name (required)")
	generateCmd.Flags().BoolVar(&empty, "empty", false, "Generate empty migration for manual editing")
	generateCmd.Flags().StringVar(&modelsPath, "models", "", "Path to Go file with model definitions")
	_ = generateCmd.MarkFlagRequired("name")
}

func runGenerate() error {
	generator := migration.NewGenerator(migrationsDir)

	// Generate empty migration
	if empty {
		migrationFile, err := generator.GenerateEmpty(migrationName)
		if err != nil {
			return fmt.Errorf("failed to generate empty migration: %w", err)
		}

		output.Success("Created empty migration: %s", migrationFile.Version)
		output.Muted("  Up:   %s", migrationFile.UpPath)
		output.Muted("  Down: %s", migrationFile.DownPath)
		fmt.Println()
		output.Info("Edit the SQL files manually to add your migration logic.")
		return nil
	}

	// Require models path for schema-based generation
	if modelsPath == "" {
		return fmt.Errorf("--models flag is required to specify model definitions")
	}

	ctx := context.Background()

	// Load models from file or directory specified in --models flag
	output.Info("🔍 Scanning models from: %s", modelsPath)

	_, err := loader.LoadModelsFromPath(modelsPath, globalRegistryWrapper{})
	if err != nil {
		return fmt.Errorf("failed to load models: %w", err)
	}

	codeSchema := registry.AllTables()

	if len(codeSchema) == 0 {
		output.Warning("No models found in %s", modelsPath)
		output.Info("Generating empty migration instead.")

		migrationFile, err := generator.GenerateEmpty(migrationName)
		if err != nil {
			return fmt.Errorf("failed to generate empty migration: %w", err)
		}

		output.Success("Created empty migration: %s", migrationFile.Version)
		output.Muted("  Up:   %s", migrationFile.UpPath)
		output.Muted("  Down: %s", migrationFile.DownPath)
		return nil
	}

	// Show discovered models
	modelNames := make([]string, 0, len(codeSchema))
	for tableName := range codeSchema {
		modelNames = append(modelNames, tableName)
	}
	output.Success("  ✓ Found %d model(s): %v", len(codeSchema), modelNames)
	fmt.Println()

	// Determine database schema (empty if no --db provided)
	var dbSchema map[string]*schema.TableMetadata

	if dbURL != "" {
		// Connect to database and introspect
		output.Info("🔄 Comparing with database...")

		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer pool.Close()

		introspector := migration.NewIntrospector(pool)
		dbSchema, err = introspector.IntrospectSchema(ctx)
		if err != nil {
			return fmt.Errorf("failed to introspect database: %w", err)
		}

		output.Success("  ✓ Found %d table(s) in database", len(dbSchema))
		fmt.Println()
	} else {
		// No database connection - treat as empty database
		output.Info("🔄 Generating initial migration (no database connection)")
		fmt.Println()
		dbSchema = make(map[string]*schema.TableMetadata)
	}

	// Compare schemas
	differ := migration.NewDiffer()
	diff := differ.Compare(codeSchema, dbSchema)

	// Check if there are changes
	if !diff.HasChanges() {
		output.Success("✓ No schema changes detected. Database is in sync with models.")
		return nil
	}

	// Show summary of changes
	output.Section("📋 Schema Changes")
	if len(diff.TablesAdded) > 0 {
		for _, table := range diff.TablesAdded {
			output.Success("  + %s (new table)", table.Name)
		}
	}
	if len(diff.TablesModified) > 0 {
		for _, tableDiff := range diff.TablesModified {
			output.Info("  ~ %s (modified)", tableDiff.TableName)
			if len(tableDiff.ColumnsAdded) > 0 {
				fmt.Printf("      + %d column(s) added\n", len(tableDiff.ColumnsAdded))
			}
			if len(tableDiff.ColumnsDropped) > 0 {
				fmt.Printf("      - %d column(s) dropped\n", len(tableDiff.ColumnsDropped))
			}
			if len(tableDiff.ColumnsModified) > 0 {
				fmt.Printf("      ~ %d column(s) modified\n", len(tableDiff.ColumnsModified))
			}
		}
	}
	if len(diff.TablesDropped) > 0 {
		for _, tableName := range diff.TablesDropped {
			output.Warning("  - %s (dropped)", tableName)
		}
	}
	fmt.Println()

	// Generate migration
	migrationFile, err := generator.Generate(migrationName, diff)
	if err != nil {
		return fmt.Errorf("failed to generate migration: %w", err)
	}

	output.Success("✅ Generated migration: %s", migrationFile.Version)
	output.Muted("  ↑ Up:   %s", migrationFile.UpPath)
	output.Muted("  ↓ Down: %s", migrationFile.DownPath)
	fmt.Println()
	output.Info("💡 Review the generated SQL files before applying the migration.")

	return nil
}
