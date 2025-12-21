package builder

import (
	"context"
	"fmt"
	"strings"
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

// Exec executes the INSERT query and returns the number of inserted rows.
func (q *InsertQuery[T]) Exec(ctx context.Context) (int64, error) {
	sql, args, err := q.ToSQL()
	if err != nil {
		return 0, err
	}

	// If no RETURNING clause, use simple Exec
	if len(q.returning) == 0 {
		return q.db.db.Exec(ctx, sql, args...)
	}

	// With RETURNING clause, we need to count the returned rows
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

// ExecReturning executes the INSERT and returns the inserted rows.
func (q *InsertQuery[T]) ExecReturning(ctx context.Context) ([]T, error) {
	// Ensure we have RETURNING clause
	if len(q.returning) == 0 {
		// Default to returning all columns
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
