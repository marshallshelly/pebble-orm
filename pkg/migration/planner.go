package migration

import (
	"fmt"
	"strings"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// quoteIdent quotes a PostgreSQL identifier (table name, column name, etc.)
// to handle reserved keywords and special characters.
func quoteIdent(name string) string {
	return fmt.Sprintf(`"%s"`, name)
}

// PlannerOptions configures migration generation behavior.
type PlannerOptions struct {
	// IfNotExists adds IF NOT EXISTS to CREATE TABLE statements.
	// This makes migrations idempotent and safe to run multiple times.
	// Default: true (safe by default)
	IfNotExists bool
}

// Planner generates SQL migration statements from schema diffs.
type Planner struct {
	options PlannerOptions
}

// NewPlanner creates a new migration planner with default options.
func NewPlanner() *Planner {
	return &Planner{
		options: PlannerOptions{
			IfNotExists: true, // Safe by default
		},
	}
}

// NewPlannerWithOptions creates a new migration planner with custom options.
func NewPlannerWithOptions(opts PlannerOptions) *Planner {
	return &Planner{
		options: opts,
	}
}

// GenerateMigration generates up and down SQL from a schema diff.
func (p *Planner) GenerateMigration(diff *SchemaDiff) (upSQL, downSQL string) {
	var upStatements []string
	var downStatements []string

	// IMPORTANT: Enum types must be created BEFORE tables that use them
	// and dropped AFTER tables that use them are dropped.

	// UP migration order:
	// 1. CREATE TYPE for new enum types
	for _, enumType := range diff.EnumTypesAdded {
		upStatements = append(upStatements, p.generateCreateEnumType(enumType))
		downStatements = append(downStatements, p.generateDropEnumType(enumType.Name))
	}

	// 2. ALTER TYPE to add new enum values
	for _, enumDiff := range diff.EnumTypesModified {
		alterSQL := p.generateAlterEnumType(enumDiff)
		upStatements = append(upStatements, alterSQL...)
		// Note: PostgreSQL does not support removing enum values — down migration is a no-op for value additions
		downStatements = append(downStatements, fmt.Sprintf("-- NOTE: Cannot automatically remove enum values from type %s (PostgreSQL limitation)", enumDiff.Name))
	}

	// 3. CREATE TABLE statements
	for _, table := range diff.TablesAdded {
		upStatements = append(upStatements, p.generateCreateTable(&table))
		downStatements = append(downStatements, p.generateDropTable(table.Name))
	}

	// 4. ALTER TABLE statements for table modifications
	for _, tableDiff := range diff.TablesModified {
		upAlter, downAlter := p.generateAlterTable(tableDiff)
		upStatements = append(upStatements, upAlter...)
		downStatements = append(downStatements, downAlter...)
	}

	// 5. DROP TABLE statements
	for _, table := range diff.TablesDropped {
		upStatements = append(upStatements, p.generateDropTable(table.Name))
		downStatements = append(downStatements, p.generateCreateTable(&table))
	}

	// 6. DROP TYPE for enum types that are no longer used
	// This should come AFTER dropping tables that use them
	for _, enumType := range diff.EnumTypesDropped {
		upStatements = append(upStatements, p.generateDropEnumType(enumType.Name))
		downStatements = append(downStatements, p.generateCreateEnumType(enumType))
	}

	// Join statements
	up := strings.Join(upStatements, "\n\n") + "\n"
	down := strings.Join(downStatements, "\n\n") + "\n"

	return up, down
}

// generateCreateTable generates a CREATE TABLE statement.
func (p *Planner) generateCreateTable(table *schema.TableMetadata) string {
	var parts []string

	// Determine if we have a single-column primary key for inline declaration
	var singlePKColumn string
	if table.PrimaryKey != nil && len(table.PrimaryKey.Columns) == 1 {
		singlePKColumn = table.PrimaryKey.Columns[0]
	}

	// Columns
	for _, col := range table.Columns {
		colDef := p.generateColumnDefinition(col)

		// Add inline PRIMARY KEY for single-column PKs
		if singlePKColumn != "" && col.Name == singlePKColumn {
			colDef += " PRIMARY KEY"
		}

		parts = append(parts, "    "+colDef)
	}

	// Primary key (composite only - single column handled inline)
	if table.PrimaryKey != nil && len(table.PrimaryKey.Columns) > 1 {
		pkCols := strings.Join(table.PrimaryKey.Columns, ", ")
		parts = append(parts, fmt.Sprintf("    CONSTRAINT %s PRIMARY KEY (%s)", table.PrimaryKey.Name, pkCols))
	}

	// Foreign keys
	for _, fk := range table.ForeignKeys {
		parts = append(parts, "    "+p.generateForeignKeyDefinition(fk))
	}

	// Constraints (CHECK, UNIQUE)
	for _, constraint := range table.Constraints {
		switch constraint.Type {
		case schema.CheckConstraint:
			parts = append(parts, fmt.Sprintf("    CONSTRAINT %s CHECK %s", constraint.Name, constraint.Expression))
		case schema.UniqueConstraint:
			// Only add UNIQUE constraint if it's not already handled by inline column UNIQUE
			// Multi-column UNIQUE constraints or explicit UNIQUE constraints go here
			if len(constraint.Columns) > 1 {
				cols := strings.Join(constraint.Columns, ", ")
				parts = append(parts, fmt.Sprintf("    CONSTRAINT %s UNIQUE (%s)", constraint.Name, cols))
			}
		}
	}

	// Build CREATE TABLE statement with optional IF NOT EXISTS
	createClause := "CREATE TABLE"
	if p.options.IfNotExists {
		createClause = "CREATE TABLE IF NOT EXISTS"
	}
	sql := fmt.Sprintf("%s %s (\n%s\n);", createClause, table.Name, strings.Join(parts, ",\n"))

	// Indexes (separate statements)
	var indexStatements []string
	for _, idx := range table.Indexes {
		indexStatements = append(indexStatements, p.generateCreateIndex(table.Name, idx))
	}

	if len(indexStatements) > 0 {
		sql += "\n\n" + strings.Join(indexStatements, "\n")
	}

	return sql
}

// generateColumnDefinition generates a column definition.
func (p *Planner) generateColumnDefinition(col schema.ColumnMetadata) string {
	parts := []string{col.Name, col.SQLType}

	// Generated columns cannot have NOT NULL, DEFAULT, or UNIQUE constraints
	// as they are computed from other columns
	if col.Generated != nil {
		// GENERATED ALWAYS AS (expression) STORED
		genClause := fmt.Sprintf("GENERATED ALWAYS AS (%s) %s",
			col.Generated.Expression,
			col.Generated.Type)
		parts = append(parts, genClause)
		return strings.Join(parts, " ")
	}

	// Identity columns (PostgreSQL 10+, SQL Standard)
	// GENERATED { ALWAYS | BY DEFAULT } AS IDENTITY
	if col.Identity != nil {
		identityClause := fmt.Sprintf("GENERATED %s AS IDENTITY", col.Identity.Generation)
		parts = append(parts, identityClause)
		// Identity columns are automatically NOT NULL, no need to add it explicitly
		return strings.Join(parts, " ")
	}

	if !col.Nullable {
		parts = append(parts, "NOT NULL")
	}

	if col.Default != nil {
		parts = append(parts, "DEFAULT", *col.Default)
	}

	if col.Unique {
		parts = append(parts, "UNIQUE")
	}

	return strings.Join(parts, " ")
}

// generateForeignKeyDefinition generates a foreign key constraint.
func (p *Planner) generateForeignKeyDefinition(fk schema.ForeignKeyMetadata) string {
	localCols := strings.Join(fk.Columns, ", ")
	refCols := strings.Join(fk.ReferencedColumns, ", ")

	parts := []string{
		fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s)", fk.Name, localCols),
		fmt.Sprintf("REFERENCES %s (%s)", fk.ReferencedTable, refCols),
	}

	if fk.OnDelete != schema.NoAction && fk.OnDelete != "" {
		parts = append(parts, "ON DELETE "+string(fk.OnDelete))
	}

	if fk.OnUpdate != schema.NoAction && fk.OnUpdate != "" {
		parts = append(parts, "ON UPDATE "+string(fk.OnUpdate))
	}

	return strings.Join(parts, " ")
}

