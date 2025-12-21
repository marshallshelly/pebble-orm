package database

import (
	"context"
	"fmt"
	"os"

	"github.com/marshallshelly/pebble-orm/examples/transactions/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/runtime"
)

// Connect establishes a database connection and registers all models
func Connect(ctx context.Context) (*runtime.DB, error) {
	// Get connection string from environment or use default
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://localhost:5432/pebble_transactions?sslmode=disable"
	}

	// Register all models
	if err := models.RegisterAll(); err != nil {
		return nil, fmt.Errorf("failed to register models: %w", err)
	}

	// Connect to database
	db, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}
