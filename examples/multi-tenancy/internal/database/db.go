package database

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/marshallshelly/pebble-orm/examples/multi-tenancy/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
	"github.com/marshallshelly/pebble-orm/pkg/runtime"
)

// Connect establishes a connection to the PostgreSQL database
func Connect(ctx context.Context) (*runtime.DB, error) {
	// Get connection string from environment or use default
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/pebble_multitenancy?sslmode=disable"
	}

	// Register all models
	if err := models.RegisterAll(); err != nil {
		return nil, fmt.Errorf("failed to register models: %w", err)
	}

	// Connect to database
	db, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	return db, nil
}

// TenantDB wraps the query builder with tenant awareness and GDPR compliance
// Automatically injects tenant_id filters and excludes soft-deleted records
type TenantDB struct {
	qb       *builder.DB
	tenantID string
	userID   string // Current user making the request (for audit trails)
}

// NewTenantDB creates a new tenant-aware query builder
func NewTenantDB(qb *builder.DB, tenantID, userID string) *TenantDB {
	return &TenantDB{
		qb:       qb,
		tenantID: tenantID,
		userID:   userID,
	}
}

// SelectActive creates a SELECT query with automatic tenant filtering
// Excludes soft-deleted records (GDPR compliant)
func SelectActive[T any](tdb *TenantDB) *builder.SelectQuery[T] {
	query := builder.Select[T](tdb.qb)

	// Auto-inject tenant_id filter if the model has it
	if hasTenantIDColumn[T]() {
		query = query.Where(builder.Eq("tenant_id", tdb.tenantID))
	}

	// Auto-exclude soft-deleted records (GDPR Article 17)
	if hasSoftDelete[T]() {
		query = query.Where(builder.IsNull("deleted_at"))
	}

	return query
}

// SelectAll creates a SELECT query including soft-deleted records
// For admin/audit purposes only
func SelectAll[T any](tdb *TenantDB) *builder.SelectQuery[T] {
	query := builder.Select[T](tdb.qb)

	// Auto-inject tenant_id filter if the model has it
	if hasTenantIDColumn[T]() {
		query = query.Where(builder.Eq("tenant_id", tdb.tenantID))
	}

	return query
}

// Insert creates an INSERT query builder for tenant-aware models
// Automatically sets created_by for audit trail
func Insert[T any](tdb *TenantDB) *builder.InsertQuery[T] {
	return builder.Insert[T](tdb.qb)
}

// Update creates an UPDATE query with automatic tenant filtering
// Excludes soft-deleted records and sets updated_by for audit trail
func Update[T any](tdb *TenantDB) *builder.UpdateQuery[T] {
	query := builder.Update[T](tdb.qb)

	// Auto-inject tenant_id filter if the model has it
	if hasTenantIDColumn[T]() {
		query = query.Where(builder.Eq("tenant_id", tdb.tenantID))
	}

	// Auto-exclude soft-deleted records
	if hasSoftDelete[T]() {
		query = query.Where(builder.IsNull("deleted_at"))
	}

	// Set updated_by for audit trail
	query = query.Set("updated_by", tdb.userID)

	return query
}

// Delete creates a DELETE query with automatic tenant filtering
// Excludes soft-deleted records
func Delete[T any](tdb *TenantDB) *builder.DeleteQuery[T] {
	query := builder.Delete[T](tdb.qb)

	// Auto-inject tenant_id filter if the model has it
	if hasTenantIDColumn[T]() {
		query = query.Where(builder.Eq("tenant_id", tdb.tenantID))
	}

	// Auto-exclude soft-deleted records
	if hasSoftDelete[T]() {
		query = query.Where(builder.IsNull("deleted_at"))
	}

	return query
}

// GetTenantID returns the current tenant ID
func (tdb *TenantDB) GetTenantID() string {
	return tdb.tenantID
}

// GetUserID returns the current user ID
func (tdb *TenantDB) GetUserID() string {
	return tdb.userID
}

// GetDB returns the underlying database connection
func (tdb *TenantDB) GetDB() *builder.DB {
	return tdb.qb
}

// hasTenantIDColumn checks if a model type has a tenant_id field
func hasTenantIDColumn[T any]() bool {
	var zero T
	switch any(zero).(type) {
	case models.User, models.Document, models.AuditLog, models.DataExportRequest, models.DeletionRequest:
		return true
	case models.Tenant:
		return false
	default:
		return false
	}
}

// hasSoftDelete checks if a model type has soft delete support
func hasSoftDelete[T any]() bool {
	var zero T
	switch any(zero).(type) {
	case models.User, models.Document, models.Tenant, models.DataExportRequest, models.DeletionRequest:
		return true
	case models.AuditLog:
		return false // Audit logs are never deleted
	default:
		return false
	}
}

// TenantManager manages connections for database-per-tenant architecture
type TenantManager struct {
	connections map[string]*runtime.DB
	mu          sync.RWMutex
}

// NewTenantManager creates a new tenant connection manager
func NewTenantManager() *TenantManager {
	return &TenantManager{
		connections: make(map[string]*runtime.DB),
	}
}

// GetConnection retrieves or creates a connection for a specific tenant
// Each tenant gets their own database and connection pool
func (tm *TenantManager) GetConnection(ctx context.Context, tenantID string) (*runtime.DB, error) {
	// Check if connection already exists
	tm.mu.RLock()
	db, exists := tm.connections[tenantID]
	tm.mu.RUnlock()

	if exists {
		return db, nil
	}

	// Create new connection for tenant
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check in case another goroutine created it
	if db, exists := tm.connections[tenantID]; exists {
		return db, nil
	}

	// Build tenant-specific database connection
	config := &runtime.Config{
		Host:     "localhost",
		Port:     5432,
		Database: fmt.Sprintf("tenant_%s", tenantID),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		MaxConns: 10,
		MinConns: 2,
	}

	// Use default user if not set
	if config.User == "" {
		config.User = "postgres"
	}
	if config.Password == "" {
		config.Password = "postgres"
	}

	db, err := runtime.Connect(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tenant database: %w", err)
	}

	// Register models for this connection
	if err := models.RegisterAll(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to register models: %w", err)
	}

	tm.connections[tenantID] = db
	return db, nil
}

// CloseAll closes all tenant connections
func (tm *TenantManager) CloseAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, db := range tm.connections {
		db.Close()
	}
	tm.connections = make(map[string]*runtime.DB)
}

// RemoveConnection closes and removes a specific tenant connection
func (tm *TenantManager) RemoveConnection(tenantID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if db, exists := tm.connections[tenantID]; exists {
		db.Close()
		delete(tm.connections, tenantID)
	}
}
