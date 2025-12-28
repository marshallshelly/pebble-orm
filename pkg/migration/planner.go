package migration

import (
	"fmt"
	"strings"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

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

	// Generate CREATE TABLE statements (up) and DROP TABLE statements (down)
	for _, table := range diff.TablesAdded {
		upStatements = append(upStatements, p.generateCreateTable(&table))
		downStatements = append(downStatements, p.generateDropTable(table.Name))
	}

	// Generate DROP TABLE statements (up) and CREATE TABLE statements (down)
	// Note: For dropped tables, we can't recreate them perfectly in down migration
	// without storing the schema, so we just generate comments
	for _, tableName := range diff.TablesDropped {
		upStatements = append(upStatements, p.generateDropTable(tableName))
		downStatements = append(downStatements, fmt.Sprintf("-- TODO: Recreate table %s", tableName))
	}

	// Generate ALTER TABLE statements for modified tables
	for _, tableDiff := range diff.TablesModified {
		upAlter, downAlter := p.generateAlterTable(tableDiff)
		upStatements = append(upStatements, upAlter...)
		downStatements = append(downStatements, downAlter...)
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

	// Check constraints
	for _, constraint := range table.Constraints {
		if constraint.Type == schema.CheckConstraint {
			parts = append(parts, fmt.Sprintf("    CONSTRAINT %s CHECK %s", constraint.Name, constraint.Expression))
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

// generateCreateIndex generates a CREATE INDEX statement.
func (p *Planner) generateCreateIndex(tableName string, idx schema.IndexMetadata) string {
	unique := ""
	if idx.Unique {
		unique = "UNIQUE "
	}

	method := ""
	if idx.Type != "" && idx.Type != "btree" {
		method = " USING " + idx.Type
	}

	// Add IF NOT EXISTS for idempotent index creation
	ifNotExists := ""
	if p.options.IfNotExists {
		ifNotExists = "IF NOT EXISTS "
	}

	cols := strings.Join(idx.Columns, ", ")
	return fmt.Sprintf("CREATE %sINDEX %s%s ON %s%s (%s);", unique, ifNotExists, idx.Name, tableName, method, cols)
}

// generateDropTable generates a DROP TABLE statement.
func (p *Planner) generateDropTable(tableName string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s;", tableName)
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
	for _, colName := range diff.ColumnsDropped {
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;",
			tableName, colName))
		downSQL = append(downSQL, fmt.Sprintf("-- TODO: Re-add column %s to table %s", colName, tableName))
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
	for _, idxName := range diff.IndexesDropped {
		upSQL = append(upSQL, fmt.Sprintf("DROP INDEX IF EXISTS %s;", idxName))
		downSQL = append(downSQL, fmt.Sprintf("-- TODO: Recreate index %s", idxName))
	}

	// Add foreign keys
	for _, fk := range diff.ForeignKeysAdded {
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s ADD %s;",
			tableName, p.generateForeignKeyDefinition(fk)))
		downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;",
			tableName, fk.Name))
	}

	// Drop foreign keys
	for _, fkName := range diff.ForeignKeysDropped {
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;",
			tableName, fkName))
		downSQL = append(downSQL, fmt.Sprintf("-- TODO: Re-add foreign key %s", fkName))
	}

	// Add constraints
	for _, c := range diff.ConstraintsAdded {
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK %s;",
			tableName, c.Name, c.Expression))
		downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;",
			tableName, c.Name))
	}

	// Drop constraints
	for _, cName := range diff.ConstraintsDropped {
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;",
			tableName, cName))
		downSQL = append(downSQL, fmt.Sprintf("-- TODO: Re-add constraint %s", cName))
	}

	return upSQL, downSQL
}

// generateColumnModification generates ALTER statements for column changes.
func (p *Planner) generateColumnModification(tableName string, colDiff ColumnDiff) (upSQL, downSQL []string) {
	colName := colDiff.ColumnName

	// Type change
	if colDiff.TypeChanged {
		upSQL = append(upSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;",
			tableName, colName, colDiff.NewColumn.SQLType))
		downSQL = append(downSQL, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;",
			tableName, colName, colDiff.OldColumn.SQLType))
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
