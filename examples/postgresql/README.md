# PostgreSQL Features Example

This example demonstrates **advanced PostgreSQL features** in Pebble ORM including:

- ✅ **JSONB** - Store and query JSON data
- ✅ **Arrays** - PostgreSQL array types
- ✅ **UUID** - Universally unique identifiers
- ✅ **Geometric Types** - Points, polygons, etc.
- ✅ **Custom Types** - Enums, composite types

## Features Demonstrated

### 1. JSONB (JSON Binary)

```go
type Document struct {
    Metadata JSONB `db:"metadata,jsonb"`
}

// Insert JSONB
doc := Document{
    Metadata: JSONB{"author": "Alice", "tags": []string{"tech", "go"}},
}

// Query JSONB
// WHERE metadata->>'author' = 'Alice'
```

### 2. PostgreSQL Arrays

```go
type Document struct {
    Tags []string `db:"tags,text[]"`
}

// Insert array
doc := Document{
    Tags: []string{"programming", "golang", "database"},
}
```

### 3. UUID Type

```go
type Session struct {
    ID   string `db:"id,uuid"`
    Name string `db:"name"`
}

// UUIDs are handled as strings
```

### 4. Geometric Types

```go
type Location struct {
    Coords string `db:"coords,point"`
}

// Point format: "(x,y)"
location := Location{
    Coords: "(37.7749,-122.4194)", // San Francisco
}
```

## Running the Example

### Prerequisites

- PostgreSQL running on `localhost:5432`
- Database: `pebble_pg_features`
- PostgreSQL 12+ (for JSONB features)

```bash
# Create database
createdb pebble_pg_features

# Run the example
cd examples/postgresql
go run cmd/postgresql/main.go
```

## Project Structure

```
postgresql/
├── cmd/
│   └── postgresql/
│       └── main.go           # Main application
├── internal/
│   ├── database/
│   │   └── db.go             # Database connection
│   └── models/
│       ├── models.go         # PostgreSQL-specific models
│       └── registry.go       # Model registration
├── go.mod
└── README.md
```

## Models

### Document (JSONB + Arrays)

```go
type Document struct {
    ID        int64     `db:"id,primary,autoIncrement"`
    Title     string    `db:"title"`
    Metadata  JSONB     `db:"metadata,jsonb"`
    Tags      []string  `db:"tags,text[]"`
    CreatedAt time.Time `db:"created_at"`
}
```

### Product (UUID + JSONB)

```go
type Product struct {
    ID      string  `db:"id,uuid"`
    Name    string  `db:"name"`
    Details JSONB   `db:"details,jsonb"`
    Active  bool    `db:"active"`
}
```

### Location (Geometric Types)

```go
type Location struct {
    ID     int64  `db:"id,primary,autoIncrement"`
    Name   string `db:"name"`
    Coords string `db:"coords,point"`
}
```

## Example Output

```
=== PostgreSQL Advanced Features Example ===

✅ Connected to database

--- Example 1: JSONB Operations ---
Created document with JSONB metadata
  Title: Getting Started with Pebble ORM
  Metadata: map[author:Alice pages:42 published:true]

--- Example 2: Array Types ---
Created document with tags
  Tags: [golang postgresql pebble-orm]

--- Example 3: UUID Primary Keys ---
Created product with UUID
  ID: 550e8400-e29b-41d4-a716-446655440000
  Name: Premium Widget

--- Example 4: Geometric Types ---
Created location
  Name: San Francisco Office
  Coordinates: (37.7749,-122.4194)

✅ All PostgreSQL features demonstrated!

Key Takeaways:
  - JSONB for flexible schema-less data
  - Arrays for multi-value columns
  - UUIDs for distributed systems
  - Geometric types for spatial data
```

## JSONB Operations

### Inserting JSONB

```go
metadata := JSONB{
    "author": "Alice",
    "tags":   []string{"tech", "programming"},
    "stats":  map[string]int{"views": 100, "likes": 50},
}

doc := Document{Metadata: metadata}
builder.Insert[Document](qb).Values(doc).Exec(ctx)
```

### Querying JSONB (Raw SQL)

