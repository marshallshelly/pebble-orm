package models

import (
	"time"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// GDPRMetadata provides standard GDPR compliance fields for all models
// Implements audit trails, soft delete, and data retention
type GDPRMetadata struct {
	// Audit trail - who and when created/updated (GDPR Article 5 - Accountability)
	CreatedAt time.Time `po:"created_at,timestamptz,default(NOW()),notNull"`
	CreatedBy *string   `po:"created_by,uuid"`
	UpdatedAt time.Time `po:"updated_at,timestamptz,default(NOW()),notNull"`
	UpdatedBy *string   `po:"updated_by,uuid"`

	// Soft delete for audit trail (GDPR Article 17 - Right to Erasure)
	DeletedAt *time.Time `po:"deleted_at,timestamptz"`
	DeletedBy *string    `po:"deleted_by,uuid"`

	// Data retention (GDPR Article 5 - Storage limitation)
	RetainUntil *time.Time `po:"retain_until,timestamptz"`

	// Anonymization flag (GDPR Recital 26)
	Anonymized bool `po:"anonymized,boolean,default(false),notNull"`
}

// table_name: tenants
type Tenant struct {
	ID        string `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	Name      string `po:"name,varchar(255),notNull"`
	Subdomain string `po:"subdomain,varchar(100),unique,notNull"`

	// Data residency (GDPR Article 44-50 - International Transfers)
	DataRegion string `po:"data_region,varchar(50),default('EU'),notNull"` // EU, US, UK, etc.

	// GDPR configuration
	DataRetentionDays int     `po:"data_retention_days,integer,default(365),notNull"`
	DPOEmail          *string `po:"dpo_email,varchar(320)"` // Data Protection Officer

	// GDPR metadata
	GDPRMetadata
}

// table_name: users
type User struct {
	ID       string `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	TenantID string `po:"tenant_id,uuid,notNull,index"`

	// Personal Data - PII (GDPR Article 4 - Personal Data)
	Name  string  `po:"name,varchar(255),notNull"`
	Email string  `po:"email,varchar(320),notNull"`
	Phone *string `po:"phone,varchar(50)"`

	// Non-personal data
	Role string `po:"role,varchar(50),default('user'),notNull"`

	// Consent tracking (GDPR Article 7 - Conditions for consent)
	MarketingConsent   bool       `po:"marketing_consent,boolean,default(false),notNull"`
	MarketingConsentAt *time.Time `po:"marketing_consent_at,timestamptz"`
	AnalyticsConsent   bool       `po:"analytics_consent,boolean,default(false),notNull"`
	AnalyticsConsentAt *time.Time `po:"analytics_consent_at,timestamptz"`

	// Legal basis for processing (GDPR Article 6)
	ProcessingBasis string `po:"processing_basis,varchar(50),default('consent'),notNull"` // consent, contract, legitimate_interest

	// GDPR metadata
	GDPRMetadata

	// Relationships
	Tenant *Tenant `po:"-,belongsTo,foreignKey(tenant_id),references(id)"`
}

// table_name: documents
type Document struct {
	ID       string `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	TenantID string `po:"tenant_id,uuid,notNull,index"`
	Title    string `po:"title,varchar(500),notNull"`
	Content  string `po:"content,text,notNull"`
	OwnerID  string `po:"owner_id,uuid,notNull"`
	IsPublic bool   `po:"is_public,boolean,default(false),notNull"`

	// GDPR metadata
	GDPRMetadata

	// Relationships
	Tenant *Tenant `po:"-,belongsTo,foreignKey(tenant_id),references(id)"`
	Owner  *User   `po:"-,belongsTo,foreignKey(owner_id),references(id)"`
}

// table_name: audit_logs
type AuditLog struct {
	ID       string `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	TenantID string `po:"tenant_id,uuid,notNull,index"`

	// Who, What, When, Where
	UserID     string `po:"user_id,uuid,notNull,index"`
	Action     string `po:"action,varchar(50),notNull"`    // CREATE, READ, UPDATE, DELETE, EXPORT, ANONYMIZE
	Resource   string `po:"resource,varchar(100),notNull"` // users, documents, etc.
	ResourceID string `po:"resource_id,uuid,notNull,index"`

	// Details
	Changes   schema.JSONB `po:"changes,jsonb"` // Before/after values
	IPAddress string       `po:"ip_address,varchar(45),notNull"`
	UserAgent string       `po:"user_agent,text"`

	CreatedAt time.Time `po:"created_at,timestamptz,default(NOW()),notNull"`

	// Relationships
	Tenant *Tenant `po:"-,belongsTo,foreignKey(tenant_id),references(id)"`
	User   *User   `po:"-,belongsTo,foreignKey(user_id),references(id)"`
}

// table_name: data_export_requests
type DataExportRequest struct {
	ID       string `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	TenantID string `po:"tenant_id,uuid,notNull,index"`
	UserID   string `po:"user_id,uuid,notNull,index"`

	Status string `po:"status,varchar(50),default('pending'),notNull"` // pending, processing, completed, failed
	Format string `po:"format,varchar(20),default('json'),notNull"`    // json, csv

	// File location after processing
	FileURL   *string    `po:"file_url,text"`
	ExpiresAt *time.Time `po:"expires_at,timestamptz"` // Auto-delete export after 30 days

	CompletedAt *time.Time `po:"completed_at,timestamptz"`

	GDPRMetadata

	Tenant *Tenant `po:"-,belongsTo,foreignKey(tenant_id),references(id)"`
	User   *User   `po:"-,belongsTo,foreignKey(user_id),references(id)"`
}

// table_name: deletion_requests
type DeletionRequest struct {
	ID       string `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	TenantID string `po:"tenant_id,uuid,notNull,index"`
	UserID   string `po:"user_id,uuid,notNull,index"`

	Status      string `po:"status,varchar(50),default('pending'),notNull"`    // pending, approved, processing, completed
	RequestType string `po:"request_type,varchar(50),default('user'),notNull"` // user, tenant

	// Legal compliance
	ApprovedBy *string    `po:"approved_by,uuid"`
	ApprovedAt *time.Time `po:"approved_at,timestamptz"`

	CompletedAt *time.Time `po:"completed_at,timestamptz"`

	GDPRMetadata

	Tenant *Tenant `po:"-,belongsTo,foreignKey(tenant_id),references(id)"`
	User   *User   `po:"-,belongsTo,foreignKey(user_id),references(id)"`
}
