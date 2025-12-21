package builder

import (
	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/runtime"
)

// DB wraps runtime.DB and provides query builder methods.
type DB struct {
	db *runtime.DB
}

// New creates a new query builder DB from a runtime DB.
func New(db *runtime.DB) *DB {
	return &DB{db: db}
}

// Runtime returns the underlying runtime.DB.
func (d *DB) Runtime() *runtime.DB {
	return d.db
}

// Select creates a new type-safe SELECT query.
// Usage: builder.Select[User](db).Where(...).All(ctx)
func Select[T any](d *DB) *SelectQuery[T] {
	// Create a zero value of T to get its type
	var model T

	// Get or register table metadata
	table, err := registry.GetOrRegister(model)
	if err != nil {
		// Return a query with error state
		return &SelectQuery[T]{
			db:    d,
			table: nil,
		}
	}

	return &SelectQuery[T]{
		db:       d,
		table:    table,
		columns:  []string{"*"}, // Default to all columns
		where:    make([]Condition, 0),
		joins:    make([]Join, 0),
		groupBy:  make([]string, 0),
		having:   make([]Condition, 0),
		orderBy:  make([]OrderBy, 0),
		preloads: make([]string, 0),
	}
}

// Insert creates a new type-safe INSERT query.
// Usage: builder.Insert[User](db).Values(user).Exec(ctx)
func Insert[T any](d *DB) *InsertQuery[T] {
	var model T

	table, err := registry.GetOrRegister(model)
	if err != nil {
		return &InsertQuery[T]{
			db:    d,
			table: nil,
		}
	}

	return &InsertQuery[T]{
		db:        d,
		table:     table,
		values:    make([]T, 0),
		returning: make([]string, 0),
	}
}

// Update creates a new type-safe UPDATE query.
// Usage: builder.Update[User](db).Set("name", "John").Where(...).Exec(ctx)
func Update[T any](d *DB) *UpdateQuery[T] {
	var model T

	table, err := registry.GetOrRegister(model)
	if err != nil {
		return &UpdateQuery[T]{
			db:    d,
			table: nil,
		}
	}

	return &UpdateQuery[T]{
		db:        d,
		table:     table,
		sets:      make(map[string]interface{}),
		where:     make([]Condition, 0),
		returning: make([]string, 0),
	}
}

// Delete creates a new type-safe DELETE query.
// Usage: builder.Delete[User](db).Where(...).Exec(ctx)
func Delete[T any](d *DB) *DeleteQuery[T] {
	var model T

	table, err := registry.GetOrRegister(model)
	if err != nil {
		return &DeleteQuery[T]{
			db:    d,
			table: nil,
		}
	}

	return &DeleteQuery[T]{
		db:        d,
		table:     table,
		where:     make([]Condition, 0),
		returning: make([]string, 0),
	}
}
