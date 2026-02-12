package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marshallshelly/pebble-orm/cmd/pebble/output"
	"github.com/marshallshelly/pebble-orm/pkg/migration"
	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/spf13/cobra"
)

var (
	// Diff flags
	outputFile string
)

// diffCmd shows schema differences
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show schema differences",
	Long: `Compare Go struct definitions with the database schema and show differences.

This command helps you understand what migrations would be generated
before actually creating migration files.

Examples:
  pebble diff                          # Show schema differences
  pebble diff --json                   # Output in JSON format
  pebble diff --output migration.sql   # Save SQL to file`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDiff()
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)

	diffCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output SQL to file")
}

func runDiff() error {
	if dbURL == "" {
		return fmt.Errorf("--db flag is required")
	}

	ctx := context.Background()

	// Connect to database
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	// Get code schema from registry
	codeSchema := registry.AllTables()

	if len(codeSchema) == 0 {
		return fmt.Errorf("no models registered - use registry.Register() to register your models")
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
		output.Success("No schema changes detected. Database is in sync with models.")
		return nil
	}

	// Output JSON
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(diff)
	}

	// Generate SQL
	planner := migration.NewPlanner()
	upSQL, downSQL := planner.GenerateMigration(diff)

	// Show summary
	output.Section("Schema Differences")

	if len(diff.TablesAdded) > 0 {
		output.Success("Tables to add (%d):", len(diff.TablesAdded))
		for _, table := range diff.TablesAdded {
			fmt.Printf("  + %s (%d columns)\n", table.Name, len(table.Columns))
		}
		fmt.Println()
	}

	if len(diff.TablesDropped) > 0 {
		output.Warning("Tables to drop (%d):", len(diff.TablesDropped))
		for _, tableName := range diff.TablesDropped {
			fmt.Printf("  - %s\n", tableName)
		}
		fmt.Println()
	}

	if len(diff.TablesModified) > 0 {
		output.Info("Tables to modify (%d):", len(diff.TablesModified))
		for _, tableDiff := range diff.TablesModified {
			fmt.Printf("  ~ %s\n", tableDiff.TableName)

			if len(tableDiff.ColumnsAdded) > 0 {
				for _, col := range tableDiff.ColumnsAdded {
					fmt.Printf("      + column: %s %s\n", col.Name, col.SQLType)
				}
			}

			if len(tableDiff.ColumnsDropped) > 0 {
				for _, colName := range tableDiff.ColumnsDropped {
					fmt.Printf("      - column: %s\n", colName)
				}
			}

			if len(tableDiff.ColumnsModified) > 0 {
				for _, colDiff := range tableDiff.ColumnsModified {
					changes := []string{}
					if colDiff.TypeChanged {
						changes = append(changes, fmt.Sprintf("type: %s -> %s", colDiff.OldColumn.SQLType, colDiff.NewColumn.SQLType))
					}
					if colDiff.NullChanged {
						oldNull := "NULL"
						if !colDiff.OldColumn.Nullable {
							oldNull = "NOT NULL"
						}
						newNull := "NULL"
						if !colDiff.NewColumn.Nullable {
							newNull = "NOT NULL"
						}
						changes = append(changes, fmt.Sprintf("%s -> %s", oldNull, newNull))
					}
					if colDiff.DefaultChanged {
						oldDefault := "no default"
						if colDiff.OldColumn.Default != nil {
							oldDefault = *colDiff.OldColumn.Default
						}
						newDefault := "no default"
						if colDiff.NewColumn.Default != nil {
							newDefault = *colDiff.NewColumn.Default
						}
						changes = append(changes, fmt.Sprintf("default: %s -> %s", oldDefault, newDefault))
					}
					fmt.Printf("      ~ column: %s (%s)\n", colDiff.ColumnName, joinStrings(changes, ", "))
				}
			}

			if len(tableDiff.IndexesAdded) > 0 {
				for _, idx := range tableDiff.IndexesAdded {
					fmt.Printf("      + index: %s\n", idx.Name)
				}
			}

			if len(tableDiff.IndexesDropped) > 0 {
				for _, idxName := range tableDiff.IndexesDropped {
					fmt.Printf("      - index: %s\n", idxName)
				}
			}

			if len(tableDiff.ForeignKeysAdded) > 0 {
				for _, fk := range tableDiff.ForeignKeysAdded {
					fmt.Printf("      + foreign key: %s\n", fk.Name)
				}
			}

			if len(tableDiff.ForeignKeysDropped) > 0 {
				for _, fkName := range tableDiff.ForeignKeysDropped {
					fmt.Printf("      - foreign key: %s\n", fkName)
				}
			}

			if tableDiff.PrimaryKeyChanged != nil {
				fmt.Printf("      ~ primary key changed\n")
			}
		}
		fmt.Println()
	}

	// Show SQL preview
	output.Section("Migration SQL (UP)")
	fmt.Println(upSQL)

	if verbose {
		output.Section("Migration SQL (DOWN)")
		fmt.Println(downSQL)
	}

	// Save to file if requested
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(upSQL), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Println()
		output.Success("SQL saved to: %s", outputFile)
	}

	return nil
}

func joinStrings(strs []string, sep string) string {
	var result strings.Builder
	for i, s := range strs {
		if i > 0 {
			result.WriteString(sep)
		}
		result.WriteString(s)
	}
	return result.String()
}
