# PostgreSQL Indexes Example - Comprehensive Index Features

This example demonstrates Pebble ORM's comprehensive PostgreSQL index support, including all advanced index features for production applications.

## Project Structure

```
indexes/
├── cmd/
│   └── indexes/
│       └── main.go           # Application demonstrating all index types
├── internal/
│   ├── database/
│   │   └── db.go             # Database connection management
│   └── models/
│       ├── models.go         # 12 model types with comprehensive index examples
│       └── registry.go       # Model registration
└── go.mod
```

## Features Demonstrated

This example covers **all** PostgreSQL index features supported by Pebble ORM:

### ✅ Basic Index Features

- **Column-Level Indexes**: Simple syntax for common indexes
- **Custom Index Names**: Explicit naming for clarity
- **Index Types**: btree, gin, gist, brin, hash
- **Column Ordering**: ASC/DESC with NULLS FIRST/LAST
- **Multicolumn Indexes**: Composite indexes on multiple columns

### ✅ Advanced Index Features

- **Expression Indexes**: Indexes on computed values (e.g., `lower(email)`)
- **Partial Indexes**: Conditional indexes with WHERE clauses
- **Covering Indexes**: INCLUDE columns for index-only scans
- **Operator Classes**: Pattern-matching optimization (varchar_pattern_ops, text_pattern_ops)
- **Collations**: Locale-specific sorting (en_US, C, etc.)
- **CONCURRENTLY**: Production-safe index creation without blocking writes
- **GIN Indexes**: JSONB and array queries
- **BRIN Indexes**: Space-efficient time-series data

### ✅ Table-Level Complex Indexes

- Full PostgreSQL CREATE INDEX syntax via table comments
- All features combinable in a single index
- Automatic migration generation

## Prerequisites

- Go 1.26+
- PostgreSQL 12+

## Setup

### 1. Create Database

```bash
createdb pebble_indexes_example
```

### 2. Set Environment Variable (Optional)

```bash
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/pebble_indexes_example?sslmode=disable"
```

If not set, the example uses the default connection string above.

### 3. Generate and Apply Migrations

```bash
cd examples/indexes

# Generate migration from models
pebble generate --name create_index_tables --db "$DATABASE_URL"

# Apply migrations
pebble migrate up --all --db "$DATABASE_URL"
```

### 4. Run Example

```bash
go run cmd/indexes/main.go
```

## Index Examples Explained

### Example 1: Simple Column-Level Indexes

**Model**: `Product`

```go
type Product struct {
    Name     string    `po:"name,varchar(255),notNull,index"`                       // Auto-named: idx_products_name
    SKU      string    `po:"sku,varchar(100),unique,notNull,index(idx_product_sku)"` // Custom name
    Price    float64   `po:"price,numeric(10,2),notNull,index"`                     // Auto-named: idx_products_price
    Category string    `po:"category,varchar(100),notNull,index"`                   // For filtering
    Tags     []string  `po:"tags,text[],index(idx_product_tags,gin)"`               // GIN index for arrays
    CreatedAt time.Time `po:"created_at,timestamptz,default(NOW()),notNull,index(idx_products_created,btree,desc)"` // DESC
}
```

**Use Cases**:

- Basic WHERE filtering on indexed columns (name, category, price)
- Sorting by created_at DESC (most recent first)
- Array searches using GIN index (find products by tags)

**Migrations Generated**:

```sql
CREATE INDEX IF NOT EXISTS idx_products_name ON products (name);
CREATE INDEX IF NOT EXISTS idx_product_sku ON products (sku);
CREATE INDEX IF NOT EXISTS idx_products_price ON products (price);
CREATE INDEX IF NOT EXISTS idx_products_category ON products (category);
CREATE INDEX IF NOT EXISTS idx_product_tags ON products USING gin (tags);
CREATE INDEX IF NOT EXISTS idx_products_created ON products (created_at DESC);
```

---

### Example 2: Expression Indexes and Partial Indexes

**Model**: `User`

```go
// table_name: users
// index: idx_email_lower ON (lower(email))
// index: idx_active_users ON (email) WHERE deleted_at IS NULL
// index: idx_premium_users ON (user_id) WHERE subscription_tier = 'premium'
type User struct {
    Email            string     `po:"email,varchar(320),unique,notNull"`
    SubscriptionTier string     `po:"subscription_tier,varchar(50),default('free'),notNull"`
    DeletedAt        *time.Time `po:"deleted_at,timestamptz"`
}
```

