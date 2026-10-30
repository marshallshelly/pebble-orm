package builder

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// rowScanner scans the rows of one result into structs of one table. Which
// field each column goes to is worked out on the first row and reused for the
// rest, so a result costs one lookup per column rather than one per column per
// row. Use a new rowScanner for each result.
type rowScanner struct {
	table      *schema.TableMetadata
	structType reflect.Type
	fields     []scanField
	targets    []interface{}
	discard    interface{}
}

// scanField is where one result column goes in the struct.
type scanField struct {
	position int   // the column's position in the result
	index    []int // the field's reflect index path
	jsonb    bool
	array    reflect.Type // the underlying slice type, for named Scanner slices
}

func newRowScanner(table *schema.TableMetadata) *rowScanner {
	return &rowScanner{table: table}
}

// scan reads the current row into dest, a pointer to a struct.
func (s *rowScanner) scan(rows pgx.Rows, dest interface{}) error {
	destValue := reflect.ValueOf(dest)
	if destValue.Kind() != reflect.Pointer {
		return fmt.Errorf("dest must be a pointer to struct")
	}

	destValue = destValue.Elem()
	if destValue.Kind() != reflect.Struct {
		return fmt.Errorf("dest must be a pointer to struct")
	}

	if s.structType != destValue.Type() {
		s.plan(rows, destValue)
	}

	var jsonbTargets []*jsonbScanTarget
	var arrayTargets []*arrayScanTarget

	for i := range s.fields {
		f := &s.fields[i]
		field := destValue.FieldByIndex(f.index)
		switch {
		case f.jsonb:
			target := &jsonbScanTarget{field: field}
			s.targets[f.position] = target
			jsonbTargets = append(jsonbTargets, target)
		case f.array != nil:
			target := &arrayScanTarget{field: field, dest: reflect.New(f.array)}
			s.targets[f.position] = target.dest.Interface()
			arrayTargets = append(arrayTargets, target)
		default:
			s.targets[f.position] = field.Addr().Interface()
		}
	}

	if err := rows.Scan(s.targets...); err != nil {
		return fmt.Errorf("failed to scan row: %w", err)
	}

	// Post-process JSONB targets - unmarshal into actual field types
	for _, target := range jsonbTargets {
		if err := target.unmarshalIntoField(); err != nil {
			return fmt.Errorf("failed to unmarshal JSONB: %w", err)
		}
	}

	// Post-process array targets - convert underlying slices to the named types
	for _, target := range arrayTargets {
		target.field.Set(target.dest.Elem().Convert(target.field.Type()))
	}

	return nil
}

// plan maps the result's columns to fields of destValue's type. A column the
// table doesn't define, or whose field can't be set, is read and discarded.
func (s *rowScanner) plan(rows pgx.Rows, destValue reflect.Value) {
	fieldDescriptions := rows.FieldDescriptions()

	columnMap := make(map[string]int, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columnMap[fd.Name] = i
	}

	s.structType = destValue.Type()
	s.fields = s.fields[:0]
	for _, col := range s.table.Columns {
		position, ok := columnMap[col.Name]
		if !ok {
			continue
		}

		field := destValue.FieldByName(col.GoField)
		if !field.IsValid() || !field.CanSet() {
			continue
		}

		structField, _ := s.structType.FieldByName(col.GoField)
		s.fields = append(s.fields, scanField{
			position: position,
			index:    structField.Index,
			// JSONB columns use intermediate scanning if the type doesn't implement Scanner
			jsonb: col.IsJSONB && !implementsScanner(field.Type()),
			array: arrayScanType(col, field.Type()),
		})
	}

	s.targets = make([]interface{}, len(fieldDescriptions))
	for i := range s.targets {
		s.targets[i] = &s.discard
	}
}

// arrayScanTarget scans an array column into the field's underlying slice
// type (e.g. []string for schema.StringArray) so pgx decodes it natively in
// both text and binary formats, then converts to the named field type.
type arrayScanTarget struct {
	field reflect.Value
	dest  reflect.Value // pointer to a value of the underlying slice type
}

// arrayScanType returns the underlying slice type to scan an array column into
// when its field is a named slice type implementing sql.Scanner (other than
// []byte), or nil if direct scanning should be used. pgx then decodes the array
// natively instead of handing sql.Scanner raw binary wire bytes under the
// default extended protocol.
func arrayScanType(col schema.ColumnMetadata, t reflect.Type) reflect.Type {
	if !strings.HasSuffix(col.SQLType, "[]") {
		return nil
	}
	if t.Kind() != reflect.Slice || t.Elem().Kind() == reflect.Uint8 {
		return nil
	}
	if t.Name() == "" || !implementsScanner(t) {
		return nil // plain slices ([]string etc.) already decode natively
	}
	return reflect.SliceOf(t.Elem())
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
	valuerType := reflect.TypeFor[driver.Valuer]()

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
	if modelValue.Kind() == reflect.Pointer {
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

		// Generated columns are always computed by the database; a value can
		// never be inserted into them (PostgreSQL errors with 428C9).
		if col.Generated != nil {
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

		value, err := columnValue(col, field)
		if err != nil {
			return nil, nil, err
		}
		values = append(values, value)
	}

	return columns, values, nil
}

// valuesForColumns extracts the values for exactly the named columns, in order,
// without applying the per-row skip logic. It is used for rows 2..N of a
// multi-row INSERT so every row emits a value for the same column list that was
// derived from the first row — otherwise a later row whose zero-value/default
// skipping differs would misalign values against the column list (silently
// writing a value into the wrong column, or a placeholder-count mismatch).
func valuesForColumns(model interface{}, table *schema.TableMetadata, columns []string) ([]interface{}, error) {
	modelValue := reflect.ValueOf(model)
	if modelValue.Kind() == reflect.Pointer {
		modelValue = modelValue.Elem()
	}
	if modelValue.Kind() != reflect.Struct {
		return nil, fmt.Errorf("model must be a struct")
	}

	values := make([]interface{}, 0, len(columns))
	for _, name := range columns {
		col := table.GetColumnByName(name)
		if col == nil {
			return nil, fmt.Errorf("column %s not found in table %s", name, table.Name)
		}
		field := modelValue.FieldByName(col.GoField)
		if !field.IsValid() {
			return nil, fmt.Errorf("field %s not found for column %s", col.GoField, name)
		}
		value, err := columnValue(*col, field)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

// columnValue returns the value to bind for a single column, marshaling JSONB
// columns whose type does not implement driver.Valuer.
func columnValue(col schema.ColumnMetadata, field reflect.Value) (interface{}, error) {
	fieldValue := field.Interface()
	if col.IsJSONB && !implementsValuer(field.Type()) {
		jsonBytes, err := marshalJSONB(fieldValue)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSONB field %s: %w", col.GoField, err)
		}
		return jsonBytes, nil
	}
	return fieldValue, nil
}

// marshalJSONB marshals a value for a JSONB column. Returns string because
// pgx correctly handles string->jsonb conversion, while []byte might be
// incorrectly encoded as bytea. Nil values (and nil pointers/interfaces)
// return nil so they are stored as SQL NULL, not an empty string, which
// PostgreSQL rejects as invalid JSON.
func marshalJSONB(value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	// Check for nil pointers/interfaces
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil, nil
		}
		// Dereference for marshaling
		value = v.Elem().Interface()
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return string(jsonBytes), nil
}
