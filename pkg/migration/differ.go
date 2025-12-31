package migration

import (
	"fmt"
	"strings"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// Differ compares schemas and generates diffs.
type Differ struct{}

// NewDiffer creates a new schema differ.
func NewDiffer() *Differ {
	return &Differ{}
}

// Compare compares code schema (from structs) with database schema.
// codeSchema: TableMetadata from parsing Go structs
// dbSchema: TableMetadata from introspecting database
func (d *Differ) Compare(codeSchema, dbSchema map[string]*schema.TableMetadata) *SchemaDiff {
	diff := &SchemaDiff{
		TablesAdded:       make([]schema.TableMetadata, 0),
		TablesDropped:     make([]string, 0),
		TablesModified:    make([]TableDiff, 0),
		EnumTypesAdded:    make([]schema.EnumType, 0),
		EnumTypesDropped:  make([]string, 0),
		EnumTypesModified: make([]EnumTypeDiff, 0),
	}

	// Find tables that exist in code but not in DB (need to create)
	for tableName, codeTable := range codeSchema {
		if _, exists := dbSchema[tableName]; !exists {
			diff.TablesAdded = append(diff.TablesAdded, *codeTable)
		}
	}

	// Find tables that exist in DB but not in code (need to drop)
	for tableName := range dbSchema {
		if _, exists := codeSchema[tableName]; !exists {
			diff.TablesDropped = append(diff.TablesDropped, tableName)
		}
	}

	// Find tables that exist in both (check for modifications)
	for tableName, codeTable := range codeSchema {
		if dbTable, exists := dbSchema[tableName]; exists {
			tableDiff := d.compareTable(codeTable, dbTable)
			if tableDiff.HasChanges() {
				diff.TablesModified = append(diff.TablesModified, tableDiff)
			}
		}
	}

	// Compare enum types across all tables
	d.compareEnumTypes(codeSchema, dbSchema, diff)

	return diff
}

// compareTable compares two versions of the same table.
func (d *Differ) compareTable(codeTable, dbTable *schema.TableMetadata) TableDiff {
	diff := TableDiff{
		TableName:          codeTable.Name,
		ColumnsAdded:       make([]schema.ColumnMetadata, 0),
		ColumnsDropped:     make([]string, 0),
		ColumnsModified:    make([]ColumnDiff, 0),
		IndexesAdded:       make([]schema.IndexMetadata, 0),
		IndexesDropped:     make([]string, 0),
		ForeignKeysAdded:   make([]schema.ForeignKeyMetadata, 0),
		ForeignKeysDropped: make([]string, 0),
		ConstraintsAdded:   make([]schema.ConstraintMetadata, 0),
		ConstraintsDropped: make([]string, 0),
	}

	// Compare columns
	d.compareColumns(codeTable, dbTable, &diff)

	// Compare primary key
	d.comparePrimaryKey(codeTable, dbTable, &diff)

	// Compare indexes
	d.compareIndexes(codeTable, dbTable, &diff)

	// Compare foreign keys
	d.compareForeignKeys(codeTable, dbTable, &diff)

	// Compare constraints
	d.compareConstraints(codeTable, dbTable, &diff)

	return diff
}

// compareColumns compares columns between code and database.
func (d *Differ) compareColumns(codeTable, dbTable *schema.TableMetadata, diff *TableDiff) {
	// Build maps for easier lookup
	codeColumns := make(map[string]schema.ColumnMetadata)
	for _, col := range codeTable.Columns {
		codeColumns[col.Name] = col
	}

	dbColumns := make(map[string]schema.ColumnMetadata)
	for _, col := range dbTable.Columns {
		dbColumns[col.Name] = col
	}

	// Find columns to add (in code but not in DB)
	for colName, codeCol := range codeColumns {
		if _, exists := dbColumns[colName]; !exists {
			diff.ColumnsAdded = append(diff.ColumnsAdded, codeCol)
		}
	}

	// Find columns to drop (in DB but not in code)
	for colName := range dbColumns {
		if _, exists := codeColumns[colName]; !exists {
			diff.ColumnsDropped = append(diff.ColumnsDropped, colName)
		}
	}

	// Find modified columns (exist in both but differ)
	for colName, codeCol := range codeColumns {
		if dbCol, exists := dbColumns[colName]; exists {
			colDiff := d.compareColumn(codeCol, dbCol)
			if colDiff.hasChanges() {
				diff.ColumnsModified = append(diff.ColumnsModified, colDiff)
			}
		}
	}
}

// compareColumn compares two versions of the same column.
func (d *Differ) compareColumn(codeCol, dbCol schema.ColumnMetadata) ColumnDiff {
	diff := ColumnDiff{
		ColumnName: codeCol.Name,
		OldColumn:  dbCol,
		NewColumn:  codeCol,
	}

	// Compare SQL type (normalize for comparison)
	diff.TypeChanged = !d.isSameType(codeCol.SQLType, dbCol.SQLType)

	// Compare nullability
	diff.NullChanged = (codeCol.Nullable != dbCol.Nullable)

	// Compare default value with special handling for serial/autoincrement columns
	diff.DefaultChanged = !d.isSameDefaultWithSerial(codeCol, dbCol)

	return diff
}

// hasChanges returns true if the column has any changes.
func (c *ColumnDiff) hasChanges() bool {
	return c.TypeChanged || c.NullChanged || c.DefaultChanged
}

// comparePrimaryKey compares primary keys.
func (d *Differ) comparePrimaryKey(codeTable, dbTable *schema.TableMetadata, diff *TableDiff) {
	codePK := codeTable.PrimaryKey
	dbPK := dbTable.PrimaryKey

	// Both nil - no change
	if codePK == nil && dbPK == nil {
		return
	}

	// One is nil - change
	if (codePK == nil) != (dbPK == nil) {
		diff.PrimaryKeyChanged = &PrimaryKeyChange{
			Old: dbPK,
			New: codePK,
		}
		return
	}

	// Both exist - compare columns
	if !d.isSameStringSlice(codePK.Columns, dbPK.Columns) {
		diff.PrimaryKeyChanged = &PrimaryKeyChange{
			Old: dbPK,
			New: codePK,
		}
	}
}

// compareIndexes compares indexes.
func (d *Differ) compareIndexes(codeTable, dbTable *schema.TableMetadata, diff *TableDiff) {
	// Build maps for easier lookup
	codeIndexes := make(map[string]schema.IndexMetadata)
	for _, idx := range codeTable.Indexes {
		codeIndexes[idx.Name] = idx
	}

	dbIndexes := make(map[string]schema.IndexMetadata)
	for _, idx := range dbTable.Indexes {
		dbIndexes[idx.Name] = idx
	}

	// Find indexes to add
	for idxName, codeIdx := range codeIndexes {
		if _, exists := dbIndexes[idxName]; !exists {
			diff.IndexesAdded = append(diff.IndexesAdded, codeIdx)
		}
	}

	// Find indexes to drop
	for idxName := range dbIndexes {
		if _, exists := codeIndexes[idxName]; !exists {
			diff.IndexesDropped = append(diff.IndexesDropped, idxName)
		}
	}
}

// compareForeignKeys compares foreign keys.
func (d *Differ) compareForeignKeys(codeTable, dbTable *schema.TableMetadata, diff *TableDiff) {
	// Build maps for easier lookup
	codeFKs := make(map[string]schema.ForeignKeyMetadata)
	for _, fk := range codeTable.ForeignKeys {
		codeFKs[fk.Name] = fk
	}

	dbFKs := make(map[string]schema.ForeignKeyMetadata)
	for _, fk := range dbTable.ForeignKeys {
		dbFKs[fk.Name] = fk
	}

	// Find foreign keys to add
	for fkName, codeFk := range codeFKs {
		if _, exists := dbFKs[fkName]; !exists {
			diff.ForeignKeysAdded = append(diff.ForeignKeysAdded, codeFk)
		}
	}

	// Find foreign keys to drop
	for fkName := range dbFKs {
		if _, exists := codeFKs[fkName]; !exists {
			diff.ForeignKeysDropped = append(diff.ForeignKeysDropped, fkName)
		}
	}
}

// compareConstraints compares check and unique constraints.
func (d *Differ) compareConstraints(codeTable, dbTable *schema.TableMetadata, diff *TableDiff) {
	// Build maps for easier lookup
	// For UNIQUE constraints, use column-based key; for CHECK constraints, use name
	codeConstraints := make(map[string]schema.ConstraintMetadata)
	for _, c := range codeTable.Constraints {
		key := d.getConstraintKey(c)
		codeConstraints[key] = c
	}

	dbConstraints := make(map[string]schema.ConstraintMetadata)
	for _, c := range dbTable.Constraints {
		key := d.getConstraintKey(c)
		dbConstraints[key] = c
	}

	// Find constraints to add (in code but not in DB)
	for key, codeC := range codeConstraints {
		if _, exists := dbConstraints[key]; !exists {
			diff.ConstraintsAdded = append(diff.ConstraintsAdded, codeC)
		}
	}

	// Find constraints to drop (in DB but not in code)
	for key, dbC := range dbConstraints {
		if _, exists := codeConstraints[key]; !exists {
			diff.ConstraintsDropped = append(diff.ConstraintsDropped, dbC.Name)
		}
	}
}

// getConstraintKey returns a unique key for constraint comparison.
// For UNIQUE constraints, key by columns (order matters).
// For CHECK constraints, key by name.
func (d *Differ) getConstraintKey(c schema.ConstraintMetadata) string {
	if c.Type == schema.UniqueConstraint {
		// For UNIQUE: key by columns (order matters)
		return fmt.Sprintf("unique:%s", strings.Join(c.Columns, ","))
	}
	// For CHECK and others: key by name
	return c.Name
}

// Helper functions

// isSameType compares SQL types, normalizing for common variations.
func (d *Differ) isSameType(type1, type2 string) bool {
	// Normalize types
	t1 := d.normalizeType(type1)
	t2 := d.normalizeType(type2)

	return t1 == t2
}

// normalizeType normalizes SQL type strings for comparison.
// Maps PostgreSQL pseudotypes (serial) to their actual underlying types.
func (d *Differ) normalizeType(sqlType string) string {
	// Convert to lowercase
	normalized := strings.ToLower(strings.TrimSpace(sqlType))

	// Handle common aliases
	switch normalized {
	case "int", "int4":
		return "integer"
	case "int2":
		return "smallint"
	case "int8":
		return "bigint"
	case "float4":
		return "real"
	case "float8":
		return "double precision"
	case "bool":
		return "boolean"

	// Timestamp variants - normalize to base type
	case "timestamp without time zone":
		return "timestamp"
	case "timestamp with time zone":
		return "timestamptz"

	// Serial types are PostgreSQL pseudotypes that expand to integer + sequence + default
	// They ONLY work in CREATE TABLE, NOT in ALTER TABLE statements
	// Map them to their underlying base types for comparison and ALTER statements
	case "serial", "serial4":
		return "integer" // serial = integer NOT NULL DEFAULT nextval('seq')
	case "bigserial", "serial8":
		return "bigint" // bigserial = bigint NOT NULL DEFAULT nextval('seq')
	case "smallserial", "serial2":
		return "smallint" // smallserial = smallint NOT NULL DEFAULT nextval('seq')
	}

	// Remove extra whitespace
	normalized = strings.Join(strings.Fields(normalized), " ")

	return normalized
}

// isSameDefaultWithSerial compares default values with special handling for serial/autoincrement columns.
// This fixes the bug where serial columns incorrectly trigger DROP DEFAULT migrations.
func (d *Differ) isSameDefaultWithSerial(codeCol, dbCol schema.ColumnMetadata) bool {
	// Special case: serial/autoincrement columns in code are equivalent to sequence defaults in database
	// serial in code = DEFAULT nextval('table_name_id_seq'::regclass) in database
	if d.isAutoIncrementColumn(codeCol) && d.isSequenceDefault(dbCol.Default) {
		return true // These are equivalent - no migration needed
	}

	// Regular default comparison
	return d.isSameDefault(codeCol.Default, dbCol.Default)
}

// isAutoIncrementColumn checks if a column is defined as auto-increment/serial in code.
func (d *Differ) isAutoIncrementColumn(col schema.ColumnMetadata) bool {
	// Check if the SQL type is a serial type
	normalizedType := strings.ToLower(strings.TrimSpace(col.SQLType))
	if normalizedType == "serial" || normalizedType == "bigserial" || normalizedType == "smallserial" ||
		normalizedType == "serial4" || normalizedType == "serial8" || normalizedType == "serial2" {
		return true
	}

	// Check AutoIncrement flag (if set by parser)
	if col.AutoIncrement {
		return true
	}

	return false
}

// isSequenceDefault checks if a default value is a PostgreSQL sequence (nextval).
func (d *Differ) isSequenceDefault(defaultVal *string) bool {
	if defaultVal == nil || *defaultVal == "" {
		return false
	}

	// Normalize and check for nextval
	normalized := strings.ToLower(strings.TrimSpace(*defaultVal))

	// Match patterns like:
	// - nextval('table_name_id_seq'::regclass)
	// - nextval('table_name_id_seq')
	// - nextval('"table_name_id_seq"'::regclass)
	return strings.Contains(normalized, "nextval") && strings.Contains(normalized, "_seq")
}

// isSameDefault compares default values.
func (d *Differ) isSameDefault(default1, default2 *string) bool {
	// Both nil - same
	if default1 == nil && default2 == nil {
		return true
	}

	// One nil - different
	if (default1 == nil) != (default2 == nil) {
		return false
	}

	// Normalize and compare
	d1 := d.normalizeDefault(*default1)
	d2 := d.normalizeDefault(*default2)

	return d1 == d2
}

// normalizeDefault normalizes default value expressions.
func (d *Differ) normalizeDefault(defaultVal string) string {
	// Remove quotes and extra whitespace
	normalized := strings.TrimSpace(defaultVal)

	// Convert to lowercase for case-insensitive comparison
	normalized = strings.ToLower(normalized)

	// Remove surrounding parentheses if both are present
	if strings.HasPrefix(normalized, "(") && strings.HasSuffix(normalized, ")") {
		normalized = strings.TrimPrefix(normalized, "(")
		normalized = strings.TrimSuffix(normalized, ")")
	}

	// Handle type casts (::type)
	if idx := strings.Index(normalized, "::"); idx != -1 {
		normalized = normalized[:idx]
	}

	return strings.TrimSpace(normalized)
}

// isSameStringSlice compares two string slices (order matters).
func (d *Differ) isSameStringSlice(slice1, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	for i := range slice1 {
		if slice1[i] != slice2[i] {
			return false
		}
	}

	return true
}

// compareEnumTypes compares enum types across all tables.
// Enum types are database-level objects, so we collect them from all tables.
func (d *Differ) compareEnumTypes(codeSchema, dbSchema map[string]*schema.TableMetadata, diff *SchemaDiff) {
	// Collect all enum types from all tables (deduplicated)
	codeEnums := d.collectEnumTypes(codeSchema)
	dbEnums := d.collectEnumTypes(dbSchema)

	// Find enum types to add (in code but not in DB)
	for enumName, codeEnum := range codeEnums {
		if _, exists := dbEnums[enumName]; !exists {
			diff.EnumTypesAdded = append(diff.EnumTypesAdded, codeEnum)
		}
	}

	// Find enum types to drop (in DB but not in code)
	for enumName := range dbEnums {
		if _, exists := codeEnums[enumName]; !exists {
			diff.EnumTypesDropped = append(diff.EnumTypesDropped, enumName)
		}
	}

	// Find enum types with new values (exist in both but values differ)
	for enumName, codeEnum := range codeEnums {
		if dbEnum, exists := dbEnums[enumName]; exists {
			newValues := d.findNewEnumValues(codeEnum.Values, dbEnum.Values)
			if len(newValues) > 0 {
				diff.EnumTypesModified = append(diff.EnumTypesModified, EnumTypeDiff{
					Name:      enumName,
					OldValues: dbEnum.Values,
					NewValues: newValues,
				})
			}
		}
	}
}

// collectEnumTypes collects all unique enum types from all tables.
func (d *Differ) collectEnumTypes(tables map[string]*schema.TableMetadata) map[string]schema.EnumType {
	enumTypes := make(map[string]schema.EnumType)

	for _, table := range tables {
		for _, enumType := range table.EnumTypes {
			if _, exists := enumTypes[enumType.Name]; !exists {
				enumTypes[enumType.Name] = enumType
			}
		}
	}

	return enumTypes
}

// findNewEnumValues finds values in codeValues that don't exist in dbValues.
// PostgreSQL only allows adding new values, not removing or reordering.
func (d *Differ) findNewEnumValues(codeValues, dbValues []string) []string {
	dbValueSet := make(map[string]bool)
	for _, val := range dbValues {
		dbValueSet[val] = true
	}

	newValues := make([]string, 0)
	for _, val := range codeValues {
		if !dbValueSet[val] {
			newValues = append(newValues, val)
		}
	}

	return newValues
}
