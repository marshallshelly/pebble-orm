# Relationships Example

This example demonstrates **relationship handling and eager loading** in Pebble ORM including:

- ✅ **hasMany** - One-to-Many relationships (Author → Books)
- ✅ **belongsTo** - Many-to-One relationships (Book → Author)
- ✅ **hasOne** - One-to-One relationships (User → Profile)
- ✅ **manyToMany** - Many-to-Many relationships (User ↔ Roles)
- ✅ **Preload** - Eager loading to prevent N+1 queries

## Features Demonstrated

### 1. hasMany (One-to-Many)

```go
type Author struct {
    ID    int64  `db:"id,primary,autoIncrement"`
    Name  string `db:"name"`
    Books []Book `po:"-,hasMany,foreignKey(author_id)"`
}

// Eager load books
authors, _ := builder.Select[Author](qb).
    Preload("Books").  // ✅ Loads all books in 2 queries (not N+1)
    All(ctx)

for _, author := range authors {
    fmt.Printf("Author: %s\n", author.Name)
    for _, book := range author.Books {
        fmt.Printf("  - %s\n", book.Title)
    }
}
```

### 2. belongsTo (Many-to-One)

```go
type Book struct {
    ID       int64   `db:"id,primary,autoIncrement"`
    Title    string  `db:"title"`
    AuthorID int64   `db:"author_id"`
    Author   *Author `po:"-,belongsTo,foreignKey(author_id)"`
}

// Eager load author
books, _ := builder.Select[Book](qb).
    Preload("Author").  // ✅ Loads all authors efficiently
    All(ctx)

for _, book := range books {
    fmt.Printf("Book: %s by %s\n", book.Title, book.Author.Name)
}
```

### 3. hasOne (One-to-One)

```go
type User struct {
    ID      int64    `db:"id,primary,autoIncrement"`
    Name    string   `db:"name"`
    Profile *Profile `po:"-,hasOne,foreignKey(user_id)"`
}

type Profile struct {
    ID     int64  `db:"id,primary,autoIncrement"`
    Bio    string `db:"bio"`
    UserID int64  `db:"user_id"`
}

// Eager load profile
users, _ := builder.Select[User](qb).
    Preload("Profile").  // ✅ Loads profile
    All(ctx)
```

### 4. manyToMany (Many-to-Many)

```go
type User struct {
    Roles []Role `po:"-,manyToMany,joinTable(user_roles)"`
}

type Role struct {
    ID   int64  `db:"id,primary,autoIncrement"`
    Name string `db:"name"`
}

// Eager load roles (through user_roles junction table)
users, _ := builder.Select[User](qb).
    Preload("Roles").  // ✅ Joins through user_roles
    All(ctx)
```

## Running the Example

### Prerequisites

- PostgreSQL running on `localhost:5432`
- Database: `pebble_relationships`

```bash
# Create database
createdb pebble_relationships

# Run the example
cd examples/relationships
go run cmd/relationships/main.go
```

## Project Structure

```
relationships/
├── cmd/
│   └── relationships/
│       └── main.go           # Main application
├── internal/
│   ├── database/
│   │   └── db.go             # Database connection
│   └── models/
│       ├── models.go         # Models with relationships
│       └── registry.go       # Model registration
├── go.mod
└── README.md
```

## Models

### Author & Book (hasMany / belongsTo)

```go
type Author struct {
    ID        int64     `db:"id,primary,autoIncrement"`
    Name      string    `db:"name"`
    CreatedAt time.Time `db:"created_at"`
    Books     []Book    `po:"-,hasMany,foreignKey(author_id)"`
}

type Book struct {
    ID        int64     `db:"id,primary,autoIncrement"`
    Title     string    `db:"title"`
    ISBN      string    `db:"isbn"`
    AuthorID  int64     `db:"author_id"`
    CreatedAt time.Time `db:"created_at"`
    Author    *Author   `po:"-,belongsTo,foreignKey(author_id)"`
}
```

