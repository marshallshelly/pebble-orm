package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marshallshelly/pebble-orm/cmd/pebble/output"
	"github.com/marshallshelly/pebble-orm/cmd/pebble/tui"
	"github.com/marshallshelly/pebble-orm/pkg/migration"
	"github.com/spf13/cobra"
)

var (
	// Migrate flags
	dryRun      bool
	all         bool
	steps       int
	target      string
	interactive bool
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long: `Run database migrations to keep your database schema in sync with your code.

Subcommands:
  up      - Apply pending migrations
  down    - Rollback migrations
  status  - Show migration status`,
}

// migrateUpCmd applies pending migrations
var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply pending migrations",
	Long: `Apply pending migrations to update the database schema.

Examples:
  pebble migrate up --all              # Apply all pending migrations
  pebble migrate up --steps 1          # Apply next migration
  pebble migrate up --dry-run --all    # Preview migrations without applying`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrateUp()
	},
}

// migrateDownCmd rolls back migrations
var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback migrations",
	Long: `Rollback applied migrations to revert database schema changes.

Examples:
  pebble migrate down --steps 1        # Rollback last migration
  pebble migrate down --target VERSION # Rollback to specific version
  pebble migrate down --dry-run        # Preview rollback without executing`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrateDown()
	},
}

// migrateStatusCmd shows migration status
var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	Long: `Show the status of all migrations (pending, applied, failed).

Examples:
  pebble migrate status                # Show migration status
  pebble migrate status --json         # Output in JSON format`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrateStatus()
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.AddCommand(migrateUpCmd, migrateDownCmd, migrateStatusCmd)

	// Flags for migrate up
	migrateUpCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Run in interactive mode with TUI")
	migrateUpCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview migrations without applying")
	migrateUpCmd.Flags().BoolVar(&all, "all", false, "Apply all pending migrations")
	migrateUpCmd.Flags().IntVar(&steps, "steps", 0, "Number of migrations to apply")

	// Flags for migrate down
	migrateDownCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Run in interactive mode with TUI")
	migrateDownCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview rollback without executing")
	migrateDownCmd.Flags().IntVar(&steps, "steps", 1, "Number of migrations to rollback")
	migrateDownCmd.Flags().StringVar(&target, "target", "", "Rollback to specific version")
}

func runMigrateUp() error {
	if dbURL == "" {
		return fmt.Errorf("--db flag is required")
	}

	// Run interactive TUI if flag is set
	if interactive {
		return tui.RunMigrateUI("up", dbURL, migrationsDir)
	}

	ctx := context.Background()

	// Connect to database
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	// Create executor
	executor := migration.NewExecutor(pool, migrationsDir)

	// Initialize schema_migrations table
	if err := executor.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize migrations: %w", err)
	}

	// Acquire lock
	if !dryRun {
		if err := executor.Lock(ctx); err != nil {
			return fmt.Errorf("failed to acquire migration lock: %w", err)
		}
		defer func() { _ = executor.Unlock(ctx) }()
	}

	// Load migrations
	generator := migration.NewGenerator(migrationsDir)
	migrationFiles, err := generator.ListMigrations()
	if err != nil {
		return fmt.Errorf("failed to list migrations: %w", err)
	}

	if len(migrationFiles) == 0 {
		output.Warning("No migrations found")
		return nil
	}

	// Read migration content
	var migrations []migration.Migration
	for _, file := range migrationFiles {
		mig, err := generator.ReadMigration(file)
		if err != nil {
			return fmt.Errorf("failed to read migration: %w", err)
		}
		migrations = append(migrations, *mig)
	}

	// Determine which migrations to apply
	var toApply []migration.Migration
	if all {
		toApply = migrations
	} else if steps > 0 {
		// Get applied migrations
		applied, err := executor.GetAppliedMigrations(ctx)
		if err != nil {
			return fmt.Errorf("failed to get applied migrations: %w", err)
		}
		appliedMap := make(map[string]bool)
		for _, m := range applied {
			appliedMap[m.Version] = true
		}

		// Find first N pending migrations
		for _, mig := range migrations {
			if !appliedMap[mig.Version] {
				toApply = append(toApply, mig)
				if len(toApply) >= steps {
					break
				}
			}
		}
	} else {
		return fmt.Errorf("must specify --all or --steps")
	}

	if len(toApply) == 0 {
		output.Info("No pending migrations")
		return nil
	}

	// Preview
	if dryRun {
		output.Section("DRY RUN - Preview")
		output.Info("The following migrations would be applied:")
		for _, mig := range toApply {
			fmt.Printf("  %s %s - %s\n", output.StatusIcon("pending"), mig.Version, mig.Name)
		}
		return nil
	}

	// Apply migrations
	output.Section("Applying Migrations")
	for _, mig := range toApply {
		output.Info("Applying %s - %s...", mig.Version, mig.Name)
		if err := executor.Apply(ctx, mig, false); err != nil {
			output.Error("Failed to apply migration %s: %v", mig.Version, err)
			return fmt.Errorf("failed to apply migration %s: %w", mig.Version, err)
		}
		output.Success("Applied %s", mig.Version)
	}

	fmt.Println()
	output.Success("Successfully applied %d migration(s)", len(toApply))
	return nil
}

