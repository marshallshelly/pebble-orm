//go:build integration

package migration

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTestDB creates a PostgreSQL container for testing
func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}

	cleanup := func() {
		pool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return pool, cleanup
}

func TestUniqueConstraintIntrospection(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a table with UNIQUE constraint
	_, err := pool.Exec(ctx, `
		CREATE TABLE test_users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) NOT NULL,
			username VARCHAR(100) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Add UNIQUE constraint via migration
	_, err = pool.Exec(ctx, `
		ALTER TABLE test_users ADD CONSTRAINT test_users_email_key UNIQUE (email)
	`)
	if err != nil {
		t.Fatalf("Failed to add UNIQUE constraint: %v", err)
	}

	// Introspect the database
	introspector := NewIntrospector(pool)
	dbSchema, err := introspector.IntrospectSchema(ctx)
	if err != nil {
		t.Fatalf("Failed to introspect schema: %v", err)
	}

	// Verify UNIQUE constraint is detected
	table, exists := dbSchema["test_users"]
	if !exists {
		t.Fatal("Table test_users not found in introspected schema")
	}

	// Find UNIQUE constraint
	var foundUniqueConstraint bool
	for _, c := range table.Constraints {
		if c.Type == schema.UniqueConstraint && len(c.Columns) == 1 && c.Columns[0] == "email" {
			foundUniqueConstraint = true
			if c.Name != "test_users_email_key" {
				t.Errorf("Expected constraint name 'test_users_email_key', got '%s'", c.Name)
			}
			break
		}
	}

	if !foundUniqueConstraint {
		t.Error("UNIQUE constraint on email column not detected")
	}
}

func TestUniqueConstraintMigration(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a table WITHOUT UNIQUE constraint
	_, err := pool.Exec(ctx, `
		CREATE TABLE test_products (
			id SERIAL PRIMARY KEY,
			sku VARCHAR(50) NOT NULL,
			name VARCHAR(255) NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Introspect current database state
	introspector := NewIntrospector(pool)
	dbSchema, err := introspector.IntrospectSchema(ctx)
	if err != nil {
		t.Fatalf("Failed to introspect schema: %v", err)
	}

	// Define code schema with UNIQUE constraint
	codeSchema := map[string]*schema.TableMetadata{
		"test_products": {
			Name: "test_products",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "serial"},
				{Name: "sku", SQLType: "varchar(50)"},
				{Name: "name", SQLType: "varchar(255)"},
			},
			Constraints: []schema.ConstraintMetadata{
				{
					Name:    "test_products_sku_key",
					Type:    schema.UniqueConstraint,
					Columns: []string{"sku"},
				},
			},
		},
	}

	// Compare schemas
	differ := NewDiffer()
	diff := differ.Compare(codeSchema, dbSchema)

	// Verify difference is detected
	if !diff.HasChanges() {
		t.Fatal("Expected schema differences to be detected")
	}

	if len(diff.TablesModified) != 1 {
		t.Fatalf("Expected 1 table to be modified, got %d", len(diff.TablesModified))
	}

	tableDiff := diff.TablesModified[0]
	if len(tableDiff.ConstraintsAdded) != 1 {
		t.Fatalf("Expected 1 constraint to be added, got %d", len(tableDiff.ConstraintsAdded))
	}

	// Generate migration SQL
	planner := NewPlanner()
	upSQL, _ := planner.GenerateMigration(diff)

	t.Logf("Generated migration SQL:\n%s", upSQL)

	// Execute the migration
	_, err = pool.Exec(ctx, upSQL)
	if err != nil {
		t.Fatalf("Failed to execute migration: %v", err)
	}

	// Verify constraint was added
	dbSchemaAfter, err := introspector.IntrospectSchema(ctx)
	if err != nil {
		t.Fatalf("Failed to introspect schema after migration: %v", err)
	}

	tableAfter := dbSchemaAfter["test_products"]
	var foundConstraint bool
	for _, c := range tableAfter.Constraints {
		if c.Type == schema.UniqueConstraint && len(c.Columns) == 1 && c.Columns[0] == "sku" {
			foundConstraint = true
			break
		}
	}

	if !foundConstraint {
		t.Error("UNIQUE constraint was not added successfully")
	}

	// Test that constraint actually works
	_, err = pool.Exec(ctx, `INSERT INTO test_products (sku, name) VALUES ('SKU001', 'Product 1')`)
	if err != nil {
		t.Fatalf("Failed to insert first product: %v", err)
	}

	// Try to insert duplicate SKU - should fail
	_, err = pool.Exec(ctx, `INSERT INTO test_products (sku, name) VALUES ('SKU001', 'Product 2')`)
	if err == nil {
		t.Error("Expected duplicate SKU insertion to fail, but it succeeded")
	}
}

