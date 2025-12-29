package models

import "time"

// table_name: tenants
type Tenant struct {
	ID        string    `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	Name      string    `po:"name,varchar(255),notNull"`
	Subdomain string    `po:"subdomain,varchar(100),unique,notNull"`
	CreatedAt time.Time `po:"created_at,timestamptz,default(NOW()),notNull"`
	UpdatedAt time.Time `po:"updated_at,timestamptz,default(NOW()),notNull"`
}

// table_name: users
type User struct {
	ID        string    `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	TenantID  string    `po:"tenant_id,uuid,notNull,index"`
	Name      string    `po:"name,varchar(255),notNull"`
	Email     string    `po:"email,varchar(320),notNull"`
	Role      string    `po:"role,varchar(50),default('user'),notNull"`
	CreatedAt time.Time `po:"created_at,timestamptz,default(NOW()),notNull"`
	UpdatedAt time.Time `po:"updated_at,timestamptz,default(NOW()),notNull"`

	// Relationships
	Tenant *Tenant `po:"-,belongsTo,foreignKey(tenant_id),references(id)"`
}

// table_name: documents
type Document struct {
	ID        string    `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	TenantID  string    `po:"tenant_id,uuid,notNull,index"`
	Title     string    `po:"title,varchar(500),notNull"`
	Content   string    `po:"content,text,notNull"`
	OwnerID   string    `po:"owner_id,uuid,notNull"`
	IsPublic  bool      `po:"is_public,boolean,default(false),notNull"`
	CreatedAt time.Time `po:"created_at,timestamptz,default(NOW()),notNull"`
	UpdatedAt time.Time `po:"updated_at,timestamptz,default(NOW()),notNull"`

	// Relationships
	Tenant *Tenant `po:"-,belongsTo,foreignKey(tenant_id),references(id)"`
	Owner  *User   `po:"-,belongsTo,foreignKey(owner_id),references(id)"`
}