### User & Profile (hasOne)

```go
type User struct {
    ID        int64     `db:"id,primary,autoIncrement"`
    Name      string    `db:"name"`
    Email     string    `db:"email,unique"`
    CreatedAt time.Time `db:"created_at"`
    Profile   *Profile  `po:"-,hasOne,foreignKey(user_id)"`
}

type Profile struct {
    ID        int64     `db:"id,primary,autoIncrement"`
    Bio       string    `db:"bio"`
    Avatar    string    `db:"avatar"`
    UserID    int64     `db:"user_id"`
    CreatedAt time.Time `db:"created_at"`
}
```

### User & Role (manyToMany)

```go
type User struct {
    Roles []Role `po:"-,manyToMany,joinTable(user_roles)"`
}

type Role struct {
    ID   int64  `db:"id,primary,autoIncrement"`
    Name string `db:"name"`
}

// Junction table (created separately)
// CREATE TABLE user_roles (
//     user_id BIGINT REFERENCES users(id),
//     role_id BIGINT REFERENCES roles(id),
//     PRIMARY KEY (user_id, role_id)
// );
```

## Example Output

```
=== Relationships & Eager Loading Example ===

✅ Connected to database

--- Example 1: hasMany (Author → Books) ---
Created author: J.K. Rowling (ID: 1)
  Created post: Harry Potter and the Philosopher's Stone (ID: 1)
  Created post: Harry Potter and the Chamber of Secrets (ID: 2)
  Created post: Harry Potter and the Prisoner of Azkaban (ID: 3)
Created 3 books

Querying authors with their books (eager loading)...

 Author: J.K. Rowling
  Books (3):
    - Harry Potter and the Philosopher's Stone (ISBN: 978-0439554930)
    - Harry Potter and the Chamber of Secrets (ISBN: 978-0439554923)
    - Harry Potter and the Prisoner of Azkaban (ISBN: 978-0439554916)

--- Example 2: hasOne (User → Profile) ---
Created user: Alice Smith (ID: 1)
Created profile

Querying users with their profiles (eager loading)...

User: Alice Smith (alice@example.com)
  Profile:
    Bio: Software engineer and book enthusiast
    Avatar: https://example.com/avatar.jpg

--- Example 3: belongsTo (Book → Author) ---
Querying books with their authors (eager loading)...

Book: Harry Potter and the Chamber of Secrets
  Author: J.K. Rowling

Book: Harry Potter and the Philosopher's Stone
  Author: J.K. Rowling

Book: Harry Potter and the Prisoner of Azkaban
  Author: J.K. Rowling

--- Example 4: manyToMany (User ↔ Roles) ---
Ensured 3 roles exist

Querying users with their roles (eager loading)...

User: Alice Smith
  Roles (0):

--- Example 5: Multiple Preloads ---
Querying with multiple relationships...

✅ Found 1 authors with complete data

✅ All relationship examples completed!

Key Takeaways:
  - Use Preload() to eager load relationships and prevent N+1 queries
  - hasMany: One parent → Many children (Author → Books)
  - belongsTo: Child → One parent (Book → Author)
  - hasOne: One parent → One child (User → Profile)
  - manyToMany: Many ↔ Many through junction table (User ↔ Roles)
```

## The N+1 Query Problem

### ❌ Without Preload (N+1 Queries)

```go
// Fetches all authors
authors, _ := builder.Select[Author](qb).All(ctx)  // 1 query

// For each author, fetch books separately
for _, author := range authors {
    books, _ := builder.Select[Book](qb).
        Where(builder.Eq("author_id", author.ID)).
        All(ctx)  // N queries (one per author)
}
// Total: 1 + N queries
```

### ✅ With Preload (2 Queries)

```go
// Fetches all authors AND their books efficiently
authors, _ := builder.Select[Author](qb).
    Preload("Books").  // ✅ One query for all books
    All(ctx)

for _, author := range authors {
    // Books already loaded!
    for _, book := range author.Books {
        fmt.Println(book.Title)
    }
}
// Total: 2 queries (authors + books)
```

