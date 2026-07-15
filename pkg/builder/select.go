package builder

import (
	"context"
	"fmt"
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
	return buildSelectSQL(selectSpec{
		table: q.table, distinct: q.distinct, columns: q.columns, joins: q.joins,
		where: q.where, groupBy: q.groupBy, having: q.having, orderBy: q.orderBy,
		limit: q.limit, offset: q.offset, forUpdate: q.forUpdate,
	})
}

// All executes the query and returns all results.
func (q *SelectQuery[T]) All(ctx context.Context) ([]T, error) {
	sql, args, err := q.ToSQL()
	if err != nil {
		return nil, err
	}
	return queryRows[T](ctx, q.db.db, q.table, sql, args, q.preloads)
}

// First executes the query and returns the first result.
func (q *SelectQuery[T]) First(ctx context.Context) (*T, error) {
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
	sql, args, err := buildCountSQL(q.table, q.where)
	if err != nil {
		return 0, err
	}
	return queryCount(ctx, q.db.db, sql, args)
}

// Exists checks if any rows match the query.
func (q *SelectQuery[T]) Exists(ctx context.Context) (bool, error) {
	count, err := q.Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
