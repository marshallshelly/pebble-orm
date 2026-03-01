package migration

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// Introspector inspects database schema.
type Introspector struct {
	pool *pgxpool.Pool
}

// NewIntrospector creates a new database introspector.
func NewIntrospector(pool *pgxpool.Pool) *Introspector {
	return &Introspector{pool: pool}
}

// query executes a query using simple query protocol to avoid prepared statement caching.
func (i *Introspector) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	conn, err := i.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	// Note: conn.Release() should be called by the caller after rows.Close()

	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		conn.Release()
		return nil, err
	}

	// Wrap rows to release connection when closed
	return &rowsWithRelease{Rows: rows, conn: conn}, nil
}

// rowsWithRelease wraps pgx.Rows and releases the connection when closed.
type rowsWithRelease struct {
	pgx.Rows
	conn *pgxpool.Conn
}

func (r *rowsWithRelease) Close() {
	r.Rows.Close()
	r.conn.Release()
}

// IntrospectSchema introspects the entire database schema.
func (i *Introspector) IntrospectSchema(ctx context.Context) (map[string]*schema.TableMetadata, error) {
	tables := make(map[string]*schema.TableMetadata)

	// Get all table names
	tableNames, err := i.getTableNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get table names: %w", err)
	}

	// Introspect each table
	for _, tableName := range tableNames {
		table, err := i.IntrospectTable(ctx, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to introspect table %s: %w", tableName, err)
		}
		tables[tableName] = table
	}

	return tables, nil
}

