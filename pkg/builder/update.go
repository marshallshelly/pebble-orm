package builder

import (
	"context"
	"fmt"
	"strings"
)

// Set sets a column value for the UPDATE.
func (q *UpdateQuery[T]) Set(column string, value interface{}) *UpdateQuery[T] {
	q.sets[column] = value
	return q
}

// SetMap sets multiple column values from a map.
func (q *UpdateQuery[T]) SetMap(values map[string]interface{}) *UpdateQuery[T] {
	for col, val := range values {
		q.sets[col] = val
	}
	return q
}

// Where adds a WHERE condition.
func (q *UpdateQuery[T]) Where(condition Condition) *UpdateQuery[T] {
	q.where = append(q.where, condition)
	return q
}

// And adds an AND condition.
func (q *UpdateQuery[T]) And(condition Condition) *UpdateQuery[T] {
	condition.Logic = LogicAnd
	return q.Where(condition)
}

// Or adds an OR condition.
func (q *UpdateQuery[T]) Or(condition Condition) *UpdateQuery[T] {
	condition.Logic = LogicOr
	return q.Where(condition)
}

// Returning specifies columns to return after update.
func (q *UpdateQuery[T]) Returning(columns ...string) *UpdateQuery[T] {
	q.returning = columns
	return q
}

// ToSQL generates the UPDATE SQL and arguments.
func (q *UpdateQuery[T]) ToSQL() (string, []interface{}, error) {
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

// Exec executes the UPDATE query and returns the number of affected rows.
func (q *UpdateQuery[T]) Exec(ctx context.Context) (int64, error) {
	sql, args, err := q.ToSQL()
	if err != nil {
		return 0, err
	}

	// If no RETURNING clause, use simple Exec
	if len(q.returning) == 0 {
		return q.db.db.Exec(ctx, sql, args...)
	}

	// With RETURNING clause, count the returned rows
	rows, err := q.db.db.Query(ctx, sql, args...)
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

// ExecReturning executes the UPDATE and returns the updated rows.
func (q *UpdateQuery[T]) ExecReturning(ctx context.Context) ([]T, error) {
	// Ensure we have RETURNING clause
	if len(q.returning) == 0 {
		q.Returning("*")
	}

	sql, args, err := q.ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := q.db.db.Query(ctx, sql, args...)
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
