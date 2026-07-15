package builder

import (
	"context"
)

// Values sets the values to insert (single or multiple rows).
func (q *InsertQuery[T]) Values(values ...T) *InsertQuery[T] {
	q.values = append(q.values, values...)
	return q
}

// Returning specifies columns to return after insert.
func (q *InsertQuery[T]) Returning(columns ...string) *InsertQuery[T] {
	q.returning = columns
	return q
}

// OnConflictDoNothing adds ON CONFLICT DO NOTHING clause.
func (q *InsertQuery[T]) OnConflictDoNothing(columns ...string) *InsertQuery[T] {
	q.onConflict = &OnConflict{
		Columns: columns,
		Action:  DoNothing,
	}
	return q
}

// OnConflictDoUpdate adds ON CONFLICT DO UPDATE clause.
func (q *InsertQuery[T]) OnConflictDoUpdate(columns []string, updates map[string]interface{}) *InsertQuery[T] {
	q.onConflict = &OnConflict{
		Columns: columns,
		Action:  DoUpdate,
		Updates: updates,
	}
	return q
}

// ToSQL generates the INSERT SQL and arguments.
func (q *InsertQuery[T]) ToSQL() (string, []interface{}, error) {
	return buildInsertSQL(insertSpec{
		table:      q.table,
		rows:       toAnySlice(q.values),
		returning:  q.returning,
		onConflict: q.onConflict,
	})
}

// Exec executes the INSERT query and returns the number of inserted rows.
func (q *InsertQuery[T]) Exec(ctx context.Context) (int64, error) {
	sql, args, err := q.ToSQL()
	if err != nil {
		return 0, err
	}
	return execWrite(ctx, q.db.db, sql, args, len(q.returning) > 0)
}

// ExecReturning executes the INSERT and returns the inserted rows.
func (q *InsertQuery[T]) ExecReturning(ctx context.Context) ([]T, error) {
	if len(q.returning) == 0 {
		q.Returning("*")
	}
	sql, args, err := q.ToSQL()
	if err != nil {
		return nil, err
	}
	return queryRows[T](ctx, q.db.db, q.table, sql, args, nil)
}
