package builder

import (
	"reflect"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
)

// Col returns the database column name for a given Go field name.
// This provides a single source of truth through the registry.
//
// Usage:
//
//	type User struct {
//	    Age   int    `po:"age,integer"`
//	    Email string `po:"email,varchar(255),unique"`
//	}
//
//	// Instead of hardcoded: Where(builder.Eq("email", value))
//	// Use: Where(builder.Eq(builder.Col[User]("Email"), value))
//
// This extracts the column name from the registered metadata,
// so you only define it once in the struct tags.
func Col[T any](goFieldName string) string {
	var zero T
	modelType := reflect.TypeOf(zero)

	// Get table metadata from registry
	table, err := registry.Get(modelType)
	if err != nil {
		// Return the field name as-is if not registered (will likely cause SQL error)
		return goFieldName
	}

	// Get column by Go field name
	column := table.GetColumnByField(goFieldName)
	if column == nil {
		// Field not found - return as-is
		return goFieldName
	}

	return column.Name
}
