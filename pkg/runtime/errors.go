// Package runtime provides runtime utilities for the ORM.
package runtime

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound is returned when a record is not found.
	ErrNotFound = errors.New("record not found")

	// ErrInvalidModel is returned when an invalid model is provided.
	ErrInvalidModel = errors.New("invalid model")

	// ErrNoPrimaryKey is returned when a table has no primary key.
	ErrNoPrimaryKey = errors.New("no primary key defined")

	// ErrDuplicateKey is returned when a unique constraint is violated.
	ErrDuplicateKey = errors.New("duplicate key value")

	// ErrForeignKeyViolation is returned when a foreign key constraint is violated.
	ErrForeignKeyViolation = errors.New("foreign key violation")

	// ErrInvalidType is returned when a type conversion fails.
	ErrInvalidType = errors.New("invalid type")

	// ErrTransactionClosed is returned when operating on a closed transaction.
	ErrTransactionClosed = errors.New("transaction already closed")

	// ErrNoConnection is returned when no database connection is available.
	ErrNoConnection = errors.New("no database connection")
)

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}

// QueryError represents a query execution error.
type QueryError struct {
	Query string
	Err   error
}

// Error implements the error interface.
func (e *QueryError) Error() string {
	return fmt.Sprintf("query error: %v\nQuery: %s", e.Err, e.Query)
}

// Unwrap returns the underlying error.
func (e *QueryError) Unwrap() error {
	return e.Err
}

// MigrationError represents a migration error.
type MigrationError struct {
	Version string
	Message string
	Err     error
}

// Error implements the error interface.
func (e *MigrationError) Error() string {
	return fmt.Sprintf("migration error (version %s): %s: %v", e.Version, e.Message, e.Err)
}

// Unwrap returns the underlying error.
func (e *MigrationError) Unwrap() error {
	return e.Err
}