## Relationship Types

### One-to-Many (hasMany)

**Use case:** One parent has multiple children

Examples:

- Author → Books
- User → Posts
- Category → Products
- Order → LineItems

```go
type Parent struct {
    Children []Child `po:"-,hasMany,foreignKey(parent_id)"`
}
```

### Many-to-One (belongsTo)

**Use case:** Many children belong to one parent

Examples:

- Book → Author
- Post → User
- Product → Category
- LineItem → Order

```go
type Child struct {
    ParentID int64   `db:"parent_id"`
    Parent   *Parent `po:"-,belongsTo,foreignKey(parent_id)"`
}
```

### One-to-One (hasOne)

**Use case:** One record has exactly one related record

Examples:

- User → Profile
- User → Settings
- Order → Invoice
- Country → Capital

```go
type Parent struct {
    Child *Child `po:"-,hasOne,foreignKey(parent_id)"`
}
```

### Many-to-Many (manyToMany)

**Use case:** Records can have multiple of each other

Examples:

- Users ↔ Roles
- Students ↔ Courses
- Tags ↔ Posts
- Products ↔ Categories

```go
type User struct {
    Roles []Role `po:"-,manyToMany,joinTable(user_roles)"`
}
```

## Multiple Preloads

Load multiple relationships in one query:

```go
authors, _ := builder.Select[Author](qb).
    Preload("Books").      // Load books
    Preload("Publisher").  // Load publisher
    All(ctx)
```

## Best Practices

### ✅ DO:

- Use `Preload()` to prevent N+1 queries
- Make relationship fields pointers for nullable relationships
- Use junction tables for many-to-many
- Index foreign key columns

### ❌ DON'T:

- Fetch relationships in loops (N+1 problem)
- Load unnecessary relationships
- Forget to index foreign keys
- Make circular preloads

## Performance Tips

### Index Foreign Keys

```sql
CREATE INDEX idx_books_author_id ON books(author_id);
CREATE INDEX idx_profiles_user_id ON profiles(user_id);
CREATE INDEX idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX idx_user_roles_role_id ON user_roles(role_id);
```

### Selective Loading

```go
// Only preload when needed
if needBooks {
    query = query.Preload("Books")
}
```

### Limit Relationships

```go
// Limit the number of related records
// (Future feature - not yet implemented)
```

## Common Patterns

### Nested Relationships

```go
// Load books and their authors' profiles (future)
books, _ := builder.Select[Book](qb).
    Preload("Author").
    Preload("Author.Profile").  // Nested preload
    All(ctx)
```

### Conditional Relationships

```go
// Load only active books (future)
authors, _ := builder.Select[Author](qb).
    PreloadWhere("Books", builder.Eq("active", true)).
    All(ctx)
```

## Troubleshooting

### Relationship Not Loading

1. Check field is exported (capitalized)
2. Verify `po` tag is correct
3. Ensure foreign key exists in database
4. Check relationship type matches data

### Duplicate Records

- Ensure junction table has PRIMARY KEY or UNIQUE constraint
- Check for data integrity issues

### Performance Issues

- Add indexes to foreign keys
- Use `Preload()` instead of loops
- Limit result sets with `Where()` and `Limit()`

## Learn More

- **Relationships Guide**: `../docs/RELATIONSHIPS.md`
- **Preload Implementation**: `pkg/builder/relationships.go`
- **Schema Package**: `pkg/schema/`

## Key Takeaways

1. **Prevent N+1** - Always use `Preload()` for relationships
2. **Type Safe** - Relationships are fully type-safe with generics
3. **Flexible** - Support all common relationship types
4. **Performant** - Efficient SQL with minimal queries
5. **Production-Ready** - Same patterns used in major ORMs

**This example shows how to model and query complex relationships!** 🎉
