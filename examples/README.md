# Examples

Eleven runnable programs, one per feature area. Each is a standalone Go module laid out the way you'd lay out a real service — a thin `main.go`, models in one place, connection code in another — so you can lift the structure straight into your own project:

```
example_name/
├── cmd/example_name/main.go      # entry point
├── internal/
│   ├── database/db.go            # connection + config
│   └── models/                   # models + registration
├── go.mod
└── README.md                     # per-example docs
```

## Run any example

```bash
cd examples/basic
export DATABASE_URL="postgres://localhost:5432/pebble_basic?sslmode=disable"
go run cmd/basic/main.go
```

The binary directory usually matches the example name; four don't:

| Example | Run command |
|---------|-------------|
| `custom_table_names` | `go run cmd/custom_tables/main.go` |
| `generated_columns` | `go run cmd/generated/main.go` |
| `identity-columns` | `go run cmd/identity-example/main.go` |
| `multi-tenancy` | `go run cmd/multi-tenancy/main.go` (starts a REST API on :3000) |

## The examples

Start with `basic`, then wander wherever your schema hurts.

| Example | What it shows |
|---------|---------------|
| [basic](basic/) | Full CRUD: insert with `RETURNING`, select with conditions/ordering/limits, update, delete, count — plus the production layout everything else copies |
| [relationships](relationships/) | All four relationship shapes (`belongsTo`, `hasOne`, `hasMany`, `manyToMany`) and eager loading with `Preload()` to kill N+1 queries |
| [transactions](transactions/) | Begin/commit/rollback, savepoints, and multi-operation atomicity |
| [migrations](migrations/) | The full pipeline: introspect a live DB, diff against your structs, generate `.up.sql`/`.down.sql`, apply with version tracking, roll back |
| [postgresql](postgresql/) | JSONB storage and scanning, array columns (including `schema.StringArray` for PgBouncer), window functions, full-text search types, geometric types |
| [indexes](indexes/) | Every index type (btree, gin, gist, brin, hash) plus expression, partial, covering, and multicolumn indexes, operator classes, collations, and `CONCURRENTLY` |
| [custom_table_names](custom_table_names/) | `// table_name:` directives, the snake_case fallback, and `pebble generate metadata` for compiled production builds |
| [cascade_delete](cascade_delete/) | Foreign key referential actions: `CASCADE`, `SET NULL`, and `RESTRICT` |
| [identity-columns](identity-columns/) | `GENERATED AS IDENTITY` (the SQL-standard successor to `serial`), both `ALWAYS` and `BY DEFAULT` variants |
| [generated_columns](generated_columns/) | `GENERATED ... STORED` columns computed from other columns — concatenations, unit conversions, price calculations |
| [multi-tenancy](multi-tenancy/) | A GDPR-flavored multi-tenant REST API (Go Fiber): tenant-scoped queries, soft delete, audit logging, consent tracking, data export |

## Prerequisites

- Go 1.26+ and PostgreSQL 14+ (Docker works fine: `docker run --name pebble-postgres -e POSTGRES_PASSWORD=password -p 5432:5432 -d postgres:alpine`)
- A database for the example — each one reads `DATABASE_URL` and falls back to a local default (the exact name is in each example's README), so either export the URL or `createdb` the default

## Shared patterns

Every example resolves column names through `builder.Col[T]("FieldName")` instead of hardcoded strings — the struct tag stays the single source of truth, so renaming a column means editing one tag, not hunting down every query that mentions it.

The wiring is identical across examples: `internal/models/registry.go` registers every model in one place, `internal/database/db.go` reads the environment and connects, and `main.go` just calls both and gets to work. Copy it.