// generateCreateIndex generates a CREATE INDEX statement with full support for:
// - Expression indexes: CREATE INDEX ... ON table (lower(email))
// - Partial indexes: CREATE INDEX ... ON table (col) WHERE condition
// - Covering indexes: CREATE INDEX ... ON table (col) INCLUDE (col2, col3)
// - Column ordering: CREATE INDEX ... ON table (col1 DESC, col2 ASC)
// - Index types: CREATE INDEX ... ON table USING gin (col)
func (p *Planner) generateCreateIndex(tableName string, idx schema.IndexMetadata) string {
	var parts []string

	// CREATE [UNIQUE] INDEX
	if idx.Unique {
		parts = append(parts, "CREATE UNIQUE INDEX")
	} else {
		parts = append(parts, "CREATE INDEX")
	}

	// [CONCURRENTLY]
	if idx.Concurrent {
		parts = append(parts, "CONCURRENTLY")
	}

	// [IF NOT EXISTS]
	if p.options.IfNotExists {
		parts = append(parts, "IF NOT EXISTS")
	}

	// index_name
	parts = append(parts, idx.Name)

	// ON table
	parts = append(parts, "ON", tableName)

	// [USING method]
	if idx.Type != "" && idx.Type != "btree" {
		parts = append(parts, "USING", idx.Type)
	}

	// (columns) or (expression)
	if idx.Expression != "" {
		// Expression index
		parts = append(parts, fmt.Sprintf("(%s)", idx.Expression))
	} else {
		// Regular column index with optional ordering
		cols := p.formatColumnsWithOrdering(idx.Columns, idx.ColumnOrdering)
		parts = append(parts, fmt.Sprintf("(%s)", cols))
	}

	// [INCLUDE (columns)]
	if len(idx.Include) > 0 {
		includeCols := strings.Join(idx.Include, ", ")
		parts = append(parts, fmt.Sprintf("INCLUDE (%s)", includeCols))
	}

	// [WHERE predicate]
	if idx.Where != "" {
		parts = append(parts, "WHERE", idx.Where)
	}

	return strings.Join(parts, " ") + ";"
}

