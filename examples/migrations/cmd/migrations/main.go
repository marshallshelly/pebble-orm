package main

import (
	"context"
	"fmt"
	"log"

	"github.com/marshallshelly/pebble-orm/examples/migrations/internal/database"
	"github.com/marshallshelly/pebble-orm/pkg/migration"
	"github.com/marshallshelly/pebble-orm/pkg/registry"
)

func main() {
	ctx := context.Background()

	log.Println("=== Migrations & Schema Management Example ===\n")

	// Connect to database
	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	log.Println("✅ Connected to database\n")

	// Example 1: Introspect current database schema
	fmt.Println("--- Example 1: Schema Introspection ---")
	introspector := migration.NewIntrospector(db.Pool())
	dbSchema, err := introspector.IntrospectSchema(ctx)
	if err != nil {
		log.Printf("Introspection error: %v\n", err)
	} else {
		fmt.Printf("✅ Found %d tables in database\n", len(dbSchema))
		for tableName := range dbSchema {
			fmt.Printf("  - %s\n", tableName)
		}
	}

	// Example 2: Get code schema from registered models
	fmt.Println("\n--- Example 2: Code Schema (from structs) ---")
	codeSchema := registry.AllTables()
	fmt.Printf("✅ Found %d models registered\n", len(codeSchema))
	for tableName, table := range codeSchema {
		fmt.Printf("  - %s (%d columns)\n", tableName, len(table.Columns))
	}

	// Example 3: Compare schemas and generate diff
	fmt.Println("\n--- Example 3: Schema Diff ---")
	differ := migration.NewDiffer()
	diff := differ.Compare(dbSchema, codeSchema)

	if !diff.HasChanges() {
		fmt.Println("✅ Database schema matches code schema (no changes)")
	} else {
		fmt.Println("⚠️  Schema differences detected:")
		if len(diff.TablesAdded) > 0 {
			fmt.Printf("  Tables to add: %d\n", len(diff.TablesAdded))
			for _, table := range diff.TablesAdded {
				fmt.Printf("    + %s\n", table.Name)
			}
		}
		if len(diff.TablesDropped) > 0 {
			fmt.Printf("  Tables to drop: %d\n", len(diff.TablesDropped))
			for _, name := range diff.TablesDropped {
				fmt.Printf("    - %s\n", name)
			}
		}
		if len(diff.TablesModified) > 0 {
			fmt.Printf("  Tables to modify: %d\n", len(diff.TablesModified))
			for _, tableDiff := range diff.TablesModified {
				fmt.Printf("    ~ %s\n", tableDiff.TableName)
			}
		}
	}

	// Example 4: Generate migration SQL (Safe by Default)
	fmt.Println("\n--- Example 4: Safe Migration SQL Generation ---")
	if diff.HasChanges() {
		// Default: Safe migrations with IF NOT EXISTS
		planner := migration.NewPlanner()
		upSQL, downSQL := planner.GenerateMigration(diff)

		fmt.Println("✅ Generated SAFE migration SQL (idempotent):\n")
		fmt.Println("UP Migration (apply changes - safe to run multiple times):")
		fmt.Println(upSQL)
		fmt.Println("\nDOWN Migration (revert changes):")
		fmt.Println(downSQL)

		fmt.Println("\n💡 Note: Migrations include 'IF NOT EXISTS' by default")
		fmt.Println("   This makes them safe to run multiple times without errors!")
	}

	// Example 4b: Custom migration options (optional)
	fmt.Println("\n--- Example 4b: Custom Migration Options ---")
	if diff.HasChanges() {
		// For strict migrations (fail if table exists)
		strictPlanner := migration.NewPlannerWithOptions(migration.PlannerOptions{
			IfNotExists: false, // Disable IF NOT EXISTS
		})
		strictSQL, _ := strictPlanner.GenerateMigration(diff)

		fmt.Println("Strict mode (without IF NOT EXISTS):")
		fmt.Println(strictSQL)
		fmt.Println("\n⚠️  Strict mode will fail if tables already exist")
	}

	// Example 5: Migration file generation
	fmt.Println("\n--- Example 5: Migration File Generation ---")
	generator := migration.NewGenerator("./migrations")

	// Create empty migration
	migrationName := "add_products_and_categories"
	file, err := generator.GenerateEmpty(migrationName)
	if err != nil {
		log.Printf("Failed to create migration: %v\n", err)
	} else {
		fmt.Printf("✅ Created migration files:\n")
		fmt.Printf("  - %s\n", file.UpPath)
		fmt.Printf("  - %s\n", file.DownPath)
	}

	// Example 6: List existing migrations
	fmt.Println("\n--- Example 6: List Migrations ---")
	migrations, err := generator.ListMigrations()
	if err != nil {
		log.Printf("Failed to list migrations: %v\n", err)
	} else {
		if len(migrations) == 0 {
			fmt.Println("No migrations found in ./migrations directory")
		} else {
			fmt.Printf("✅ Found %d migrations:\n", len(migrations))
			for _, m := range migrations {
				fmt.Printf("  - %s\n", m)
			}
		}
	}

	log.Println("\n✅ Migration examples completed!")
	log.Println("\nKey Takeaways:")
	log.Println("  - Introspect database to get current schema")
	log.Println("  - Compare DB schema with code schema to detect changes")
	log.Println("  - Generate migration SQL automatically")
	log.Println("  - Migrations are SAFE by default (IF NOT EXISTS)")
	log.Println("  - Safe to run migrations multiple times without errors")
	log.Println("  - Create timestamped migration files")
	log.Println("  - Use 'pebble' CLI for production migrations")
}
