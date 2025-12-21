package builder

import (
	"fmt"
	"strings"
)

// WhereBuilder helps build WHERE clauses.
type WhereBuilder struct {
	conditions []Condition
	paramCount int
	paramStart int
}

// NewWhereBuilder creates a new WhereBuilder.
func NewWhereBuilder() *WhereBuilder {
	return &WhereBuilder{
		conditions: make([]Condition, 0),
		paramCount: 0,
		paramStart: 1,
	}
}

// NewWhereBuilderWithStart creates a new WhereBuilder with a starting parameter number.
func NewWhereBuilderWithStart(paramStart int) *WhereBuilder {
	return &WhereBuilder{
		conditions: make([]Condition, 0),
		paramCount: 0,
		paramStart: paramStart,
	}
}

// Add adds a condition to the WHERE clause.
func (w *WhereBuilder) Add(condition Condition) {
	w.conditions = append(w.conditions, condition)
}

// Build generates the WHERE clause SQL and arguments.
func (w *WhereBuilder) Build() (string, []interface{}, error) {
	if len(w.conditions) == 0 {
		return "", nil, nil
	}

	var args []interface{}
	sql, newArgs, err := w.buildConditions(w.conditions, w.paramStart)
	if err != nil {
		return "", nil, err
	}

	args = append(args, newArgs...)
	return "WHERE " + sql, args, nil
}

// buildConditions recursively builds conditions.
func (w *WhereBuilder) buildConditions(conditions []Condition, paramStart int) (string, []interface{}, error) {
	if len(conditions) == 0 {
		return "", nil, nil
	}

	var parts []string
	var args []interface{}
	paramNum := paramStart

	for i, cond := range conditions {
		// Handle grouped conditions
		if len(cond.Group) > 0 {
			groupSQL, groupArgs, err := w.buildConditions(cond.Group, paramNum)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, "("+groupSQL+")")
			args = append(args, groupArgs...)
			paramNum += len(groupArgs)
		} else {
			// Build individual condition
			condSQL, condArgs, err := w.buildCondition(cond, paramNum)
			if err != nil {
				return "", nil, err
			}

			if cond.Not {
				condSQL = "NOT (" + condSQL + ")"
			}

			parts = append(parts, condSQL)
			args = append(args, condArgs...)
			paramNum += len(condArgs)
		}

		// Add logic operator between conditions
		if i < len(conditions)-1 {
			logic := conditions[i+1].Logic
			if logic == "" {
				logic = LogicAnd
			}
			parts[len(parts)-1] += " " + string(logic)
		}
	}

	return strings.Join(parts, " "), args, nil
}

// buildCondition builds a single condition.
func (w *WhereBuilder) buildCondition(cond Condition, paramNum int) (string, []interface{}, error) {
	column := cond.Column
	operator := cond.Operator
	value := cond.Value

	switch operator {
	case OpEqual, OpNotEqual, OpGreaterThan, OpGreaterThanOrEqual, OpLessThan, OpLessThanOrEqual:
		return fmt.Sprintf("%s %s $%d", column, operator, paramNum), []interface{}{value}, nil

	case OpLike, OpILike, OpNotLike:
		return fmt.Sprintf("%s %s $%d", column, operator, paramNum), []interface{}{value}, nil

	case OpIn, OpNotIn:
		// Handle IN/NOT IN with array values
		values, ok := value.([]interface{})
		if !ok {
			return "", nil, fmt.Errorf("IN/NOT IN operator requires []interface{} value")
		}

		placeholders := make([]string, len(values))
		for i := range values {
			placeholders[i] = fmt.Sprintf("$%d", paramNum+i)
		}

		sql := fmt.Sprintf("%s %s (%s)", column, operator, strings.Join(placeholders, ", "))
		return sql, values, nil

	case OpIsNull:
		return fmt.Sprintf("%s IS NULL", column), nil, nil

	case OpIsNotNull:
		return fmt.Sprintf("%s IS NOT NULL", column), nil, nil

	case OpBetween:
		// Expect value to be [min, max]
		values, ok := value.([]interface{})
		if !ok || len(values) != 2 {
			return "", nil, fmt.Errorf("BETWEEN operator requires [min, max] array")
		}

		sql := fmt.Sprintf("%s BETWEEN $%d AND $%d", column, paramNum, paramNum+1)
		return sql, values, nil

	case OpExists:
		// Expect value to be a subquery string
		subquery, ok := value.(string)
		if !ok {
			return "", nil, fmt.Errorf("EXISTS operator requires subquery string")
		}

		return fmt.Sprintf("EXISTS (%s)", subquery), nil, nil

	default:
		return "", nil, fmt.Errorf("unknown operator: %s", operator)
	}
}

// Helper functions for building conditions

// Eq creates an equality condition.
func Eq(column string, value interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: OpEqual,
		Value:    value,
		Logic:    LogicAnd,
	}
}

// NotEq creates a not-equal condition.
func NotEq(column string, value interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: OpNotEqual,
		Value:    value,
		Logic:    LogicAnd,
	}
}

// Gt creates a greater-than condition.
func Gt(column string, value interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: OpGreaterThan,
		Value:    value,
		Logic:    LogicAnd,
	}
}

// Gte creates a greater-than-or-equal condition.
func Gte(column string, value interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: OpGreaterThanOrEqual,
		Value:    value,
		Logic:    LogicAnd,
	}
}

// Lt creates a less-than condition.
func Lt(column string, value interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: OpLessThan,
		Value:    value,
		Logic:    LogicAnd,
	}
}

// Lte creates a less-than-or-equal condition.
func Lte(column string, value interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: OpLessThanOrEqual,
		Value:    value,
		Logic:    LogicAnd,
	}
}

// In creates an IN condition.
func In(column string, values ...interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: OpIn,
		Value:    values,
		Logic:    LogicAnd,
	}
}

// NotIn creates a NOT IN condition.
func NotIn(column string, values ...interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: OpNotIn,
		Value:    values,
		Logic:    LogicAnd,
	}
}

// Like creates a LIKE condition.
func Like(column string, pattern string) Condition {
	return Condition{
		Column:   column,
		Operator: OpLike,
		Value:    pattern,
		Logic:    LogicAnd,
	}
}

// ILike creates an ILIKE condition (case-insensitive).
func ILike(column string, pattern string) Condition {
	return Condition{
		Column:   column,
		Operator: OpILike,
		Value:    pattern,
		Logic:    LogicAnd,
	}
}

// IsNull creates an IS NULL condition.
func IsNull(column string) Condition {
	return Condition{
		Column:   column,
		Operator: OpIsNull,
		Logic:    LogicAnd,
	}
}

// IsNotNull creates an IS NOT NULL condition.
func IsNotNull(column string) Condition {
	return Condition{
		Column:   column,
		Operator: OpIsNotNull,
		Logic:    LogicAnd,
	}
}

// Between creates a BETWEEN condition.
func Between(column string, min, max interface{}) Condition {
	return Condition{
		Column:   column,
		Operator: OpBetween,
		Value:    []interface{}{min, max},
		Logic:    LogicAnd,
	}
}

// Or sets the logic operator to OR for the next condition.
func Or(cond Condition) Condition {
	cond.Logic = LogicOr
	return cond
}

// Not negates a condition.
func Not(cond Condition) Condition {
	cond.Not = true
	return cond
}

// Group creates a grouped condition.
func Group(conditions ...Condition) Condition {
	return Condition{
		Group: conditions,
		Logic: LogicAnd,
	}
}
