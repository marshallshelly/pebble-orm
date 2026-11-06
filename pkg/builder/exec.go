package builder

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// queryExecutor is the shared execution surface used by every query builder.
// It is satisfied by *runtime.DB (the pool) directly and by pgx.Tx through
// txExecutor, so the SELECT/INSERT/UPDATE/DELETE build and execution logic is
// written once and reused by both the connection-pool and transaction paths.
type queryExecutor interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, sql string, args ...interface{}) (int64, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// txExecutor adapts a pgx.Tx to queryExecutor, converting the CommandTag from
// Exec into the affected-row count the builders expose.
type txExecutor struct{ tx pgx.Tx }

func (t txExecutor) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return t.tx.Query(ctx, sql, args...)
}

func (t txExecutor) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

func (t txExecutor) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return t.tx.SendBatch(ctx, b)
}

func (t txExecutor) Exec(ctx context.Context, sql string, args ...interface{}) (int64, error) {
	tag, err := t.tx.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ---- Shared SQL builders -------------------------------------------------

type selectSpec struct {
	table     *schema.TableMetadata
	distinct  bool
	columns   []string
	joins     []Join
	where     []Condition
	groupBy   []string
	having    []Condition
	orderBy   []OrderBy
	limit     *int
	offset    *int
	forUpdate bool
}

// buildSelectSQL assembles a SELECT statement with sequential placeholder
// numbering across JOIN, WHERE and HAVING clauses.
func buildSelectSQL(s selectSpec) (string, []interface{}, error) {
	if s.table == nil {
		return "", nil, fmt.Errorf("table metadata not available")
	}

	var sql strings.Builder
	var args []interface{}
	paramNum := 1

	sql.WriteString("SELECT ")
	if s.distinct {
		sql.WriteString("DISTINCT ")
	}
	if len(s.columns) == 0 || (len(s.columns) == 1 && s.columns[0] == "*") {
		sql.WriteString("*")
	} else {
		sql.WriteString(strings.Join(s.columns, ", "))
	}

	sql.WriteString(" FROM ")
	sql.WriteString(schema.QuoteReservedIdent(s.table.Name))

	// JOINs: each join condition's own $1.. are renumbered to the running
	// parameter position so args never collide across clauses.
	for _, join := range s.joins {
		sql.WriteString(" ")
		sql.WriteString(string(join.Type))
		sql.WriteString(" ")
		if join.Lateral {
			sql.WriteString("LATERAL ")
		}
		sql.WriteString(join.Table)
		sql.WriteString(" ON ")
		sql.WriteString(shiftPlaceholders(join.Condition, paramNum-1))
		for _, arg := range join.Args {
			args = append(args, arg)
			paramNum++
		}
	}

	// WHERE, numbered continuing from join args.
	if len(s.where) > 0 {
		wb := NewWhereBuilderWithStart(paramNum)
		wb.conditions = s.where
		whereSQL, whereArgs, err := wb.Build()
		if err != nil {
			return "", nil, fmt.Errorf("failed to build WHERE clause: %w", err)
		}
		if whereSQL != "" {
			sql.WriteString(" ")
			sql.WriteString(whereSQL)
			args = append(args, whereArgs...)
			paramNum += len(whereArgs)
		}
	}

	if len(s.groupBy) > 0 {
		sql.WriteString(" GROUP BY ")
		sql.WriteString(strings.Join(s.groupBy, ", "))
	}

	// HAVING, numbered continuing from WHERE args.
	if len(s.having) > 0 {
		hb := NewWhereBuilderWithStart(paramNum)
		hb.conditions = s.having
		havingSQL, havingArgs, err := hb.Build()
		if err != nil {
			return "", nil, fmt.Errorf("failed to build HAVING clause: %w", err)
		}
		if havingSQL != "" {
			havingSQL = strings.Replace(havingSQL, "WHERE", "HAVING", 1)
			sql.WriteString(" ")
			sql.WriteString(havingSQL)
			args = append(args, havingArgs...)
			paramNum += len(havingArgs)
		}
	}

	if len(s.orderBy) > 0 {
		sql.WriteString(" ORDER BY ")
		parts := make([]string, len(s.orderBy))
		for i, order := range s.orderBy {
			parts[i] = order.Column + " " + string(order.Direction)
			if order.NullsPos != NullsDefault {
				parts[i] += " " + string(order.NullsPos)
			}
		}
		sql.WriteString(strings.Join(parts, ", "))
	}

	if s.limit != nil {
		fmt.Fprintf(&sql, " LIMIT %d", *s.limit)
	}
	if s.offset != nil {
		fmt.Fprintf(&sql, " OFFSET %d", *s.offset)
	}
	if s.forUpdate {
		sql.WriteString(" FOR UPDATE")
	}

	return sql.String(), args, nil
}

// buildCountSQL assembles a SELECT COUNT(*) statement with an optional WHERE.
func buildCountSQL(table *schema.TableMetadata, where []Condition) (string, []interface{}, error) {
	if table == nil {
		return "", nil, fmt.Errorf("table metadata not available")
	}
	var sql strings.Builder
	sql.WriteString("SELECT COUNT(*) FROM ")
	sql.WriteString(schema.QuoteReservedIdent(table.Name))

	var args []interface{}
	if len(where) > 0 {
		wb := NewWhereBuilder()
		wb.conditions = where
		whereSQL, whereArgs, err := wb.Build()
		if err != nil {
			return "", nil, err
		}
		if whereSQL != "" {
			sql.WriteString(" ")
			sql.WriteString(whereSQL)
			args = append(args, whereArgs...)
		}
	}
	return sql.String(), args, nil
}

type insertSpec struct {
	table      *schema.TableMetadata
	rows       []interface{}
	returning  []string
	onConflict *OnConflict
}

// buildInsertSQL assembles a multi-row INSERT. Each row is reduced to the
// columns it would insert on its own (zero values on defaulted columns are
// left out); the column list is every column some row sets, in table order,
// and a row that leaves one out gets DEFAULT there. A row therefore stores the
// same values in a batch as it would alone, whatever order the rows come in.
func buildInsertSQL(s insertSpec) (string, []interface{}, error) {
	if s.table == nil {
		return "", nil, fmt.Errorf("table metadata not available")
	}
	if len(s.rows) == 0 {
		return "", nil, fmt.Errorf("no values to insert")
	}

	var sql strings.Builder
	var args []interface{}
	paramNum := 1

	sql.WriteString("INSERT INTO ")
	sql.WriteString(schema.QuoteReservedIdent(s.table.Name))

	rowValues := make([]map[string]interface{}, len(s.rows))
	used := make(map[string]bool)
	for i, row := range s.rows {
		if t := reflect.TypeOf(row); t != s.table.GoType && (t == nil || t.Kind() != reflect.Pointer || t.Elem() != s.table.GoType) {
			return "", nil, fmt.Errorf("row %d is %T, not %s", i, row, s.table.GoType)
		}
		cols, vals, err := structToValues(row, s.table, true)
		if err != nil {
			return "", nil, fmt.Errorf("failed to extract values from row %d: %w", i, err)
		}
		rowValues[i] = make(map[string]interface{}, len(cols))
		for j, col := range cols {
			rowValues[i][col] = vals[j]
			used[col] = true
		}
	}
	var columns []string
	for _, col := range s.table.Columns {
		if used[col.Name] {
			columns = append(columns, col.Name)
		}
	}

	sql.WriteString(" (")
	sql.WriteString(strings.Join(schema.QuoteReservedIdents(columns), ", "))
	sql.WriteString(") VALUES ")

	valueClauses := make([]string, len(s.rows))
	for i, values := range rowValues {
		placeholders := make([]string, len(columns))
		for j, col := range columns {
			value, ok := values[col]
			if !ok {
				placeholders[j] = "DEFAULT"
				continue
			}
			placeholders[j] = fmt.Sprintf("$%d", paramNum)
			paramNum++
			args = append(args, value)
		}
		valueClauses[i] = "(" + strings.Join(placeholders, ", ") + ")"
	}
	sql.WriteString(strings.Join(valueClauses, ", "))

	if s.onConflict != nil {
		sql.WriteString(" ON CONFLICT")
		if len(s.onConflict.Columns) > 0 {
			sql.WriteString(" (")
			sql.WriteString(strings.Join(s.onConflict.Columns, ", "))
			sql.WriteString(")")
		}
		if s.onConflict.Action == DoNothing {
			sql.WriteString(" DO NOTHING")
		} else if s.onConflict.Action == DoUpdate {
			sql.WriteString(" ")
			sql.WriteString(string(DoUpdate))
			if len(s.onConflict.Updates) > 0 {
				updates := make([]string, 0, len(s.onConflict.Updates))
				for _, col := range slices.Sorted(maps.Keys(s.onConflict.Updates)) {
					updates = append(updates, fmt.Sprintf("%s = $%d", schema.QuoteReservedIdent(col), paramNum))
					paramNum++
					args = append(args, s.onConflict.Updates[col])
				}
				sql.WriteString(" ")
				sql.WriteString(strings.Join(updates, ", "))
			}
		}
	}

	if len(s.returning) > 0 {
		sql.WriteString(" RETURNING ")
		sql.WriteString(strings.Join(s.returning, ", "))
	}

	return sql.String(), args, nil
}

type updateSpec struct {
	table     *schema.TableMetadata
	sets      map[string]interface{}
	where     []Condition
	returning []string
}

// buildUpdateSQL assembles an UPDATE with SET assignments numbered before WHERE.
func buildUpdateSQL(s updateSpec) (string, []interface{}, error) {
	if s.table == nil {
		return "", nil, fmt.Errorf("table metadata not available")
	}
	if len(s.sets) == 0 {
		return "", nil, fmt.Errorf("no columns to update")
	}

	var sql strings.Builder
	var args []interface{}
	paramNum := 1

	sql.WriteString("UPDATE ")
	sql.WriteString(schema.QuoteReservedIdent(s.table.Name))
	sql.WriteString(" SET ")

	// Sorted, so one call shape is one statement text: in pgx's default mode
	// every distinct text is prepared separately on each connection.
	setClauses := make([]string, 0, len(s.sets))
	for _, col := range slices.Sorted(maps.Keys(s.sets)) {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", schema.QuoteReservedIdent(col), paramNum))
		args = append(args, s.sets[col])
		paramNum++
	}
	sql.WriteString(strings.Join(setClauses, ", "))

	if len(s.where) > 0 {
		wb := NewWhereBuilderWithStart(paramNum)
		wb.conditions = s.where
		whereSQL, whereArgs, err := wb.Build()
		if err != nil {
			return "", nil, fmt.Errorf("failed to build WHERE clause: %w", err)
		}
		if whereSQL != "" {
			sql.WriteString(" ")
			sql.WriteString(whereSQL)
			args = append(args, whereArgs...)
		}
	}

	if len(s.returning) > 0 {
		sql.WriteString(" RETURNING ")
		sql.WriteString(strings.Join(s.returning, ", "))
	}

	return sql.String(), args, nil
}

type deleteSpec struct {
	table     *schema.TableMetadata
	where     []Condition
	returning []string
}

// buildDeleteSQL assembles a DELETE statement.
func buildDeleteSQL(s deleteSpec) (string, []interface{}, error) {
	if s.table == nil {
		return "", nil, fmt.Errorf("table metadata not available")
	}

	var sql strings.Builder
	var args []interface{}

	sql.WriteString("DELETE FROM ")
	sql.WriteString(schema.QuoteReservedIdent(s.table.Name))

	if len(s.where) > 0 {
		wb := NewWhereBuilder()
		wb.conditions = s.where
		whereSQL, whereArgs, err := wb.Build()
		if err != nil {
			return "", nil, fmt.Errorf("failed to build WHERE clause: %w", err)
		}
		if whereSQL != "" {
			sql.WriteString(" ")
			sql.WriteString(whereSQL)
			args = append(args, whereArgs...)
		}
	}

	if len(s.returning) > 0 {
		sql.WriteString(" RETURNING ")
		sql.WriteString(strings.Join(s.returning, ", "))
	}

	return sql.String(), args, nil
}

// ---- Shared execution ----------------------------------------------------

// toAnySlice converts a typed slice to []interface{} for the shared builders.
func toAnySlice[T any](values []T) []interface{} {
	out := make([]interface{}, len(values))
	for i := range values {
		out[i] = values[i]
	}
	return out
}

// queryRows scans every row of the query into a []T, then loads any preloads
// through the same executor (so it works inside a transaction). Result rows are
// closed before preload queries, which a single-connection transaction requires.
func queryRows[T any](ctx context.Context, exec queryExecutor, table *schema.TableMetadata, sqlStr string, args []interface{}, preloads []string) ([]T, error) {
	rows, err := exec.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []T
	scanner := newRowScanner(table)
	for rows.Next() {
		var item T
		if err := scanner.scan(rows, &item); err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(preloads) > 0 && len(results) > 0 {
		rows.Close()
		loader := &relationshipLoader{exec: exec, table: table, preloads: preloads}
		if err := loader.loadRelationships(ctx, &results); err != nil {
			return nil, err
		}
	}
	return results, nil
}

// execWrite runs a write statement, returning the affected/returned row count.
func execWrite(ctx context.Context, exec queryExecutor, sqlStr string, args []interface{}, hasReturning bool) (int64, error) {
	if !hasReturning {
		return exec.Exec(ctx, sqlStr, args...)
	}
	rows, err := exec.Query(ctx, sqlStr, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var count int64
	for rows.Next() {
		count++
	}
	return count, rows.Err()
}

// queryCount runs a COUNT(*) statement.
func queryCount(ctx context.Context, exec queryExecutor, sqlStr string, args []interface{}) (int64, error) {
	var count int64
	if err := exec.QueryRow(ctx, sqlStr, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
