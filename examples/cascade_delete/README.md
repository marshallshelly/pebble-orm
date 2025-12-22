# CASCADE DELETE Example

This example demonstrates **foreign key cascade actions** in Pebble ORM including:

- ✅ **CASCADE DELETE** - Automatically delete child records
- ✅ **SET NULL** - Set foreign key to NULL when parent is deleted
- ✅ **RESTRICT** - Prevent deletion if child records exist

## Features Demonstrated

### 1. CASCADE DELETE

```go
type Post struct {
    AuthorID int64 `db:"author_id,fk:users.id,onDelete:cascade"`
}
```

When a user is deleted, all their posts are automatically deleted by the database.

### 2. SET NULL

```go
type Comment struct {
    AuthorID *int64 `db:"author_id,fk:users.id,onDelete:setnull"`
}
```

When a user is deleted, their comments remain but `author_id` is set to NULL.

### 3. RESTRICT

```go
type Product struct {
    CategoryID int64 `db:"category_id,fk:categories.id,onDelete:restrict"`
}
```

Prevents deletion of a category if products still reference it.

## Running the Example

### Prerequisites

- PostgreSQL running on `localhost:5432`
- Database: `pebble_cascade_demo`

```bash
# Create database
createdb pebble_cascade_demo

# Run the example
cd examples/cascade_delete
go run cmd/cascade_delete/main.go
```

## Project Structure

```
cascade_delete/
├── cmd/
│   └── cascade_delete/
│       └── main.go           # Main application
├── internal/
│   ├── database/
│   │   └── db.go             # Database connection
│   └── models/
│       ├── models.go         # Model definitions with FK constraints
│       └── registry.go       # Model registration
├── go.mod
└── README.md
```

## Models

### User

```go
type User struct {
    ID        int64     `db:"id,primary,autoIncrement"`
    Name      string    `db:"name"`
    Email     string    `db:"email,unique"`
    CreatedAt time.Time `db:"created_at"`
}
```

### Post (CASCADE on user delete)

```go
type Post struct {
    ID        int64     `db:"id,primary,autoIncrement"`
    Title     string    `db:"title"`
    AuthorID  int64     `db:"author_id,fk:users.id,onDelete:cascade"`
    CreatedAt time.Time `db:"created_at"`
}
```

### Comment (CASCADE on post delete, SET NULL on author delete)

```go
type Comment struct {
    ID       int64  `db:"id,primary,autoIncrement"`
    PostID   int64  `db:"post_id,fk:posts.id,onDelete:cascade"`
    AuthorID *int64 `db:"author_id,fk:users.id,onDelete:setnull"`
}
```

### Product & Category (RESTRICT)

```go
type Product struct {
    CategoryID int64 `db:"category_id,fk:categories.id,onDelete:restrict"`
}
```

## Example Output

```
=== CASCADE DELETE & Foreign Key Actions Example ===

✅ Connected to database

--- Example 1: CASCADE DELETE ---
Created user: Alice (ID: 1)
  Created post: Post #1 (ID: 1)
  Created post: Post #2 (ID: 2)

Before deleting user:
  Posts by user 1: 3
  Total comments: 6

🗑️  Deleting user 1...
Deleted 1 user

After deleting user:
  Posts by user 1: 0 ✅ (CASCADE deleted)
  Total comments: 0 ✅ (CASCADE deleted via posts)

--- Example 2: SET NULL ---
Created users: Bob (ID: 2), Charlie (ID: 3)
Bob created post (ID: 4)
Charlie commented on Bob's post

🗑️  Deleting Charlie...

After deleting Charlie:
  Comments with NULL author: 1 ✅ (SET NULL applied)
  Comments still exist, just author is NULL

--- Example 3: RESTRICT ---
Created category: Electronics (ID: 1)
  Created product: Product 1
  Created product: Product 2

🗑️  Attempting to delete category 1...
❌ Deletion prevented by RESTRICT constraint!
✅ This is correct behavior

🗑️  Deleting products first...
Deleted 2 products
🗑️  Now deleting category...
✅ Category deleted successfully

✅ All examples completed!
```

## Key Takeaways

1. **Database Enforces Constraints** - No application code needed
2. **Atomic Operations** - All cascades happen in one transaction
3. **Type Safety** - Use `*int64` for nullable foreign keys (SET NULL)
4. **Performance** - Database-level cascades are fast and efficient

## Learn More

- See `docs/CASCADE_DELETE.md` for complete documentation
- See other examples for more patterns