func TestCompositeUniqueConstraint(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a table without composite UNIQUE constraint
	_, err := pool.Exec(ctx, `
		CREATE TABLE test_user_roles (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			role_id INTEGER NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Define code schema with composite UNIQUE constraint
	codeSchema := map[string]*schema.TableMetadata{
		"test_user_roles": {
			Name: "test_user_roles",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "serial"},
				{Name: "user_id", SQLType: "integer"},
				{Name: "role_id", SQLType: "integer"},
			},
			Constraints: []schema.ConstraintMetadata{
				{
					Name:    "test_user_roles_user_id_role_id_key",
					Type:    schema.UniqueConstraint,
					Columns: []string{"user_id", "role_id"},
				},
			},
		},
	}

	// Introspect and compare
	introspector := NewIntrospector(pool)
	dbSchema, err := introspector.IntrospectSchema(ctx)
	if err != nil {
		t.Fatalf("Failed to introspect schema: %v", err)
	}

	differ := NewDiffer()
	diff := differ.Compare(codeSchema, dbSchema)

	// Generate and execute migration
	planner := NewPlanner()
	upSQL, _ := planner.GenerateMigration(diff)

	t.Logf("Generated migration SQL:\n%s", upSQL)

	_, err = pool.Exec(ctx, upSQL)
	if err != nil {
		t.Fatalf("Failed to execute migration: %v", err)
	}

	// Test that composite constraint works
	_, err = pool.Exec(ctx, `INSERT INTO test_user_roles (user_id, role_id) VALUES (1, 1)`)
	if err != nil {
		t.Fatalf("Failed to insert first user_role: %v", err)
	}

	// Should allow same user with different role
	_, err = pool.Exec(ctx, `INSERT INTO test_user_roles (user_id, role_id) VALUES (1, 2)`)
	if err != nil {
		t.Fatalf("Failed to insert second user_role: %v", err)
	}

	// Should NOT allow duplicate (user_id, role_id) combination
	_, err = pool.Exec(ctx, `INSERT INTO test_user_roles (user_id, role_id) VALUES (1, 1)`)
	if err == nil {
		t.Error("Expected duplicate (user_id, role_id) insertion to fail, but it succeeded")
	}
}

func TestDropUniqueConstraint(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a table WITH UNIQUE constraint
	_, err := pool.Exec(ctx, `
		CREATE TABLE test_items (
			id SERIAL PRIMARY KEY,
			code VARCHAR(50) NOT NULL,
			CONSTRAINT test_items_code_key UNIQUE (code)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Define code schema WITHOUT UNIQUE constraint (removing it)
	codeSchema := map[string]*schema.TableMetadata{
		"test_items": {
			Name: "test_items",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "serial"},
				{Name: "code", SQLType: "varchar(50)"},
			},
			Constraints: []schema.ConstraintMetadata{},
		},
	}

	// Introspect and compare
	introspector := NewIntrospector(pool)
	dbSchema, err := introspector.IntrospectSchema(ctx)
	if err != nil {
		t.Fatalf("Failed to introspect schema: %v", err)
	}

	differ := NewDiffer()
	diff := differ.Compare(codeSchema, dbSchema)

	// Verify constraint drop is detected
	if len(diff.TablesModified) != 1 {
		t.Fatalf("Expected 1 table to be modified, got %d", len(diff.TablesModified))
	}

	tableDiff := diff.TablesModified[0]
	if len(tableDiff.ConstraintsDropped) != 1 {
		t.Fatalf("Expected 1 constraint to be dropped, got %d", len(tableDiff.ConstraintsDropped))
	}

	// Generate and execute migration
	planner := NewPlanner()
	upSQL, _ := planner.GenerateMigration(diff)

	t.Logf("Generated migration SQL:\n%s", upSQL)

	_, err = pool.Exec(ctx, upSQL)
	if err != nil {
		t.Fatalf("Failed to execute migration: %v", err)
	}

	// Verify constraint was removed
	dbSchemaAfter, err := introspector.IntrospectSchema(ctx)
	if err != nil {
		t.Fatalf("Failed to introspect schema after migration: %v", err)
	}

	tableAfter := dbSchemaAfter["test_items"]
	for _, c := range tableAfter.Constraints {
		if c.Type == schema.UniqueConstraint && len(c.Columns) == 1 && c.Columns[0] == "code" {
			t.Error("UNIQUE constraint was not dropped successfully")
		}
	}

	// Test that duplicates are now allowed
	_, err = pool.Exec(ctx, `INSERT INTO test_items (code) VALUES ('ITEM001')`)
	if err != nil {
		t.Fatalf("Failed to insert first item: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO test_items (code) VALUES ('ITEM001')`)
	if err != nil {
		t.Errorf("Expected duplicate insertion to succeed after dropping constraint, but it failed: %v", err)
	}
}