// formatColumnsWithOrdering formats columns with optional modifiers.
// Supports: column_name [opclass] [COLLATE "collation"] [ASC|DESC] [NULLS FIRST|LAST]
// Examples:
//   - ["col1", "col2"] with no ordering -> "col1, col2"
//   - ["col1"] with opclass -> "col1 varchar_pattern_ops"
//   - ["col1"] with collation -> "col1 COLLATE \"en_US\""
//   - ["col1"] with all modifiers -> "col1 varchar_pattern_ops COLLATE \"C\" DESC NULLS LAST"
func (p *Planner) formatColumnsWithOrdering(columns []string, ordering []schema.ColumnOrder) string {
	if len(ordering) == 0 {
		return strings.Join(columns, ", ")
	}

	// Build a map for quick lookup
	orderMap := make(map[string]schema.ColumnOrder)
	for _, ord := range ordering {
		orderMap[ord.Column] = ord
	}

	var parts []string
	for _, col := range columns {
		part := col

		if ord, ok := orderMap[col]; ok {
			// Add operator class if specified
			if ord.OpClass != "" {
				part += " " + ord.OpClass
			}

			// Add collation if specified
			if ord.Collation != "" {
				part += fmt.Sprintf(` COLLATE "%s"`, ord.Collation)
			}

			// Add direction if it's not the default (ASC)
			if ord.Direction == schema.Descending {
				part += " DESC"
			}

			// Add NULLS ordering if specified
			if ord.Nulls != "" {
				part += " " + string(ord.Nulls)
			}
		}

		parts = append(parts, part)
	}

	return strings.Join(parts, ", ")
}