```sql
-- Get documents by author
SELECT * FROM documents
WHERE metadata->>'author' = 'Alice';

-- Query nested JSON
SELECT * FROM documents
WHERE metadata->'stats'->>'views' > '100';

-- Check JSON key existence
SELECT * FROM documents
WHERE metadata ? 'author';
```

## Array Operations

### Inserting Arrays

```go
doc := Document{
    Tags: []string{"golang", "postgresql", "orm"},
}
```

### Querying Arrays (Raw SQL)

```sql
-- Contains any element
SELECT * FROM documents 1688
WHERE 'golang' = ANY(tags);

-- Array overlap
SELECT * FROM documents
WHERE tags && ARRAY['golang', 'python'];

-- Array length
SELECT * FROM documents
WHERE array_length(tags, 1) > 2;
```

## UUID Best Practices

### Generating UUIDs

```go
import "github.com/google/uuid"

product := Product{
    ID:   uuid.New().String(),
    Name: "Widget",
}
```

### Advantages

- ✅ Globally unique (distributed systems)
- ✅ No auto-increment conflicts
- ✅ Unpredictable (security)
- ✅ Merge-friendly (multi-database)

### Disadvantages

- ❌ Larger than BIGINT (16 bytes vs 8)
- ❌ Slower indexes
- ❌ Not sequential

## Geometric Types Supported

### Point

```go
Coords string `db:"coords,point"`
// Format: "(x,y)"
// Example: "(37.7749,-122.4194)"
```

### Other Types

- `line` - Infinite line
- `lseg` - Line segment
- `box` - Rectangle
- `path` - Geometric path
- `polygon` - Closed path
- `circle` - Circle

**Note:** Most geometric types are stored as strings in Go. For advanced spatial queries, consider PostGIS extension.

## JSONB vs JSON

| Feature         | JSONB (Binary)    | JSON (Text)  |
| --------------- | ----------------- | ------------ |
| **Storage**     | Binary            | Text         |
| **Performance** | ✅ Faster queries | ❌ Slower    |
| **Indexing**    | ✅ GIN indexes    | ❌ Limited   |
| **Ordering**    | ❌ Reordered      | ✅ Preserved |
| **Whitespace**  | ❌ Removed        | ✅ Preserved |

**Recommendation: Use JSONB for almost everything.**

## Array Types Supported

```go
// Text array
Tags []string `db:"tags,text[]"`

// Integer array
Numbers []int64 `db:"numbers,bigint[]"`

// Boolean array
Flags []bool `db:"flags,boolean[]"`

// Float array
Prices []float64 `db:"prices,double precision[]"`
```

## Custom PostgreSQL Types

### Enums (Future Support)

```sql
CREATE TYPE status AS ENUM ('pending', 'active', 'archived');

-- In Go (current workaround):
type Status string

const (
    StatusPending  Status = "pending"
    StatusActive   Status = "active"
    StatusArchived Status = "archived"
)
```

## Performance Tips

### JSONB Indexing

```sql
-- GIN index for JSONB
CREATE INDEX idx_metadata_gin ON documents USING GIN (metadata);

-- Index specific JSONB path
CREATE INDEX idx_metadata_author ON documents ((metadata->>'author'));
```

### Array Indexing

```sql
-- GIN index for arrays
CREATE INDEX idx_tags_gin ON documents USING GIN (tags);
```

### UUID Performance

```sql
-- Use UUID v7 for better indexing (time-ordered)
-- Or stick with BIGSERIAL for maximum performance
```

## Learn More

- **PostgreSQL JSONB**: https://www.postgresql.org/docs/current/datatype-json.html
- **PostgreSQL Arrays**: https://www.postgresql.org/docs/current/arrays.html
- **PostgreSQL UUID**: https://www.postgresql.org/docs/current/datatype-uuid.html
- **Geometric Types**: https://www.postgresql.org/docs/current/datatype-geometric.html

## Key Takeaways

1. **JSONB** - Perfect for flexible, schema-less data
2. **Arrays** - Native multi-value support (no junction tables needed)
3. **UUID** - Great for distributed systems
4. **Type Safety** - Pebble handles PostgreSQL types in Go
5. **Performance** - Use appropriate indexes for each type

**This example shows how to leverage PostgreSQL's powerful type system!** 🚀
