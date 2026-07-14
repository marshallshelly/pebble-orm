# PostgreSQL Features

<em>The types that make PostgreSQL worth it — JSONB, arrays, tsvector, point — as plain struct fields.</em>

Four models exercising PostgreSQL-native column types. Everything round-trips through the ordinary `Insert`/`Select` builders; no special-case APIs to learn.

## Run

```bash
createdb pebble_postgresql
export DATABASE_URL="postgres://localhost:5432/pebble_postgresql?sslmode=disable"

cd examples/postgresql
pebble generate --name initial_schema --models ./internal/models
pebble migrate up --all --db "$DATABASE_URL"
go run cmd/postgresql/main.go
```

## What it shows

| Type | Struct field | Tag |
|------|-------------|-----|
| JSONB | `Metadata schema.JSONB` | `po:"metadata,jsonb"` |
| Text array | `Tags []string` | `po:"tags,text[]"` |
| Integer array | `Prices []int` | `po:"prices,integer[]"` |
| PgBouncer-safe array | `Days schema.StringArray` | `po:"days,text[]"` |
| Full-text search | `SearchVec string` | `po:"search_vec,tsvector"` |
| Geometric point | `Coords string` | `po:"coords,point"` |

## JSONB in, JSONB out

`schema.JSONB` is a `map[string]any` that marshals on insert and scans back on select:

```go
doc := models.Document{
    Title:   "PostgreSQL Guide",
    Content: "Complete guide to PostgreSQL features",
    Metadata: schema.JSONB{
        "author": "John Doe",
        "tags":   []string{"database", "postgresql"},
        "views":  1000,
    },
}

result, err := builder.Insert[models.Document](qb).
    Values(doc).
    Returning("*").
    ExecReturning(ctx)
// result[0].Metadata is the map, round-tripped
```

For a typed alternative, point a struct at the column instead — see the root README's JSONB section. For filtering on JSONB contents (`metadata->>'author' = ...`), drop to raw SQL.

## Arrays, two ways

Native slices work with the default pgx protocol. Behind PgBouncer in `simple_protocol` mode, arrays arrive as text (`{Monday,Tuesday}`) that plain slices can't scan — use the `schema` array types:

```go
schedule := models.Schedule{
    Name: "Work Week",
    Days: schema.StringArray{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday"},
}
```

Available: `schema.StringArray`, `Int32Array`, `Int64Array`, `Float64Array`, `BoolArray`.

## Full-text search and geometry

The example stores a `tsvector` column and a `point` column (`Coords: "(37.7749,-122.4194)"`). Populating the search vector and querying with `@@`, `<->`, `&&` etc. is raw-SQL territory — the column types are the ORM's job, the operators are PostgreSQL's.

<details>
<summary><strong>Expected output</strong></summary>

```
=== PostgreSQL Features Example ===
✅ Connected to database

--- Example 1: JSONB (JSON Binary) ---
✅ Created document with JSONB metadata
  Title: PostgreSQL Guide
  Metadata: map[author:John Doe tags:[database postgresql] views:1000]

--- Example 2: PostgreSQL Arrays ---
✅ Created document with tags array
✅ Created product with prices array

--- Example 2b: PgBouncer-Compatible Arrays ---
✅ Created schedule with PgBouncer-compatible array
  Days: [Monday Tuesday Wednesday Thursday Friday]

--- Example 3: Full-Text Search ---
✅ Full-text search uses tsvector and tsquery

--- Example 4: Geometric Types ---
✅ Created location with geometric point
  Coords: (37.7749,-122.4194)

✅ All PostgreSQL feature examples completed!
```

</details>
