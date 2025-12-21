package builder

import (
	"context"
	"fmt"
	"strings"
)

// Columns specifies which columns to select.
func (q *SelectQuery[T]) Columns(cols ...string) *SelectQuery[T] {
	q.columns = cols
	return q
}

// Where adds a WHERE condition.
func (q *SelectQuery[T]) Where(condition Condition) *SelectQuery[T] {
	q.where = append(q.where, condition)
	return q
}

// And adds an AND condition (alias for Where).
func (q *SelectQuery[T]) And(condition Condition) *SelectQuery[T] {
	condition.Logic = LogicAnd
	return q.Where(condition)
}

// Or adds an OR condition.
func (q *SelectQuery[T]) Or(condition Condition) *SelectQuery[T] {
	condition.Logic = LogicOr
	return q.Where(condition)
}

// OrderBy adds an ORDER BY clause.
func (q *SelectQuery[T]) OrderBy(column string, direction OrderDirection) *SelectQuery[T] {
	q.orderBy = append(q.orderBy, OrderBy{
		Column:    column,
		Direction: direction,
		NullsPos:  NullsDefault,
	})
	return q
}

// OrderByAsc adds an ascending ORDER BY clause.
func (q *SelectQuery[T]) OrderByAsc(column string) *SelectQuery[T] {
	return q.OrderBy(column, Asc)
}

// OrderByDesc adds a descending ORDER BY clause.
func (q *SelectQuery[T]) OrderByDesc(column string) *SelectQuery[T] {
	return q.OrderBy(column, Desc)
}

// Limit sets the LIMIT clause.
func (q *SelectQuery[T]) Limit(limit int) *SelectQuery[T] {
	q.limit = &limit
	return q
}

// Offset sets the OFFSET clause.
func (q *SelectQuery[T]) Offset(offset int) *SelectQuery[T] {
	q.offset = &offset
	return q
}

// Distinct adds DISTINCT to the query.
func (q *SelectQuery[T]) Distinct() *SelectQuery[T] {
	q.distinct = true
	return q
}

// ForUpdate adds FOR UPDATE lock.
func (q *SelectQuery[T]) ForUpdate() *SelectQuery[T] {
	q.forUpdate = true
	return q
}

// Preload specifies relationships to eagerly load.
// Pass the name of the Go struct field that contains the relationship.
// Example: query.Preload("Posts").Preload("Comments")
func (q *SelectQuery[T]) Preload(relationships ...string) *SelectQuery[T] {
	q.preloads = append(q.preloads, relationships...)
	return q
}

// GroupBy adds a GROUP BY clause.
func (q *SelectQuery[T]) GroupBy(columns ...string) *SelectQuery[T] {
	q.groupBy = append(q.groupBy, columns...)
	return q
}

// Having adds a HAVING condition.
func (q *SelectQuery[T]) Having(condition Condition) *SelectQuery[T] {
	q.having = append(q.having, condition)
	return q
}

// InnerJoin adds an INNER JOIN.
func (q *SelectQuery[T]) InnerJoin(table string, condition string, args ...interface{}) *SelectQuery[T] {
	q.joins = append(q.joins, Join{
		Type:      InnerJoin,
		Table:     table,
		Condition: condition,
		Args:      args,
	})
	return q
}

// LeftJoin adds a LEFT JOIN.
func (q *SelectQuery[T]) LeftJoin(table string, condition string, args ...interface{}) *SelectQuery[T] {
	q.joins = append(q.joins, Join{
		Type:      LeftJoin,
		Table:     table,
		Condition: condition,
		Args:      args,
	})
	return q
}

// RightJoin adds a RIGHT JOIN.
func (q *SelectQuery[T]) RightJoin(table string, condition string, args ...interface{}) *SelectQuery[T] {
	q.joins = append(q.joins, Join{
		Type:      RightJoin,
		Table:     table,
		Condition: condition,
		Args:      args,
	})
	return q
}

// FullJoin adds a FULL OUTER JOIN.
func (q *SelectQuery[T]) FullJoin(table string, condition string, args ...interface{}) *SelectQuery[T] {
	q.joins = append(q.joins, Join{
		Type:      FullJoin,
		Table:     table,
		Condition: condition,
		Args:      args,
	})
	return q
}

// LateralJoin adds a LATERAL JOIN clause to the query.
// LATERAL joins allow subqueries to reference columns from preceding FROM items
func (q *SelectQuery[T]) LateralJoin(table string, condition string, args ...interface{}) *SelectQuery[T] {
	q.joins = append(q.joins, Join{
		Type:      InnerJoin,
		Table:     table,
		Condition: condition,
		Args:      args,
		Lateral:   true,
	})
	return q
}

// LeftLateralJoin adds a LEFT LATERAL JOIN clause to the query.
func (q *SelectQuery[T]) LeftLateralJoin(table string, condition string, args ...interface{}) *SelectQuery[T] {
	q.joins = append(q.joins, Join{
		Type:      LeftJoin,
		Table:     table,
		Condition: condition,
		Args:      args,
		Lateral:   true,
	})
	return q
}

// ToSQL generates the SQL query and arguments.
func (q *SelectQuery[T]) ToSQL() (string, []interface{}, error) {
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
		if join.Lateral {
			sql.WriteString("LATERAL ")
		}
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
func (q *SelectQuery[T]) All(ctx context.Context) ([]T, error) {
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

	// Load preloaded relationships
	if len(q.preloads) > 0 && len(results) > 0 {
		if err := q.loadRelationships(ctx, &results); err != nil {
			return nil, err
		}
	}

	return results, nil
}

// First executes the query and returns the first result.
func (q *SelectQuery[T]) First(ctx context.Context) (*T, error) {
	// Limit to 1 result
	q.Limit(1)

	results, err := q.All(ctx)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no rows found")
	}

	return &results[0], nil
}

// Count executes a COUNT query.
func (q *SelectQuery[T]) Count(ctx context.Context) (int64, error) {
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
	err := q.db.db.QueryRow(ctx, sql.String(), args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Exists checks if any rows match the query.
func (q *SelectQuery[T]) Exists(ctx context.Context) (bool, error) {
	count, err := q.Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
