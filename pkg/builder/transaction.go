package builder

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// Tx wraps a pgx transaction and provides query builder methods.
type Tx struct {
	tx  pgx.Tx
	ctx context.Context
}

// Begin starts a new transaction.
func (d *DB) Begin(ctx context.Context) (*Tx, error) {
	tx, err := d.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &Tx{tx: tx, ctx: ctx}, nil
}

// BeginTx starts a new transaction with custom options.
func (d *DB) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (*Tx, error) {
	tx, err := d.db.BeginTx(ctx, txOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &Tx{tx: tx, ctx: ctx}, nil
}

// Commit commits the transaction.
func (t *Tx) Commit() error {
	if err := t.tx.Commit(t.ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Rollback rolls back the transaction.
func (t *Tx) Rollback() error {
	if err := t.tx.Rollback(t.ctx); err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	return nil
}

// Savepoint creates a savepoint within the transaction.
func (t *Tx) Savepoint(name string) error {
	_, err := t.tx.Exec(t.ctx, fmt.Sprintf("SAVEPOINT %s", name))
	if err != nil {
		return fmt.Errorf("failed to create savepoint %s: %w", name, err)
	}
	return nil
}

// RollbackToSavepoint rolls back to a savepoint.
func (t *Tx) RollbackToSavepoint(name string) error {
	_, err := t.tx.Exec(t.ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", name))
	if err != nil {
		return fmt.Errorf("failed to rollback to savepoint %s: %w", name, err)
	}
	return nil
}

// ReleaseSavepoint releases a savepoint.
func (t *Tx) ReleaseSavepoint(name string) error {
	_, err := t.tx.Exec(t.ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", name))
	if err != nil {
		return fmt.Errorf("failed to release savepoint %s: %w", name, err)
	}
	return nil
}

// TxSelect creates a new type-safe SELECT query within the transaction.
// Usage: builder.TxSelect[User](tx).Where(...).All()
func TxSelect[T any](t *Tx) *TxSelectQuery[T] {
	var model T

	table, err := registry.GetOrRegister(model)
	if err != nil {
		return &TxSelectQuery[T]{
			tx:    t,
			table: nil,
		}
	}

	return &TxSelectQuery[T]{
		tx:       t,
		table:    table,
		columns:  []string{"*"},
		where:    make([]Condition, 0),
		joins:    make([]Join, 0),
		groupBy:  make([]string, 0),
		having:   make([]Condition, 0),
		orderBy:  make([]OrderBy, 0),
		preloads: make([]string, 0),
	}
}

// Select creates a new SELECT query within the transaction.
func (t *Tx) Select(model interface{}) *TxSelectQuery[interface{}] {
	return t.SelectTyped(model)
}

// SelectTyped creates a type-safe SELECT query within the transaction.
func (t *Tx) SelectTyped(model interface{}) *TxSelectQuery[interface{}] {
	table, err := registry.GetOrRegister(model)
	if err != nil {
		return &TxSelectQuery[interface{}]{
			tx:    t,
			table: nil,
		}
	}

	return &TxSelectQuery[interface{}]{
		tx:       t,
		table:    table,
		columns:  []string{"*"},
		where:    make([]Condition, 0),
		joins:    make([]Join, 0),
		groupBy:  make([]string, 0),
		having:   make([]Condition, 0),
		orderBy:  make([]OrderBy, 0),
		preloads: make([]string, 0),
	}
}

// TxInsert creates a new type-safe INSERT query within the transaction.
// Usage: builder.TxInsert[User](tx).Values(user).ExecReturning()
func TxInsert[T any](t *Tx) *TxInsertQuery[T] {
	var model T

	table, err := registry.GetOrRegister(model)
	if err != nil {
		return &TxInsertQuery[T]{
			tx:    t,
			table: nil,
		}
	}

	return &TxInsertQuery[T]{
		tx:        t,
		table:     table,
		values:    make([]interface{}, 0),
		returning: make([]string, 0),
	}
}

// Insert creates a new INSERT query within the transaction.
func (t *Tx) Insert(model interface{}) *TxInsertQuery[interface{}] {
	return t.InsertTyped(model)
}

// InsertTyped creates a type-safe INSERT query within the transaction.
func (t *Tx) InsertTyped(model interface{}) *TxInsertQuery[interface{}] {
	table, err := registry.GetOrRegister(model)
	if err != nil {
		return &TxInsertQuery[interface{}]{
			tx:    t,
			table: nil,
		}
	}

	return &TxInsertQuery[interface{}]{
		tx:        t,
		table:     table,
		values:    make([]interface{}, 0),
		returning: make([]string, 0),
	}
}

// TxUpdate creates a new type-safe UPDATE query within the transaction.
// Usage: builder.TxUpdate[User](tx).Set("name", "John").Where(...).Exec()
func TxUpdate[T any](t *Tx) *TxUpdateQuery[T] {
	var model T

	table, err := registry.GetOrRegister(model)
	if err != nil {
		return &TxUpdateQuery[T]{
			tx:    t,
			table: nil,
		}
	}

	return &TxUpdateQuery[T]{
		tx:        t,
		table:     table,
		sets:      make(map[string]interface{}),
		where:     make([]Condition, 0),
		returning: make([]string, 0),
	}
}

// Update creates a new UPDATE query within the transaction.
func (t *Tx) Update(model interface{}) *TxUpdateQuery[interface{}] {
	return t.UpdateTyped(model)
}

// UpdateTyped creates a type-safe UPDATE query within the transaction.
func (t *Tx) UpdateTyped(model interface{}) *TxUpdateQuery[interface{}] {
	table, err := registry.GetOrRegister(model)
	if err != nil {
		return &TxUpdateQuery[interface{}]{
			tx:    t,
			table: nil,
		}
	}

	return &TxUpdateQuery[interface{}]{
		tx:        t,
		table:     table,
		sets:      make(map[string]interface{}),
		where:     make([]Condition, 0),
		returning: make([]string, 0),
	}
}

// TxDelete creates a new type-safe DELETE query within the transaction.
// Usage: builder.TxDelete[User](tx).Where(...).Exec()
func TxDelete[T any](t *Tx) *TxDeleteQuery[T] {
	var model T

	table, err := registry.GetOrRegister(model)
	if err != nil {
		return &TxDeleteQuery[T]{
			tx:    t,
			table: nil,
		}
	}

	return &TxDeleteQuery[T]{
		tx:        t,
		table:     table,
		where:     make([]Condition, 0),
		returning: make([]string, 0),
	}
}

// Delete creates a new DELETE query within the transaction.
func (t *Tx) Delete(model interface{}) *TxDeleteQuery[interface{}] {
	return t.DeleteTyped(model)
}

// DeleteTyped creates a type-safe DELETE query within the transaction.
func (t *Tx) DeleteTyped(model interface{}) *TxDeleteQuery[interface{}] {
	table, err := registry.GetOrRegister(model)
	if err != nil {
		return &TxDeleteQuery[interface{}]{
			tx:    t,
			table: nil,
		}
	}

	return &TxDeleteQuery[interface{}]{
		tx:        t,
		table:     table,
		where:     make([]Condition, 0),
		returning: make([]string, 0),
	}
}

// TxSelectQuery represents a SELECT query within a transaction.
type TxSelectQuery[T any] struct {
	tx        *Tx
	table     *schema.TableMetadata
	columns   []string
	where     []Condition
	joins     []Join
	groupBy   []string
	having    []Condition
	orderBy   []OrderBy
	limit     *int
	offset    *int
	distinct  bool
	forUpdate bool
	preloads  []string // Relationship fields to eagerly load
}

// Columns specifies which columns to select.
func (q *TxSelectQuery[T]) Columns(cols ...string) *TxSelectQuery[T] {
	q.columns = cols
	return q
}

// Where adds a WHERE condition.
func (q *TxSelectQuery[T]) Where(condition Condition) *TxSelectQuery[T] {
	q.where = append(q.where, condition)
	return q
}

// And adds an AND condition.
func (q *TxSelectQuery[T]) And(condition Condition) *TxSelectQuery[T] {
	condition.Logic = LogicAnd
	return q.Where(condition)
}

// Or adds an OR condition.
func (q *TxSelectQuery[T]) Or(condition Condition) *TxSelectQuery[T] {
	condition.Logic = LogicOr
	return q.Where(condition)
}

// OrderBy adds an ORDER BY clause.
func (q *TxSelectQuery[T]) OrderBy(column string, direction OrderDirection) *TxSelectQuery[T] {
	q.orderBy = append(q.orderBy, OrderBy{
		Column:    column,
		Direction: direction,
		NullsPos:  NullsDefault,
	})
	return q
}

// Limit sets the LIMIT clause.
func (q *TxSelectQuery[T]) Limit(limit int) *TxSelectQuery[T] {
	q.limit = &limit
	return q
}

// Offset sets the OFFSET clause.
func (q *TxSelectQuery[T]) Offset(offset int) *TxSelectQuery[T] {
	q.offset = &offset
	return q
}

// Distinct adds DISTINCT to the query.
func (q *TxSelectQuery[T]) Distinct() *TxSelectQuery[T] {
	q.distinct = true
	return q
}

// ForUpdate adds FOR UPDATE lock.
func (q *TxSelectQuery[T]) ForUpdate() *TxSelectQuery[T] {
	q.forUpdate = true
	return q
}

// Preload specifies relationships to eagerly load.
// Pass the name of the Go struct field that contains the relationship.
// Example: query.Preload("Posts").Preload("Comments")
func (q *TxSelectQuery[T]) Preload(relationships ...string) *TxSelectQuery[T] {
	q.preloads = append(q.preloads, relationships...)
	return q
}

// InnerJoin adds an INNER JOIN.
func (q *TxSelectQuery[T]) InnerJoin(table string, condition string, args ...interface{}) *TxSelectQuery[T] {
	q.joins = append(q.joins, Join{
		Type:      InnerJoin,
		Table:     table,
		Condition: condition,
		Args:      args,
	})
	return q
}

// LeftJoin adds a LEFT JOIN.
func (q *TxSelectQuery[T]) LeftJoin(table string, condition string, args ...interface{}) *TxSelectQuery[T] {
	q.joins = append(q.joins, Join{
		Type:      LeftJoin,
		Table:     table,
		Condition: condition,
		Args:      args,
	})
	return q
}

// RightJoin adds a RIGHT JOIN.
func (q *TxSelectQuery[T]) RightJoin(table string, condition string, args ...interface{}) *TxSelectQuery[T] {
	q.joins = append(q.joins, Join{
		Type:      RightJoin,
		Table:     table,
		Condition: condition,
		Args:      args,
	})
	return q
}

// FullJoin adds a FULL OUTER JOIN.
func (q *TxSelectQuery[T]) FullJoin(table string, condition string, args ...interface{}) *TxSelectQuery[T] {
	q.joins = append(q.joins, Join{
		Type:      FullJoin,
		Table:     table,
		Condition: condition,
		Args:      args,
	})
	return q
}

// GroupBy adds a GROUP BY clause.
func (q *TxSelectQuery[T]) GroupBy(columns ...string) *TxSelectQuery[T] {
	q.groupBy = append(q.groupBy, columns...)
	return q
}

// Having adds a HAVING condition.
func (q *TxSelectQuery[T]) Having(condition Condition) *TxSelectQuery[T] {
	q.having = append(q.having, condition)
	return q
}

// ToSQL generates the SQL query and arguments.
func (q *TxSelectQuery[T]) ToSQL() (string, []interface{}, error) {
	if q.table == nil {
		return "", nil, fmt.Errorf("table metadata not available")
	}

	var sql strings.Builder
	var args []interface{}
	paramNum := 1

	// SELECT clause
	sql.WriteString("SELECT ")
	if q.distinct {
		sql.WriteString("DISTINCT ")
	}

	if len(q.columns) == 0 || (len(q.columns) == 1 && q.columns[0] == "*") {
		sql.WriteString("*")
	} else {
		sql.WriteString(strings.Join(q.columns, ", "))
	}

	// FROM clause
	sql.WriteString(" FROM ")
	sql.WriteString(q.table.Name)

	// JOIN clauses
	for _, join := range q.joins {
		sql.WriteString(" ")
		sql.WriteString(string(join.Type))
		sql.WriteString(" ")
		sql.WriteString(join.Table)
		sql.WriteString(" ON ")
		sql.WriteString(join.Condition)

		// Add join arguments
		for _, arg := range join.Args {
			args = append(args, arg)
			paramNum++
		}
	}

	// WHERE clause
	if len(q.where) > 0 {
		whereBuilder := NewWhereBuilder()
		whereBuilder.conditions = q.where
		whereSql, whereArgs, err := whereBuilder.Build()
		if err != nil {
			return "", nil, fmt.Errorf("failed to build WHERE clause: %w", err)
		}

		if whereSql != "" {
			sql.WriteString(" ")
			sql.WriteString(whereSql)
			args = append(args, whereArgs...)
			paramNum += len(whereArgs)
		}
	}

	// GROUP BY clause
	if len(q.groupBy) > 0 {
		sql.WriteString(" GROUP BY ")
		sql.WriteString(strings.Join(q.groupBy, ", "))
	}

	// HAVING clause
	if len(q.having) > 0 {
		whereBuilder := NewWhereBuilder()
		whereBuilder.conditions = q.having
		havingSql, havingArgs, err := whereBuilder.Build()
		if err != nil {
			return "", nil, fmt.Errorf("failed to build HAVING clause: %w", err)
		}

		if havingSql != "" {
			// Replace WHERE with HAVING
			havingSql = strings.Replace(havingSql, "WHERE", "HAVING", 1)
			sql.WriteString(" ")
			sql.WriteString(havingSql)
			args = append(args, havingArgs...)
			paramNum += len(havingArgs)
		}
	}

	// ORDER BY clause
	if len(q.orderBy) > 0 {
		sql.WriteString(" ORDER BY ")
		orderParts := make([]string, len(q.orderBy))
		for i, order := range q.orderBy {
			orderParts[i] = order.Column + " " + string(order.Direction)
			if order.NullsPos != NullsDefault {
				orderParts[i] += " " + string(order.NullsPos)
			}
		}
		sql.WriteString(strings.Join(orderParts, ", "))
	}

	// LIMIT clause
	if q.limit != nil {
		sql.WriteString(fmt.Sprintf(" LIMIT %d", *q.limit))
	}

	// OFFSET clause
	if q.offset != nil {
		sql.WriteString(fmt.Sprintf(" OFFSET %d", *q.offset))
	}

	// FOR UPDATE clause
	if q.forUpdate {
		sql.WriteString(" FOR UPDATE")
	}

	return sql.String(), args, nil
}

// All executes the query and returns all results.
func (q *TxSelectQuery[T]) All() ([]T, error) {
	sql, args, err := q.ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := q.tx.tx.Query(q.tx.ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []T
	for rows.Next() {
		var item T
		if err := scanIntoStruct(rows, &item, q.table); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// First executes the query and returns the first result.
func (q *TxSelectQuery[T]) First() (T, error) {
	q.Limit(1)

	sql, args, err := q.ToSQL()
	if err != nil {
		var zero T
		return zero, err
	}

	rows, err := q.tx.tx.Query(q.tx.ctx, sql, args...)
	if err != nil {
		var zero T
		return zero, err
	}
	defer rows.Close()

	if !rows.Next() {
		var zero T
		return zero, pgx.ErrNoRows
	}

	var result T
	if err := scanIntoStruct(rows, &result, q.table); err != nil {
		var zero T
		return zero, err
	}

	return result, nil
}

// Count executes a COUNT query.
func (q *TxSelectQuery[T]) Count() (int64, error) {
	if q.table == nil {
		return 0, fmt.Errorf("table metadata not available")
	}

	// Build COUNT query
	var sql strings.Builder
	sql.WriteString("SELECT COUNT(*) FROM ")
	sql.WriteString(q.table.Name)

	var args []interface{}

	// Add WHERE clause
	if len(q.where) > 0 {
		whereBuilder := NewWhereBuilder()
		whereBuilder.conditions = q.where
		whereSql, whereArgs, err := whereBuilder.Build()
		if err != nil {
			return 0, err
		}

		if whereSql != "" {
			sql.WriteString(" ")
			sql.WriteString(whereSql)
			args = append(args, whereArgs...)
		}
	}

	var count int64
	err := q.tx.tx.QueryRow(q.tx.ctx, sql.String(), args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Exists checks if any rows match the query.
func (q *TxSelectQuery[T]) Exists() (bool, error) {
	count, err := q.Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// TxInsertQuery represents an INSERT query within a transaction.
type TxInsertQuery[T any] struct {
	tx         *Tx
	table      *schema.TableMetadata
	values     []interface{}
	returning  []string
	onConflict *OnConflict
}

// Values adds values to insert.
func (q *TxInsertQuery[T]) Values(values ...interface{}) *TxInsertQuery[T] {
	q.values = append(q.values, values...)
	return q
}

// Returning specifies columns to return.
func (q *TxInsertQuery[T]) Returning(columns ...string) *TxInsertQuery[T] {
	q.returning = append(q.returning, columns...)
	return q
}

// OnConflictDoNothing adds ON CONFLICT DO NOTHING clause.
func (q *TxInsertQuery[T]) OnConflictDoNothing(columns ...string) *TxInsertQuery[T] {
	q.onConflict = &OnConflict{
		Columns: columns,
		Action:  DoNothing,
	}
	return q
}

// OnConflictDoUpdate adds ON CONFLICT DO UPDATE clause.
func (q *TxInsertQuery[T]) OnConflictDoUpdate(columns []string, updates map[string]interface{}) *TxInsertQuery[T] {
	q.onConflict = &OnConflict{
		Columns: columns,
		Action:  DoUpdate,
		Updates: updates,
	}
	return q
}

// ToSQL generates the INSERT SQL and arguments.
func (q *TxInsertQuery[T]) ToSQL() (string, []interface{}, error) {
	if q.table == nil {
		return "", nil, fmt.Errorf("table metadata not available")
	}

	if len(q.values) == 0 {
		return "", nil, fmt.Errorf("no values to insert")
	}

	var sql strings.Builder
	var args []interface{}
	paramNum := 1

	sql.WriteString("INSERT INTO ")
	sql.WriteString(q.table.Name)

	// Get columns and values from the first row
	columns, firstRowValues, err := structToValues(q.values[0], q.table, true)
	if err != nil {
		return "", nil, fmt.Errorf("failed to extract values: %w", err)
	}

	// Column names
	sql.WriteString(" (")
	sql.WriteString(strings.Join(columns, ", "))
	sql.WriteString(") VALUES ")

	// Values for all rows
	valueClauses := make([]string, len(q.values))
	for i, val := range q.values {
		var rowValues []interface{}

		if i == 0 {
			rowValues = firstRowValues
		} else {
			_, rowValues, err = structToValues(val, q.table, true)
			if err != nil {
				return "", nil, fmt.Errorf("failed to extract values from row %d: %w", i, err)
			}
		}

		// Build placeholders for this row
		placeholders := make([]string, len(rowValues))
		for j := range rowValues {
			placeholders[j] = fmt.Sprintf("$%d", paramNum)
			paramNum++
			args = append(args, rowValues[j])
		}

		valueClauses[i] = "(" + strings.Join(placeholders, ", ") + ")"
	}

	sql.WriteString(strings.Join(valueClauses, ", "))

	// ON CONFLICT clause
	if q.onConflict != nil {
		sql.WriteString(" ON CONFLICT")

		if len(q.onConflict.Columns) > 0 {
			sql.WriteString(" (")
			sql.WriteString(strings.Join(q.onConflict.Columns, ", "))
			sql.WriteString(")")
		}

		if q.onConflict.Action == DoNothing {
			sql.WriteString(" DO NOTHING")
		} else if q.onConflict.Action == DoUpdate {
			sql.WriteString(" ")
			sql.WriteString(string(DoUpdate))

			if len(q.onConflict.Updates) > 0 {
				updates := make([]string, 0, len(q.onConflict.Updates))
				for col, val := range q.onConflict.Updates {
					updates = append(updates, fmt.Sprintf("%s = $%d", col, paramNum))
					paramNum++
					args = append(args, val)
				}
				sql.WriteString(" ")
				sql.WriteString(strings.Join(updates, ", "))
			}
		}
	}

	// RETURNING clause
	if len(q.returning) > 0 {
		sql.WriteString(" RETURNING ")
		sql.WriteString(strings.Join(q.returning, ", "))
	}

	return sql.String(), args, nil
}

// Exec executes the INSERT query.
func (q *TxInsertQuery[T]) Exec() (int64, error) {
	sql, args, err := q.ToSQL()
	if err != nil {
		return 0, err
	}

	// If no RETURNING clause, use simple Exec
	if len(q.returning) == 0 {
		tag, err := q.tx.tx.Exec(q.tx.ctx, sql, args...)
		if err != nil {
			return 0, fmt.Errorf("failed to execute insert: %w", err)
		}
		return tag.RowsAffected(), nil
	}

	// With RETURNING clause, count the returned rows
	rows, err := q.tx.tx.Query(q.tx.ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute insert: %w", err)
	}
	defer rows.Close()

	var count int64
	for rows.Next() {
		count++
	}

	return count, rows.Err()
}

// ExecReturning executes the INSERT and scans the RETURNING values.
func (q *TxInsertQuery[T]) ExecReturning() ([]T, error) {
	// Ensure we have RETURNING clause
	if len(q.returning) == 0 {
		q.Returning("*")
	}

	sql, args, err := q.ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := q.tx.tx.Query(q.tx.ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute insert: %w", err)
	}
	defer rows.Close()

	var results []T
	for rows.Next() {
		var item T
		if err := scanIntoStruct(rows, &item, q.table); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// TxUpdateQuery represents an UPDATE query within a transaction.
type TxUpdateQuery[T any] struct {
	tx        *Tx
	table     *schema.TableMetadata
	sets      map[string]interface{}
	where     []Condition
	returning []string
}

// Set sets a single column value.
func (q *TxUpdateQuery[T]) Set(column string, value interface{}) *TxUpdateQuery[T] {
	q.sets[column] = value
	return q
}

// SetMap sets multiple column values from a map.
func (q *TxUpdateQuery[T]) SetMap(values map[string]interface{}) *TxUpdateQuery[T] {
	for k, v := range values {
		q.sets[k] = v
	}
	return q
}

// Where adds a WHERE condition.
func (q *TxUpdateQuery[T]) Where(condition Condition) *TxUpdateQuery[T] {
	q.where = append(q.where, condition)
	return q
}

// And adds an AND condition.
func (q *TxUpdateQuery[T]) And(condition Condition) *TxUpdateQuery[T] {
	condition.Logic = LogicAnd
	return q.Where(condition)
}

// Or adds an OR condition.
func (q *TxUpdateQuery[T]) Or(condition Condition) *TxUpdateQuery[T] {
	condition.Logic = LogicOr
	return q.Where(condition)
}

// Returning specifies columns to return.
func (q *TxUpdateQuery[T]) Returning(columns ...string) *TxUpdateQuery[T] {
	q.returning = append(q.returning, columns...)
	return q
}

// ToSQL generates the UPDATE SQL and arguments.
func (q *TxUpdateQuery[T]) ToSQL() (string, []interface{}, error) {
	if q.table == nil {
		return "", nil, fmt.Errorf("table metadata not available")
	}

	if len(q.sets) == 0 {
		return "", nil, fmt.Errorf("no columns to update")
	}

	var sql strings.Builder
	var args []interface{}
	paramNum := 1

	sql.WriteString("UPDATE ")
	sql.WriteString(q.table.Name)
	sql.WriteString(" SET ")

	// SET clause
	setClauses := make([]string, 0, len(q.sets))
	for col, val := range q.sets {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, paramNum))
		args = append(args, val)
		paramNum++
	}
	sql.WriteString(strings.Join(setClauses, ", "))

	// WHERE clause
	if len(q.where) > 0 {
		whereBuilder := NewWhereBuilderWithStart(paramNum)
		whereBuilder.conditions = q.where
		whereSql, whereArgs, err := whereBuilder.Build()
		if err != nil {
			return "", nil, fmt.Errorf("failed to build WHERE clause: %w", err)
		}

		if whereSql != "" {
			sql.WriteString(" ")
			sql.WriteString(whereSql)
			args = append(args, whereArgs...)
		}
	}

	// RETURNING clause
	if len(q.returning) > 0 {
		sql.WriteString(" RETURNING ")
		sql.WriteString(strings.Join(q.returning, ", "))
	}

	return sql.String(), args, nil
}

// Exec executes the UPDATE query.
func (q *TxUpdateQuery[T]) Exec() (int64, error) {
	sql, args, err := q.ToSQL()
	if err != nil {
		return 0, err
	}

	// If no RETURNING clause, use simple Exec
	if len(q.returning) == 0 {
		tag, err := q.tx.tx.Exec(q.tx.ctx, sql, args...)
		if err != nil {
			return 0, fmt.Errorf("failed to execute update: %w", err)
		}
		return tag.RowsAffected(), nil
	}

	// With RETURNING clause, count the returned rows
	rows, err := q.tx.tx.Query(q.tx.ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute update: %w", err)
	}
	defer rows.Close()

	var count int64
	for rows.Next() {
		count++
	}

	return count, rows.Err()
}

// ExecReturning executes the UPDATE and scans the RETURNING values.
func (q *TxUpdateQuery[T]) ExecReturning() ([]T, error) {
	// Ensure we have RETURNING clause
	if len(q.returning) == 0 {
		q.Returning("*")
	}

	sql, args, err := q.ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := q.tx.tx.Query(q.tx.ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute update: %w", err)
	}
	defer rows.Close()

	var results []T
	for rows.Next() {
		var item T
		if err := scanIntoStruct(rows, &item, q.table); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// TxDeleteQuery represents a DELETE query within a transaction.
type TxDeleteQuery[T any] struct {
	tx        *Tx
	table     *schema.TableMetadata
	where     []Condition
	returning []string
}

// Where adds a WHERE condition.
func (q *TxDeleteQuery[T]) Where(condition Condition) *TxDeleteQuery[T] {
	q.where = append(q.where, condition)
	return q
}

// And adds an AND condition.
func (q *TxDeleteQuery[T]) And(condition Condition) *TxDeleteQuery[T] {
	condition.Logic = LogicAnd
	return q.Where(condition)
}

// Or adds an OR condition.
func (q *TxDeleteQuery[T]) Or(condition Condition) *TxDeleteQuery[T] {
	condition.Logic = LogicOr
	return q.Where(condition)
}

// Returning specifies columns to return.
func (q *TxDeleteQuery[T]) Returning(columns ...string) *TxDeleteQuery[T] {
	q.returning = append(q.returning, columns...)
	return q
}

// ToSQL generates the DELETE SQL and arguments.
func (q *TxDeleteQuery[T]) ToSQL() (string, []interface{}, error) {
	if q.table == nil {
		return "", nil, fmt.Errorf("table metadata not available")
	}

	var sql strings.Builder
	var args []interface{}

	sql.WriteString("DELETE FROM ")
	sql.WriteString(q.table.Name)

	// WHERE clause
	if len(q.where) > 0 {
		whereBuilder := NewWhereBuilder()
		whereBuilder.conditions = q.where
		whereSql, whereArgs, err := whereBuilder.Build()
		if err != nil {
			return "", nil, fmt.Errorf("failed to build WHERE clause: %w", err)
		}

		if whereSql != "" {
			sql.WriteString(" ")
			sql.WriteString(whereSql)
			args = append(args, whereArgs...)
		}
	}

	// RETURNING clause
	if len(q.returning) > 0 {
		sql.WriteString(" RETURNING ")
		sql.WriteString(strings.Join(q.returning, ", "))
	}

	return sql.String(), args, nil
}

// Exec executes the DELETE query.
func (q *TxDeleteQuery[T]) Exec() (int64, error) {
	sql, args, err := q.ToSQL()
	if err != nil {
		return 0, err
	}

	// If no RETURNING clause, use simple Exec
	if len(q.returning) == 0 {
		tag, err := q.tx.tx.Exec(q.tx.ctx, sql, args...)
		if err != nil {
			return 0, fmt.Errorf("failed to execute delete: %w", err)
		}
		return tag.RowsAffected(), nil
	}

	// With RETURNING clause, count the returned rows
	rows, err := q.tx.tx.Query(q.tx.ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute delete: %w", err)
	}
	defer rows.Close()

	var count int64
	for rows.Next() {
		count++
	}

	return count, rows.Err()
}

// ExecReturning executes the DELETE and scans the RETURNING values.
func (q *TxDeleteQuery[T]) ExecReturning() ([]T, error) {
	// Ensure we have RETURNING clause
	if len(q.returning) == 0 {
		q.Returning("*")
	}

	sql, args, err := q.ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := q.tx.tx.Query(q.tx.ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute delete: %w", err)
	}
	defer rows.Close()

	var results []T
	for rows.Next() {
		var item T
		if err := scanIntoStruct(rows, &item, q.table); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}
