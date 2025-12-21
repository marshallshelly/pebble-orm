package migration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Executor executes and tracks database migrations.
type Executor struct {
	pool          *pgxpool.Pool
	migrationsDir string
	lockID        int64 // PostgreSQL advisory lock ID
}

// NewExecutor creates a new migration executor.
func NewExecutor(pool *pgxpool.Pool, migrationsDir string) *Executor {
	return &Executor{
		pool:          pool,
		migrationsDir: migrationsDir,
		lockID:        1234567890, // Default lock ID
	}
}

// WithLockID sets a custom advisory lock ID.
func (e *Executor) WithLockID(lockID int64) *Executor {
	e.lockID = lockID
	return e
}

// Initialize creates the schema_migrations table if it doesn't exist.
func (e *Executor) Initialize(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(14) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			applied_at TIMESTAMP,
			error TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_schema_migrations_status
		ON schema_migrations(status);
	`

	_, err := e.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	return nil
}

// Lock acquires an advisory lock to prevent concurrent migrations.
func (e *Executor) Lock(ctx context.Context) error {
	var acquired bool
	err := e.pool.QueryRow(ctx, "SELECT pg_advisory_lock($1)", e.lockID).Scan(&acquired)
	if err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	return nil
}

// Unlock releases the advisory lock.
func (e *Executor) Unlock(ctx context.Context) error {
	var released bool
	err := e.pool.QueryRow(ctx, "SELECT pg_advisory_unlock($1)", e.lockID).Scan(&released)
	if err != nil {
		return fmt.Errorf("failed to release migration lock: %w", err)
	}
	if !released {
		return fmt.Errorf("lock was not held")
	}
	return nil
}

// TryLock attempts to acquire an advisory lock without blocking.
func (e *Executor) TryLock(ctx context.Context) (bool, error) {
	var acquired bool
	err := e.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", e.lockID).Scan(&acquired)
	if err != nil {
		return false, fmt.Errorf("failed to try migration lock: %w", err)
	}
	return acquired, nil
}

// GetAppliedMigrations returns all migrations that have been applied.
func (e *Executor) GetAppliedMigrations(ctx context.Context) ([]MigrationRecord, error) {
	query := `
		SELECT version, name, status, applied_at, error
		FROM schema_migrations
		WHERE status = 'applied'
		ORDER BY version ASC
	`

	rows, err := e.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	var records []MigrationRecord
	for rows.Next() {
		var record MigrationRecord
		err := rows.Scan(&record.Version, &record.Name, &record.Status, &record.AppliedAt, &record.Error)
		if err != nil {
			return nil, fmt.Errorf("failed to scan migration record: %w", err)
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// GetAllMigrations returns all migration records.
func (e *Executor) GetAllMigrations(ctx context.Context) ([]MigrationRecord, error) {
	query := `
		SELECT version, name, status, applied_at, error
		FROM schema_migrations
		ORDER BY version ASC
	`

	rows, err := e.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	var records []MigrationRecord
	for rows.Next() {
		var record MigrationRecord
		err := rows.Scan(&record.Version, &record.Name, &record.Status, &record.AppliedAt, &record.Error)
		if err != nil {
			return nil, fmt.Errorf("failed to scan migration record: %w", err)
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// IsMigrationApplied checks if a specific migration has been applied.
func (e *Executor) IsMigrationApplied(ctx context.Context, version string) (bool, error) {
	var count int
	err := e.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM schema_migrations WHERE version = $1 AND status = 'applied'",
		version,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check migration status: %w", err)
	}
	return count > 0, nil
}

// Apply executes a migration's up SQL.
func (e *Executor) Apply(ctx context.Context, migration Migration, dryRun bool) error {
	// Check if already applied
	applied, err := e.IsMigrationApplied(ctx, migration.Version)
	if err != nil {
		return err
	}
	if applied {
		return fmt.Errorf("migration %s is already applied", migration.Version)
	}

	if dryRun {
		// In dry-run mode, just log what would happen
		return nil
	}

	// Start a transaction
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Record migration as pending
	_, err = tx.Exec(ctx,
		"INSERT INTO schema_migrations (version, name, status) VALUES ($1, $2, 'pending') ON CONFLICT (version) DO UPDATE SET status = 'pending'",
		migration.Version, migration.Name,
	)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Execute migration SQL
	statements := splitSQL(migration.UpSQL)
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		_, err = tx.Exec(ctx, stmt)
		if err != nil {
			// Record failure
			now := time.Now()
			errMsg := fmt.Sprintf("Statement %d failed: %v", i+1, err)
			_, _ = tx.Exec(ctx,
				"UPDATE schema_migrations SET status = 'failed', error = $1, applied_at = $2 WHERE version = $3",
				errMsg, now, migration.Version,
			)
			_ = tx.Commit(ctx)
			return fmt.Errorf("migration failed at statement %d: %w", i+1, err)
		}
	}

	// Record success
	now := time.Now()
	_, err = tx.Exec(ctx,
		"UPDATE schema_migrations SET status = 'applied', applied_at = $1, error = NULL WHERE version = $2",
		now, migration.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to update migration status: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	return nil
}

// Rollback executes a migration's down SQL.
func (e *Executor) Rollback(ctx context.Context, migration Migration, dryRun bool) error {
	// Check if applied
	applied, err := e.IsMigrationApplied(ctx, migration.Version)
	if err != nil {
		return err
	}
	if !applied {
		return fmt.Errorf("migration %s is not applied", migration.Version)
	}

	if dryRun {
		// In dry-run mode, just log what would happen
		return nil
	}

	// Start a transaction
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Execute rollback SQL
	statements := splitSQL(migration.DownSQL)
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		_, err = tx.Exec(ctx, stmt)
		if err != nil {
			return fmt.Errorf("rollback failed at statement %d: %w", i+1, err)
		}
	}

	// Remove migration record
	_, err = tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", migration.Version)
	if err != nil {
		return fmt.Errorf("failed to delete migration record: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit rollback: %w", err)
	}

	return nil
}

// ApplyAll applies all pending migrations.
func (e *Executor) ApplyAll(ctx context.Context, migrations []Migration, dryRun bool) error {
	// Get applied migrations
	appliedMap := make(map[string]bool)
	applied, err := e.GetAppliedMigrations(ctx)
	if err != nil {
		return err
	}
	for _, m := range applied {
		appliedMap[m.Version] = true
	}

	// Apply pending migrations in order
	for _, migration := range migrations {
		if appliedMap[migration.Version] {
			continue
		}

		if err := e.Apply(ctx, migration, dryRun); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Version, err)
		}
	}

	return nil
}

// RollbackLast rolls back the most recently applied migration.
func (e *Executor) RollbackLast(ctx context.Context, migration Migration, dryRun bool) error {
	return e.Rollback(ctx, migration, dryRun)
}

// RollbackTo rolls back all migrations after the specified version.
func (e *Executor) RollbackTo(ctx context.Context, targetVersion string, migrations []Migration, dryRun bool) error {
	// Get applied migrations
	applied, err := e.GetAppliedMigrations(ctx)
	if err != nil {
		return err
	}

	// Build migration map
	migrationMap := make(map[string]Migration)
	for _, m := range migrations {
		migrationMap[m.Version] = m
	}

	// Roll back migrations in reverse order
	for i := len(applied) - 1; i >= 0; i-- {
		record := applied[i]

		// Stop when we reach target version
		if record.Version == targetVersion {
			break
		}

		// Skip if version is less than or equal to target
		if record.Version <= targetVersion {
			continue
		}

		// Get migration
		migration, exists := migrationMap[record.Version]
		if !exists {
			return fmt.Errorf("migration file not found for version %s", record.Version)
		}

		// Roll back
		if err := e.Rollback(ctx, migration, dryRun); err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", record.Version, err)
		}
	}

	return nil
}

// GetStatus returns the status of all migrations.
func (e *Executor) GetStatus(ctx context.Context, migrations []Migration) ([]MigrationRecord, error) {
	// Get applied migrations
	appliedMap := make(map[string]MigrationRecord)
	applied, err := e.GetAllMigrations(ctx)
	if err != nil {
		return nil, err
	}
	for _, m := range applied {
		appliedMap[m.Version] = m
	}

	// Build status for all migrations
	var records []MigrationRecord
	for _, migration := range migrations {
		if record, exists := appliedMap[migration.Version]; exists {
			records = append(records, record)
		} else {
			records = append(records, MigrationRecord{
				Version:   migration.Version,
				Name:      migration.Name,
				Status:    StatusPending,
				AppliedAt: nil,
				Error:     nil,
			})
		}
	}

	return records, nil
}

// Validate checks that all migrations in the database have corresponding files.
func (e *Executor) Validate(ctx context.Context, migrations []Migration) error {
	// Get all migrations from database
	dbMigrations, err := e.GetAllMigrations(ctx)
	if err != nil {
		return err
	}

	// Build migration map
	migrationMap := make(map[string]Migration)
	for _, m := range migrations {
		migrationMap[m.Version] = m
	}

	// Check that all DB migrations have files
	var missing []string
	for _, record := range dbMigrations {
		if _, exists := migrationMap[record.Version]; !exists {
			missing = append(missing, record.Version)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing migration files: %v", missing)
	}

	return nil
}

// WithTransaction executes a function within a transaction.
func (e *Executor) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// splitSQL splits a SQL string into individual statements.
// This is a simple implementation that splits on semicolons.
// A more robust implementation would use a proper SQL parser.
func splitSQL(sql string) []string {
	// Remove comments
	lines := strings.Split(sql, "\n")
	var cleanedLines []string
	for _, line := range lines {
		// Skip comment lines
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		cleanedLines = append(cleanedLines, line)
	}

	cleaned := strings.Join(cleanedLines, "\n")

	// Split on semicolons
	statements := strings.Split(cleaned, ";")

	// Filter out empty statements
	var result []string
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			result = append(result, stmt)
		}
	}

	return result
}
