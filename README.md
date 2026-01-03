<p align="center">
  <img src="assets/images/logo.png" alt="Pebble ORM Logo" width="200"/>
</p>

<h1 align="center">Pebble ORM</h1>

> **Type-safe PostgreSQL ORM for Go. Compile-time safety, runtime speed.**

A production-ready ORM leveraging Go generics for type-safe queries, struct-tag schemas, and zero-overhead performance with native pgx integration.

[![Go Reference](https://pkg.go.dev/badge/github.com/marshallshelly/pebble-orm.svg)](https://pkg.go.dev/github.com/marshallshelly/pebble-orm)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14+-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/marshallshelly/pebble-orm/actions/workflows/ci.yml/badge.svg)](https://github.com/marshallshelly/pebble-orm/actions)

## ✨ Features

- **Type-Safe Queries**: Write `builder.Select[User](db)` and get `[]User`, not `[]interface{}`
- **Zero Overhead**: Compile-time generics, no reflection in query execution
- **Struct-Tag Schemas**: Define database schemas with intuitive struct tags
- **Native pgx**: 30-50% faster than database/sql with rich PostgreSQL features
- **Auto-Migrations**: Generate migrations from schema diffs automatically
- **Relationships**: hasMany, hasOne, belongsTo, manyToMany with eager loading
- **CASCADE DELETE**: Database-level foreign key constraints via tags
- **Transactions**: Full transaction support with proper error handling
- **PostgreSQL Features**: JSONB, arrays, enum types, UUID, geometric types, full-text search

## Quick Start

### Installation

```bash
go get github.com/marshallshelly/pebble-orm
```

### Project Structure

For production applications, organize your code with proper separation of concerns:

```
myapp/
├── cmd/
│   └── myapp/
│       └── main.go           # Application entry point
├── internal/
│   ├── database/
│   │   └── db.go             # Database connection & config
│   └── models/
│       ├── user.go           # User model
│       ├── post.go           # Post model
│       └── registry.go       # Model registration
├── config/
│   └── config.go             # Application configuration
└── go.mod
```

### Quick Example

#### 1. Define Models (`internal/models/user.go`)

```go
package models

import "time"

// table_name: users
type User struct {
    ID        string    `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
    Name      string    `po:"name,varchar(255),notNull"`
    Email     string    `po:"email,varchar(320),unique,notNull"`
    Age       int       `po:"age,integer,notNull"`
    CreatedAt time.Time `po:"created_at,timestamptz,default(NOW()),notNull"`
}
```

#### 2. Model Registry (`internal/models/registry.go`)

```go
package models

import "github.com/marshallshelly/pebble-orm/pkg/registry"

// RegisterAll registers all models with Pebble ORM
func RegisterAll() error {
    if err := registry.Register(User{}); err != nil {
        return err
    }
    // Register other models...
    return nil
}
```

#### 3. Database Connection (`internal/database/db.go`)

```go
package database

import (
    "context"
    "fmt"
    "os"

    "github.com/marshallshelly/pebble-orm/pkg/runtime"
    "myapp/internal/models"
)

func Connect(ctx context.Context) (*runtime.DB, error) {
    // Get connection string from environment
    connStr := os.Getenv("DATABASE_URL")
    if connStr == "" {
        connStr = "postgres://localhost:5432/mydb?sslmode=disable"
    }

    // Register models
    if err := models.RegisterAll(); err != nil {
        return nil, fmt.Errorf("failed to register models: %w", err)
    }

    // Connect to database
    db, err := runtime.ConnectWithURL(ctx, connStr)
    if err != nil {
        return nil, fmt.Errorf("failed to connect: %w", err)
    }

    return db, nil
}
```

#### 4. Main Application (`cmd/myapp/main.go`)

```go
package main

import (
    "context"
    "log"

    "github.com/marshallshelly/pebble-orm/pkg/builder"
    "myapp/internal/database"
    "myapp/internal/models"
)

func main() {
    ctx := context.Background()

    // Connect to database
    db, err := database.Connect(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create query builder
    qb := builder.New(db)

    // Type-safe queries with full inference!
    // Query with type-safe column names
    users, err := builder.Select[models.User](qb).
        Where(builder.Gte(builder.Col[models.User]("Age"), 18)).
        OrderByDesc(builder.Col[models.User]("CreatedAt")).
        Limit(10).
        All(ctx)
    if err != nil {
        log.Fatal(err)
    }

    // users is []models.User, not []interface{} ✅
    for _, user := range users {
        log.Printf("%s (%s)", user.Name, user.Email)  // Direct access!
    }
}
```

### Schema Tag Syntax

Tags follow the format: `po:"column_name,option1,option2(value),option3"`

**Supported Options:**

- **Types**: `uuid`, `varchar(n)`, `text`, `smallint`, `integer`, `bigint`, `numeric(p,s)`, `boolean`, `timestamp`, `jsonb`, etc.
- **Constraints**: `primaryKey`, `notNull`, `unique`, `default(value)`
- **Auto-increment**: `serial`, `bigserial`, `autoIncrement`

## Table of Contents

- [Quick Start](#quick-start)
- [Query Builder](#query-builder)
- [Relationships](#relationships)
- [Transactions](#transactions)
- [Migrations](#migrations)
- [PostgreSQL Features](#postgresql-features)
- [CLI](#cli)
- [Testing](#testing)
- [Examples](#examples)

## Query Builder

### SELECT Queries

```go
// Basic query
users, err := builder.Select[User](qb).
    Where(builder.Eq("active", true)).
    All(ctx)

// Complex conditions
users, err := builder.Select[User](qb).
    Where(builder.Gt("age", 18)).
    And(builder.Like("email", "%@example.com")).
    OrderByDesc("created_at").
    Limit(10).
    All(ctx)

// First result
user, err := builder.Select[User](qb).
    Where(builder.Eq("id", 1)).
    First(ctx)

// Count and aggregation
count, err := builder.Select[User](qb).
    Where(builder.Gt("age", 21)).
    Count(ctx)

// Group by
results, err := builder.Select[User](qb).
    Columns("role", "COUNT(*) as count").
    GroupBy("role").
    Having(builder.Gt("COUNT(*)", 5)).
    All(ctx)

// Joins
users, err := builder.Select[User](qb).
    InnerJoin("orders", "orders.user_id = users.id").
    Where(builder.Eq("orders.status", "completed")).
    All(ctx)
```

### INSERT Queries

```go
// Single insert with RETURNING
inserted, err := builder.Insert[User](qb).
    Values(newUser).
    Returning("*").
    ExecReturning(ctx)

// Bulk insert
users := []User{{Name: "John"}, {Name: "Jane"}}
count, err := builder.Insert[User](qb).
    Values(users...).
    Exec(ctx)

// Upsert (ON CONFLICT)
count, err := builder.Insert[User](qb).
    Values(user).
    OnConflictDoUpdate([]string{"email"}, map[string]interface{}{"name": "Updated"}).
    Exec(ctx)
```

### UPDATE and DELETE

```go
// Update
count, err := builder.Update[User](qb).
    Set("age", 31).
    Where(builder.Eq("id", 1)).
    Exec(ctx)

// Delete
count, err := builder.Delete[User](qb).
    Where(builder.Lt("age", 18)).
    Exec(ctx)
```

## Relationships

Define relationships with struct tags:

```go
// One-to-many
type Author struct {
    ID    int    `po:"id,primaryKey,serial"`
    Name  string `po:"name,varchar(100),notNull"`
    Books []Book `po:"-,hasMany,foreignKey(author_id),references(id)"`
}

// Eager loading (prevents N+1 queries)
authors, err := builder.Select[Author](qb).
    Preload("Books").
    All(ctx)

// One-to-one
type User struct {
    Profile *Profile `po:"-,hasOne,foreignKey(user_id),references(id)"`
}

// Many-to-many
type User struct {
    Roles []Role `po:"-,manyToMany,joinTable(user_roles),foreignKey(user_id)"`
}
```

## Transactions

```go
tx, err := qb.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback() // Rollback if we don't reach Commit

// Perform operations
_, err = tx.Insert(User{}).Values(user).Exec()
_, err = tx.Update(Account{}).Set("balance", 1000).Exec()

// Savepoints
tx.Savepoint("before_update")
// ... operations ...
tx.RollbackToSavepoint("before_update")

// Commit
if err := tx.Commit(); err != nil {
    return err
}
```

## Migrations

### Generate from Schema Diff

```go
// Compare code vs database
diff := differ.Compare(codeSchema, dbSchema)

// Generate migration files
generator := migration.NewGenerator("./migrations")
file, err := generator.Generate("add_users", diff)
// Creates: ./migrations/20240115120000_add_users.up.sql
//          ./migrations/20240115120000_add_users.down.sql
```

### Apply Migrations

```go
executor := migration.NewExecutor(pool, "./migrations")
executor.ApplyAll(ctx, migrations, false)
```

### CLI

```bash
# Generate migration from models (no database required!)
pebble generate --name initial_schema --models ./internal/models

# Generate migration by comparing with existing database
pebble generate --name add_users --db "postgres://..." --models ./internal/models

# Apply migrations
pebble migrate up --all --db "postgres://..."

# Interactive mode
pebble migrate up --interactive --db "postgres://..."

# Rollback
pebble migrate down --steps 1 --db "postgres://..."

# Check status
pebble migrate status --db "postgres://..."
```

## PostgreSQL Features

### JSONB

pebble-orm supports three ways to work with JSONB fields:

```go
// 1. Direct struct scanning (Recommended - uses pgx native support)
type Attributes struct {
    Color  string   `json:"color"`
    Sizes  []string `json:"sizes"`
    InStock bool    `json:"inStock"`
}

type Product struct {
    ID         int         `po:"id,primaryKey,serial"`
    Name       string      `po:"name,varchar(255),notNull"`
    Attributes *Attributes `po:"attributes,jsonb"` // Use pointer for NULL handling
}

product := Product{
    Name: "T-Shirt",
    Attributes: &Attributes{
        Color:   "red",
        Sizes:   []string{"S", "M", "L"},
        InStock: true,
    },
}

// 2. Generic map (flexible schema)
type ProductWithMap struct {
    ID         int          `po:"id,primaryKey,serial"`
    Attributes schema.JSONB `po:"attributes,jsonb"` // map[string]interface{}
}

// 3. Typed wrapper (for backward compatibility)
type ProductWithWrapper struct {
    ID         int                             `po:"id,primaryKey,serial"`
    Attributes schema.JSONBStruct[Attributes] `po:"attributes,jsonb"`
}

// Query JSONB (works with all three approaches)
products, err := builder.Select[Product](db).
    Where(builder.JSONBContains("attributes", `{"color": "red"}`)).
    All(ctx)

// JSONB operators
products, err := builder.Select[Product](db).
    Where(builder.JSONBHasKey("attributes", "sizes")).
    All(ctx)
```

### Arrays

```go
type Post struct {
    Tags []string `po:"tags,text[]"`
}

posts, err := builder.Select[Post](qb).
    Where(builder.ArrayContains("tags", []string{"golang"})).
    All(ctx)
```

### Enum Types

```go
type OrderStatus string

type Order struct {
    ID     int         `po:"id,primaryKey,serial"`
    Status OrderStatus `po:"status,enum(pending,active,completed),notNull"`
}

// Automatic migration generates:
// CREATE TYPE order_status AS ENUM ('pending', 'active', 'completed');
// CREATE TABLE orders (
//     id serial PRIMARY KEY,
//     status order_status NOT NULL
// );

// Query using enum values
orders, err := builder.Select[Order](qb).
    Where(builder.Eq(builder.Col[Order]("Status"), "active")).
    All(ctx)
```

### CTEs and Subqueries

```go
// CTE
cte := builder.NewCTEBuilder()
cte.Add("active_users", "SELECT * FROM users WHERE active = true")

// Subquery
subquery := builder.NewSubquery("SELECT AVG(age) FROM users")
users, err := builder.Select[User](qb).
    Where(builder.GtSubquery("age", subquery)).
    All(ctx)
```

## CLI

```bash
# Install
go install github.com/marshallshelly/pebble-orm/cmd/pebble@latest

# Commands
pebble generate --name migration_name [--empty]
pebble generate metadata --scan ./internal/models  # Generate table name metadata from comment directives
pebble migrate up [--all|--steps N] [--interactive]
pebble migrate down [--steps N|--target VERSION]
pebble migrate status
pebble introspect [--table TABLE]
pebble diff

# Global flags
--db "postgres://..."
--migrations-dir ./migrations
--verbose
--json
```

## Testing

### Integration Tests

Integration tests use testcontainers to spin up a real PostgreSQL instance:

```bash
# Ensure Docker is running
docker ps

# Run integration tests
make test-integration

# Or with go test (requires build tag)
go test -tags=integration -v ./...
```

Note: Integration tests require Docker. They will be skipped if Docker is not available.

## Examples

See [`examples/`](examples/) for comprehensive working examples:

- [Basic](examples/basic/) - CRUD operations, type-safe queries
- [Custom Table Names](examples/custom_table_names/) - Table name customization
- [Relationships](examples/relationships/) - hasMany, hasOne, belongsTo, manyToMany, eager loading
- [Transactions](examples/transactions/) - Commits, rollbacks, atomic operations
- [Migrations](examples/migrations/) - Schema introspection, diff generation, migration files
- [PostgreSQL Features](examples/postgresql/) - JSONB, arrays, UUID, geometric types
- [CASCADE DELETE](examples/cascade_delete/) - Foreign key cascade actions

Each example follows production-ready structure. Run any example:

```bash
cd examples/basic
go run cmd/basic/main.go
```

## Development

### Prerequisites

- Go 1.24 or higher
- PostgreSQL 12 or higher
- golangci-lint (for linting)

### Building

```bash
make build        # Build the CLI
make install      # Install the CLI
make test         # Run tests
make lint         # Run linter
make help         # Show all commands
```

### Running Tests

```bash
make test              # All tests
make test-unit         # Unit tests only
make test-coverage     # With coverage report
```

## Architecture

```
pebble-orm/
├── cmd/pebble/          # CLI application
├── pkg/
│   ├── schema/          # Schema parsing & metadata
│   ├── builder/         # Type-safe query builder
│   ├── migration/       # Migration system
│   ├── dialect/         # PostgreSQL-specific SQL
│   ├── registry/        # Schema registry
│   └── runtime/         # Runtime utilities
├── docs/                # Documentation
└── examples/            # Example applications
```

## Design Philosophy

1. **SQL-First**: Embrace SQL rather than hiding it
2. **Type Safety via Generics**: Leverage Go 1.18+ for compile-time safety
3. **Zero Magic**: Explicit over implicit
4. **pgx Native**: Built specifically for pgx, not database/sql
5. **Developer Experience**: Inspired by drizzle-orm's intuitive API

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Run `make lint` and `make test`
5. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) for details

## Inspiration

This project is inspired by:

- [drizzle-orm](https://orm.drizzle.team/) - Developer experience and API design
- [GORM](https://gorm.io/) - Go ORM patterns
- [Bun](https://bun.uptrace.dev/) - SQL-first approach
- [sqlc](https://sqlc.dev/) - Type safety from SQL

---
