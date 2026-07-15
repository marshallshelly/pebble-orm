//go:build integration
// +build integration

package pebbleorm_test

import (
	"context"
	"os"
	"path/filepath"
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
	// Charlie's Active:false is omitted from INSERT (zero value on a defaulted
	// column, by design), so persist the inactive state explicitly via UPDATE,
	// which does not skip zero values.
	if _, err := builder.Update[User](qb).Set("active", false).Where(builder.Eq("name", "Charlie")).Exec(ctx); err != nil {
		t.Fatalf("failed to deactivate Charlie: %v", err)
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

// TestIntegration_ManyToManyPreload is a regression test for junction PKs being
// scanned into bare interface{} (pgx int32) that never matched the struct's int
// keys, so manyToMany Preload silently returned empty slices.
func TestIntegration_ManyToManyPreload(t *testing.T) {
	_, connStr, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	runtimeDB, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer runtimeDB.Close()

	type Role struct {
		ID   int    `po:"id,primaryKey,serial"`
		Name string `po:"name,text,notNull"`
	}
	type Account struct {
		ID    int    `po:"id,primaryKey,serial"`
		Name  string `po:"name,text,notNull"`
		Roles []Role `po:"-,manyToMany,joinTable(account_roles),foreignKey(account_id)"`
	}
	if err := registry.Register(Role{}); err != nil {
		t.Fatalf("register Role: %v", err)
	}
	if err := registry.Register(Account{}); err != nil {
		t.Fatalf("register Account: %v", err)
	}
	if _, err := runtimeDB.Pool().Exec(ctx, `
		CREATE TABLE account (id serial PRIMARY KEY, name text NOT NULL);
		CREATE TABLE role (id serial PRIMARY KEY, name text NOT NULL);
		CREATE TABLE account_roles (account_id integer NOT NULL, role_id integer NOT NULL);
		INSERT INTO account (id, name) VALUES (1, 'alice'), (2, 'bob');
		INSERT INTO role (id, name) VALUES (1, 'admin'), (2, 'editor'), (3, 'viewer');
		INSERT INTO account_roles (account_id, role_id) VALUES (1,1), (1,2), (2,3);`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	qb := builder.New(runtimeDB)
	accounts, err := builder.Select[Account](qb).OrderByAsc("id").Preload("Roles").All(ctx)
	if err != nil {
		t.Fatalf("preload: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}
	if len(accounts[0].Roles) != 2 {
		t.Errorf("alice: expected 2 roles, got %d (%+v)", len(accounts[0].Roles), accounts[0].Roles)
	}
	if len(accounts[1].Roles) != 1 || accounts[1].Roles[0].Name != "viewer" {
		t.Errorf("bob: expected [viewer], got %+v", accounts[1].Roles)
	}
}

// TestIntegration_NestedPreloadValueSlice is a regression test for nested
// preloads panicking when the intermediate hasMany field is a value slice
// ([]Post rather than []*Post).
func TestIntegration_NestedPreloadValueSlice(t *testing.T) {
	_, connStr, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	runtimeDB, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer runtimeDB.Close()

	type Comment struct {
		ID     int    `po:"id,primaryKey,serial"`
		PostID int    `po:"post_id,integer,notNull"`
		Body   string `po:"body,text,notNull"`
	}
	type BlogPost struct {
		ID       int       `po:"id,primaryKey,serial"`
		AuthorID int       `po:"author_id,integer,notNull"`
		Title    string    `po:"title,text,notNull"`
		Comments []Comment `po:"-,hasMany,foreignKey(post_id),references(id)"`
	}
	type Writer struct {
		ID    int        `po:"id,primaryKey,serial"`
		Name  string     `po:"name,text,notNull"`
		Posts []BlogPost `po:"-,hasMany,foreignKey(author_id),references(id)"` // value slice
	}
	for _, m := range []interface{}{Comment{}, BlogPost{}, Writer{}} {
		if err := registry.Register(m); err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	if _, err := runtimeDB.Pool().Exec(ctx, `
		CREATE TABLE writer (id serial PRIMARY KEY, name text NOT NULL);
		CREATE TABLE blog_post (id serial PRIMARY KEY, author_id integer NOT NULL, title text NOT NULL);
		CREATE TABLE comment (id serial PRIMARY KEY, post_id integer NOT NULL, body text NOT NULL);
		INSERT INTO writer (id, name) VALUES (1, 'ann');
		INSERT INTO blog_post (id, author_id, title) VALUES (1, 1, 'hello'), (2, 1, 'world');
		INSERT INTO comment (id, post_id, body) VALUES (1, 1, 'nice'), (2, 1, 'agree'), (3, 2, 'ok');`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	qb := builder.New(runtimeDB)
	// Must not panic; must load Comments onto each value-slice Post.
	writers, err := builder.Select[Writer](qb).Preload("Posts.Comments").All(ctx)
	if err != nil {
		t.Fatalf("nested preload: %v", err)
	}
	if len(writers) != 1 || len(writers[0].Posts) != 2 {
		t.Fatalf("expected 1 writer with 2 posts, got %d writers", len(writers))
	}
	byTitle := map[string]BlogPost{}
	for _, p := range writers[0].Posts {
		byTitle[p.Title] = p
	}
	if len(byTitle["hello"].Comments) != 2 {
		t.Errorf("post 'hello': expected 2 comments, got %d", len(byTitle["hello"].Comments))
	}
	if len(byTitle["world"].Comments) != 1 {
		t.Errorf("post 'world': expected 1 comment, got %d", len(byTitle["world"].Comments))
	}
}

// TestIntegration_MigrationDollarQuotedBody verifies a migration containing a
// dollar-quoted function body (with interior semicolons) applies as a single
// statement instead of being shattered by the SQL splitter.
func TestIntegration_MigrationDollarQuotedBody(t *testing.T) {
	_, connStr, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	dir := t.TempDir()
	up := `CREATE TABLE widget (id serial PRIMARY KEY, updated_at timestamptz);

CREATE FUNCTION touch_widget() RETURNS trigger AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER widget_touch BEFORE UPDATE ON widget
FOR EACH ROW EXECUTE FUNCTION touch_widget();
`
	if err := os.WriteFile(filepath.Join(dir, "20260101000000_fn.up.sql"), []byte(up), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "20260101000000_fn.down.sql"),
		[]byte("DROP TABLE IF EXISTS widget; DROP FUNCTION IF EXISTS touch_widget();"), 0644); err != nil {
		t.Fatal(err)
	}

	executor := migration.NewExecutor(pool, dir)
	if err := executor.Initialize(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	gen := migration.NewGenerator(dir)
	files, err := gen.ListMigrations()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	mig, err := gen.ReadMigration(files[0])
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := executor.Apply(ctx, *mig, false); err != nil {
		t.Fatalf("apply dollar-quoted migration: %v", err)
	}

	// The trigger works → the function body was applied intact.
	if _, err := pool.Exec(ctx, "INSERT INTO widget (id) VALUES (1)"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE widget SET id = 1 WHERE id = 1"); err != nil {
		t.Fatalf("update: %v", err)
	}
	var updatedAt *string
	if err := pool.QueryRow(ctx, "SELECT updated_at::text FROM widget WHERE id = 1").Scan(&updatedAt); err != nil {
		t.Fatalf("select: %v", err)
	}
	if updatedAt == nil {
		t.Error("trigger did not fire; function body was not applied intact")
	}
}

// TestIntegration_AdvisoryLockLifecycle verifies the advisory lock is held on a
// dedicated connection so Unlock succeeds on the same session, and a second
// executor cannot acquire it while held.
func TestIntegration_AdvisoryLockLifecycle(t *testing.T) {
	_, connStr, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	e1 := migration.NewExecutor(pool, t.TempDir())
	if err := e1.Lock(ctx); err != nil {
		t.Fatalf("e1 lock: %v", err)
	}

	// A second executor must not be able to grab the same lock while held.
	e2 := migration.NewExecutor(pool, t.TempDir())
	got, err := e2.TryLock(ctx)
	if err != nil {
		t.Fatalf("e2 trylock: %v", err)
	}
	if got {
		t.Error("e2 acquired the lock while e1 holds it")
		_ = e2.Unlock(ctx)
	}

	// Unlock must succeed on the same session (previously failed on the pool).
	if err := e1.Unlock(ctx); err != nil {
		t.Fatalf("e1 unlock: %v", err)
	}

	// Now e2 can acquire it.
	got, err = e2.TryLock(ctx)
	if err != nil {
		t.Fatalf("e2 trylock after release: %v", err)
	}
	if !got {
		t.Error("e2 could not acquire lock after e1 released it")
	} else if err := e2.Unlock(ctx); err != nil {
		t.Fatalf("e2 unlock: %v", err)
	}
}
