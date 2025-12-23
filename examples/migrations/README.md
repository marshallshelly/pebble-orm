# Migrations Example

This example demonstrates **schema migration and management** in Pebble ORM including:

- ✅ **Schema Introspection** - Inspect current database schema
- ✅ **Schema Diff** - Compare database vs code schemas
- ✅ **Migration Generation** - Auto-generate SQL migration files
- ✅ **Migration Management** - Track and apply migrations

## Features Demonstrated

### 1. Schema Introspection

```go
introspector := migration.NewIntrospector(db.Pool())
dbSchema, err := introspector.IntrospectSchema(ctx)
// Returns map of table metadata from database
```

### 2. Code Schema from Models

```go
codeSchema := registry.AllTables()
// Returns map of table metadata from Go structs
```

### 3. Schema Diff

```go
differ := migration.NewDiffer()
diff := differ.Compare(dbSchema, codeSchema)

if diff.HasChanges() {
    // Tables added, dropped, or modified
}
```

### 4. Safe Migration SQL Generation (⭐ NEW in v1.4.0)

**Migrations are now idempotent by default!**

```go
// Default: Safe migrations with IF NOT EXISTS
planner := migration.NewPlanner()
upSQL, downSQL := planner.GenerateMigration(diff)

// Generated SQL includes IF NOT EXISTS:
// CREATE TABLE IF NOT EXISTS users (...);
// CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);
```

**Benefits:**

- ✅ Safe to run multiple times without errors
- ✅ Applications can restart without migration failures
- ✅ Deployments are more robust
- ✅ No manual error handling needed

**Custom Options (Optional):**

```go
// For strict migrations (fail if table exists)
strictPlanner := migration.NewPlannerWithOptions(migration.PlannerOptions{
    IfNotExists: false, // Disable IF NOT EXISTS
})
upSQL, downSQL := strictPlanner.GenerateMigration(diff)
```

### 5. Migration File Generation

```go
generator := migration.NewGenerator("./migrations")
file, err := generator.GenerateEmpty("add_users_table")
// Creates timestamped .up.sql and .down.sql files
```

## Running the Example

### Prerequisites

- PostgreSQL running on `localhost:5432`
- Database: `pebble_migrations_demo`

```bash
# Create database
createdb pebble_migrations_demo

# Run the example
cd examples/migrations
go run cmd/migrations/main.go
```

### For Production Deployments

This example uses custom table names (`// table_name: products`). For production Docker builds:

```bash
# Generate table name metadata before building
pebble generate metadata --scan ./internal/models

# This creates internal/models/table_names.gen.go
# Commit this file so custom table names work in production!
```

See [`../custom_table_names/README.md`](../custom_table_names/README.md) for details.

## Project Structure

```
migrations/
├── cmd/
│   └── migrations/
│       └── main.go           # Main application
├── internal/
│   ├── database/
│   │   └── db.go             # Database connection
│   └── models/
│       ├── models.go         # Model definitions
│       └── registry.go       # Model registration
├── migrations/               # Generated migration files (created at runtime)
│   ├── 20240101120000_add_users.up.sql
│   └── 20240101120000_add_users.down.sql
├── go.mod
└── README.md
```

## Models

### User

```go
type User struct {
    ID        int       `po:"id,primaryKey,serial"`
    Name      string    `po:"name,varchar(255),notNull"`
    Email     string    `po:"email,varchar(255),unique,notNull"`
    CreatedAt time.Time `po:"created_at,timestamp,default(NOW()),notNull"`
}
```

### Product

```go
type Product struct {
    ID    int    `po:"id,primaryKey,serial"`
    Name  string `po:"name,varchar(255),notNull"`
    Price int    `po:"price,integer,notNull"`
}
```

## Example Output

```
=== Migrations & Schema Management Example ===

✅ Connected to database

--- Example 1: Schema Introspection ---
✅ Found 2 tables in database
  - users
  - products

--- Example 2: Code Schema (from structs) ---
✅ Found 2 models registered
  - users (4 columns)
  - products (3 columns)

--- Example 3: Schema Diff ---
✅ Database schema matches code schema (no changes)

--- Example 4: Migration SQL Generation ---
(No changes to generate)

--- Example 5: Migration File Generation ---
✅ Created migration files:
  - ./migrations/20240122030000_add_products_and_categories.up.sql
  - ./migrations/20240122030000_add_products_and_categories.down.sql

--- Example 6: List Migrations ---
✅ Found 1 migrations:
  - 20240122030000_add_products_and_categories

✅ Migration examples completed!

Key Takeaways:
  - Introspect database to get current schema
  - Compare DB schema with code schema to detect changes
  - Generate migration SQL automatically
  - Create timestamped migration files
  - Use 'pebble' CLI for production migrations
```