// generateDropTable generates a DROP TABLE statement.
func (p *Planner) generateDropTable(tableName string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s;", quoteIdent(tableName))
}

// generateAlterTable generates ALTER TABLE statements for table modifications.
func (p *Planner) generateAlterTable(diff TableDiff) (upSQL, downSQL []string) {
	tableName := diff.TableName

	// Add columns
	for _, col := range diff.ColumnsAdded {
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;",
			tableName, p.generateColumnDefinition(col)))
		downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;",
			tableName, col.Name))
	}

	// Drop columns
	for _, col := range diff.ColumnsDropped {
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;",
			tableName, col.Name))
		downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;",
			tableName, p.generateColumnDefinition(col)))
	}

	// Modify columns
	for _, colDiff := range diff.ColumnsModified {
		alterUp, alterDown := p.generateColumnModification(tableName, colDiff)
		upSQL = append(upSQL, alterUp...)
		downSQL = append(downSQL, alterDown...)
	}

	// Primary key changes
	if diff.PrimaryKeyChanged != nil {
		pkUp, pkDown := p.generatePrimaryKeyChange(tableName, diff.PrimaryKeyChanged)
		upSQL = append(upSQL, pkUp...)
		downSQL = append(downSQL, pkDown...)
	}

	// Add indexes
	for _, idx := range diff.IndexesAdded {
		upSQL = append(upSQL, p.generateCreateIndex(tableName, idx))
		downSQL = append(downSQL, fmt.Sprintf("DROP INDEX IF EXISTS %s;", idx.Name))
	}

	// Drop indexes
	for _, idx := range diff.IndexesDropped {
		upSQL = append(upSQL, fmt.Sprintf("DROP INDEX IF EXISTS %s;", idx.Name))
		downSQL = append(downSQL, p.generateCreateIndex(tableName, idx))
	}

	// Add foreign keys
	for _, fk := range diff.ForeignKeysAdded {
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s ADD %s;",
			tableName, p.generateForeignKeyDefinition(fk)))
		downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;",
			tableName, fk.Name))
	}

	// Drop foreign keys
	for _, fk := range diff.ForeignKeysDropped {
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;",
			tableName, fk.Name))
		downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s ADD %s;",
			tableName, p.generateForeignKeyDefinition(fk)))
	}

	// Add constraints
	for _, c := range diff.ConstraintsAdded {
		upSQL = append(upSQL, p.generateAddConstraintSQL(tableName, c))
		downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;",
			tableName, c.Name))
	}

	// Drop constraints
	for _, c := range diff.ConstraintsDropped {
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;",
			tableName, c.Name))
		downSQL = append(downSQL, p.generateAddConstraintSQL(tableName, c))
	}

	return upSQL, downSQL
}

