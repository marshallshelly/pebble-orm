# Custom Table Names

<em>Your struct is `User`; your legacy table is `custom_users_table`. Both can be right.</em>

By default Pebble snake_cases the struct name (`Order` → `order`). When that's wrong — legacy schemas, plural conventions, team standards — a comment directive above the struct overrides it.

## Run

```bash
createdb pebble_custom_tables
export DATABASE_URL="postgres://user:password@localhost:5432/pebble_custom_tables?sslmode=disable"

cd examples/custom_table_names
go run cmd/custom_tables/main.go
```

If `DATABASE_URL` is unset, it defaults to `postgres://localhost:5432/pebble_custom_tables?sslmode=disable`.

## What it shows

| Struct | Directive | Table |
|--------|-----------|-------|
| `User` | `// table_name: custom_users_table` | `custom_users_table` |
| `Product` | `// table_name: products_inventory` | `products_inventory` |
| `Order` | none | `order` (snake_case fallback) |

Plus: all builders (`Insert`, `Select`, `Preload`) resolve the custom name automatically, and `registry.Get` reports the resolved name at runtime.

## The directive

```go
// table_name: custom_users_table
type User struct {
    ID    int    `po:"id,primaryKey,serial"`
    Name  string `po:"name,varchar(100),notNull"`
    Email string `po:"email,varchar(255),unique,notNull"`
}
```

A line comment directly above the struct, mixed freely with regular comments; whitespace around the colon is optional. Queries need no changes — `builder.Select[models.User](qb).All(ctx)` reads from `custom_users_table`, and `pebble generate` uses the custom names in migrations.

## Production builds

The directive is read from source files at `registry.Register` time — which don't exist inside a compiled Docker binary. Bake the names in first:

```bash
pebble generate metadata --scan ./internal/models
git add internal/models/table_names.gen.go   # commit it
```

That emits an `init()` that calls `schema.RegisterTableName("User", "custom_users_table")` for each directive, so the mapping survives compilation. If the source can't be found and no generated file exists, Pebble falls back to snake_case.

<details>
<summary>CI guard to keep the generated file fresh</summary>

```bash
pebble generate metadata --scan ./internal/models
git diff --exit-code internal/models/table_names.gen.go || exit 1
```

</details>