## Migration Workflow

### 1. Development Workflow

```bash
# 1. Modify your models
type User struct {
    // Add new field
    Age int `po:"age,integer"`
}

# 2. Run migration example to see diff
go run cmd/migrations/main.go

# 3. Generate migration
# (In production, use pebble CLI)
```

### 2. Generated Migration Files

**UP Migration** (`YYYYMMDDHHMMSS_name.up.sql`):

```sql
-- Safe migrations with IF NOT EXISTS (default)
CREATE TABLE IF NOT EXISTS users (
    id serial NOT NULL,
    name varchar(255) NOT NULL,
    email varchar(255) NOT NULL UNIQUE,
    age integer,
    created_at timestamp NOT NULL DEFAULT NOW(),
    CONSTRAINT users_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email);

-- Add new column (if modifying existing table)
ALTER TABLE users ADD COLUMN IF NOT EXISTS age INTEGER;
```

**DOWN Migration** (`YYYYMMDDHHMMSS_name.down.sql`):

```sql
-- Remove column (rollback)
ALTER TABLE users DROP COLUMN age;
```

### 3. Apply Migrations

```bash
# Apply all pending migrations
pebble migrate up

# Rollback last migration
pebble migrate down

# Check migration status
pebble migrate status
```

## Schema Diff Detection

The differ detects:

- ✅ **Tables Added** - New tables in code
- ✅ **Tables Dropped** - Tables removed from code
- ✅ **Tables Modified** - Changes to existing tables
  - Columns added/dropped/modified
  - Indexes added/dropped
  - Foreign keys added/dropped
  - Constraints added/dropped
  - Primary key changes

## Migration Best Practices

### ✅ DO:

- **Use safe migrations** (default IF NOT EXISTS behavior)
- Use timestamped filenames (auto-generated)
- Write reversible migrations (both up and down)
- Test migrations on staging before production
- Keep migrations small and focused
- Version control your migration files
- Run migrations idempotently (safe to re-run)

### ❌ DON'T:

- Edit applied migrations (create new ones instead)
- Delete migration files (breaks version tracking)
- Skip migrations (apply them in order)
- Mix schema and data changes in one migration
- Disable IF NOT EXISTS unless you have a specific reason

## Safe Migrations (v1.4.0+)

### Why Safe by Default?

Traditional migrations fail when re-run:

```sql
CREATE TABLE users (...);
-- ERROR: relation "users" already exists ❌
```

Pebble ORM migrations are idempotent:

```sql
CREATE TABLE IF NOT EXISTS users (...);
-- ✅ No error if table exists
```

### When to Use Strict Mode

Disable IF NOT EXISTS only when:

- You need to detect schema drift errors
- You want migrations to fail loudly if tables exist
- You're doing one-time setup in controlled environments

```go
// Strict mode
planner := migration.NewPlannerWithOptions(migration.PlannerOptions{
    IfNotExists: false,
})
```

## Common Scenarios

### Adding a Column

```sql
-- up
ALTER TABLE users ADD COLUMN phone VARCHAR(20);

-- down
ALTER TABLE users DROP COLUMN phone;
```

### Adding an Index

```sql
-- up
CREATE INDEX idx_users_email ON users(email);

-- down
DROP INDEX idx_users_email;
```

### Adding a Foreign Key

```sql
-- up
ALTER TABLE posts
ADD CONSTRAINT fk_posts_author
FOREIGN KEY (author_id) REFERENCES users(id)
ON DELETE CASCADE;

-- down
ALTER TABLE posts DROP CONSTRAINT fk_posts_author;
```

## Troubleshooting

### Schema Mismatch

If database and code schemas don't match:

1. Run the example to see the diff
2. Generate a migration to sync them
3. Apply the migration

### Migration Files Not Found

```bash
# Create migrations directory
mkdir -p migrations

# Run example again
go run cmd/migrations/main.go
```

## Learn More

- **Migration Docs**: `../docs/MIGRATIONS.md`
- **Schema Package**: `pkg/schema/`
- **Migration Package**: `pkg/migration/`

## Key Takeaways

1. **Automatic Detection** - Pebble compares DB vs code automatically
2. **Type-Safe** - Schema derived from Go structs
3. **Reversible** - Both up and down migrations generated
4. **Safe by Default** - IF NOT EXISTS makes migrations idempotent ⭐
5. **Production-Ready** - Use `pebble` CLI for deployment
6. **Git-Friendly** - Track migrations in version control

**This example shows the foundation for production schema management!** 🎉
