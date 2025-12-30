package migration

import (
	"context"
	"fmt"
	"strings"

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

	rows, err := i.pool.Query(ctx, query)
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

	rows, err := i.pool.Query(ctx, query, tableName)
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

	var name string
	var columns []string

	err := i.pool.QueryRow(ctx, query, tableName).Scan(&name, &columns)
	if err != nil {
		// No primary key is not an error
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
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

	rows, err := i.pool.Query(ctx, query, tableName)
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
	query := `
		SELECT
			i.relname as index_name,
			array_agg(a.attname ORDER BY x.ordinality) as columns,
			ix.indisunique as is_unique,
			am.amname as index_type
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_am am ON i.relam = am.oid
		CROSS JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS x(attnum, ordinality)
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = x.attnum
		LEFT JOIN pg_constraint c ON c.conindid = ix.indexrelid
		WHERE t.relname = $1
			AND t.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'public')
			AND NOT ix.indisprimary
			AND c.conindid IS NULL  -- Exclude constraint-backed indexes
		GROUP BY i.relname, ix.indisunique, am.amname
		ORDER BY i.relname
	`

	rows, err := i.pool.Query(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []schema.IndexMetadata
	for rows.Next() {
		var idx schema.IndexMetadata

		err := rows.Scan(
			&idx.Name,
			&idx.Columns,
			&idx.Unique,
			&idx.Type,
		)
		if err != nil {
			return nil, err
		}

		indexes = append(indexes, idx)
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

	rows, err := i.pool.Query(ctx, query, tableName)
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
