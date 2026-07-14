# Migrations

<em>The introspect → diff → plan → generate pipeline, taken apart piece by piece.</em>

`pebble generate` is one command, but under it sits a pipeline: introspect the live database, compare it against your registered structs, plan the SQL, write timestamped files. This example runs each stage programmatically via `pkg/migration`, so you can see exactly what the CLI does — and drive it yourself if you need migrations inside your own tooling.

## Run

```bash
cd examples/migrations
createdb pebble_migrations

export DATABASE_URL="postgres://localhost:5432/pebble_migrations?sslmode=disable"
```

CLI route (what you'd do in a real project):

```bash
pebble generate --name initial_schema --models ./internal/models --verbose   # no DB needed
pebble migrate up --all --db "$DATABASE_URL"
pebble migrate status --db "$DATABASE_URL"
```

Programmatic route (this example):

```bash
go run cmd/migrations/main.go
```

`DATABASE_URL` is optional — the example falls back to the URL above.

## What it shows

Two small models ([internal/models/models.go](internal/models/models.go)) — `Product` and `Category`, both `serial` primary keys — pushed through every stage:

| Stage | API | What it does |
|-------|-----|--------------|
| Introspect | `migration.NewIntrospector(db.Pool()).IntrospectSchema(ctx)` | Live schema from `information_schema` |
| Code schema | `registry.AllTables()` | Schema from your registered structs |
| Diff | `migration.NewDiffer().Compare(codeSchema, dbSchema)` | Tables added/dropped/modified, down to columns, indexes, FKs |
| Plan | `migration.NewPlanner().GenerateMigration(diff)` | Up + down SQL, `IF NOT EXISTS` by default |
| Generate | `migration.NewGenerator("./migrations")` | Timestamped `.up.sql` / `.down.sql` files |

The core loop from [cmd/migrations/main.go](cmd/migrations/main.go):

```go
introspector := migration.NewIntrospector(db.Pool())
dbSchema, err := introspector.IntrospectSchema(ctx)

codeSchema := registry.AllTables()

differ := migration.NewDiffer()
diff := differ.Compare(codeSchema, dbSchema)

if diff.HasChanges() {
    planner := migration.NewPlanner()
    upSQL, downSQL := planner.GenerateMigration(diff)
    // upSQL: CREATE TABLE IF NOT EXISTS products (...); ...
}
```

Migrations are idempotent by default — re-running one doesn't error out mid-deploy. If you'd rather have loud failures on schema drift, opt into strict mode:

```go
strictPlanner := migration.NewPlannerWithOptions(migration.PlannerOptions{
    IfNotExists: false, // CREATE TABLE fails if the table already exists
})
```

File generation and listing:

```go
generator := migration.NewGenerator("./migrations")
file, err := generator.GenerateEmpty("add_products_and_categories")
// file.UpPath:   ./migrations/20240122030000_add_products_and_categories.up.sql
// file.DownPath: ./migrations/20240122030000_add_products_and_categories.down.sql

migrations, err := generator.ListMigrations()
```

<details>
<summary><strong>Expected output</strong></summary>

```
=== Migrations & Schema Management Example ===

Connected to database

--- Example 1: Schema Introspection ---
Found 2 tables in database
  - products
  - categories

--- Example 2: Code Schema (from structs) ---
Found 2 models registered
  - products (5 columns)
  - categories (2 columns)

--- Example 3: Schema Diff ---
Database schema matches code schema (no changes)

--- Example 4: Safe Migration SQL Generation ---
(no changes to generate)

--- Example 5: Migration File Generation ---
Created migration files:
  - ./migrations/20240122030000_add_products_and_categories.up.sql
  - ./migrations/20240122030000_add_products_and_categories.down.sql

--- Example 6: List Migrations ---
Found 1 migrations:
  - 20240122030000_add_products_and_categories
```

On a fresh database, Example 3 instead reports both tables as "to add" and Example 4 prints the generated `CREATE TABLE IF NOT EXISTS` up SQL plus the matching `DROP TABLE` down SQL.

</details>

Ground rules that keep this pipeline honest: never edit an applied migration (write a new one), never delete migration files (they're the version history), and let the CLI apply them in production — it takes an advisory lock and records each version in `schema_migrations`.

Related: [basic](../basic/) · [identity-columns](../identity-columns/) · root README's [Migrations section](../../README.md#migrations)
