package builder

import (
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
	columnMap := make(map[string]int) // Map column name to field description index

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

		// Create a pointer to the field for scanning
		scanTargets[idx] = field.Addr().Interface()
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

	return nil
}

// structToValues converts a struct to column names and values.
// It intelligently omits fields from INSERT when:
// 1. Field has AutoIncrement (existing behavior)
// 2. Field has a database Default and the Go value is zero (new smart behavior)
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
		values = append(values, field.Interface())
	}

	return columns, values, nil
}
