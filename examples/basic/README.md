# Basic CRUD

<em>The full insert → select → update → delete loop, laid out the way a real project is.</em>

This is the starter example: every core builder operation against two related models (`User`, `Post`), organized into the `cmd/` + `internal/models/` + `internal/database/` layout you'd actually ship. If you read one example first, read this one.

```
basic/
├── cmd/basic/main.go              # nine numbered examples, top to bottom
└── internal/
    ├── database/db.go             # connection + model registration
    └── models/
        ├── models.go              # User, Post, JSONB structs, enum
        └── registry.go            # RegisterAll()
```

## Run

```bash
createdb pebble_test
export DATABASE_URL="postgres://user:password@localhost:5432/pebble_test?sslmode=disable"

cd examples/basic
go run cmd/basic/main.go
```

If `DATABASE_URL` is unset, it defaults to `postgres://postgres:postgres@localhost:5432/pebble_test?sslmode=disable`.

## What it shows

| Operation | Where |
|-----------|-------|
| Bulk `INSERT ... RETURNING` with JSONB fields | Example 1 |
| `SELECT` with `Where` / `OrderByDesc` / `Limit` | Example 2 |
| `UPDATE` with a condition | Example 3 |
| Enum column (`enum(draft,published,archived)`) | Example 4, 6, 9 |
| `Preload("Author")` — batched, no N+1 | Example 5 |
| `Count` | Example 7 |
| JSONB stored as a plain struct pointer | throughout |
| Column, DESC, GIN, and partial indexes | `models.go` tags |

## JSONB without wrappers

Point a struct pointer at a `jsonb` column and pgx scans it directly — no wrapper type, `nil` for NULL:

```go
type User struct {
    ID          string           `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
    Email       string           `po:"email,varchar(320),unique,notNull,index"`
    Preferences *UserPreferences `po:"preferences,jsonb"`
}

inserted, err := builder.Insert[models.User](qb).
    Values(newUsers...).
    Returning(builder.Col[models.User]("ID"), builder.Col[models.User]("Preferences")).
    ExecReturning(ctx)
// inserted[0].Preferences.Theme == "dark"
```

## Enums and relationships

```go
type PostStatus string

// table_name: posts
// index: idx_posts_status_created ON (status, created_at DESC) WHERE status = 'published'
type Post struct {
    ID       string     `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
    AuthorID string     `po:"author_id,uuid,notNull,index"`
    Status   PostStatus `po:"status,enum(draft,published,archived),default('draft'),notNull"`
    Author   *User      `po:"-,belongsTo,foreignKey(author_id),references(id)"`
}

posts, err := builder.Select[models.Post](qb).
    Where(builder.Eq(builder.Col[models.Post]("Status"), "published")).
    Preload("Author").
    All(ctx)
```

The `enum(...)` tag generates the `CREATE TYPE post_status AS ENUM (...)` in migrations; adding a value to the tag diffs to `ALTER TYPE ... ADD VALUE`.

## Next

[relationships](../relationships) for all four association shapes, [transactions](../transactions) for atomicity, [migrations](../migrations) for schema management.