**Use Cases**:

- **Expression Index**: Case-insensitive email lookups (`WHERE lower(email) = lower(?)`)
- **Partial Indexes**: Index only relevant rows (active users, premium users)
- Reduces index size and improves performance

**Migrations Generated**:

```sql
CREATE INDEX IF NOT EXISTS idx_email_lower ON users (lower(email));
CREATE INDEX IF NOT EXISTS idx_active_users ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_premium_users ON users (user_id) WHERE subscription_tier = 'premium';
```

**Why Partial Indexes?**

- Smaller index size (only indexes filtered rows)
- Faster queries on filtered data
- Ideal for soft-delete patterns or status-based filtering

---

### Example 3: Covering Indexes (INCLUDE columns)

**Model**: `Order`

```go
// table_name: orders
// index: idx_orders_customer_status ON (customer_id, status) INCLUDE (total_amount, created_at)
// index: idx_orders_created_covering ON (created_at DESC) INCLUDE (customer_id, total_amount, status)
type Order struct {
    CustomerID   int64     `po:"customer_id,bigint,notNull,index"`
    Status       string    `po:"status,varchar(50),default('pending'),notNull"`
    TotalAmount  float64   `po:"total_amount,numeric(12,2),notNull"`
}
```

**Use Cases**:

- **Index-Only Scans**: Query satisfied entirely from the index (no table access needed!)
- Perfect for dashboard queries that need a few specific columns
- Significantly faster for common queries

**Migrations Generated**:

```sql
CREATE INDEX IF NOT EXISTS idx_orders_customer_status
    ON orders (customer_id, status) INCLUDE (total_amount, created_at);

CREATE INDEX IF NOT EXISTS idx_orders_created_covering
    ON orders (created_at DESC) INCLUDE (customer_id, total_amount, status);
```

**Query Example**:

```go
// This query uses index-only scan (never touches the table!)
orders, _ := builder.Select[Order](db).
    Columns("customer_id", "status", "total_amount", "created_at").
    Where(builder.Eq("customer_id", 123)).
    And(builder.Eq("status", "pending")).
    All(ctx)
```

---

### Example 4: Multicolumn Indexes with Mixed Ordering

**Model**: `Event`

```go
// table_name: events
// index: idx_events_tenant_created ON (tenant_id, created_at DESC NULLS LAST)
// index: idx_events_user_type_created ON (user_id, event_type, created_at DESC)
type Event struct {
    TenantID  int64          `po:"tenant_id,bigint,notNull"`
    UserID    int64          `po:"user_id,bigint,notNull"`
    EventType string         `po:"event_type,varchar(100),notNull"`
    Data      schema.JSONB   `po:"data,jsonb,index(idx_events_data,gin)"` // GIN for JSONB
    CreatedAt time.Time      `po:"created_at,timestamptz,default(NOW()),notNull"`
}
```

**Use Cases**:

- Multi-tenant queries (filter by tenant_id, order by created_at DESC)
- Perfect for paginated recent event lists
- GIN index enables complex JSONB queries

**Migrations Generated**:

```sql
CREATE INDEX IF NOT EXISTS idx_events_tenant_created
    ON events (tenant_id, created_at DESC NULLS LAST);

CREATE INDEX IF NOT EXISTS idx_events_user_type_created
    ON events (user_id, event_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_events_data ON events USING gin (data);
```

---

### Example 5: Operator Classes for Pattern Matching

**Model**: `SearchTerm`

```go
// table_name: search_terms
// index: idx_search_term_pattern ON (term varchar_pattern_ops)
// index: idx_search_description_pattern ON (description text_pattern_ops)
type SearchTerm struct {
    Term        string `po:"term,varchar(255),notNull"`
    Description string `po:"description,text"`
    SearchCount int64  `po:"search_count,bigint,default(0),notNull,index(idx_search_count,btree,desc)"`
}
```

**Use Cases**:

- **Operator Classes**: Optimize LIKE queries with leading wildcards
- `varchar_pattern_ops` and `text_pattern_ops` enable efficient pattern matching
- Essential for search features

**Migrations Generated**:

```sql
CREATE INDEX IF NOT EXISTS idx_search_term_pattern
    ON search_terms (term varchar_pattern_ops);

CREATE INDEX IF NOT EXISTS idx_search_description_pattern
    ON search_terms (description text_pattern_ops);

CREATE INDEX IF NOT EXISTS idx_search_count
    ON search_terms (search_count DESC);
```

