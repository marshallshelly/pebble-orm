package builder

import (
	"fmt"
	"strings"
)

// CTE represents a Common Table Expression (WITH clause)
type CTE struct {
	Name    string
	Columns []string
	Query   string
	Args    []interface{}
}

// CTEBuilder helps build queries with Common Table Expressions
type CTEBuilder struct {
	ctes []CTE
}

// NewCTEBuilder creates a new CTEBuilder
func NewCTEBuilder() *CTEBuilder {
	return &CTEBuilder{
		ctes: make([]CTE, 0),
	}
}

// Add adds a CTE to the builder
func (c *CTEBuilder) Add(name, query string, args ...interface{}) *CTEBuilder {
	c.ctes = append(c.ctes, CTE{
		Name:  name,
		Query: query,
		Args:  args,
	})
	return c
}

// AddWithColumns adds a CTE with explicit column names
func (c *CTEBuilder) AddWithColumns(name string, columns []string, query string, args ...interface{}) *CTEBuilder {
	c.ctes = append(c.ctes, CTE{
		Name:    name,
		Columns: columns,
		Query:   query,
		Args:    args,
	})
	return c
}

// Build generates the WITH clause SQL
func (c *CTEBuilder) Build() (string, []interface{}) {
	if len(c.ctes) == 0 {
		return "", nil
	}

	var parts []string
	var allArgs []interface{}

	for _, cte := range c.ctes {
		var ctePart string
		if len(cte.Columns) > 0 {
			ctePart = fmt.Sprintf("%s (%s) AS (%s)",
				cte.Name,
				strings.Join(cte.Columns, ", "),
				cte.Query,
			)
		} else {
			ctePart = fmt.Sprintf("%s AS (%s)", cte.Name, cte.Query)
		}
		parts = append(parts, ctePart)
		allArgs = append(allArgs, cte.Args...)
	}

	return "WITH " + strings.Join(parts, ", "), allArgs
}

// CTESelect represents a SELECT query with CTEs
type CTESelect[T any] struct {
	*SelectQuery[T]
	cteBuilder *CTEBuilder
}

// WithCTE adds a CTE to the query
func (q *SelectQuery[T]) WithCTE(name, query string, args ...interface{}) *CTESelect[T] {
	cteSelect := &CTESelect[T]{
		SelectQuery: q,
		cteBuilder:  NewCTEBuilder(),
	}
	cteSelect.cteBuilder.Add(name, query, args...)
	return cteSelect
}

// AndCTE adds another CTE to the query
func (q *CTESelect[T]) AndCTE(name, query string, args ...interface{}) *CTESelect[T] {
	q.cteBuilder.Add(name, query, args...)
	return q
}

// ToSQL builds the complete SQL with CTEs
func (q *CTESelect[T]) ToSQL() (string, []interface{}, error) {
	// Build CTE clause
	cteSQL, cteArgs := q.cteBuilder.Build()

	// Build main query
	mainSQL, mainArgs, err := q.SelectQuery.ToSQL()
	if err != nil {
		return "", nil, err
	}

	// Combine
	if cteSQL != "" {
		mainSQL = cteSQL + " " + mainSQL
		allArgs := append(cteArgs, mainArgs...)
		return mainSQL, allArgs, nil
	}

	return mainSQL, mainArgs, nil
}

// Recursive CTE support

// RecursiveCTE represents a recursive Common Table Expression
type RecursiveCTE struct {
	Name           string
	Columns        []string
	BaseQuery      string
	BaseArgs       []interface{}
	RecursiveQuery string
	RecursiveArgs  []interface{}
}

// CTERecursiveBuilder helps build recursive CTEs
type CTERecursiveBuilder struct {
	cte RecursiveCTE
}

// NewRecursiveCTE creates a new recursive CTE builder
func NewRecursiveCTE(name string, columns []string) *CTERecursiveBuilder {
	return &CTERecursiveBuilder{
		cte: RecursiveCTE{
			Name:    name,
			Columns: columns,
		},
	}
}

// BaseCase sets the base case query
func (r *CTERecursiveBuilder) BaseCase(query string, args ...interface{}) *CTERecursiveBuilder {
	r.cte.BaseQuery = query
	r.cte.BaseArgs = args
	return r
}

// RecursiveCase sets the recursive case query
func (r *CTERecursiveBuilder) RecursiveCase(query string, args ...interface{}) *CTERecursiveBuilder {
	r.cte.RecursiveQuery = query
	r.cte.RecursiveArgs = args
	return r
}

// Build generates the recursive CTE SQL
func (r *CTERecursiveBuilder) Build() (string, []interface{}) {
	var columnsPart string
	if len(r.cte.Columns) > 0 {
		columnsPart = fmt.Sprintf(" (%s)", strings.Join(r.cte.Columns, ", "))
	}

	sql := fmt.Sprintf("WITH RECURSIVE %s%s AS (%s UNION ALL %s)",
		r.cte.Name,
		columnsPart,
		r.cte.BaseQuery,
		r.cte.RecursiveQuery,
	)

	allArgs := append(r.cte.BaseArgs, r.cte.RecursiveArgs...)
	return sql, allArgs
}

// Example usage:
//
// Hierarchical query example:
// cte := NewRecursiveCTE("employee_hierarchy", []string{"id", "name", "manager_id", "level"}).
//     BaseCase("SELECT id, name, manager_id, 1 FROM employees WHERE manager_id IS NULL").
//     RecursiveCase("SELECT e.id, e.name, e.manager_id, eh.level + 1 FROM employees e JOIN employee_hierarchy eh ON e.manager_id = eh.id")
//
// sql, args := cte.Build()
// // WITH RECURSIVE employee_hierarchy (id, name, manager_id, level) AS (
// //   SELECT id, name, manager_id, 1 FROM employees WHERE manager_id IS NULL
// //   UNION ALL
// //   SELECT e.id, e.name, e.manager_id, eh.level + 1 FROM employees e JOIN employee_hierarchy eh ON e.manager_id = eh.id
// // )
//
// Then use in main query:
// db.Select(Employee{}).
//     FromRaw("employee_hierarchy").
//     Where(...).
//     All(ctx)
