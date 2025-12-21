package database

import (
	"context"

	"github.com/marshallshelly/pebble-orm/examples/cascade_delete/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/runtime"
)

// Connect establishes a connection to the database
func Connect(ctx context.Context) (*runtime.DB, error) {
	// Get connection URL from environment or use default
	connURL := "postgres://postgres:postgres@localhost:5432/pebble_cascade_demo?sslmode=disable"

	// Connect to database
	db, err := runtime.ConnectWithURL(ctx, connURL)
	if err != nil {
		return nil, err
	}

	// Register all models
	if err := models.RegisterAll(); err != nil {
		return nil, err
	}

	return db, nil
}
