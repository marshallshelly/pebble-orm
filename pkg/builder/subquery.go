package builder

import (
	"fmt"
)

// Subquery represents a subquery that can be used in various parts of a SQL statement
type Subquery struct {
	SQL   string
	Args  []interface{}
	Alias string
}

// NewSubquery creates a new subquery
func NewSubquery(sql string, args ...interface{}) *Subquery {
	return &Subquery{
		SQL:  sql,
		Args: args,
	}
}

// As sets an alias for the subquery
func (s *Subquery) As(alias string) *Subquery {
	s.Alias = alias
	return s
}

// ToSQL returns the SQL representation of the subquery
func (s *Subquery) ToSQL() (string, []interface{}) {
	if s.Alias != "" {
		return fmt.Sprintf("(%s) AS %s", s.SQL, s.Alias), s.Args
	}
	return fmt.Sprintf("(%s)", s.SQL), s.Args
}

// SubqueryCondition creates a condition using a subquery

// InSubquery creates an IN condition with a subquery
func InSubquery(column string, subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   column,
		Operator: OpIn,
		Value:    sql,
		Raw:      true,
	}
}

// NotInSubquery creates a NOT IN condition with a subquery
func NotInSubquery(column string, subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   column,
		Operator: OpNotIn,
		Value:    sql,
		Raw:      true,
	}
}

// ExistsSubquery creates an EXISTS condition with a subquery
func ExistsSubquery(subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   "",
		Operator: "EXISTS",
		Value:    sql,
		Raw:      true,
	}
}

// NotExistsSubquery creates a NOT EXISTS condition with a subquery
func NotExistsSubquery(subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   "",
		Operator: "NOT EXISTS",
		Value:    sql,
		Raw:      true,
	}
}

// Comparison operators with subqueries

// GtSubquery creates a > (subquery) condition
func GtSubquery(column string, subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   column,
		Operator: OpGreaterThan,
		Value:    sql,
		Raw:      true,
	}
}

// GteSubquery creates a >= (subquery) condition
func GteSubquery(column string, subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   column,
		Operator: OpGreaterThanOrEqual,
		Value:    sql,
		Raw:      true,
	}
}

// LtSubquery creates a < (subquery) condition
func LtSubquery(column string, subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   column,
		Operator: OpLessThan,
		Value:    sql,
		Raw:      true,
	}
}

// LteSubquery creates a <= (subquery) condition
func LteSubquery(column string, subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   column,
		Operator: OpLessThanOrEqual,
		Value:    sql,
		Raw:      true,
	}
}

// EqSubquery creates a = (subquery) condition
func EqSubquery(column string, subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   column,
		Operator: OpEqual,
		Value:    sql,
		Raw:      true,
	}
}

// NotEqSubquery creates a != (subquery) condition
func NotEqSubquery(column string, subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   column,
		Operator: OpNotEqual,
		Value:    sql,
		Raw:      true,
	}
}

// ALL and ANY quantifiers

// AllSubquery creates an ALL (subquery) condition
// Example: price > ALL (SELECT price FROM products WHERE category = 'electronics')
func AllSubquery(column string, operator Operator, subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   column,
		Operator: operator,
		Value:    fmt.Sprintf("ALL %s", sql),
		Raw:      true,
	}
}

// AnySubquery creates an ANY (subquery) condition
// Example: price > ANY (SELECT price FROM products WHERE category = 'electronics')
func AnySubquery(column string, operator Operator, subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   column,
		Operator: operator,
		Value:    fmt.Sprintf("ANY %s", sql),
		Raw:      true,
	}
}

// SomeSubquery creates a SOME (subquery) condition (synonym for ANY)
func SomeSubquery(column string, operator Operator, subquery *Subquery) Condition {
	sql, _ := subquery.ToSQL()
	return Condition{
		Column:   column,
		Operator: operator,
		Value:    fmt.Sprintf("SOME %s", sql),
		Raw:      true,
	}
}

// Scalar subqueries in SELECT

// ScalarSubquery represents a subquery that returns a single value
type ScalarSubquery struct {
	*Subquery
}

// NewScalarSubquery creates a new scalar subquery for use in SELECT
func NewScalarSubquery(sql string, args ...interface{}) *ScalarSubquery {
	return &ScalarSubquery{
		Subquery: NewSubquery(sql, args...),
	}
}

// AsColumn returns the subquery as a column expression
func (s *ScalarSubquery) AsColumn(alias string) string {
	if alias != "" {
		return fmt.Sprintf("(%s) AS %s", s.SQL, alias)
	}
	return fmt.Sprintf("(%s)", s.SQL)
}

// Example usage:
//
// Subquery in WHERE clause:
// subquery := NewSubquery("SELECT AVG(salary) FROM employees")
// users := db.Select(User{}).
//     Where(GtSubquery("salary", subquery)).
//     All(ctx)
//
// Subquery with IN:
// subquery := NewSubquery("SELECT department_id FROM departments WHERE location = ?", "NYC")
// users := db.Select(User{}).
//     Where(InSubquery("department_id", subquery)).
//     All(ctx)
//
// EXISTS:
// subquery := NewSubquery("SELECT 1 FROM orders WHERE orders.user_id = users.id AND status = 'pending'")
// users := db.Select(User{}).
//     Where(ExistsSubquery(subquery)).
//     All(ctx)
//
// Scalar subquery in SELECT:
// orderCount := NewScalarSubquery("SELECT COUNT(*) FROM orders WHERE orders.user_id = users.id")
// users := db.Select(User{}).
//     Columns("id", "name", orderCount.AsColumn("order_count")).
//     All(ctx)
