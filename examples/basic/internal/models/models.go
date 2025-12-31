package models

import "time"

// PostStatus represents the publication status of a post
type PostStatus string

// table_name: users
type User struct {
	ID        string    `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	Name      string    `po:"name,varchar(255),notNull"`
	Email     string    `po:"email,varchar(320),unique,notNull"`
	Age       int       `po:"age,integer,notNull"`
	CreatedAt time.Time `po:"created_at,timestamptz,default(NOW()),notNull"`
	UpdatedAt time.Time `po:"updated_at,timestamptz,default(NOW()),notNull"`
}

// table_name: posts
type Post struct {
	ID        string     `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	Title     string     `po:"title,varchar(500),notNull"`
	Content   string     `po:"content,text,notNull"`
	AuthorID  string     `po:"author_id,uuid,notNull"`
	Status    PostStatus `po:"status,enum(draft,published,archived),default('draft'),notNull"`
	CreatedAt time.Time  `po:"created_at,timestamptz,default(NOW()),notNull"`
	UpdatedAt time.Time  `po:"updated_at,timestamptz,default(NOW()),notNull"`

	// Relationships
	Author *User `po:"-,belongsTo,foreignKey(author_id),references(id)"`
}
