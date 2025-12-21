package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marshallshelly/pebble-orm/cmd/pebble/output"
	"github.com/marshallshelly/pebble-orm/pkg/migration"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
	"github.com/spf13/cobra"
)

var (
	// Introspect flags
	tableName string
)

// introspectCmd introspects database schema
var introspectCmd = &cobra.Command{
	Use:   "introspect",
	Short: "Introspect database schema",
	Long: `Introspect the database schema and display table structures.

This command queries the PostgreSQL information_schema to extract
table definitions, including columns, indexes, foreign keys, and constraints.

Examples:
  pebble introspect                    # Show all tables
  pebble introspect --table users      # Show specific table
  pebble introspect --json             # Output in JSON format`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIntrospect()
	},
}

func init() {
	rootCmd.AddCommand(introspectCmd)

	introspectCmd.Flags().StringVarP(&tableName, "table", "t", "", "Specific table to introspect")
}

func runIntrospect() error {
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

	// Create introspector
	introspector := migration.NewIntrospector(pool)

	// Introspect specific table or all tables
	if tableName != "" {
		table, err := introspector.IntrospectTable(ctx, tableName)
		if err != nil {
			return fmt.Errorf("failed to introspect table %s: %w", tableName, err)
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(table)
		}

		printTable(table)
		return nil
	}

	// Introspect all tables
	schema, err := introspector.IntrospectSchema(ctx)
	if err != nil {
		return fmt.Errorf("failed to introspect schema: %w", err)
	}

	if len(schema) == 0 {
		output.Warning("No tables found in database")
		return nil
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(schema)
	}

	// Print summary
	output.Section(fmt.Sprintf("Database Schema (%d tables)", len(schema)))
	for _, table := range schema {
		printTableSummary(table)
		fmt.Println()
	}

	return nil
}

func printTable(table *schema.TableMetadata) {
	fmt.Printf("Table: %s\n", table.Name)
	fmt.Println(strings.Repeat("=", len(table.Name)+7))
	fmt.Println()

	// Columns
	fmt.Println("Columns:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tTYPE\tNULLABLE\tDEFAULT")
	_, _ = fmt.Fprintln(w, "----\t----\t--------\t-------")

	for _, col := range table.Columns {
		nullable := "NO"
		if col.Nullable {
			nullable = "YES"
		}

		defaultVal := "NULL"
		if col.Default != nil {
			defaultVal = *col.Default
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			col.Name,
			col.SQLType,
			nullable,
			defaultVal,
		)
	}
	_ = w.Flush()
	fmt.Println()

	// Primary key
	if table.PrimaryKey != nil {
		fmt.Printf("Primary Key: %s (%s)\n", table.PrimaryKey.Name, strings.Join(table.PrimaryKey.Columns, ", "))
		fmt.Println()
	}

	// Foreign keys
	if len(table.ForeignKeys) > 0 {
		fmt.Println("Foreign Keys:")
		for _, fk := range table.ForeignKeys {
			fmt.Printf("  %s: (%s) -> %s(%s)\n",
				fk.Name,
				strings.Join(fk.Columns, ", "),
				fk.ReferencedTable,
				strings.Join(fk.ReferencedColumns, ", "),
			)
			if fk.OnDelete != "" {
				fmt.Printf("    ON DELETE: %s\n", fk.OnDelete)
			}
			if fk.OnUpdate != "" {
				fmt.Printf("    ON UPDATE: %s\n", fk.OnUpdate)
			}
		}
		fmt.Println()
	}

	// Indexes
	if len(table.Indexes) > 0 {
		fmt.Println("Indexes:")
		for _, idx := range table.Indexes {
			unique := ""
			if idx.Unique {
				unique = "UNIQUE "
			}
			fmt.Printf("  %s%s (%s)\n", unique, idx.Name, strings.Join(idx.Columns, ", "))
		}
		fmt.Println()
	}

	// Constraints
	if len(table.Constraints) > 0 {
		fmt.Println("Constraints:")
		for _, c := range table.Constraints {
			fmt.Printf("  %s: %s\n", c.Name, c.Expression)
		}
	}
}

func printTableSummary(table *schema.TableMetadata) {
	fmt.Printf("Table: %s\n", table.Name)
	fmt.Printf("  Columns: %d\n", len(table.Columns))

	if table.PrimaryKey != nil {
		fmt.Printf("  Primary Key: %s\n", strings.Join(table.PrimaryKey.Columns, ", "))
	}

	if len(table.ForeignKeys) > 0 {
		fmt.Printf("  Foreign Keys: %d\n", len(table.ForeignKeys))
	}

	if len(table.Indexes) > 0 {
		fmt.Printf("  Indexes: %d\n", len(table.Indexes))
	}
}
