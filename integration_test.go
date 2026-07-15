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

// TestIntegration_NamedArrayTypes verifies schema.StringArray and friends
// scan correctly under pgx's default extended protocol, where array results
// arrive in binary format. Regression test for named Scanner slices receiving
// raw wire bytes instead of decoded values.
func TestIntegration_NamedArrayTypes(t *testing.T) {
	_, connStr, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	runtimeDB, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer runtimeDB.Close()

	type ScheduleItem struct {
		ID     int                 `po:"id,primaryKey,serial"`
		Days   schema.StringArray  `po:"days,text[]"`
		Counts schema.Int32Array   `po:"counts,integer[]"`
		Rates  schema.Float64Array `po:"rates,double precision[]"`
		Flags  schema.BoolArray    `po:"flags,boolean[]"`
	}

	if err := registry.Register(ScheduleItem{}); err != nil {
		t.Fatalf("Failed to register ScheduleItem: %v", err)
	}

	_, err = runtimeDB.Pool().Exec(ctx, `
		CREATE TABLE schedule_item (
			id serial PRIMARY KEY, days text[], counts integer[],
			rates double precision[], flags boolean[]
		)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	qb := builder.New(runtimeDB)

	item := ScheduleItem{
		Days:   schema.StringArray{"monday", "friday, with a comma"},
		Counts: schema.Int32Array{1, 2, 3},
		Rates:  schema.Float64Array{1.5, 2.25},
		Flags:  schema.BoolArray{true, false},
	}

	returned, err := builder.Insert[ScheduleItem](qb).Values(item).Returning("*").ExecReturning(ctx)
	if err != nil {
		t.Fatalf("ExecReturning failed: %v", err)
	}
	if len(returned) != 1 || len(returned[0].Days) != 2 || returned[0].Days[1] != "friday, with a comma" {
		t.Errorf("ExecReturning: unexpected Days: %v", returned[0].Days)
	}

	fetched, err := builder.Select[ScheduleItem](qb).All(ctx)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if len(fetched) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(fetched))
	}
	got := fetched[0]
	if len(got.Days) != 2 || got.Days[0] != "monday" || got.Days[1] != "friday, with a comma" {
		t.Errorf("Days: got %v", got.Days)
	}
	if len(got.Counts) != 3 || got.Counts[2] != 3 {
		t.Errorf("Counts: got %v", got.Counts)
	}
	if len(got.Rates) != 2 || got.Rates[1] != 2.25 {
		t.Errorf("Rates: got %v", got.Rates)
	}
	if len(got.Flags) != 2 || !got.Flags[0] || got.Flags[1] {
		t.Errorf("Flags: got %v", got.Flags)
	}
}

// TestIntegration_BulkInsertColumnAlignment is a regression test for the
// multi-row INSERT bug where per-row default-skipping could misalign values
// against the column list derived from row 0. Row 1 leaves a defaulted column
// zero while row 0 sets it; both must still land in the correct columns.
func TestIntegration_BulkInsertColumnAlignment(t *testing.T) {
	_, connStr, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	runtimeDB, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer runtimeDB.Close()

	type Item struct {
		ID   int    `po:"id,primaryKey,serial"`
		Name string `po:"name,text,notNull"`
		Note string `po:"note,text,default('none')"`
	}
	if err := registry.Register(Item{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := runtimeDB.Pool().Exec(ctx,
		`CREATE TABLE item (id serial PRIMARY KEY, name text NOT NULL, note text DEFAULT 'none')`); err != nil {
		t.Fatalf("create: %v", err)
	}

	qb := builder.New(runtimeDB)
	// Row 0 sets Note (so "note" is in the column list); row 1 leaves it zero.
	got, err := builder.Insert[Item](qb).Values(
		Item{Name: "a", Note: "row0-note"},
		Item{Name: "b"},
	).Returning("*").ExecReturning(ctx)
	if err != nil {
		t.Fatalf("bulk insert: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	byName := map[string]Item{got[0].Name: got[0], got[1].Name: got[1]}
	if byName["a"].Note != "row0-note" {
		t.Errorf("row a Note: got %q, want row0-note", byName["a"].Note)
	}
	// Row b passed the column list from row 0 (which includes note), so its
	// explicit zero value "" is inserted — not misaligned into another column.
	if byName["b"].Name != "b" {
		t.Errorf("row b Name misaligned: got %q, want b", byName["b"].Name)
	}
	if byName["b"].Note != "" {
		t.Errorf("row b Note: got %q, want empty string (explicit zero)", byName["b"].Note)
	}
}

// TestIntegration_EnumIntrospection verifies existing database enum types are
// visible to the introspector (regression for the bad pg_class join that made
// getEnumTypes always return zero rows).
func TestIntegration_EnumIntrospection(t *testing.T) {
	_, connStr, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	runtimeDB, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer runtimeDB.Close()

	if _, err := runtimeDB.Pool().Exec(ctx, `
		CREATE TYPE order_status AS ENUM ('pending', 'active', 'completed');
		CREATE TABLE orders (id serial PRIMARY KEY, status order_status NOT NULL)`); err != nil {
		t.Fatalf("create: %v", err)
	}

	introspector := migration.NewIntrospector(runtimeDB.Pool())
	table, err := introspector.IntrospectTable(ctx, "orders")
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}
	if len(table.EnumTypes) != 1 {
		t.Fatalf("expected 1 enum type, got %d: %+v", len(table.EnumTypes), table.EnumTypes)
	}
	et := table.EnumTypes[0]
	if et.Name != "order_status" || len(et.Values) != 3 || et.Values[0] != "pending" || et.Values[2] != "completed" {
		t.Errorf("enum mismatch: %+v", et)
	}
}
