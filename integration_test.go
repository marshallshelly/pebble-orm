//go:build integration
// +build integration

package pebbleorm_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
	"github.com/marshallshelly/pebble-orm/pkg/migration"
	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/runtime"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Test models
type User struct {
	ID        int       `po:"id,primaryKey,serial"`
	Name      string    `po:"name,varchar(100),notNull"`
	Email     string    `po:"email,varchar(255),unique,notNull"`
	Age       int       `po:"age,integer"`
	Active    bool      `po:"active,boolean,default(true)"`
	CreatedAt time.Time `po:"created_at,timestamp,default(NOW())"`
}

type Post struct {
	ID      int      `po:"id,primaryKey,serial"`
	Title   string   `po:"title,varchar(255),notNull"`
	Content string   `po:"content,text"`
	UserID  int      `po:"user_id,integer,notNull"`
	Tags    []string `po:"tags,text[]"`
}

type UserProfile struct {
	ID       int          `po:"id,primaryKey,serial"`
	UserID   int          `po:"user_id,integer,notNull,unique"`
	Bio      string       `po:"bio,text"`
	Metadata schema.JSONB `po:"metadata,jsonb"`
}

// setupTestDB creates a PostgreSQL container and returns connection details
func setupTestDB(t *testing.T) (*postgres.PostgresContainer, string, func()) {
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

	cleanup := func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return pgContainer, connStr, cleanup
}

// createTestSchema creates the test schema using Pebble ORM's migration system
func createTestSchema(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()

	// Register all test models
	if err := registry.Register(User{}); err != nil {
		t.Fatalf("Failed to register User: %v", err)
	}
	if err := registry.Register(Post{}); err != nil {
		t.Fatalf("Failed to register Post: %v", err)
	}
	if err := registry.Register(UserProfile{}); err != nil {
		t.Fatalf("Failed to register UserProfile: %v", err)
	}

	// Get code schema from registered models
	codeSchema := registry.GetAllTables()

	// Create migration executor
	executor := migration.NewExecutor(pool, "./test_migrations")

	// Initialize schema_migrations table
	if err := executor.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize migrations: %v", err)
	}

	// Introspect current database (should be empty)
	introspector := migration.NewIntrospector(pool)
	dbSchema, err := introspector.IntrospectSchema(ctx)
	if err != nil {
		t.Fatalf("Failed to introspect schema: %v", err)
	}

	// Compare schemas to get diff
	differ := migration.NewDiffer()
	diff := differ.Compare(codeSchema, dbSchema)

	if diff.HasChanges() {
		// Generate migration SQL
		planner := migration.NewPlanner()
		upSQL, _ := planner.GenerateMigration(diff)

		// Execute the up migration directly (no need for file generation in tests)
		if _, err := pool.Exec(ctx, upSQL); err != nil {
			t.Fatalf("Failed to execute migration: %v\n%s", err, upSQL)
		}

		t.Logf("Created test schema using Pebble ORM migrations ✅")
	}
}

func TestIntegration_BasicCRUD(t *testing.T) {
	_, connStr, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Connect to database
	runtimeDB, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer runtimeDB.Close()

	// Create schema (this also registers models)
	createTestSchema(t, runtimeDB.Pool())

	// Create DB instance
	qb := builder.New(runtimeDB)

	// Test INSERT
	t.Run("INSERT", func(t *testing.T) {
		newUser := User{
			Name:  "John Doe",
			Email: "john@example.com",
			Age:   30,
		}

		insertedUsers, err := builder.Insert[User](qb).
			Values(newUser).
			Returning("*").
			ExecReturning(ctx)

		if err != nil {
			t.Fatalf("Failed to insert user: %v", err)
		}

		if len(insertedUsers) != 1 {
			t.Fatalf("Expected 1 user, got %d", len(insertedUsers))
		}

		if insertedUsers[0].Name != "John Doe" {
			t.Errorf("Expected name 'John Doe', got '%s'", insertedUsers[0].Name)
		}
	})

	// Test SELECT
	t.Run("SELECT", func(t *testing.T) {
		users, err := builder.Select[User](qb).
			Where(builder.Eq("email", "john@example.com")).
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to select user: %v", err)
		}

		if len(users) != 1 {
			t.Fatalf("Expected 1 user, got %d", len(users))
		}

		if users[0].Email != "john@example.com" {
			t.Errorf("Expected email 'john@example.com', got '%s'", users[0].Email)
		}
	})

	// Test UPDATE
	t.Run("UPDATE", func(t *testing.T) {
		count, err := builder.Update[User](qb).
			Set("age", 31).
			Where(builder.Eq("email", "john@example.com")).
			Exec(ctx)

		if err != nil {
			t.Fatalf("Failed to update user: %v", err)
		}

		if count != 1 {
			t.Errorf("Expected 1 row updated, got %d", count)
		}

		// Verify update
		updatedUsers, err := builder.Select[User](qb).
			Where(builder.Eq("email", "john@example.com")).
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to select user after update: %v", err)
		}

		if len(updatedUsers) != 1 {
			t.Fatalf("Expected 1 user after update, got %d", len(updatedUsers))
		}

		if updatedUsers[0].Age != 31 {
			t.Errorf("Expected age 31, got %d", updatedUsers[0].Age)
		}
	})

	// Test DELETE
	t.Run("DELETE", func(t *testing.T) {
		count, err := builder.Delete[User](qb).
			Where(builder.Eq("email", "john@example.com")).
			Exec(ctx)

		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		if count != 1 {
			t.Errorf("Expected 1 row deleted, got %d", count)
		}

		// Verify deletion
		users, err := builder.Select[User](qb).
			Where(builder.Eq("email", "john@example.com")).
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to select user after delete: %v", err)
		}

		if len(users) != 0 {
			t.Errorf("Expected 0 users, got %d", len(users))
		}
	})
}

