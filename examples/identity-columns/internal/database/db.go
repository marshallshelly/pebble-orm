package database

import (
	"context"
	"fmt"
	"os"

	"github.com/marshallshelly/pebble-orm/examples/identity-columns/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/builder"
	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/runtime"
)

// Initialize sets up the database connection and returns query builder and cleanup function
func Initialize(ctx context.Context) (*builder.DB, func(), error) {
	db, err := Connect(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect: %w", err)
	}

	qb := builder.New(db)

	cleanup := func() {
		db.Close()
	}

	return qb, cleanup, nil
}

// Connect establishes a connection to the PostgreSQL database
func Connect(ctx context.Context) (*runtime.DB, error) {
	// Get connection string from environment or use default
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/pebble_test?sslmode=disable"
	}

	// Connect to database
	db, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	// Register all models
	if err := registry.Register(models.Product{}); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to register models: %w", err)
	}
	if err := registry.Register(models.Order{}); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to register models: %w", err)
	}
	if err := registry.Register(models.Customer{}); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to register models: %w", err)
	}

	return db, nil
}
