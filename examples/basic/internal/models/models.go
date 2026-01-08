package models

import "time"

// PostStatus represents the publication status of a post
type PostStatus string

// UserPreferences stores user-specific settings as JSONB
type UserPreferences struct {
	Theme            string   `json:"theme"`
	EmailNotifications bool     `json:"emailNotifications"`
	Language         string   `json:"language"`
	FavoriteTopics   []string `json:"favoriteTopics,omitempty"`
}

// PostMetadata stores additional post information as JSONB
type PostMetadata struct {
	Tags           []string `json:"tags,omitempty"`
	ReadTimeMinutes int      `json:"readTimeMinutes,omitempty"`
	FeaturedImage  string   `json:"featuredImage,omitempty"`
	SEOKeywords    []string `json:"seoKeywords,omitempty"`
}

// table_name: users
type User struct {
	ID          string           `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	Name        string           `po:"name,varchar(255),notNull"`
	Email       string           `po:"email,varchar(320),unique,notNull,index"` // Auto-named index for fast lookups
	Age         int              `po:"age,integer,notNull,index"`                // Index for age-based queries
	Preferences *UserPreferences `po:"preferences,jsonb"`                        // JSONB field with direct struct scanning
	CreatedAt   time.Time        `po:"created_at,timestamptz,default(NOW()),notNull,index(idx_users_created,btree,desc)"` // DESC index for recent-first queries
	UpdatedAt   time.Time        `po:"updated_at,timestamptz,default(NOW()),notNull"`
}

// table_name: posts
// index: idx_posts_status_created ON (status, created_at DESC) WHERE status = 'published'
type Post struct {
	ID        string        `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	Title     string        `po:"title,varchar(500),notNull"`
	Content   string        `po:"content,text,notNull"`
	AuthorID  string        `po:"author_id,uuid,notNull,index"` // Index for author lookups
	Status    PostStatus    `po:"status,enum(draft,published,archived),default('draft'),notNull"`
	Metadata  *PostMetadata `po:"metadata,jsonb,index(idx_posts_metadata,gin)"` // GIN index for JSONB queries
	CreatedAt time.Time     `po:"created_at,timestamptz,default(NOW()),notNull"`
	UpdatedAt time.Time     `po:"updated_at,timestamptz,default(NOW()),notNull"`

	// Relationships
	Author *User `po:"-,belongsTo,foreignKey(author_id),references(id)"`
}
