# Generated Columns

<em>Columns that compute themselves. You write the expression once; PostgreSQL keeps it true forever.</em>

PostgreSQL `GENERATED ALWAYS AS (...) STORED` columns, declared in struct tags. Insert `first_name` and `last_name`; read back `full_name`. Update a source column and the generated value follows — no application code, no drift.

## Run

```bash
createdb pebble_generated_demo
export DATABASE_URL="postgres://user:password@localhost:5432/pebble_generated_demo?sslmode=disable"

cd examples/generated_columns
go run cmd/generated/main.go
```

If `DATABASE_URL` is unset, it defaults to `postgres://postgres:postgres@localhost:5432/pebble_generated_demo?sslmode=disable`.

## What it shows

| Pattern | Model | Expression |
|---------|-------|------------|
| String concatenation | `Person.FullName` | `first_name \|\| ' ' \|\| last_name` |
| Unit conversion | `Measurement.HeightIn`, `WeightLbs` | `height_cm / 2.54`, `weight_kg * 2.20462` |
| Price math | `Product.NetPrice` | list price + tax% − discount% |
| Querying/sorting by a generated column | Example 4 | `Where(Gt(...NetPrice, 100))` |
| Auto-update when a source column changes | Example 5 | update `FirstName`, re-read `FullName` |

## Tag syntax

```go
// table_name: people
type Person struct {
    ID        int64  `po:"id,primaryKey,autoIncrement"`
    FirstName string `po:"first_name,varchar(100),notNull"`
    LastName  string `po:"last_name,varchar(100),notNull"`
    FullName  string `po:"full_name,varchar(255),generated:first_name || ' ' || last_name,stored"`
}
```

`generated:EXPRESSION` sets the SQL expression; `stored` marks it `GENERATED ALWAYS AS (...) STORED`. Migrations emit:

```sql
full_name varchar(255) GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED
```

You never set the field on insert — the database fills it, and `Returning("*")` brings it back:

```go
inserted, err := builder.Insert[models.Person](qb).
    Values(models.Person{FirstName: "John", LastName: "Doe"}).
    Returning("*").
    ExecReturning(ctx)
// inserted[0].FullName == "John Doe"
```

Generated columns query and sort like any other column:

```go
expensive, err := builder.Select[models.Product](qb).
    Where(builder.Gt(builder.Col[models.Product]("NetPrice"), 100.00)).
    OrderByDesc(builder.Col[models.Product]("NetPrice")).
    All(ctx)
```

<details>
<summary>PostgreSQL rules worth knowing</summary>

- Read-only: you can't INSERT or UPDATE a generated column directly
- Expressions must be immutable — no subqueries, no `NOW()`/`RANDOM()`, no references to other generated columns
- No `DEFAULT`, `IDENTITY`, or direct `UNIQUE`/`NOT NULL` on the column itself
  - Want uniqueness? `CREATE UNIQUE INDEX ... ON table (generated_column)`
  - Want NOT NULL semantics? Mark the *source* columns `notNull` (as this example does)
- Generated columns can be indexed like regular columns

Full details: [PostgreSQL docs on generated columns](https://www.postgresql.org/docs/current/ddl-generated-columns.html)

</details>
