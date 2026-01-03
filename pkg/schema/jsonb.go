package schema

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONB represents a PostgreSQL JSONB column as a generic map.
// It provides automatic marshaling/unmarshaling for JSON data.
//
// pebble-orm supports three ways to work with JSONB fields:
//
// 1. Direct struct scanning (Recommended - uses pgx native support):
//    type Metadata struct {
//        Premium bool     `json:"premium"`
//        Tags    []string `json:"tags"`
//    }
//    type User struct {
//        ID       int       `po:"id,primaryKey,serial"`
//        Metadata *Metadata `po:"metadata,jsonb"` // Use pointer for NULL handling
//    }
//
// 2. Generic map (flexible schema):
//    type User struct {
//        ID       int          `po:"id,primaryKey,serial"`
//        Metadata schema.JSONB `po:"metadata,jsonb"` // map[string]interface{}
//    }
//
// 3. Typed wrapper (for backward compatibility):
//    type User struct {
//        ID       int                              `po:"id,primaryKey,serial"`
//        Metadata schema.JSONBStruct[MyStructType] `po:"metadata,jsonb"`
//    }
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface for database writes
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for database reads
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	// Handle different value types that pgx might pass
	switch v := value.(type) {
	case []byte:
		// Raw JSON bytes from database
		var result map[string]interface{}
		if err := json.Unmarshal(v, &result); err != nil {
			return err
		}
		*j = result
		return nil
	case string:
		// JSON as string
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(v), &result); err != nil {
			return err
		}
		*j = result
		return nil
	case map[string]interface{}:
		// Already decoded by pgx
		*j = v
		return nil
	default:
		return errors.New("failed to scan JSONB: unsupported type")
	}
}

// JSONBArray represents a PostgreSQL JSONB array
type JSONBArray []interface{}

// Value implements the driver.Valuer interface for database writes
func (j JSONBArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for database reads
func (j *JSONBArray) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	// Handle different value types that pgx might pass
	switch v := value.(type) {
	case []byte:
		// Raw JSON bytes from database
		var result []interface{}
		if err := json.Unmarshal(v, &result); err != nil {
			return err
		}
		*j = result
		return nil
	case string:
		// JSON as string
		var result []interface{}
		if err := json.Unmarshal([]byte(v), &result); err != nil {
			return err
		}
		*j = result
		return nil
	case []interface{}:
		// Already decoded by pgx
		*j = v
		return nil
	default:
		return errors.New("failed to scan JSONBArray: unsupported type")
	}
}

// JSONBStruct is a generic wrapper type for storing structs as JSONB.
// This is provided for backward compatibility. For new code, consider using
// direct struct scanning instead (just use *YourStruct for the field type).
//
// Example using JSONBStruct (old approach):
//    type User struct {
//        Metadata schema.JSONBStruct[MyMetadata] `po:"metadata,jsonb"`
//    }
//
// Recommended alternative (direct struct scanning):
//    type User struct {
//        Metadata *MyMetadata `po:"metadata,jsonb"` // Cleaner, no wrapper needed
//    }
type JSONBStruct[T any] struct {
	Data T
}

// Value implements the driver.Valuer interface for database writes
func (j JSONBStruct[T]) Value() (driver.Value, error) {
	return json.Marshal(j.Data)
}

// Scan implements the sql.Scanner interface for database reads
func (j *JSONBStruct[T]) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	// Handle different value types that pgx might pass
	switch v := value.(type) {
	case []byte:
		// Raw JSON bytes from database
		return json.Unmarshal(v, &j.Data)
	case string:
		// JSON as string
		return json.Unmarshal([]byte(v), &j.Data)
	default:
		// Try JSON marshaling the value and then unmarshaling into our struct
		// This handles cases where pgx passes already-decoded values
		bytes, err := json.Marshal(v)
		if err != nil {
			return errors.New("failed to scan JSONBStruct: unsupported type")
		}
		return json.Unmarshal(bytes, &j.Data)
	}
}

// MarshalJSON implements json.Marshaler
func (j JSONBStruct[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.Data)
}

// UnmarshalJSON implements json.Unmarshaler
func (j *JSONBStruct[T]) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &j.Data)
}