// IntrospectTable introspects a single table.
func (i *Introspector) IntrospectTable(ctx context.Context, tableName string) (*schema.TableMetadata, error) {
	table := &schema.TableMetadata{
		Name:        tableName,
		Columns:     make([]schema.ColumnMetadata, 0),
		ForeignKeys: make([]schema.ForeignKeyMetadata, 0),
		Indexes:     make([]schema.IndexMetadata, 0),
		Constraints: make([]schema.ConstraintMetadata, 0),
		EnumTypes:   make([]schema.EnumType, 0),
	}

	// Get columns
	columns, err := i.getColumns(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	table.Columns = columns

	// Get primary key
	pk, err := i.getPrimaryKey(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary key: %w", err)
	}
	table.PrimaryKey = pk

	// Get foreign keys
	fks, err := i.getForeignKeys(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %w", err)
	}
	table.ForeignKeys = fks

	// Get indexes
	indexes, err := i.getIndexes(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	table.Indexes = indexes

	// Get check constraints
	constraints, err := i.getConstraints(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get constraints: %w", err)
	}
	table.Constraints = constraints

	// Get enum types used by this table
	enumTypes, err := i.getEnumTypes(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get enum types: %w", err)
	}
	table.EnumTypes = enumTypes

	return table, nil
}

// getTableNames retrieves all table names in the public schema.
func (i *Introspector) getTableNames(ctx context.Context) ([]string, error) {
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type = 'BASE TABLE'
		  AND table_name != 'schema_migrations'
		ORDER BY table_name
	`

	rows, err := i.query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}

	return tables, rows.Err()
}

// getColumns retrieves column information for a table.
func (i *Introspector) getColumns(ctx context.Context, tableName string) ([]schema.ColumnMetadata, error) {
	query := `
		SELECT
			column_name,
			data_type,
			udt_name,
			character_maximum_length,
			numeric_precision,
			numeric_scale,
			is_nullable,
			column_default,
			ordinal_position
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position
	`

	rows, err := i.query(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []schema.ColumnMetadata
	for rows.Next() {
		var col schema.ColumnMetadata
		var dataType, udtName string
		var maxLength, precision, scale *int
		var isNullable string
		var defaultVal *string
		var position int

		err := rows.Scan(
			&col.Name,
			&dataType,
			&udtName,
			&maxLength,
			&precision,
			&scale,
			&isNullable,
			&defaultVal,
			&position,
		)
		if err != nil {
			return nil, err
		}

		// Build SQL type
		col.SQLType = buildSQLType(dataType, udtName, maxLength, precision, scale)
		col.Nullable = (isNullable == "YES")
		col.Default = defaultVal
		col.Position = position - 1 // Zero-indexed

		// Check if auto-increment (serial)
		if defaultVal != nil && strings.Contains(*defaultVal, "nextval") {
			col.AutoIncrement = true
		}

		// Check if column uses enum type
		if dataType == "USER-DEFINED" {
			col.EnumType = udtName
			// Enum values will be populated from table.EnumTypes
		}

		columns = append(columns, col)
	}

	return columns, rows.Err()
}

// getPrimaryKey retrieves primary key information.
func (i *Introspector) getPrimaryKey(ctx context.Context, tableName string) (*schema.PrimaryKeyMetadata, error) {
	query := `
		SELECT
			tc.constraint_name,
			array_agg(kcu.column_name ORDER BY kcu.ordinal_position) as columns
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.table_schema = 'public'
			AND tc.table_name = $1
			AND tc.constraint_type = 'PRIMARY KEY'
		GROUP BY tc.constraint_name
	`

	rows, err := i.query(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		// No primary key is not an error
		return nil, nil
	}

	var name string
	var columns []string

	if err := rows.Scan(&name, &columns); err != nil {
		return nil, err
	}

	return &schema.PrimaryKeyMetadata{
		Name:    name,
		Columns: columns,
	}, nil
}

// getForeignKeys retrieves foreign key information.
func (i *Introspector) getForeignKeys(ctx context.Context, tableName string) ([]schema.ForeignKeyMetadata, error) {
	query := `
		SELECT
			tc.constraint_name,
			array_agg(DISTINCT kcu.column_name) as columns,
			ccu.table_name as foreign_table,
			array_agg(DISTINCT ccu.column_name) as foreign_columns,
			rc.update_rule,
			rc.delete_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
		JOIN information_schema.referential_constraints rc
			ON rc.constraint_name = tc.constraint_name
		WHERE tc.table_schema = 'public'
			AND tc.table_name = $1
			AND tc.constraint_type = 'FOREIGN KEY'
		GROUP BY tc.constraint_name, ccu.table_name, rc.update_rule, rc.delete_rule
	`

	rows, err := i.query(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var foreignKeys []schema.ForeignKeyMetadata
	for rows.Next() {
		var fk schema.ForeignKeyMetadata
		var updateRule, deleteRule string

		err := rows.Scan(
			&fk.Name,
			&fk.Columns,
			&fk.ReferencedTable,
			&fk.ReferencedColumns,
			&updateRule,
			&deleteRule,
		)
		if err != nil {
			return nil, err
		}

		fk.OnUpdate = parseReferenceAction(updateRule)
		fk.OnDelete = parseReferenceAction(deleteRule)

		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, rows.Err()
}

// getIndexes retrieves index information.
// NOTE: This only returns standalone indexes. Indexes that back constraints
// (UNIQUE, PRIMARY KEY) are managed through the constraints themselves and
// should not be dropped with DROP INDEX.
func (i *Introspector) getIndexes(ctx context.Context, tableName string) ([]schema.IndexMetadata, error) {
	// This query retrieves comprehensive index information including:
	// - Expression indexes
	// - Partial indexes (WHERE clause)
	// - INCLUDE columns (covering indexes)
	// Note: We use pg_get_indexdef() to extract column info instead of indkey/indoption
	// because int2vector types are problematic to scan with pgx.
	query := `
		SELECT
			i.relname as index_name,
			am.amname as index_type,
			ix.indisunique as is_unique,
			pg_get_indexdef(ix.indexrelid) as index_def,
			pg_get_expr(ix.indpred, ix.indrelid) as predicate,
			ix.indexprs IS NOT NULL as is_expression
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_am am ON i.relam = am.oid
		LEFT JOIN pg_constraint c ON c.conindid = ix.indexrelid
		WHERE t.relname = $1
			AND t.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'public')
			AND NOT ix.indisprimary
			AND c.conindid IS NULL  -- Exclude constraint-backed indexes
		ORDER BY i.relname
	`

	rows, err := i.query(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []schema.IndexMetadata
	for rows.Next() {
		var indexName, indexType, indexDef string
		var isUnique, isExpression bool
		var predicate *string

		err := rows.Scan(
			&indexName,
			&indexType,
			&isUnique,
			&indexDef,
			&predicate,
			&isExpression,
		)
		if err != nil {
			return nil, err
		}

		// Parse the index definition to extract detailed information
		idx, err := i.parseIndexDefinition(tableName, indexName, indexType, isUnique, indexDef, predicate, isExpression)
		if err != nil {
			// If parsing fails, create a basic index structure
			idx = &schema.IndexMetadata{
				Name:   indexName,
				Type:   indexType,
				Unique: isUnique,
			}
		}

		// Add WHERE clause if it's a partial index
		if predicate != nil && *predicate != "" {
			idx.Where = *predicate
		}

		indexes = append(indexes, *idx)
	}

	return indexes, rows.Err()
}

// getConstraints retrieves CHECK and UNIQUE constraint information.
func (i *Introspector) getConstraints(ctx context.Context, tableName string) ([]schema.ConstraintMetadata, error) {
	query := `
		SELECT
			con.conname as constraint_name,
			con.contype as constraint_type,
			pg_get_constraintdef(con.oid) as constraint_def,
			ARRAY(
				SELECT a.attname
				FROM unnest(con.conkey) AS u(attnum)
				JOIN pg_attribute AS a ON a.attrelid = con.conrelid AND a.attnum = u.attnum
				ORDER BY u.attnum
			) as column_names
		FROM pg_constraint con
		JOIN pg_class rel ON rel.oid = con.conrelid
		JOIN pg_namespace nsp ON nsp.oid = connamespace
		WHERE nsp.nspname = 'public'
			AND rel.relname = $1
			AND con.contype IN ('c', 'u')
	`

	rows, err := i.query(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints []schema.ConstraintMetadata
	for rows.Next() {
		var c schema.ConstraintMetadata
		var contype byte
		var columnNames []string

		err := rows.Scan(&c.Name, &contype, &c.Expression, &columnNames)
		if err != nil {
			return nil, err
		}

		// Set constraint type
		switch contype {
		case 'c':
			c.Type = schema.CheckConstraint
		case 'u':
			c.Type = schema.UniqueConstraint
			c.Columns = columnNames
		}

		constraints = append(constraints, c)
	}

	return constraints, rows.Err()
}

// Helper functions

// buildSQLType constructs the SQL type string from column metadata.
func buildSQLType(dataType, udtName string, maxLength, precision, scale *int) string {
	switch dataType {
	case "character varying":
		if maxLength != nil {
			return fmt.Sprintf("varchar(%d)", *maxLength)
		}
		return "varchar"
	case "character":
		if maxLength != nil {
			return fmt.Sprintf("char(%d)", *maxLength)
		}
		return "char"
	case "numeric", "decimal":
		if precision != nil && scale != nil {
			return fmt.Sprintf("numeric(%d,%d)", *precision, *scale)
		}
		return "numeric"
	case "ARRAY":
		// Array types use udt_name with leading underscore
		if strings.HasPrefix(udtName, "_") {
			baseType := udtName[1:]
			return baseType + "[]"
		}
		return udtName
	case "USER-DEFINED":
		return udtName
	default:
		return dataType
	}
}

// parseReferenceAction converts PostgreSQL rule to ReferenceAction.
func parseReferenceAction(rule string) schema.ReferenceAction {
	switch strings.ToUpper(rule) {
	case "CASCADE":
		return schema.Cascade
	case "SET NULL":
		return schema.SetNull
	case "SET DEFAULT":
		return schema.SetDefault
	case "RESTRICT":
		return schema.Restrict
	default:
		return schema.NoAction
	}
}

// getEnumTypes retrieves enum types used by columns in this table.
func (i *Introspector) getEnumTypes(ctx context.Context, tableName string) ([]schema.EnumType, error) {
	query := `
		SELECT DISTINCT
			t.typname as enum_name,
			array_agg(e.enumlabel ORDER BY e.enumsortorder) as enum_values
		FROM pg_type t
		JOIN pg_enum e ON t.oid = e.enumtypid
		JOIN pg_class c ON c.oid = t.oid
		WHERE t.typname IN (
			SELECT udt_name
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = $1
			  AND data_type = 'USER-DEFINED'
		)
		GROUP BY t.typname
		ORDER BY t.typname
	`

	rows, err := i.query(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var enumTypes []schema.EnumType
	for rows.Next() {
		var enumType schema.EnumType
		var values []string

		err := rows.Scan(&enumType.Name, &values)
		if err != nil {
			return nil, err
		}

		enumType.Values = values
		enumTypes = append(enumTypes, enumType)
	}

	return enumTypes, rows.Err()
}

// parseIndexDefinition parses the output of pg_get_indexdef() to extract index components.
// Example input: "CREATE INDEX idx_email ON users USING btree (email varchar_pattern_ops COLLATE \"C\" DESC NULLS LAST)"
func (i *Introspector) parseIndexDefinition(tableName, indexName, indexType string, isUnique bool, indexDef string, predicate *string, isExpression bool) (*schema.IndexMetadata, error) {
	idx := &schema.IndexMetadata{
		Name:   indexName,
		Type:   indexType,
		Unique: isUnique,
	}

	// Extract column list - find first '(' after table name and matching ')'
	// Note: Index definition may include schema name (e.g., "ON public.refresh_tokens")
	// Try with schema-qualified name first, then fall back to unqualified
	tableNamePos := strings.Index(indexDef, " ON public."+tableName)
	if tableNamePos == -1 {
		// Try without schema qualifier
		tableNamePos = strings.Index(indexDef, " ON "+tableName)
		if tableNamePos == -1 {
			return idx, nil // Fallback to basic index
		}
	}

	// Find the column list start (after USING clause if present, or after table name)
	startPos := strings.Index(indexDef[tableNamePos:], "(")
	if startPos == -1 {
		return idx, nil
	}
	startPos += tableNamePos

	// Extract balanced parentheses for column list
	columnList, remaining := extractBalancedParens(indexDef[startPos+1:])
	if columnList == "" {
		return idx, nil
	}

	// Check for INCLUDE clause in remaining string
	if strings.Contains(remaining, "INCLUDE") {
		_, after, _ := strings.Cut(remaining, "INCLUDE")
		includeRest := after // Skip "INCLUDE"
		includeRest = strings.TrimSpace(includeRest)
		if strings.HasPrefix(includeRest, "(") {
			includeCols, _ := extractBalancedParens(includeRest[1:])
			// Split include columns by comma
			for col := range strings.SplitSeq(includeCols, ",") {
				col = strings.TrimSpace(col)
				if col != "" {
					idx.Include = append(idx.Include, col)
				}
			}
		}
	}

	// For expression indexes, store the entire expression
	if isExpression {
		idx.Expression = strings.TrimSpace(columnList)
		return idx, nil
	}

	// Parse column list - split by commas (but not inside nested parentheses)
	columns, orderings := parseIndexColumnList(columnList)
	idx.Columns = columns
	idx.ColumnOrdering = orderings

	return idx, nil
}

// extractBalancedParens extracts content within balanced parentheses.
// Returns the content and the remaining string after the closing parenthesis.
func extractBalancedParens(s string) (content, remaining string) {
	depth := 0
	for i, ch := range s {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == -1 {
				return s[:i], s[i+1:]
			}
		}
	}
	return s, "" // If no closing paren found, return entire string
}

// parseIndexColumnList parses a comma-separated list of columns with modifiers.
// Example: "email varchar_pattern_ops COLLATE \"C\" DESC NULLS LAST, name, created_at"
func parseIndexColumnList(columnList string) ([]string, []schema.ColumnOrder) {
	var columns []string
	var orderings []schema.ColumnOrder

	// Split by commas, but respect nested parentheses (for expressions)
	parts := splitRespectingParens(columnList, ',')

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Parse individual column with modifiers
		colName, ordering := parseIndexColumn(part)
		columns = append(columns, colName)

		// Only add ordering if it has non-default values (not just the column name)
		hasNonDefaultOrdering := ordering.Direction == schema.Descending ||
			ordering.Nulls != "" ||
			ordering.OpClass != "" ||
			ordering.Collation != ""
		if hasNonDefaultOrdering {
			orderings = append(orderings, ordering)
		}
	}

	return columns, orderings
}

// parseIndexColumn parses a single column definition with modifiers.
// Example: "email varchar_pattern_ops COLLATE \"C\" DESC NULLS LAST"
func parseIndexColumn(s string) (string, schema.ColumnOrder) {
	order := schema.ColumnOrder{}

	// Tokenize the string, respecting quoted strings
	tokens := tokenizeIndexColumn(s)
	if len(tokens) == 0 {
		return "", order
	}

	// First token is the column name
	order.Column = tokens[0]
	columnName := tokens[0]

	// Parse remaining tokens
	for i := 1; i < len(tokens); i++ {
		token := strings.ToUpper(tokens[i])

		switch token {
		case "DESC":
			order.Direction = schema.Descending
		case "ASC":
			order.Direction = schema.Ascending
		case "NULLS":
			if i+1 < len(tokens) {
				nullsToken := strings.ToUpper(tokens[i+1])
				if nullsToken == "FIRST" {
					order.Nulls = schema.NullsFirst
					i++
				} else if nullsToken == "LAST" {
					order.Nulls = schema.NullsLast
					i++
				}
			}
		case "COLLATE":
			if i+1 < len(tokens) {
				// Remove quotes from collation if present
				collation := tokens[i+1]
				collation = strings.Trim(collation, "\"")
				order.Collation = collation
				i++
			}
		default:
			// Check if this is an operator class (not a reserved keyword)
			if !isReservedIndexKeyword(token) && order.OpClass == "" {
				order.OpClass = tokens[i] // Use original case for operator class
			}
		}
	}

	return columnName, order
}

// tokenizeIndexColumn splits a column definition into tokens, respecting quoted strings.
// Example: "email varchar_pattern_ops COLLATE \"en_US\" DESC" -> ["email", "varchar_pattern_ops", "COLLATE", "\"en_US\"", "DESC"]
func tokenizeIndexColumn(s string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	quoteChar := rune(0)

	for _, ch := range s {
		switch ch {
		case '"', '\'':
			if inQuotes && ch == quoteChar {
				current.WriteRune(ch)
				tokens = append(tokens, current.String())
				current.Reset()
				inQuotes = false
				quoteChar = 0
			} else if !inQuotes {
				inQuotes = true
				quoteChar = ch
				current.WriteRune(ch)
			} else {
				current.WriteRune(ch)
			}
		case ' ', '\t', '\n':
			if inQuotes {
				current.WriteRune(ch)
			} else if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// isReservedIndexKeyword checks if a token is a reserved PostgreSQL index keyword.
func isReservedIndexKeyword(token string) bool {
	reserved := map[string]bool{
		"ASC":     true,
		"DESC":    true,
		"NULLS":   true,
		"FIRST":   true,
		"LAST":    true,
		"COLLATE": true,
		"USING":   true,
		"WHERE":   true,
		"INCLUDE": true,
	}
	return reserved[strings.ToUpper(token)]
}

// splitRespectingParens splits a string by delimiter, but respects nested parentheses.
func splitRespectingParens(s string, delim rune) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for _, ch := range s {
		switch ch {
		case '(':
			depth++
			current.WriteRune(ch)
		case ')':
			depth--
			current.WriteRune(ch)
		case delim:
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