// generateColumnModification generates ALTER statements for column changes.
func (p *Planner) generateColumnModification(tableName string, colDiff ColumnDiff) (upSQL, downSQL []string) {
	colName := colDiff.ColumnName

	// Type change
	if colDiff.TypeChanged {
		// Check if this type conversion requires explicit USING clause
		if requiresUsingClause(colDiff.OldColumn.SQLType, colDiff.NewColumn.SQLType) {
			// Try to generate automatic USING clause
			usingClause := generateUsingClause(colName, colDiff.OldColumn.SQLType, colDiff.NewColumn.SQLType)

			if usingClause != "" {
				// We have a safe conversion
				upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s %s;",
					tableName, colName, colDiff.NewColumn.SQLType, usingClause))
			} else {
				// No safe automatic conversion - require manual intervention
				upSQL = append(upSQL, fmt.Sprintf("-- MANUAL MIGRATION REQUIRED: Cannot auto-convert %s from %s to %s",
					colName, colDiff.OldColumn.SQLType, colDiff.NewColumn.SQLType))
				upSQL = append(upSQL, "-- Please review and uncomment/modify the following statement:")
				upSQL = append(upSQL, fmt.Sprintf("-- ALTER TABLE %s ALTER COLUMN %s TYPE %s USING <expression>;",
					tableName, colName, colDiff.NewColumn.SQLType))
			}

			// Down migration (reverse)
			usingClauseDown := generateUsingClause(colName, colDiff.NewColumn.SQLType, colDiff.OldColumn.SQLType)
			if usingClauseDown != "" {
				downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s %s;",
					tableName, colName, colDiff.OldColumn.SQLType, usingClauseDown))
			} else {
				downSQL = append(downSQL, fmt.Sprintf("-- MANUAL MIGRATION REQUIRED: Reverse type conversion for %s", colName))
				downSQL = append(downSQL, fmt.Sprintf("-- ALTER TABLE %s ALTER COLUMN %s TYPE %s USING <expression>;",
					tableName, colName, colDiff.OldColumn.SQLType))
			}
		} else {
			// Simple type conversion (implicit cast available)
			upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;",
				tableName, colName, colDiff.NewColumn.SQLType))
			downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;",
				tableName, colName, colDiff.OldColumn.SQLType))
		}
	}

	// Nullability change
	if colDiff.NullChanged {
		if colDiff.NewColumn.Nullable {
			upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;",
				tableName, colName))
			downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;",
				tableName, colName))
		} else {
			upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;",
				tableName, colName))
			downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;",
				tableName, colName))
		}
	}

	// Default value change
	if colDiff.DefaultChanged {
		if colDiff.NewColumn.Default != nil {
			upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;",
				tableName, colName, *colDiff.NewColumn.Default))
		} else {
			upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;",
				tableName, colName))
		}

		if colDiff.OldColumn.Default != nil {
			downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;",
				tableName, colName, *colDiff.OldColumn.Default))
		} else {
			downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;",
				tableName, colName))
		}
	}

	return upSQL, downSQL
}

// generatePrimaryKeyChange generates ALTER statements for primary key changes.
func (p *Planner) generatePrimaryKeyChange(tableName string, pkChange *PrimaryKeyChange) (upSQL, downSQL []string) {
	// Drop old primary key
	if pkChange.Old != nil {
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;",
			tableName, pkChange.Old.Name))
	}

	// Add new primary key
	if pkChange.New != nil {
		cols := strings.Join(pkChange.New.Columns, ", ")
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s PRIMARY KEY (%s);",
			tableName, pkChange.New.Name, cols))
	}

	// Reverse for down migration
	if pkChange.New != nil {
		downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;",
			tableName, pkChange.New.Name))
	}

	if pkChange.Old != nil {
		cols := strings.Join(pkChange.Old.Columns, ", ")
		downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s PRIMARY KEY (%s);",
			tableName, pkChange.Old.Name, cols))
	}

	return upSQL, downSQL
}

// requiresUsingClause determines if a type conversion requires an explicit USING clause.
// PostgreSQL can implicitly cast some types but not others.
func requiresUsingClause(fromType, toType string) bool {
	// Normalize types for comparison
	from := strings.ToLower(strings.TrimSpace(fromType))
	to := strings.ToLower(strings.TrimSpace(toType))

	// Same type - no USING needed
	if from == to {
		return false
	}

	// Common conversions that require USING
	incompatibleConversions := map[string]map[string]bool{
		"text": {
			"text[]":  true,
			"jsonb":   true,
			"json":    true,
			"integer": true,
			"bigint":  true,
			"numeric": true,
		},
		"varchar": {
			"text[]":  true,
			"jsonb":   true,
			"json":    true,
			"integer": true,
			"bigint":  true,
			"numeric": true,
		},
		"integer": {
			"text[]": true,
			"jsonb":  true,
		},
		"bigint": {
			"text[]": true,
			"jsonb":  true,
		},
	}

	// Check if conversion is in the incompatible list
	if toTypes, exists := incompatibleConversions[from]; exists {
		if toTypes[to] {
			return true
		}
	}

	// Check for array conversions (e.g., text → text[])
	if strings.HasSuffix(to, "[]") && !strings.HasSuffix(from, "[]") {
		return true
	}

	// Check for jsonb/json conversions from non-json types
	if (to == "jsonb" || to == "json") && from != "jsonb" && from != "json" {
		return true
	}

	return false
}

