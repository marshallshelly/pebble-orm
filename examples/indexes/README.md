# Indexes

<em>Every PostgreSQL index feature, declared next to the column it serves.</em>

Indexes usually live in a migration file nobody reads, three refactors away from the struct they index. Pebble puts simple indexes in the column tag and complex ones in a comment directly above the struct — and `pebble generate` turns both into real `CREATE INDEX` statements. This example defines 12 models covering the whole feature matrix: btree/gin/brin/hash, expression, partial, covering, operator classes, collations, and `CONCURRENTLY`.

## Run

```bash
cd examples/indexes
createdb pebble_indexes_example

export DATABASE_URL="postgres://postgres:postgres@localhost:5432/pebble_indexes_example?sslmode=disable"

pebble generate --name create_index_tables --models ./internal/models --db "$DATABASE_URL"
pebble migrate up --all --db "$DATABASE_URL"

go run cmd/indexes/main.go
```

`DATABASE_URL` is optional — the example falls back to the URL above.

## What it shows

Two ways to declare an index. Column-level, inside the `po:` tag:

| Tag | Generated SQL |
|-----|---------------|
| `index` | `CREATE INDEX idx_products_name ON products (name)` (auto-named) |
| `index(idx_product_sku)` | same, with your name |
| `index(idx_product_tags,gin)` | `... USING gin (tags)` |
| `index(idx_products_created,btree,desc)` | `... (created_at DESC)` |

Table-level, in comments above the struct — full `CREATE INDEX` syntax:

| Comment | Feature |
|---------|---------|
| `// index: idx_email_lower ON (lower(email))` | expression index |
| `// index: idx_active_users ON (email) WHERE deleted_at IS NULL` | partial index |
| `// index: idx_orders_customer_status ON (customer_id, status) INCLUDE (total_amount, created_at)` | covering index (index-only scans) |
| `// index: idx_events_tenant_created ON (tenant_id, created_at DESC NULLS LAST)` | multicolumn, mixed ordering |
| `// index: idx_search_term_pattern ON (term varchar_pattern_ops)` | operator class for `LIKE 'prefix%'` |
| `// index: idx_intl_name_en ON (name COLLATE "en_US")` | collation |
| `// index: idx_sensor_timestamp ON (recorded_at) USING brin` | BRIN for time-series |
| `// index: idx_api_key_hash ON (key_hash) USING hash` | hash for equality-only |
| `// index: idx_analytics_timestamp ON (event_timestamp DESC) CONCURRENTLY` | non-blocking build |

Everything composes. The kitchen-sink index from the `Article` model:

```go
// table_name: articles
// index: idx_articles_advanced ON (author varchar_pattern_ops COLLATE "C" DESC, published_at DESC NULLS LAST) INCLUDE (title, slug) WHERE status = 'published' CONCURRENTLY
type Article struct {
    ID     int64  `po:"id,primaryKey,bigint,identity"`
    Slug   string `po:"slug,varchar(255),unique,notNull"`
    Author string `po:"author,varchar(255),notNull"`
    Status string `po:"status,varchar(50),default('draft'),notNull"`
}
```

All 12 models live in [internal/models/models.go](internal/models/models.go). The runner ([cmd/indexes/main.go](cmd/indexes/main.go)) inserts a row into each table and queries it back along the indexed path. A query shaped for the covering index on `orders`:

```go
// Satisfiable entirely from idx_orders_customer_status — an index-only scan.
orders, err := builder.Select[models.Order](qb).
    Columns(
        builder.Col[models.Order]("CustomerID"),
        builder.Col[models.Order]("Status"),
        builder.Col[models.Order]("TotalAmount"),
        builder.Col[models.Order]("CreatedAt"),
    ).
    Where(builder.Eq(builder.Col[models.Order]("CustomerID"), customerID)).
    And(builder.Eq(builder.Col[models.Order]("Status"), "pending")).
    All(ctx)
```

## Picking an index type

| Type | Best for | Caveat |
|------|----------|--------|
| btree | ~90% of cases: equality, ranges, `ORDER BY` | — |
| gin | JSONB, arrays, full-text | larger, slower writes |
| brin | append-only time-series, huge tables | only pays off on naturally-ordered data |
| hash | equality-only lookups (API key hashes) | no `<`, `>`, `LIKE`, no sorting |
| gist | geometric/range types | niche |

Rules of thumb the models encode: partial indexes for soft-delete patterns (`WHERE deleted_at IS NULL`), `INCLUDE` for dashboard queries you want served from the index alone, `CONCURRENTLY` for anything built against a live production table, and skip the `index` tag on `unique` columns — PostgreSQL already indexes those.

<details>
<summary><strong>Sample of the generated SQL</strong></summary>

```sql
-- Column-level tags (Product)
CREATE INDEX IF NOT EXISTS idx_products_name ON products (name);
CREATE INDEX IF NOT EXISTS idx_product_tags ON products USING gin (tags);
CREATE INDEX IF NOT EXISTS idx_products_created ON products (created_at DESC);

-- Expression + partial (User)
CREATE INDEX IF NOT EXISTS idx_email_lower ON users (lower(email));
CREATE INDEX IF NOT EXISTS idx_active_users ON users (email) WHERE deleted_at IS NULL;

-- Covering (Order)
CREATE INDEX IF NOT EXISTS idx_orders_customer_status
    ON orders (customer_id, status) INCLUDE (total_amount, created_at);

-- BRIN (SensorReading)
CREATE INDEX IF NOT EXISTS idx_sensor_timestamp ON sensor_readings USING brin (recorded_at);

-- Hash (APIKey)
CREATE INDEX IF NOT EXISTS idx_api_key_hash ON api_keys USING hash (key_hash);

-- Everything at once (Article)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_articles_advanced
    ON articles (author varchar_pattern_ops COLLATE "C" DESC, published_at DESC NULLS LAST)
    INCLUDE (title, slug)
    WHERE status = 'published';
```

</details>

Verify what the planner actually does with `EXPLAIN ANALYZE` — an index you never hit is just write overhead.

Related: [basic](../basic/) · [multi-tenancy](../multi-tenancy/) · [PostgreSQL index types](https://www.postgresql.org/docs/current/indexes-types.html) · [index-only scans](https://www.postgresql.org/docs/current/indexes-index-only-scans.html)
