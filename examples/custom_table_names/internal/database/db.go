package database

import (
	"context"
	"fmt"
	"os"
	"reflect"

	"github.com/marshallshelly/pebble-orm/examples/custom_table_names/internal/models"
	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/runtime"
)

// Connect establishes a database connection and registers all models
func Connect(ctx context.Context) (*runtime.DB, error) {
	// Get connection string from environment or use default
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://localhost:5432/pebble_custom_tables?sslmode=disable"
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

// GetTableNames returns the actual table names used in the database
func GetTableNames() map[string]string {
	return map[string]string{
		"User":    getTableName(models.User{}),
		"Product": getTableName(models.Product{}),
		"Order":   getTableName(models.Order{}),
	}
}

func getTableName(model any) string {
	table, err := registry.Get(reflect.TypeOf(model))
	if err != nil {
		return "unknown"
	}
	return table.Name
}