**Why Operator Classes?**

- Standard indexes don't optimize `LIKE 'pattern%'` queries well in non-C locales
- Pattern operator classes ensure efficient prefix searches
- Critical for autocomplete and search features

---

### Example 6: Collations for Locale-Specific Sorting

**Model**: `InternationalProduct`

```go
// table_name: international_products
// index: idx_intl_name_en ON (name COLLATE "en_US")
// index: idx_intl_name_case_sensitive ON (name COLLATE "C")
type InternationalProduct struct {
    Name        string `po:"name,varchar(255),notNull"`
    Locale      string `po:"locale,varchar(10),notNull,index"`
    CountryCode string `po:"country_code,varchar(2),notNull,index"`
}
```

**Use Cases**:

- Locale-specific alphabetical sorting
- Case-sensitive vs case-insensitive sorting
- Multi-language product catalogs

**Migrations Generated**:

```sql
CREATE INDEX IF NOT EXISTS idx_intl_name_en
    ON international_products (name COLLATE "en_US");

CREATE INDEX IF NOT EXISTS idx_intl_name_case_sensitive
    ON international_products (name COLLATE "C");
```

**Collation Options**:

- `en_US`: English (US) locale sorting
- `C`: Byte-order (case-sensitive, fastest)
- `en_US.utf8`: UTF-8 aware English sorting
- `fr_FR`, `de_DE`, etc.: Language-specific sorting

---

### Example 7: CONCURRENTLY for Production

**Model**: `AnalyticsEvent`

```go
// table_name: analytics_events
// index: idx_analytics_timestamp ON (event_timestamp DESC) CONCURRENTLY
// index: idx_analytics_user_session ON (user_id, session_id) CONCURRENTLY
// index: idx_analytics_event_type ON (event_type) CONCURRENTLY WHERE processed = false
type AnalyticsEvent struct {
    UserID         *int64         `po:"user_id,bigint,index"`
    SessionID      string         `po:"session_id,uuid,notNull"`
    EventType      string         `po:"event_type,varchar(100),notNull"`
    EventTimestamp time.Time      `po:"event_timestamp,timestamptz,default(NOW()),notNull"`
    Processed      bool           `po:"processed,boolean,default(false),notNull"`
    Properties     schema.JSONB   `po:"properties,jsonb,index(idx_analytics_props,gin)"`
}
```

**Use Cases**:

- **Production-Safe Index Creation**: Build indexes without blocking writes
- Critical for large, high-traffic tables
- Prevents downtime during index creation

