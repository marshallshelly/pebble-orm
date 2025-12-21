// Package builder provides a type-safe query builder for PostgreSQL.
package builder

import (
	"context"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// Query represents a generic database query.
type Query interface {
	// ToSQL generates the SQL query and parameter values.
	ToSQL() (sql string, args []interface{}, err error)
}

// Executable represents a query that can be executed.
type Executable interface {
	Query
	// Exec executes the query and returns the number of affected rows.
	Exec(ctx context.Context) (int64, error)
}

// SelectQuery represents a SELECT query with type safety.
type SelectQuery[T any] struct {
	db         *DB
	table      *schema.TableMetadata
	columns    []string
	where      []Condition
	joins      []Join
	groupBy    []string
	having     []Condition
	orderBy    []OrderBy
	limit      *int
	offset     *int
	distinct   bool
	forUpdate  bool
	preloads   []string // Relationship fields to eagerly load
}

// InsertQuery represents an INSERT query.
type InsertQuery[T any] struct {
	db        *DB
	table     *schema.TableMetadata
	values    []T
	returning []string
	onConflict *OnConflict
}

// UpdateQuery represents an UPDATE query.
type UpdateQuery[T any] struct {
	db        *DB
	table     *schema.TableMetadata
	sets      map[string]interface{}
	where     []Condition
	returning []string
}

// DeleteQuery represents a DELETE query.
type DeleteQuery[T any] struct {
	db        *DB
	table     *schema.TableMetadata
	where     []Condition
	returning []string
}

// Condition represents a WHERE/HAVING condition.
type Condition struct {
	Column   string
	Operator Operator
	Value    interface{}
	Logic    LogicOperator
	Not      bool
	Group    []Condition // For grouped conditions
	Raw      bool        // true if Value should be used as raw SQL instead of parameterized
}

// Join represents a JOIN clause.
type Join struct {
	Type      JoinType
	Table     string
	Condition string
	Args      []interface{}
	Lateral   bool // true for LATERAL joins
}

// OrderBy represents an ORDER BY clause.
type OrderBy struct {
	Column    string
	Direction OrderDirection
	NullsPos  NullsPosition
}

// OnConflict represents an ON CONFLICT clause for upserts.
type OnConflict struct {
	Columns []string
	Action  ConflictAction
	Updates map[string]interface{}
}

// Operator represents a comparison operator.
type Operator string

const (
	// OpEqual represents the = operator.
	OpEqual Operator = "="
	// OpNotEqual represents the != operator.
	OpNotEqual Operator = "!="
	// OpGreaterThan represents the > operator.
	OpGreaterThan Operator = ">"
	// OpGreaterThanOrEqual represents the >= operator.
	OpGreaterThanOrEqual Operator = ">="
	// OpLessThan represents the < operator.
	OpLessThan Operator = "<"
	// OpLessThanOrEqual represents the <= operator.
	OpLessThanOrEqual Operator = "<="
	// OpIn represents the IN operator.
	OpIn Operator = "IN"
	// OpNotIn represents the NOT IN operator.
	OpNotIn Operator = "NOT IN"
	// OpLike represents the LIKE operator.
	OpLike Operator = "LIKE"
	// OpILike represents the ILIKE operator (case-insensitive).
	OpILike Operator = "ILIKE"
	// OpNotLike represents the NOT LIKE operator.
	OpNotLike Operator = "NOT LIKE"
	// OpIsNull represents the IS NULL operator.
	OpIsNull Operator = "IS NULL"
	// OpIsNotNull represents the IS NOT NULL operator.
	OpIsNotNull Operator = "IS NOT NULL"
	// OpBetween represents the BETWEEN operator.
	OpBetween Operator = "BETWEEN"
	// OpExists represents the EXISTS operator.
	OpExists Operator = "EXISTS"
)

// LogicOperator represents a logical operator (AND/OR).
type LogicOperator string

const (
	// LogicAnd represents the AND operator.
	LogicAnd LogicOperator = "AND"
	// LogicOr represents the OR operator.
	LogicOr LogicOperator = "OR"
)

// JoinType represents a type of JOIN.
type JoinType string

const (
	// InnerJoin represents an INNER JOIN.
	InnerJoin JoinType = "INNER JOIN"
	// LeftJoin represents a LEFT JOIN.
	LeftJoin JoinType = "LEFT JOIN"
	// RightJoin represents a RIGHT JOIN.
	RightJoin JoinType = "RIGHT JOIN"
	// FullJoin represents a FULL OUTER JOIN.
	FullJoin JoinType = "FULL OUTER JOIN"
	// CrossJoin represents a CROSS JOIN.
	CrossJoin JoinType = "CROSS JOIN"
)

// OrderDirection represents the sort direction.
type OrderDirection string

const (
	// Asc represents ascending order.
	Asc OrderDirection = "ASC"
	// Desc represents descending order.
	Desc OrderDirection = "DESC"
)

// NullsPosition represents NULL positioning in ORDER BY.
type NullsPosition string

const (
	// NullsFirst positions NULL values first.
	NullsFirst NullsPosition = "NULLS FIRST"
	// NullsLast positions NULL values last.
	NullsLast NullsPosition = "NULLS LAST"
	// NullsDefault uses database default NULL positioning.
	NullsDefault NullsPosition = ""
)

// ConflictAction represents the action for ON CONFLICT.
type ConflictAction string

const (
	// DoNothing does nothing on conflict.
	DoNothing ConflictAction = "DO NOTHING"
	// DoUpdate updates on conflict.
	DoUpdate ConflictAction = "DO UPDATE SET"
)
