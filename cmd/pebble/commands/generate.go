package commands

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marshallshelly/pebble-orm/cmd/pebble/output"
	"github.com/marshallshelly/pebble-orm/pkg/migration"
	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/spf13/cobra"
)

var (
	// Generate flags
	migrationName string
	empty         bool
	modelsPath    string
)

// generateCmd generates migration files
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate migration files",
	Long: `Generate migration files by comparing Go struct definitions with the database schema.

The command introspects the database, compares it with registered models,
and generates timestamped up/down SQL migration files.

Examples:
  pebble generate --name add_users_table    # Generate from schema diff
  pebble generate --name custom_sql --empty # Generate empty migration for manual editing`,
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

	// Generate from schema diff
	if dbURL == "" {
		return fmt.Errorf("--db flag is required for schema diff")
	}

	if modelsPath == "" {
		return fmt.Errorf("--models flag is required to specify model definitions")
	}

	ctx := context.Background()

	// Connect to database
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	// TODO: Load models from file specified in --models flag
	// For now, we'll use the global registry
	// In a real implementation, you would parse the Go file and register models
	codeSchema := registry.AllTables()

	if len(codeSchema) == 0 {
		output.Warning("No models registered. Use --models to specify model definitions.")
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

	// Introspect database
	introspector := migration.NewIntrospector(pool)
	dbSchema, err := introspector.IntrospectSchema(ctx)
	if err != nil {
		return fmt.Errorf("failed to introspect database: %w", err)
	}

	// Compare schemas
	differ := migration.NewDiffer()
	diff := differ.Compare(codeSchema, dbSchema)

	// Check if there are changes
	if !diff.HasChanges() {
		output.Info("No schema changes detected. Database is in sync with models.")
		return nil
	}

	// Show summary of changes
	output.Section("Detected Schema Changes")
	if len(diff.TablesAdded) > 0 {
		output.Success("Tables to add: %d", len(diff.TablesAdded))
		for _, table := range diff.TablesAdded {
			fmt.Printf("    + %s\n", table.Name)
		}
	}
	if len(diff.TablesDropped) > 0 {
		output.Warning("Tables to drop: %d", len(diff.TablesDropped))
		for _, tableName := range diff.TablesDropped {
			fmt.Printf("    - %s\n", tableName)
		}
	}
	if len(diff.TablesModified) > 0 {
		output.Info("Tables to modify: %d", len(diff.TablesModified))
		for _, tableDiff := range diff.TablesModified {
			fmt.Printf("    ~ %s\n", tableDiff.TableName)
			if len(tableDiff.ColumnsAdded) > 0 {
				fmt.Printf("      + %d column(s)\n", len(tableDiff.ColumnsAdded))
			}
			if len(tableDiff.ColumnsDropped) > 0 {
				fmt.Printf("      - %d column(s)\n", len(tableDiff.ColumnsDropped))
			}
			if len(tableDiff.ColumnsModified) > 0 {
				fmt.Printf("      ~ %d column(s)\n", len(tableDiff.ColumnsModified))
			}
		}
	}

	// Generate migration
	migrationFile, err := generator.Generate(migrationName, diff)
	if err != nil {
		return fmt.Errorf("failed to generate migration: %w", err)
	}

	fmt.Println()
	output.Success("Created migration: %s", migrationFile.Version)
	output.Muted("  Up:   %s", migrationFile.UpPath)
	output.Muted("  Down: %s", migrationFile.DownPath)
	fmt.Println()
	output.Info("Review the generated SQL files before applying the migration.")

	return nil
}
