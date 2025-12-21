# Basic CRUD Example - Production Structure

This example demonstrates Pebble ORM's basic CRUD operations using a production-ready project structure.

## Project Structure

```
basic/
├── cmd/
│   └── basic/
│       └── main.go           # Application entry point
├── internal/
│   ├── database/
│   │   └── db.go             # Database connection management
│   └── models/
│       ├── models.go         # User and Post models
│       └── registry.go       # Model registration
└── go.mod
```

## Features Demonstrated

- ✅ **Project Organization**: Proper separation of models, database, and application logic
- ✅ **Model Registration**: Centralized model registration
- ✅ **Environment Variables**: Database URL from environment
- ✅ **INSERT**: Creating new records with RETURNING
- ✅ **SELECT**: Querying with WHERE, ORDER BY, LIMIT
- ✅ **UPDATE**: Updating records with conditions
- ✅ **DELETE**: Removing records
- ✅ **Relationships**: Eager loading with Preload
- ✅ **COUNT**: Aggregate queries
- ✅ **Error Handling**: Proper error checking

## Prerequisites

- Go 1.24+
- PostgreSQL 12+

## Setup

### 1. Create Database

```bash
createdb pebble_basic_example
```

### 2. Set Environment Variable (Optional)

```bash
export DATABASE_URL="postgres://user:password@localhost:5432/pebble_basic_example?sslmode=disable"
```

If not set, defaults to `postgres://localhost:5432/pebble_basic_example?sslmode=disable`

### 3. Run Example

```bash
cd examples/basic
go run cmd/basic/main.go
```

## What It Does

1. **Connects to Database**: Establishes connection and registers models
2. **Inserts User**: Creates a new user record
3. **Queries Users**: Fetches users with WHERE conditions
4. **Updates User**: Modifies user age
5. **Inserts Post**: Creates a post associated with user
6. **Queries with Relationships**: Fetches posts with author information
7. **Counts Records**: Gets total user count
8. **Deletes Records**: Removes unpublished posts

## Expected Output

```
✅ Connected to database successfully

--- Example 1: INSERT ---
Inserted user: {ID:uuid... Name:Alice Johnson Email:alice@example.com Age:28}

--- Example 2: SELECT with WHERE ---
Found 1 users:
  - Alice Johnson (alice@example.com)

--- Example 3: UPDATE ---
Updated 1 rows

--- Example 4: INSERT Post ---
Inserted post: Getting Started with Pebble ORM

--- Example 5: SELECT with Relationship ---
Found 1 published posts:
  - Getting Started with Pebble ORM by Alice Johnson

--- Example 6: COUNT ---
Total users in database: 1

--- Example 7: DELETE ---
Deleted 0 unpublished posts

✅ All examples completed!
```

## Key Takeaways

### 1. Separation of Concerns

Models, database connection, and business logic are in separate packages:

```go
// internal/models/models.go - Define your schema
type User struct {
    ID   string `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
    Name string `po:"name,varchar(255),notNull"`
}

// internal/database/db.go - Handle connections
func Connect(ctx context.Context) (*runtime.DB, error) {
    // Connection logic
}

// cmd/basic/main.go - Application logic
func main() {
    db, _ := database.Connect(ctx)
    // Use db...
}
```

### 2. Model Registration

All models registered in one place:

```go
// internal/models/registry.go
func RegisterAll() error {
    registry.Register(User{})
    registry.Register(Post{})
    return nil
}
```

### 3. Environment-Based Configuration

```go
connStr := os.Getenv("DATABASE_URL")
if connStr == "" {
    connStr = "postgres://localhost:5432/mydb"
}
```

## Next Steps

- Try the [Relationships Example](../relationships) for more complex associations
- See [Transactions Example](../transactions) for atomicity
- Check [Migrations Example](../migrations) for schema management
