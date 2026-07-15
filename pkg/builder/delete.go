package builder

import (
	"context"
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
	return buildDeleteSQL(deleteSpec{
		table:     q.table,
		where:     q.where,
		returning: q.returning,
	})
}

// Exec executes the DELETE query and returns the number of affected rows.
func (q *DeleteQuery[T]) Exec(ctx context.Context) (int64, error) {
	sql, args, err := q.ToSQL()
	if err != nil {
		return 0, err
	}
	return execWrite(ctx, q.db.db, sql, args, len(q.returning) > 0)
}

// ExecReturning executes the DELETE and returns the deleted rows.
func (q *DeleteQuery[T]) ExecReturning(ctx context.Context) ([]T, error) {
	if len(q.returning) == 0 {
		q.Returning("*")
	}
	sql, args, err := q.ToSQL()
	if err != nil {
		return nil, err
	}
	return queryRows[T](ctx, q.db.db, q.table, sql, args, nil)
}