**Migrations Generated**:

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_analytics_timestamp
    ON analytics_events (event_timestamp DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_analytics_user_session
    ON analytics_events (user_id, session_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_analytics_event_type
    ON analytics_events (event_type) WHERE processed = false;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_analytics_props
    ON analytics_events USING gin (properties);
```

**CONCURRENTLY Benefits**:

- Index builds without blocking table writes
- Takes longer to build, but no downtime
- Essential for production databases

---

### Example 8: Complex Multi-Feature Index

**Model**: `Document`

```go
// table_name: documents
// index: idx_documents_complex ON (owner_id, status, updated_at DESC NULLS LAST)
//        INCLUDE (title, version) WHERE deleted_at IS NULL
// index: idx_documents_search ON (to_tsvector('english', title || ' ' || content))
// index: idx_documents_tags ON (tags) USING gin
type Document struct {
    OwnerID   int64      `po:"owner_id,bigint,notNull"`
    Title     string     `po:"title,varchar(500),notNull"`
    Content   string     `po:"content,text,notNull"`
    Status    string     `po:"status,varchar(50),default('draft'),notNull"`
    Version   int        `po:"version,integer,default(1),notNull"`
    Tags      []string   `po:"tags,text[]"`
    DeletedAt *time.Time `po:"deleted_at,timestamptz"`
    UpdatedAt time.Time  `po:"updated_at,timestamptz,default(NOW()),notNull"`
}
```

**Use Cases**:

- **Ultimate Index**: Combines multiple features (multicolumn, ordering, INCLUDE, WHERE)
- Perfect for user document dashboards
- Index-only scans for common queries
- Full-text search with expression index
- Array search with GIN index

**Migrations Generated**:

```sql
CREATE INDEX IF NOT EXISTS idx_documents_complex
    ON documents (owner_id, status, updated_at DESC NULLS LAST)
    INCLUDE (title, version)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_documents_search
    ON documents (to_tsvector('english', title || ' ' || content));

CREATE INDEX IF NOT EXISTS idx_documents_tags
    ON documents USING gin (tags);
```

---

### Example 9: BRIN Index for Time-Series Data

**Model**: `SensorReading`

```go
// table_name: sensor_readings
// index: idx_sensor_timestamp ON (recorded_at) USING brin
// index: idx_sensor_device_timestamp ON (device_id, recorded_at) USING brin
type SensorReading struct {
    DeviceID   string    `po:"device_id,varchar(100),notNull"`
    SensorType string    `po:"sensor_type,varchar(50),notNull"`
    Value      float64   `po:"value,numeric(10,4),notNull"`
    RecordedAt time.Time `po:"recorded_at,timestamptz,default(NOW()),notNull"`
}
```

**Use Cases**:

- **BRIN (Block Range Index)**: Extremely space-efficient for naturally-ordered data
- Perfect for time-series, log data, append-only tables
- 100-1000x smaller than btree indexes
- Ideal for range queries on sequential data

**Migrations Generated**:

```sql
CREATE INDEX IF NOT EXISTS idx_sensor_timestamp
    ON sensor_readings USING brin (recorded_at);

CREATE INDEX IF NOT EXISTS idx_sensor_device_timestamp
    ON sensor_readings USING brin (device_id, recorded_at);
```

**BRIN Benefits**:

- **Tiny size**: Stores min/max per block range (128 pages by default)
- **Fast for sequential scans**: Ideal for time-range queries
- **Low maintenance**: Minimal update overhead
- **Best for**: Append-only data with natural ordering (timestamps, IDs)

---

### Example 10: Hash Index for Equality-Only Queries

**Model**: `APIKey`

```go
// table_name: api_keys
// index: idx_api_key_hash ON (key_hash) USING hash
type APIKey struct {
    KeyHash   string     `po:"key_hash,varchar(64),unique,notNull"` // SHA-256 hash
    UserID    int64      `po:"user_id,bigint,notNull,index"`
    ExpiresAt *time.Time `po:"expires_at,timestamptz"`
}
```

**Use Cases**:

- **Hash Index**: Fastest for exact equality checks (`WHERE key_hash = ?`)
- Perfect for API key lookups, hash-based authentication
- Smaller than btree for high-cardinality data

**Migrations Generated**:

```sql
CREATE INDEX IF NOT EXISTS idx_api_key_hash
    ON api_keys USING hash (key_hash);
```

**Hash Index Limitations**:

- ⚠️ Only supports `=` operator (no `<`, `>`, `LIKE`, etc.)
- ⚠️ Cannot be used for sorting
- ✅ Faster than btree for equality
- ✅ Smaller index size

---

### Example 11: Advanced NULLS Ordering

**Model**: `Task`

```go
// table_name: tasks
// index: idx_tasks_priority ON (priority DESC NULLS LAST, due_date ASC NULLS FIRST)
// index: idx_tasks_assigned ON (assigned_to) WHERE assigned_to IS NOT NULL AND completed_at IS NULL
type Task struct {
    Title       string     `po:"title,varchar(500),notNull"`
    Priority    *int       `po:"priority,integer"` // NULL means no priority
    DueDate     *time.Time `po:"due_date,timestamptz"`
    AssignedTo  *int64     `po:"assigned_to,bigint"`
    CompletedAt *time.Time `po:"completed_at,timestamptz"`
}
```

**Use Cases**:

- **NULLS Ordering**: Control where NULL values appear in sorted results
- `NULLS LAST`: Show prioritized tasks first, unprioritized last
- `NULLS FIRST`: Show tasks without due dates first
- Essential for task management, priority queues

**Migrations Generated**:

```sql
CREATE INDEX IF NOT EXISTS idx_tasks_priority
    ON tasks (priority DESC NULLS LAST, due_date ASC NULLS FIRST);

CREATE INDEX IF NOT EXISTS idx_tasks_assigned
    ON tasks (assigned_to)
    WHERE assigned_to IS NOT NULL AND completed_at IS NULL;
```

---

### Example 12: Composite with All Features

**Model**: `Article`

```go
// table_name: articles
// index: idx_articles_advanced ON (author varchar_pattern_ops COLLATE "C" DESC, published_at DESC NULLS LAST)
//        INCLUDE (title, slug) WHERE status = 'published' CONCURRENTLY
type Article struct {
    Slug        string     `po:"slug,varchar(255),unique,notNull"`
    Title       string     `po:"title,varchar(500),notNull"`
    Author      string     `po:"author,varchar(255),notNull"`
    Status      string     `po:"status,varchar(50),default('draft'),notNull"`
    PublishedAt *time.Time `po:"published_at,timestamptz"`
}
```

**Use Cases**:

- **Ultimate Example**: Combines operator class, collation, ordering, INCLUDE, WHERE, and CONCURRENTLY
- Demonstrates all index features working together
- Production-ready for high-traffic content sites

**Migrations Generated**:

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_articles_advanced
    ON articles (author varchar_pattern_ops COLLATE "C" DESC, published_at DESC NULLS LAST)
    INCLUDE (title, slug)
    WHERE status = 'published';
```

---

## Expected Output

```
=== Pebble ORM Index Examples ===

--- Example 1: Simple Column-Level Indexes ---
✓ Inserted product: PostgreSQL Database Book
✓ Found 1 products (uses idx_products_category and idx_products_price)

--- Example 2: GIN Index for Array Searches ---
✓ Found 1 products with 'postgresql' tag (uses idx_product_tags GIN index)

--- Example 3: Expression Indexes and Partial Indexes ---
✓ Inserted user: Alice Smith
✓ Found 1 active premium users (uses idx_premium_users partial index)

--- Example 4: Covering Indexes (INCLUDE columns) ---
✓ Inserted order: ID=1, Amount=$149.99
✓ Found 1 pending orders (uses idx_orders_customer_status covering index - index-only scan!)

--- Example 5: Multicolumn Indexes with Mixed Ordering ---
✓ Inserted event: page_view for tenant 1
✓ Found 1 recent events for tenant (uses idx_events_tenant_created multicolumn index)
✓ Found 1 events on /products page (uses idx_events_data GIN index)

--- Example 6: Operator Classes for Pattern Matching ---
✓ Inserted search term: postgresql database
✓ Found 1 terms matching 'postgres%' (uses idx_search_term_pattern with varchar_pattern_ops)

--- Example 7: DESC Ordering with NULLS LAST ---
✓ Inserted task: Implement PostgreSQL indexes
✓ Found 1 incomplete tasks ordered by priority (uses idx_tasks_priority)

--- Example 8: BRIN Index for Time-Series Data ---
✓ Inserted sensor reading: 72.50 fahrenheit
✓ Found 1 readings in last hour (uses idx_sensor_timestamp BRIN index)

--- Example 9: Hash Index for Equality-Only Queries ---
✓ Inserted API key: Production API Key
✓ Found API key by hash (uses idx_api_key_hash HASH index)
  Key: Production API Key

--- Example 10: Complex Multi-Feature Index ---
✓ Inserted document: PostgreSQL Index Optimization Guide
✓ Found 1 published documents (uses idx_documents_complex partial covering index)
✓ Found 1 documents tagged with 'postgresql' (uses idx_documents_tags GIN index)

--- Example 11: Collations for Locale-Specific Sorting ---
✓ Inserted international product: Café Parisien (fr_FR)
✓ Found 1 French products sorted by name (uses idx_intl_name_en with en_US collation)

--- Example 12: CONCURRENTLY for Production ---
✓ Inserted analytics event: page_view
✓ Found 1 unprocessed analytics events (uses idx_analytics_event_type partial index)

=== Index Examples Complete ===

Key Takeaways:
• Simple column indexes improve basic WHERE and ORDER BY queries
• GIN indexes enable efficient JSONB and array queries
• Expression indexes optimize queries on computed values
• Partial indexes reduce size and improve performance for filtered data
• Covering indexes (INCLUDE) enable index-only scans
• Operator classes optimize specific query patterns (LIKE, etc.)
• Multicolumn indexes with mixed ordering support complex queries
• BRIN indexes are space-efficient for time-series data
• Hash indexes are fastest for equality-only lookups
• CONCURRENTLY creates indexes without blocking writes in production
```

## Index Types Reference

| Type      | Best For                     | Operators                                             | Size          | Speed     |
| --------- | ---------------------------- | ----------------------------------------------------- | ------------- | --------- | --------- |
| **btree** | General purpose, sorting     | `=`, `<`, `>`, `<=`, `>=`, `BETWEEN`, `IN`, `IS NULL` | Medium        | Fast      |
| **gin**   | JSONB, arrays, full-text     | `@>`, `<@`, `?`, `?                                   | `, `?&`, `@@` | Large     | Very Fast |
| **gist**  | Geometric, ranges, full-text | Spatial operators, range operators                    | Large         | Fast      |
| **brin**  | Time-series, sequential data | `=`, `<`, `>`, `<=`, `>=`, `BETWEEN`                  | Tiny          | Medium    |
| **hash**  | Equality-only lookups        | `=` only                                              | Small         | Very Fast |

## Operator Classes Reference

| Operator Class        | Column Type | Use Case                             |
| --------------------- | ----------- | ------------------------------------ |
| `varchar_pattern_ops` | varchar     | LIKE patterns with leading wildcards |
| `text_pattern_ops`    | text        | LIKE patterns with leading wildcards |
| `varchar_ops`         | varchar     | Standard operations (default)        |
| `text_ops`            | text        | Standard operations (default)        |

## Collations Reference

| Collation              | Description                          |
| ---------------------- | ------------------------------------ |
| `en_US`                | English (US) locale sorting          |
| `C`                    | Byte-order (case-sensitive, fastest) |
| `POSIX`                | Same as C                            |
| `en_US.utf8`           | UTF-8 aware English sorting          |
| `fr_FR`, `de_DE`, etc. | Language-specific sorting            |

## Key Takeaways

### 1. Column-Level Index Syntax

Simple and clean syntax for common indexes:

```go
type User struct {
    Email     string `po:"email,varchar(320),index"`                       // Auto-named
    Age       int    `po:"age,integer,index(idx_users_age)"`               // Custom name
    CreatedAt time.Time `po:"created_at,timestamptz,index(idx_created,btree,desc)"` // With options
}
```

### 2. Table-Level Complex Indexes

Full PostgreSQL syntax for advanced features:

```go
// table_name: users
// index: idx_email_lower ON (lower(email))
// index: idx_active ON (email) WHERE deleted_at IS NULL
// index: idx_complex ON (col1, col2 DESC) INCLUDE (col3, col4) WHERE condition CONCURRENTLY
type User struct {
    // fields...
}
```

### 3. Index Selection Guidelines

**Use btree when:**

- General purpose queries (90% of cases)
- Range queries (`<`, `>`, `BETWEEN`)
- Sorting (`ORDER BY`)
- Unique constraints

**Use gin when:**

- JSONB queries (`@>`, `?`, etc.)
- Array containment (`@>`, `&&`)
- Full-text search

**Use gist when:**

- Geometric data (PostGIS)
- Range types
- Full-text search (alternative to gin)

**Use brin when:**

- Very large tables (billions of rows)
- Naturally ordered data (timestamps, sequential IDs)
- Append-only workloads
- Space is critical

**Use hash when:**

- Only equality checks (`=`)
- High-cardinality data
- No sorting needed

### 4. Production Best Practices

1. **Use CONCURRENTLY**: Always use `CONCURRENTLY` for production index creation
2. **Partial Indexes**: Index only the data you query (reduce size, improve speed)
3. **Covering Indexes**: Use INCLUDE for index-only scans (huge performance boost)
4. **Monitor Size**: Check `pg_indexes` regularly, drop unused indexes
5. **Test Performance**: Use `EXPLAIN ANALYZE` to verify index usage
6. **BRIN for Time-Series**: For append-only tables, BRIN saves massive space

### 5. Common Patterns

**User Authentication:**

```go
// Expression index for case-insensitive email lookup
// index: idx_email_lower ON (lower(email))
```

**Multi-Tenant SaaS:**

```go
// Compound index with tenant isolation
// index: idx_tenant_created ON (tenant_id, created_at DESC)
```

**E-Commerce Products:**

```go
// Partial index for in-stock products
// index: idx_available_products ON (category, price) WHERE in_stock = true
```

**Analytics/Logging:**

```go
// BRIN index for time-series data
// index: idx_timestamp ON (created_at) USING brin
```

## Next Steps

- Try the [Basic Example](../basic) for simpler index usage
- See [Transactions Example](../transactions) for atomicity
- Check [Relationships Example](../relationships) for complex associations
- Review [Multi-Tenancy Example](../multi-tenancy) for SaaS patterns

## References

- [PostgreSQL Index Types](https://www.postgresql.org/docs/current/indexes-types.html)
- [BRIN Indexes](https://www.postgresql.org/docs/current/brin-intro.html)
- [Index-Only Scans](https://www.postgresql.org/docs/current/indexes-index-only-scans.html)
- [Operator Classes](https://www.postgresql.org/docs/current/indexes-opclass.html)
