package schema

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONB represents a PostgreSQL JSONB column
// It provides automatic marshaling/unmarshaling for JSON data
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

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan JSONB: value is not []byte")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}

	*j = result
	return nil
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

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan JSONBArray: value is not []byte")
	}

	var result []interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}

	*j = result
	return nil
}

// JSONBStruct is a generic type for storing structs as JSONB
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

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan JSONBStruct: value is not []byte")
	}

	return json.Unmarshal(bytes, &j.Data)
}

// MarshalJSON implements json.Marshaler
func (j JSONBStruct[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.Data)
}

// UnmarshalJSON implements json.Unmarshaler
func (j *JSONBStruct[T]) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &j.Data)
}
