package models

import (
	"time"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// Example 1: Simple Column-Level Indexes
// table_name: products
type Product struct {
	ID          int64     `po:"id,primaryKey,bigint,identity"`
	Name        string    `po:"name,varchar(255),notNull,index"`                       // Auto-named: idx_products_name
	SKU         string    `po:"sku,varchar(100),unique,notNull,index(idx_product_sku)"` // Custom name
	Price       float64   `po:"price,numeric(10,2),notNull,index"`                     // Auto-named: idx_products_price
	Category    string    `po:"category,varchar(100),notNull,index"`                   // For filtering
	InStock     bool      `po:"in_stock,boolean,default(true),notNull"`
	Tags        []string  `po:"tags,text[],index(idx_product_tags,gin)"` // GIN index for array searches
	Description string    `po:"description,text"`
	CreatedAt   time.Time `po:"created_at,timestamptz,default(NOW()),notNull,index(idx_products_created,btree,desc)"` // DESC for recent-first
	UpdatedAt   time.Time `po:"updated_at,timestamptz,default(NOW()),notNull"`
}

// Example 2: Expression Indexes and Partial Indexes
// table_name: users
// index: idx_email_lower ON (lower(email))
// index: idx_active_users ON (email) WHERE deleted_at IS NULL
// index: idx_premium_users ON (user_id) WHERE subscription_tier = 'premium'
type User struct {
	ID               int64      `po:"id,primaryKey,bigint,identity"`
	Email            string     `po:"email,varchar(320),unique,notNull"`
	FirstName        string     `po:"first_name,varchar(100),notNull"`
	LastName         string     `po:"last_name,varchar(100),notNull"`
	SubscriptionTier string     `po:"subscription_tier,varchar(50),default('free'),notNull"`
	DeletedAt        *time.Time `po:"deleted_at,timestamptz"`
	CreatedAt        time.Time  `po:"created_at,timestamptz,default(NOW()),notNull"`
	UpdatedAt        time.Time  `po:"updated_at,timestamptz,default(NOW()),notNull"`
}

// Example 3: Covering Indexes (INCLUDE columns)
// table_name: orders
// index: idx_orders_customer_status ON (customer_id, status) INCLUDE (total_amount, created_at)
// index: idx_orders_created_covering ON (created_at DESC) INCLUDE (customer_id, total_amount, status)
type Order struct {
	ID           int64     `po:"id,primaryKey,bigint,identity"`
	CustomerID   int64     `po:"customer_id,bigint,notNull,index"`
	Status       string    `po:"status,varchar(50),default('pending'),notNull"`
	TotalAmount  float64   `po:"total_amount,numeric(12,2),notNull"`
	ShippingAddr string    `po:"shipping_addr,text,notNull"`
	CreatedAt    time.Time `po:"created_at,timestamptz,default(NOW()),notNull"`
	UpdatedAt    time.Time `po:"updated_at,timestamptz,default(NOW()),notNull"`
}

// Example 4: Multicolumn Indexes with Mixed Ordering
// table_name: events
// index: idx_events_tenant_created ON (tenant_id, created_at DESC NULLS LAST)
// index: idx_events_user_type_created ON (user_id, event_type, created_at DESC)
type Event struct {
	ID        int64          `po:"id,primaryKey,bigint,identity"`
	TenantID  int64          `po:"tenant_id,bigint,notNull"`
	UserID    int64          `po:"user_id,bigint,notNull"`
	EventType string         `po:"event_type,varchar(100),notNull"`
	Data      schema.JSONB   `po:"data,jsonb,index(idx_events_data,gin)"` // GIN for JSONB queries
	IPAddress string         `po:"ip_address,varchar(45),notNull"`
	CreatedAt time.Time      `po:"created_at,timestamptz,default(NOW()),notNull"`
}

// Example 5: Operator Classes for Pattern Matching
// table_name: search_terms
// index: idx_search_term_pattern ON (term varchar_pattern_ops)
// index: idx_search_description_pattern ON (description text_pattern_ops)
type SearchTerm struct {
	ID          int64     `po:"id,primaryKey,bigint,identity"`
	Term        string    `po:"term,varchar(255),notNull"`
	Description string    `po:"description,text"`
	SearchCount int64     `po:"search_count,bigint,default(0),notNull,index(idx_search_count,btree,desc)"` // Most searched first
	LastUsed    time.Time `po:"last_used,timestamptz,default(NOW()),notNull"`
}

// Example 6: Collations for Locale-Specific Sorting
// table_name: international_products
// index: idx_intl_name_en ON (name COLLATE "en_US")
// index: idx_intl_name_case_sensitive ON (name COLLATE "C")
type InternationalProduct struct {
	ID          int64   `po:"id,primaryKey,bigint,identity"`
	Name        string  `po:"name,varchar(255),notNull"`
	NameLocal   string  `po:"name_local,varchar(255),notNull"`
	Price       float64 `po:"price,numeric(10,2),notNull"`
	Locale      string  `po:"locale,varchar(10),notNull,index"` // en_US, de_DE, fr_FR, etc.
	CountryCode string  `po:"country_code,varchar(2),notNull,index"`
}

// Example 7: CONCURRENTLY for Production (Large Tables)
// table_name: analytics_events
// index: idx_analytics_timestamp ON (event_timestamp DESC) CONCURRENTLY
// index: idx_analytics_user_session ON (user_id, session_id) CONCURRENTLY
// index: idx_analytics_event_type ON (event_type) CONCURRENTLY WHERE processed = false
type AnalyticsEvent struct {
	ID             int64          `po:"id,primaryKey,bigint,identity"`
	UserID         *int64         `po:"user_id,bigint,index"`
	SessionID      string         `po:"session_id,uuid,notNull"`
	EventType      string         `po:"event_type,varchar(100),notNull"`
	EventTimestamp time.Time      `po:"event_timestamp,timestamptz,default(NOW()),notNull"`
	Processed      bool           `po:"processed,boolean,default(false),notNull"`
	Properties     schema.JSONB   `po:"properties,jsonb,index(idx_analytics_props,gin)"` // JSONB search
	CreatedAt      time.Time      `po:"created_at,timestamptz,default(NOW()),notNull"`
}

// Example 8: Complex Multi-Feature Index
// table_name: documents
// index: idx_documents_complex ON (owner_id, status, updated_at DESC NULLS LAST) INCLUDE (title, version) WHERE deleted_at IS NULL
// index: idx_documents_search ON (to_tsvector('english', title || ' ' || content))
// index: idx_documents_tags ON (tags) USING gin
type Document struct {
	ID        int64      `po:"id,primaryKey,bigint,identity"`
	OwnerID   int64      `po:"owner_id,bigint,notNull"`
	Title     string     `po:"title,varchar(500),notNull"`
	Content   string     `po:"content,text,notNull"`
	Status    string     `po:"status,varchar(50),default('draft'),notNull"`
	Version   int        `po:"version,integer,default(1),notNull"`
	Tags      []string   `po:"tags,text[]"`
	DeletedAt *time.Time `po:"deleted_at,timestamptz"`
	CreatedAt time.Time  `po:"created_at,timestamptz,default(NOW()),notNull"`
	UpdatedAt time.Time  `po:"updated_at,timestamptz,default(NOW()),notNull"`
}

// Example 9: BRIN Index for Time-Series Data (Very Large Tables)
// table_name: sensor_readings
// index: idx_sensor_timestamp ON (recorded_at) USING brin
// index: idx_sensor_device_timestamp ON (device_id, recorded_at) USING brin
type SensorReading struct {
	ID         int64     `po:"id,primaryKey,bigint,identity"`
	DeviceID   string    `po:"device_id,varchar(100),notNull"`
	SensorType string    `po:"sensor_type,varchar(50),notNull"`
	Value      float64   `po:"value,numeric(10,4),notNull"`
	Unit       string    `po:"unit,varchar(20),notNull"`
	RecordedAt time.Time `po:"recorded_at,timestamptz,default(NOW()),notNull"`
}

// Example 10: Hash Index (Equality-Only)
// table_name: api_keys
// index: idx_api_key_hash ON (key_hash) USING hash
type APIKey struct {
	ID        int64      `po:"id,primaryKey,bigint,identity"`
	KeyHash   string     `po:"key_hash,varchar(64),unique,notNull"` // SHA-256 hash
	UserID    int64      `po:"user_id,bigint,notNull,index"`
	Name      string     `po:"name,varchar(255),notNull"`
	ExpiresAt *time.Time `po:"expires_at,timestamptz"`
	CreatedAt time.Time  `po:"created_at,timestamptz,default(NOW()),notNull"`
	LastUsed  *time.Time `po:"last_used,timestamptz"`
}

// Example 11: Advanced NULLS Ordering
// table_name: tasks
// index: idx_tasks_priority ON (priority DESC NULLS LAST, due_date ASC NULLS FIRST)
// index: idx_tasks_assigned ON (assigned_to) WHERE assigned_to IS NOT NULL AND completed_at IS NULL
type Task struct {
	ID          int64      `po:"id,primaryKey,bigint,identity"`
	Title       string     `po:"title,varchar(500),notNull"`
	Priority    *int       `po:"priority,integer"` // NULL means no priority set
	DueDate     *time.Time `po:"due_date,timestamptz"`
	AssignedTo  *int64     `po:"assigned_to,bigint"`
	CompletedAt *time.Time `po:"completed_at,timestamptz"`
	CreatedAt   time.Time  `po:"created_at,timestamptz,default(NOW()),notNull"`
}

// Example 12: Composite with Operator Class and Collation
// table_name: articles
// index: idx_articles_advanced ON (author varchar_pattern_ops COLLATE "C" DESC, published_at DESC NULLS LAST) INCLUDE (title, slug) WHERE status = 'published' CONCURRENTLY
type Article struct {
	ID          int64      `po:"id,primaryKey,bigint,identity"`
	Slug        string     `po:"slug,varchar(255),unique,notNull"`
	Title       string     `po:"title,varchar(500),notNull"`
	Author      string     `po:"author,varchar(255),notNull"`
	Content     string     `po:"content,text,notNull"`
	Status      string     `po:"status,varchar(50),default('draft'),notNull"`
	PublishedAt *time.Time `po:"published_at,timestamptz"`
	CreatedAt   time.Time  `po:"created_at,timestamptz,default(NOW()),notNull"`
	UpdatedAt   time.Time  `po:"updated_at,timestamptz,default(NOW()),notNull"`
}
