package builder

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// scanIntoStruct scans a database row into a struct.
func scanIntoStruct(rows pgx.Rows, dest interface{}, table *schema.TableMetadata) error {
	// Get the value and type
	destValue := reflect.ValueOf(dest)
	if destValue.Kind() != reflect.Ptr {
		return fmt.Errorf("dest must be a pointer to struct")
	}

	destValue = destValue.Elem()
	if destValue.Kind() != reflect.Struct {
		return fmt.Errorf("dest must be a pointer to struct")
	}

	// Get column descriptions
	fieldDescriptions := rows.FieldDescriptions()

	// Create scan targets
	scanTargets := make([]interface{}, len(fieldDescriptions))
	jsonbTargets := make(map[int]*jsonbScanTarget) // Track JSONB columns for post-processing
	columnMap := make(map[string]int)              // Map column name to field description index

	for i, fd := range fieldDescriptions {
		columnMap[fd.Name] = i
	}

	// Map struct fields to scan targets
	for _, col := range table.Columns {
		idx, ok := columnMap[col.Name]
		if !ok {
			continue
		}

		// Get the struct field
		field := destValue.FieldByName(col.GoField)
		if !field.IsValid() || !field.CanSet() {
			continue
		}

		// For JSONB columns, use intermediate scanning if the type doesn't implement Scanner
		if col.IsJSONB && !implementsScanner(field.Type()) {
			target := &jsonbScanTarget{field: field}
			scanTargets[idx] = target
			jsonbTargets[idx] = target
		} else {
			// Create a pointer to the field for scanning
			scanTargets[idx] = field.Addr().Interface()
		}
	}

	// Fill any nil scan targets with dummy variables
	var dummy interface{}
	for i := range scanTargets {
		if scanTargets[i] == nil {
			scanTargets[i] = &dummy
		}
	}

	// Scan the row
	if err := rows.Scan(scanTargets...); err != nil {
		return fmt.Errorf("failed to scan row: %w", err)
	}

	// Post-process JSONB targets - unmarshal into actual field types
	for _, target := range jsonbTargets {
		if err := target.unmarshalIntoField(); err != nil {
			return fmt.Errorf("failed to unmarshal JSONB: %w", err)
		}
	}

	return nil
}

// jsonbScanTarget is an intermediate scan target for JSONB columns
// that don't implement sql.Scanner.
type jsonbScanTarget struct {
	field reflect.Value
	data  []byte
}

// Scan implements sql.Scanner for intermediate JSONB scanning.
func (j *jsonbScanTarget) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []byte:
		j.data = v
	case string:
		j.data = []byte(v)
	default:
		// If pgx already decoded it, marshal back to JSON for re-decoding
		var err error
		j.data, err = json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal decoded JSONB: %w", err)
		}
	}
	return nil
}

// unmarshalIntoField unmarshals the scanned JSON data into the target field.
func (j *jsonbScanTarget) unmarshalIntoField() error {
	if j.data == nil {
		return nil
	}

	// Create a new instance of the target type
	targetPtr := j.field.Addr().Interface()
	return json.Unmarshal(j.data, targetPtr)
}

// implementsScanner checks if a type implements sql.Scanner.
func implementsScanner(t reflect.Type) bool {
	scannerType := reflect.TypeOf((*interface{ Scan(interface{}) error })(nil)).Elem()

	// Check the type itself
	if t.Implements(scannerType) {
		return true
	}

	// Check pointer to the type
	if reflect.PointerTo(t).Implements(scannerType) {
		return true
	}

	return false
}

// implementsValuer checks if a type implements driver.Valuer.
func implementsValuer(t reflect.Type) bool {
	valuerType := reflect.TypeOf((*driver.Valuer)(nil)).Elem()

	// Check the type itself
	if t.Implements(valuerType) {
		return true
	}

	// Check pointer to the type
	if reflect.PointerTo(t).Implements(valuerType) {
		return true
	}

	return false
}

// structToValues converts a struct to column names and values.
// It intelligently omits fields from INSERT when:
// 1. Field has AutoIncrement (existing behavior)
// 2. Field has a database Default and the Go value is zero (new smart behavior)
// 3. For JSONB columns, automatically marshals values to JSON bytes.
func structToValues(model interface{}, table *schema.TableMetadata, skipPrimaryKey bool) ([]string, []interface{}, error) {
	modelValue := reflect.ValueOf(model)
	if modelValue.Kind() == reflect.Ptr {
		modelValue = modelValue.Elem()
	}

	if modelValue.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("model must be a struct")
	}

	var columns []string
	var values []interface{}

	for _, col := range table.Columns {
		// Skip primary key with auto-increment (existing behavior)
		if skipPrimaryKey && table.IsPrimaryKey(col.Name) && col.AutoIncrement {
			continue
		}

		field := modelValue.FieldByName(col.GoField)
		if !field.IsValid() {
			continue
		}

		// Smart default detection: Skip zero-valued fields that have database defaults
		// This allows natural non-pointer types like:
		//   ID string `po:"id,uuid,default(gen_random_uuid())"`
		// Instead of requiring:
		//   ID *string `po:"id,uuid,default(gen_random_uuid())"`
		if col.Default != nil && field.IsZero() {
			continue
		}

		// Skip identity columns when zero (they're auto-generated)
		if col.Identity != nil && field.IsZero() {
			continue
		}

		columns = append(columns, col.Name)

		// Get the field value
		fieldValue := field.Interface()

		// For JSONB columns, automatically marshal if type doesn't implement Valuer
		if col.IsJSONB && !implementsValuer(field.Type()) {
			jsonBytes, err := marshalJSONB(fieldValue)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal JSONB field %s: %w", col.GoField, err)
			}
			values = append(values, jsonBytes)
		} else {
			values = append(values, fieldValue)
		}
	}

	return columns, values, nil
}

// marshalJSONB marshals a value to a JSON string for JSONB columns.
// Returns string because pgx correctly handles string->jsonb conversion,
// while []byte might be incorrectly encoded as bytea.
func marshalJSONB(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}

	// Check for nil pointers/interfaces
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return "", nil
		}
		// Dereference for marshaling
		value = v.Elem().Interface()
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}
