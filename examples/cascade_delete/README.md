# Cascade Delete

<em>Let PostgreSQL clean up after you.</em>

Foreign key referential actions — `CASCADE`, `SET NULL`, `RESTRICT` — declared in struct tags and enforced by the database, not application code. Delete a user and their posts vanish, their comments get orphaned gracefully, and a category with products refuses to die.

## Run

```bash
createdb pebble_cascade_demo

cd examples/cascade_delete
go run cmd/cascade_delete/main.go
```

The connection URL is set in `internal/database/db.go`: `postgres://postgres:postgres@localhost:5432/pebble_cascade_demo?sslmode=disable`.

## What it shows

| Action | Behavior | Model |
|--------|----------|-------|
| `onDelete:cascade` | Deleting a user deletes their posts; deleting a post deletes its comments | `Post`, `Comment` |
| `onDelete:setnull` | Deleting a commenter keeps the comment, nulls `author_id` | `Comment` (note the `*int64`) |
| `onDelete:restrict` | Deleting a category with products fails until the products go first | `Product` |

## The tags

```go
type Post struct {
    AuthorID int64 `db:"author_id,fk:users.id,onDelete:cascade"`
}

type Comment struct {
    PostID   int64  `db:"post_id,fk:posts.id,onDelete:cascade"`
    AuthorID *int64 `db:"author_id,fk:users.id,onDelete:setnull"` // pointer: column must be nullable
}

type Product struct {
    CategoryID int64 `db:"category_id,fk:categories.id,onDelete:restrict"`
}
```

Then a plain delete triggers the whole chain — no ORM callbacks, one atomic operation:

```go
deleted, err := builder.Delete[models.User](qb).
    Where(builder.Eq("id", userID)).
    Exec(ctx)
// posts by this user: gone (CASCADE)
// comments on those posts: gone (CASCADE via posts)
```

<details>
<summary>Expected output (abridged)</summary>

```
--- Example 1: CASCADE DELETE ---
Before deleting user:  Posts by user 1: 3, Total comments: 6
Deleted 1 user
After deleting user:   Posts by user 1: 0, Total comments: 0

--- Example 2: SET NULL ---
Deleting Charlie...
Comments with NULL author: 1 (SET NULL applied)

--- Example 3: RESTRICT ---
Attempting to delete category 1 (has products)...
Deletion prevented by RESTRICT constraint!
Deleting all products in category first... Deleted 2 products
Category deleted successfully
```

</details>

## Takeaways

- Constraints live in the database — they hold even when someone bypasses the ORM
- `SET NULL` foreign keys need a pointer type (`*int64`) so the Go side can represent NULL
- `RESTRICT` failures surface as a normal error from `Exec` — handle and delete children first
