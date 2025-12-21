package builder

import (
	"context"
	"fmt"
	"strings"
)

// Where adds a WHERE condition to the DELETE query.
func (q *DeleteQuery[T]) Where(condition Condition) *DeleteQuery[T] {
	q.where = append(q.where, condition)
	return q
}

// And adds an AND condition.
func (q *DeleteQuery[T]) And(condition Condition) *DeleteQuery[T] {
	condition.Logic = LogicAnd
	return q.Where(condition)
}

// Or adds an OR condition.
func (q *DeleteQuery[T]) Or(condition Condition) *DeleteQuery[T] {
	condition.Logic = LogicOr
	return q.Where(condition)
}

// Returning specifies columns to return after delete.
func (q *DeleteQuery[T]) Returning(columns ...string) *DeleteQuery[T] {
	q.returning = columns
	return q
}

// ToSQL generates the DELETE SQL and arguments.
func (q *DeleteQuery[T]) ToSQL() (string, []interface{}, error) {
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

// Exec executes the DELETE query and returns the number of affected rows.
func (q *DeleteQuery[T]) Exec(ctx context.Context) (int64, error) {
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

// ExecReturning executes the DELETE and returns the deleted rows.
func (q *DeleteQuery[T]) ExecReturning(ctx context.Context) ([]T, error) {
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
