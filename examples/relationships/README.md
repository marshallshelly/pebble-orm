# Relationships

<em>All four relationship shapes, eager-loaded in two queries instead of N+1.</em>

Five models covering `hasMany` (Author → Books), `belongsTo` (Book → Author), `hasOne` (User → Profile), and `manyToMany` (User ↔ Roles through `user_roles`). Every query uses `Preload`, which batches related rows with `ANY($1)` — one extra query per relationship, regardless of row count.

## Run

```bash
createdb pebble_relationships
export DATABASE_URL="postgres://localhost:5432/pebble_relationships?sslmode=disable"

cd examples/relationships
pebble generate --name initial_schema --models ./internal/models
pebble migrate up --all --db "$DATABASE_URL"

# manyToMany needs the junction table (relationship tags don't create it):
psql "$DATABASE_URL" -c "CREATE TABLE IF NOT EXISTS user_roles (
  user_id INTEGER REFERENCES users(id),
  role_id INTEGER REFERENCES roles(id),
  PRIMARY KEY (user_id, role_id))"

go run cmd/relationships/main.go
```

## What it shows

| Shape | Models | Tag on the parent side |
|-------|--------|------------------------|
| hasMany | Author → Books | `po:"-,hasMany,foreignKey(author_id),references(id)"` |
| belongsTo | Book → Author | `po:"-,belongsTo,foreignKey(author_id),references(id)"` |
| hasOne | User → Profile | `po:"-,hasOne,foreignKey(user_id),references(id)"` |
| manyToMany | User ↔ Roles | `po:"-,manyToMany,joinTable(user_roles),foreignKey(user_id),references(id)"` |

Relationship fields use `-` as the column name — they exist in Go, not in the table.

## The tags

```go
// table_name: authors
type Author struct {
    ID    int    `po:"id,primaryKey,serial"`
    Name  string `po:"name,varchar(100),notNull"`
    Books []Book `po:"-,hasMany,foreignKey(author_id),references(id)"`
}

// table_name: books
type Book struct {
    ID       int     `po:"id,primaryKey,serial"`
    Title    string  `po:"title,varchar(255),notNull"`
    ISBN     string  `po:"isbn,varchar(20),unique"`
    AuthorID int     `po:"author_id,integer,notNull"`
    Author   *Author `po:"-,belongsTo,foreignKey(author_id),references(id)"`
}
```

## Eager loading

Without `Preload`, fetching books per author in a loop costs 1 + N queries. With it:

```go
authors, err := builder.Select[models.Author](qb).
    Preload("Books").
    Where(builder.Eq(builder.Col[models.Author]("Name"), "J.K. Rowling")).
    All(ctx)

for _, a := range authors {
    fmt.Printf("%s wrote %d books\n", a.Name, len(a.Books))
}
```

Two queries total: authors, then `books WHERE author_id = ANY($1)`. Chain multiple `Preload` calls to load several relationships, or use dot notation (`Preload("Author.Profile")`) for nested loads.

`Preload` works on plain `Select` queries and on `TxSelect` inside transactions.

<details>
<summary><strong>Expected output (excerpt)</strong></summary>

```
--- Example 1: hasMany (Author → Books) ---
Created author: J.K. Rowling (ID: 1)
Created 3 books

 Author: J.K. Rowling
  Books (3):
    - Harry Potter and the Philosopher's Stone (ISBN: 978-0439554930)
    - Harry Potter and the Chamber of Secrets (ISBN: 978-0439554923)
    - Harry Potter and the Prisoner of Azkaban (ISBN: 978-0439554916)

--- Example 2: hasOne (User → Profile) ---
User: Alice Smith (alice@example.com)
  Profile:
    Bio: Software engineer and book enthusiast

--- Example 3: belongsTo (Book → Author) ---
Book: Harry Potter and the Chamber of Secrets
  Author: J.K. Rowling

--- Example 4: manyToMany (User ↔ Roles) ---
User: Alice Smith
  Roles (0):        # empty until rows exist in user_roles
```

The demo doesn't insert junction rows, so roles come back empty — add a row to `user_roles` and rerun to see the many-to-many load populate.

</details>