// generateUsingClause generates an appropriate USING clause for common type conversions.
// Returns empty string if no safe automatic conversion is available.
func generateUsingClause(columnName, fromType, toType string) string {
	from := strings.ToLower(strings.TrimSpace(fromType))
	to := strings.ToLower(strings.TrimSpace(toType))

	// text/varchar → text[]
	if (from == "text" || strings.HasPrefix(from, "varchar")) && to == "text[]" {
		return fmt.Sprintf("USING CASE WHEN %s IS NULL THEN NULL WHEN %s = '' THEN ARRAY[]::text[] ELSE ARRAY[%s]::text[] END",
			columnName, columnName, columnName)
	}

	// text/varchar → jsonb
	if (from == "text" || strings.HasPrefix(from, "varchar")) && to == "jsonb" {
		return fmt.Sprintf("USING CASE WHEN %s IS NULL THEN NULL WHEN %s = '' THEN '{}'::jsonb ELSE %s::jsonb END",
			columnName, columnName, columnName)
	}

	// text/varchar → json
	if (from == "text" || strings.HasPrefix(from, "varchar")) && to == "json" {
		return fmt.Sprintf("USING CASE WHEN %s IS NULL THEN NULL WHEN %s = '' THEN '{}'::json ELSE %s::json END",
			columnName, columnName, columnName)
	}

	// text/varchar → integer/bigint
	if (from == "text" || strings.HasPrefix(from, "varchar")) && (to == "integer" || to == "bigint") {
		return fmt.Sprintf("USING CASE WHEN %s ~ '^[0-9]+$' THEN %s::%s ELSE NULL END",
			columnName, columnName, to)
	}

	// No safe automatic conversion available
	return ""
}

// generateAddConstraintSQL generates SQL for adding a constraint.
func (p *Planner) generateAddConstraintSQL(tableName string, c schema.ConstraintMetadata) string {
	switch c.Type {
	case schema.UniqueConstraint:
		return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s UNIQUE (%s);",
			tableName, c.Name, strings.Join(c.Columns, ", "))
	case schema.CheckConstraint:
		return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK %s;",
			tableName, c.Name, c.Expression)
	default:
		return fmt.Sprintf("-- Unknown constraint type: %s", c.Type)
	}
}

// generateCreateEnumType generates a CREATE TYPE statement for an enum.
func (p *Planner) generateCreateEnumType(enumType schema.EnumType) string {
	// Quote each enum value
	quotedValues := make([]string, len(enumType.Values))
	for i, val := range enumType.Values {
		quotedValues[i] = fmt.Sprintf("'%s'", val)
	}

	values := strings.Join(quotedValues, ", ")
	return fmt.Sprintf("CREATE TYPE %s AS ENUM (%s);", enumType.Name, values)
}

// generateDropEnumType generates a DROP TYPE statement for an enum.
func (p *Planner) generateDropEnumType(enumName string) string {
	return fmt.Sprintf("DROP TYPE IF EXISTS %s;", enumName)
}

// generateAlterEnumType generates ALTER TYPE statements to add new enum values.
// Note: PostgreSQL doesn't support removing or reordering enum values.
func (p *Planner) generateAlterEnumType(enumDiff EnumTypeDiff) []string {
	statements := make([]string, 0)

	// Add each new value
	for _, newValue := range enumDiff.NewValues {
		// ALTER TYPE ... ADD VALUE 'new_value'
		// Note: Adding values to enums cannot be done in a transaction block in older PostgreSQL versions,
		// but in PostgreSQL 12+ it can be done with IF NOT EXISTS
		statements = append(statements, fmt.Sprintf("ALTER TYPE %s ADD VALUE IF NOT EXISTS '%s';",
			enumDiff.Name, newValue))
	}

	return statements
}
