package builder

import (
	"context"
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
	return buildUpdateSQL(updateSpec{
		table:     q.table,
		sets:      q.sets,
		where:     q.where,
		returning: q.returning,
	})
}

// Exec executes the UPDATE query and returns the number of affected rows.
func (q *UpdateQuery[T]) Exec(ctx context.Context) (int64, error) {
	sql, args, err := q.ToSQL()
	if err != nil {
		return 0, err
	}
	return execWrite(ctx, q.db.db, sql, args, len(q.returning) > 0)
}

// ExecReturning executes the UPDATE and returns the updated rows.
func (q *UpdateQuery[T]) ExecReturning(ctx context.Context) ([]T, error) {
	if len(q.returning) == 0 {
		q.Returning("*")
	}
	sql, args, err := q.ToSQL()
	if err != nil {
		return nil, err
	}
	return queryRows[T](ctx, q.db.db, q.table, sql, args, nil)
}
