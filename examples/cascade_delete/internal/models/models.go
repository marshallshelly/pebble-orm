package models

import "time"

// User represents a user in the system.
type User struct {
	ID        int64     `db:"id,primary,autoIncrement"`
	Name      string    `db:"name"`
	Email     string    `db:"email,unique"`
	CreatedAt time.Time `db:"created_at"`
}

// Post represents a blog post with CASCADE DELETE on user deletion.
type Post struct {
	ID        int64     `db:"id,primary,autoIncrement"`
	Title     string    `db:"title"`
	Content   string    `db:"content"`
	AuthorID  int64     `db:"author_id,fk:users.id,ondelete:cascade"`
	CreatedAt time.Time `db:"created_at"`
}

// Comment represents a comment with CASCADE DELETE on post deletion
// and SET NULL on author deletion.
type Comment struct {
	ID        int64     `db:"id,primary,autoIncrement"`
	Content   string    `db:"content"`
	PostID    int64     `db:"post_id,fk:posts.id,ondelete:cascade"`
	AuthorID  *int64    `db:"author_id,fk:users.id,ondelete:setnull"`
	CreatedAt time.Time `db:"created_at"`
}

// Category represents a product category with RESTRICT to prevent deletion
// if products exist.
type Category struct {
	ID   int64  `db:"id,primary,autoIncrement"`
	Name string `db:"name"`
}

// Product represents a product with RESTRICT on category deletion.
type Product struct {
	ID         int64   `db:"id,primary,autoIncrement"`
	Name       string  `db:"name"`
	Price      float64 `db:"price"`
	CategoryID int64   `db:"category_id,fk:categories.id,ondelete:restrict"`
}
