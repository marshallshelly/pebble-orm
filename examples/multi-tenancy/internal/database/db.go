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

// TenantDB wraps the query builder with tenant awareness
// Automatically injects tenant_id filters for all queries
type TenantDB struct {
	qb       *builder.DB
	tenantID string
}

// NewTenantDB creates a new tenant-aware query builder
func NewTenantDB(qb *builder.DB, tenantID string) *TenantDB {
	return &TenantDB{
		qb:       qb,
		tenantID: tenantID,
	}
}

// Select creates a SELECT query with automatic tenant filtering
func Select[T any](tdb *TenantDB) *builder.SelectQuery[T] {
	query := builder.Select[T](tdb.qb)

	// Auto-inject tenant_id filter if the model has it
	// This prevents accidental data leaks across tenants
	if hasTenantIDColumn[T]() {
		query = query.Where(builder.Eq("tenant_id", tdb.tenantID))
	}

	return query
}

// Insert creates an INSERT query builder for tenant-aware models
func Insert[T any](tdb *TenantDB) *builder.InsertQuery[T] {
	// Note: You'll need to manually set tenant_id on the model before inserting
	return builder.Insert[T](tdb.qb)
}

// Update creates an UPDATE query with automatic tenant filtering
func Update[T any](tdb *TenantDB) *builder.UpdateQuery[T] {
	query := builder.Update[T](tdb.qb)

	// Auto-inject tenant_id filter if the model has it
	if hasTenantIDColumn[T]() {
		query = query.Where(builder.Eq("tenant_id", tdb.tenantID))
	}

	return query
}

// Delete creates a DELETE query with automatic tenant filtering
func Delete[T any](tdb *TenantDB) *builder.DeleteQuery[T] {
	query := builder.Delete[T](tdb.qb)

	// Auto-inject tenant_id filter if the model has it
	if hasTenantIDColumn[T]() {
		query = query.Where(builder.Eq("tenant_id", tdb.tenantID))
	}

	return query
}

// GetTenantID returns the current tenant ID
func (tdb *TenantDB) GetTenantID() string {
	return tdb.tenantID
}

// hasTenantIDColumn checks if a model type has a tenant_id field
func hasTenantIDColumn[T any]() bool {
	var zero T
	switch any(zero).(type) {
	case models.User, models.Document:
		return true
	case models.Tenant:
		return false
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