func TestIntegration_PostgreSQLFeatures(t *testing.T) {
	_, connStr, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	runtimeDB, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer runtimeDB.Close()

	createTestSchema(t, runtimeDB.Pool())
	registry.Register(Post{})
	registry.Register(UserProfile{})

	qb := builder.New(runtimeDB)

	t.Run("Array operations", func(t *testing.T) {
		// Insert post with tags
		post := Post{
			Title:   "Getting Started with Go",
			Content: "This is a tutorial...",
			UserID:  1,
			Tags:    []string{"golang", "tutorial", "programming"},
		}

		posts, err := builder.Insert[Post](qb).
			Values(post).
			Returning("*").
			ExecReturning(ctx)

		if err != nil {
			t.Fatalf("Failed to insert post: %v", err)
		}

		// Query with array contains
		foundPosts, err := builder.Select[Post](qb).
			Where(builder.ArrayContains("tags", []string{"golang"})).
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to query posts: %v", err)
		}

		if len(foundPosts) != 1 {
			t.Errorf("Expected 1 post, got %d", len(foundPosts))
		}

		if len(posts[0].Tags) != 3 {
			t.Errorf("Expected 3 tags, got %d", len(posts[0].Tags))
		}
	})

	t.Run("JSONB operations", func(t *testing.T) {
		// Insert user profile with metadata
		profile := UserProfile{
			UserID: 1,
			Bio:    "Software developer",
			Metadata: schema.JSONB{
				"premium":  true,
				"location": "New York",
				"skills":   []interface{}{"Go", "PostgreSQL", "Docker"},
			},
		}

		profiles, err := builder.Insert[UserProfile](qb).
			Values(profile).
			Returning("*").
			ExecReturning(ctx)

		if err != nil {
			t.Fatalf("Failed to insert profile: %v", err)
		}

		// Query with JSONB contains
		foundProfiles, err := builder.Select[UserProfile](qb).
			Where(builder.JSONBContains("metadata", `{"premium": true}`)).
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to query profiles: %v", err)
		}

		if len(foundProfiles) != 1 {
			t.Errorf("Expected 1 profile, got %d", len(foundProfiles))
		}

		if profiles[0].Metadata["premium"] != true {
			t.Errorf("Expected premium to be true")
		}
	})
}

func TestIntegration_Transactions(t *testing.T) {
	_, connStr, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	runtimeDB, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer runtimeDB.Close()

	createTestSchema(t, runtimeDB.Pool())
	registry.Register(User{})

	qb := builder.New(runtimeDB)

	t.Run("Commit transaction", func(t *testing.T) {
		tx, err := qb.Begin(ctx)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}

		// Insert user in transaction
		user := User{
			Name:  "Jane Doe",
			Email: "jane@example.com",
			Age:   25,
		}

		_, err = tx.Insert(User{}).
			Values(user).
			Exec()

		if err != nil {
			t.Fatalf("Failed to insert in transaction: %v", err)
		}

		// Commit
		if err := tx.Commit(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Verify user exists
		users, _ := builder.Select[User](qb).
			Where(builder.Eq("email", "jane@example.com")).
			All(ctx)

		if len(users) != 1 {
			t.Errorf("Expected 1 user after commit, got %d", len(users))
		}
	})

	t.Run("Rollback transaction", func(t *testing.T) {
		tx, err := qb.Begin(ctx)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		// Insert user in transaction
		user := User{
			Name:  "Bob Smith",
			Email: "bob@example.com",
			Age:   35,
		}

		_, err = tx.Insert(User{}).
			Values(user).
			Exec()

		if err != nil {
			t.Fatalf("Failed to insert in transaction: %v", err)
		}

		// Rollback (via defer)

		// Verify user doesn't exist
		users, _ := builder.Select[User](qb).
			Where(builder.Eq("email", "bob@example.com")).
			All(ctx)

		if len(users) != 0 {
			t.Errorf("Expected 0 users after rollback, got %d", len(users))
		}
	})
}

func TestIntegration_ComplexQueries(t *testing.T) {
	_, connStr, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	runtimeDB, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer runtimeDB.Close()

	createTestSchema(t, runtimeDB.Pool())
	registry.Register(User{})

	qb := builder.New(runtimeDB)

	// Insert test data
	users := []User{
		{Name: "Alice", Email: "alice@example.com", Age: 25, Active: true},
		{Name: "Bob", Email: "bob@example.com", Age: 30, Active: true},
		{Name: "Charlie", Email: "charlie@example.com", Age: 35, Active: false},
		{Name: "Diana", Email: "diana@example.com", Age: 28, Active: true},
	}

	for _, user := range users {
		_, _ = builder.Insert[User](qb).Values(user).Exec(ctx)
	}

	t.Run("Complex WHERE conditions", func(t *testing.T) {
		results, err := builder.Select[User](qb).
			Where(builder.Gt("age", 25)).
			And(builder.Eq("active", true)).
			OrderByAsc("age").
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to query: %v", err)
		}

		if len(results) != 2 { // Bob and Diana
			t.Errorf("Expected 2 users, got %d", len(results))
		}
	})

	t.Run("Aggregation with GROUP BY", func(t *testing.T) {
		type AgeStats struct {
			Active bool `po:"active"`
			Count  int  `po:"count"`
		}

		// Get count by active status
		_, err := builder.Select[User](qb).
			Columns("active", "COUNT(*) as count").
			GroupBy("active").
			OrderByAsc("active").
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to aggregate: %v", err)
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		results, err := builder.Select[User](qb).
			OrderByAsc("age").
			Limit(2).
			Offset(1).
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to paginate: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 users, got %d", len(results))
		}
	})
}
