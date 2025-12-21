// +build integration

package pebbleorm_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/runtime"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Helper function to convert []interface{} to typed slice
func toTyped[T any](raw []interface{}) []T {
	result := make([]T, len(raw))
	for i, v := range raw {
		result[i] = v.(T)
	}
	return result
}

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
	ID      int    `po:"id,primaryKey,serial"`
	Title   string `po:"title,varchar(255),notNull"`
	Content string `po:"content,text"`
	UserID  int    `po:"user_id,integer,notNull"`
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
		"postgres:16-alpine",
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

// createTestSchema creates the test schema
func createTestSchema(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()

	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			age INTEGER,
			active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS posts (
			id SERIAL PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			content TEXT,
			user_id INTEGER NOT NULL,
			tags TEXT[]
		)`,
		`CREATE TABLE IF NOT EXISTS user_profiles (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL UNIQUE,
			bio TEXT,
			metadata JSONB
		)`,
	}

	for _, query := range queries {
		if _, err := pool.Exec(ctx, query); err != nil {
			t.Fatalf("Failed to create schema: %v", err)
		}
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

	// Create schema
	createTestSchema(t, runtimeDB.Pool())

	// Register models
	registry.Register(User{})

	// Create DB instance
	db := builder.New(runtimeDB)

	// Test INSERT
	t.Run("INSERT", func(t *testing.T) {
		newUser := User{
			Name:  "John Doe",
			Email: "john@example.com",
			Age:   30,
		}

		usersRaw, err := db.Insert(User{}).
			Values(newUser).
			Returning("*").
			ExecReturning(ctx)

		if err != nil {
			t.Fatalf("Failed to insert user: %v", err)
		}

		users := toTyped[User](usersRaw)

		if len(users) != 1 {
			t.Fatalf("Expected 1 user, got %d", len(users))
		}

		if users[0].Name != "John Doe" {
			t.Errorf("Expected name 'John Doe', got '%s'", users[0].Name)
		}
	})

	// Test SELECT
	t.Run("SELECT", func(t *testing.T) {
		usersRaw, err := db.Select(User{}).
			Where(builder.Eq("email", "john@example.com")).
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to select user: %v", err)
		}

		users := toTyped[User](usersRaw)

		if len(users) != 1 {
			t.Fatalf("Expected 1 user, got %d", len(users))
		}

		if users[0].Email != "john@example.com" {
			t.Errorf("Expected email 'john@example.com', got '%s'", users[0].Email)
		}
	})

	// Test UPDATE
	t.Run("UPDATE", func(t *testing.T) {
		count, err := db.Update(User{}).
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
		usersRaw, _ := db.Select(User{}).
			Where(builder.Eq("email", "john@example.com")).
			All(ctx)

		users := toTyped[User](usersRaw)

		if users[0].Age != 31 {
			t.Errorf("Expected age 31, got %d", users[0].Age)
		}
	})

	// Test DELETE
	t.Run("DELETE", func(t *testing.T) {
		count, err := db.Delete(User{}).
			Where(builder.Eq("email", "john@example.com")).
			Exec(ctx)

		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		if count != 1 {
			t.Errorf("Expected 1 row deleted, got %d", count)
		}

		// Verify deletion
		usersRaw, _ := db.Select(User{}).
			Where(builder.Eq("email", "john@example.com")).
			All(ctx)

		users := toTyped[User](usersRaw)

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

	db := builder.New(runtimeDB)

	t.Run("Array operations", func(t *testing.T) {
		// Insert post with tags
		post := Post{
			Title:   "Getting Started with Go",
			Content: "This is a tutorial...",
			UserID:  1,
			Tags:    []string{"golang", "tutorial", "programming"},
		}

		postsRaw, err := db.Insert(Post{}).
			Values(post).
			Returning("*").
			ExecReturning(ctx)

		if err != nil {
			t.Fatalf("Failed to insert post: %v", err)
		}

		posts := toTyped[Post](postsRaw)

		// Query with array contains
		foundPostsRaw, err := db.Select(Post{}).
			Where(builder.ArrayContains("tags", []string{"golang"})).
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to query posts: %v", err)
		}

		foundPosts := toTyped[Post](foundPostsRaw)

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

		profilesRaw, err := db.Insert(UserProfile{}).
			Values(profile).
			Returning("*").
			ExecReturning(ctx)

		if err != nil {
			t.Fatalf("Failed to insert profile: %v", err)
		}

		profiles := toTyped[UserProfile](profilesRaw)

		// Query with JSONB contains
		foundProfilesRaw, err := db.Select(UserProfile{}).
			Where(builder.JSONBContains("metadata", `{"premium": true}`)).
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to query profiles: %v", err)
		}

		foundProfiles := toTyped[UserProfile](foundProfilesRaw)

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

	db := builder.New(runtimeDB)

	t.Run("Commit transaction", func(t *testing.T) {
		tx, err := db.Begin(ctx)
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
		usersRaw, _ := db.Select(User{}).
			Where(builder.Eq("email", "jane@example.com")).
			All(ctx)

		users := toTyped[User](usersRaw)

		if len(users) != 1 {
			t.Errorf("Expected 1 user after commit, got %d", len(users))
		}
	})

	t.Run("Rollback transaction", func(t *testing.T) {
		tx, err := db.Begin(ctx)
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
		usersRaw, _ := db.Select(User{}).
			Where(builder.Eq("email", "bob@example.com")).
			All(ctx)

		users := toTyped[User](usersRaw)

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

	db := builder.New(runtimeDB)

	// Insert test data
	users := []User{
		{Name: "Alice", Email: "alice@example.com", Age: 25, Active: true},
		{Name: "Bob", Email: "bob@example.com", Age: 30, Active: true},
		{Name: "Charlie", Email: "charlie@example.com", Age: 35, Active: false},
		{Name: "Diana", Email: "diana@example.com", Age: 28, Active: true},
	}

	for _, user := range users {
		_, _ = db.Insert(User{}).Values(user).Exec(ctx)
	}

	t.Run("Complex WHERE conditions", func(t *testing.T) {
		resultsRaw, err := db.Select(User{}).
			Where(builder.Gt("age", 25)).
			And(builder.Eq("active", true)).
			OrderByAsc("age").
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to query: %v", err)
		}

		results := toTyped[User](resultsRaw)

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
		_, err := db.Select(User{}).
			Columns("active", "COUNT(*) as count").
			GroupBy("active").
			OrderByAsc("active").
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to aggregate: %v", err)
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		resultsRaw, err := db.Select(User{}).
			OrderByAsc("age").
			Limit(2).
			Offset(1).
			All(ctx)

		if err != nil {
			t.Fatalf("Failed to paginate: %v", err)
		}

		results := toTyped[User](resultsRaw)

		if len(results) != 2 {
			t.Errorf("Expected 2 users, got %d", len(results))
		}
	})
}