func runMigrateDown() error {
	if dbURL == "" {
		return fmt.Errorf("--db flag is required")
	}

	// Run interactive TUI if flag is set
	if interactive {
		return tui.RunMigrateUI("down", dbURL, migrationsDir)
	}

	ctx := context.Background()

	// Connect to database
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	// Create executor
	executor := migration.NewExecutor(pool, migrationsDir)

	// Acquire lock
	if !dryRun {
		if err := executor.Lock(ctx); err != nil {
			return fmt.Errorf("failed to acquire migration lock: %w", err)
		}
		defer func() { _ = executor.Unlock(ctx) }()
	}

	// Load migrations
	generator := migration.NewGenerator(migrationsDir)
	migrationFiles, err := generator.ListMigrations()
	if err != nil {
		return fmt.Errorf("failed to list migrations: %w", err)
	}

	// Read migration content
	var migrations []migration.Migration
	for _, file := range migrationFiles {
		mig, err := generator.ReadMigration(file)
		if err != nil {
			return fmt.Errorf("failed to read migration: %w", err)
		}
		migrations = append(migrations, *mig)
	}

	// Handle target version rollback
	if target != "" {
		if dryRun {
			output.Info("DRY RUN - Would rollback to version %s", target)
			return nil
		}

		output.Section("Rolling Back to Target Version")
		if err := executor.RollbackTo(ctx, target, migrations, false); err != nil {
			output.Error("Failed to rollback to %s: %v", target, err)
			return fmt.Errorf("failed to rollback to %s: %w", target, err)
		}
		output.Success("Rolled back to version %s", target)
		return nil
	}

	// Get applied migrations
	applied, err := executor.GetAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	if len(applied) == 0 {
		output.Info("No migrations to rollback")
		return nil
	}

	// Rollback last N migrations
	toRollback := min(steps, len(applied))

	// Preview
	if dryRun {
		output.Section("DRY RUN - Preview")
		output.Info("The following migrations would be rolled back:")
		for i := len(applied) - 1; i >= len(applied)-toRollback; i-- {
			fmt.Printf("  %s %s - %s\n", output.StatusIcon("applied"), applied[i].Version, applied[i].Name)
		}
		return nil
	}

	// Rollback migrations
	output.Section("Rolling Back Migrations")
	migrationMap := make(map[string]migration.Migration)
	for _, m := range migrations {
		migrationMap[m.Version] = m
	}

	for i := range toRollback {
		idx := len(applied) - 1 - i
		record := applied[idx]

		mig, exists := migrationMap[record.Version]
		if !exists {
			return fmt.Errorf("migration file not found for version %s", record.Version)
		}

		output.Warning("Rolling back %s - %s...", mig.Version, mig.Name)
		if err := executor.Rollback(ctx, mig, false); err != nil {
			output.Error("Failed to rollback migration %s: %v", mig.Version, err)
			return fmt.Errorf("failed to rollback migration %s: %w", mig.Version, err)
		}
		output.Success("Rolled back %s", mig.Version)
	}

	fmt.Println()
	output.Success("Successfully rolled back %d migration(s)", toRollback)
	return nil
}

func runMigrateStatus() error {
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

	// Create executor
	executor := migration.NewExecutor(pool, migrationsDir)

	// Initialize schema_migrations table
	if err := executor.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize migrations: %w", err)
	}

	// Load migrations
	generator := migration.NewGenerator(migrationsDir)
	migrationFiles, err := generator.ListMigrations()
	if err != nil {
		return fmt.Errorf("failed to list migrations: %w", err)
	}

	if len(migrationFiles) == 0 {
		output.Warning("No migrations found")
		return nil
	}

	// Read migration content
	var migrations []migration.Migration
	for _, file := range migrationFiles {
		mig, err := generator.ReadMigration(file)
		if err != nil {
			return fmt.Errorf("failed to read migration: %w", err)
		}
		migrations = append(migrations, *mig)
	}

	// Get status
	status, err := executor.GetStatus(ctx, migrations)
	if err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}

	// Output
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "VERSION\tNAME\tSTATUS\tAPPLIED AT")
	_, _ = fmt.Fprintln(w, "-------\t----\t------\t----------")

	for _, record := range status {
		appliedAt := "N/A"
		if record.AppliedAt != nil {
			appliedAt = record.AppliedAt.Format("2006-01-02 15:04:05")
		}

		statusSymbol := output.StatusIcon(string(record.Status))
		statusText := string(record.Status)

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s %s\t%s\n",
			record.Version,
			record.Name,
			statusSymbol,
			statusText,
			appliedAt,
		)
	}
	_ = w.Flush()

	// Summary
	pending := 0
	applied := 0
	failed := 0
	for _, record := range status {
		switch record.Status {
		case migration.StatusPending:
			pending++
		case migration.StatusApplied:
			applied++
		case migration.StatusFailed:
			failed++
		}
	}

	fmt.Printf("\nSummary: %d applied, %d pending", applied, pending)
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	fmt.Println()

	return nil
}
