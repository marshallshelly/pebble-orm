package models

import "time"

// table_name: users
type User struct {
	ID        int64     `po:"id,primaryKey,bigserial"`
	Name      string    `po:"name,varchar(255),notNull"`
	Email     string    `po:"email,varchar(320),unique,notNull"`
	CreatedAt time.Time `po:"created_at,timestamptz,notNull,default(NOW())"`
}

// Post cascades when its author is deleted.
//
// table_name: posts
type Post struct {
	ID        int64     `po:"id,primaryKey,bigserial"`
	Title     string    `po:"title,varchar(255),notNull"`
	Content   string    `po:"content,text,notNull"`
	AuthorID  int64     `po:"author_id,bigint,notNull,fk:users(id),onDelete:CASCADE"`
	CreatedAt time.Time `po:"created_at,timestamptz,notNull,default(NOW())"`
}

// Comment cascades when its post is deleted, and its author is set NULL when the
// user is deleted.
//
// table_name: comments
type Comment struct {
	ID        int64     `po:"id,primaryKey,bigserial"`
	Content   string    `po:"content,text,notNull"`
	PostID    int64     `po:"post_id,bigint,notNull,fk:posts(id),onDelete:CASCADE"`
	AuthorID  *int64    `po:"author_id,bigint,fk:users(id),onDelete:SETNULL"`
	CreatedAt time.Time `po:"created_at,timestamptz,notNull,default(NOW())"`
}

// table_name: categories
type Category struct {
	ID   int64  `po:"id,primaryKey,bigserial"`
	Name string `po:"name,varchar(255),notNull"`
}

// Product restricts deletion of a category that still has products.
//
// table_name: products
type Product struct {
	ID         int64   `po:"id,primaryKey,bigserial"`
	Name       string  `po:"name,varchar(255),notNull"`
	Price      float64 `po:"price,numeric(10,2),notNull"`
	CategoryID int64   `po:"category_id,bigint,notNull,fk:categories(id),onDelete:RESTRICT"`
}
