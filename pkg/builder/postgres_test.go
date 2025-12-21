package builder

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func TestJSONBOperators(t *testing.T) {
	tests := []struct{
		name string
		condition Condition
		expectedOp Operator
	}{
		{
			name: "JSONBContains",
			condition: JSONBContains("data", `{"key": "value"}`),
			expectedOp: "@>",
		},
		{
			name: "JSONBContainedBy",
			condition: JSONBContainedBy("data", `{"key": "value"}`),
			expectedOp: "<@",
		},
		{
			name: "JSONBHasKey",
			condition: JSONBHasKey("data", "email"),
			expectedOp: "?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.condition.Operator != tt.expectedOp {
				t.Errorf("expected operator %s, got %s", tt.expectedOp, tt.condition.Operator)
			}
		})
	}
}

func TestArrayOperators(t *testing.T) {
	tests := []struct{
		name string
		condition Condition
		expectedOp Operator
	}{
		{
			name: "ArrayContains",
			condition: ArrayContains("tags", []string{"go", "postgresql"}),
			expectedOp: "@>",
		},
		{
			name: "ArrayContainedBy",
			condition: ArrayContainedBy("tags", []string{"go", "postgresql", "database"}),
			expectedOp: "<@",
		},
		{
			name: "ArrayOverlap",
			condition: ArrayOverlap("tags", []string{"go"}),
			expectedOp: "&&",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.condition.Operator != tt.expectedOp {
				t.Errorf("expected operator %s, got %s", tt.expectedOp, tt.condition.Operator)
			}
		})
	}
}

func TestSubqueryConditions(t *testing.T) {
	t.Run("InSubquery", func(t *testing.T) {
		subquery := NewSubquery("SELECT id FROM users WHERE active = true")
		cond := InSubquery("user_id", subquery)

		if cond.Operator != OpIn {
			t.Errorf("expected OpIn, got %s", cond.Operator)
		}
		if !cond.Raw {
			t.Error("expected Raw to be true")
		}
	})

	t.Run("ExistsSubquery", func(t *testing.T) {
		subquery := NewSubquery("SELECT 1 FROM orders WHERE orders.user_id = users.id")
		cond := ExistsSubquery(subquery)

		if cond.Operator != "EXISTS" {
			t.Errorf("expected EXISTS, got %s", cond.Operator)
		}
		if !cond.Raw {
			t.Error("expected Raw to be true")
		}
	})
}

func TestCTEBuilder(t *testing.T) {
	t.Run("simple CTE", func(t *testing.T) {
		builder := NewCTEBuilder()
		builder.Add("active_users", "SELECT * FROM users WHERE active = true")

		sql, _ := builder.Build()
		expected := "WITH active_users AS (SELECT * FROM users WHERE active = true)"
		if sql != expected {
			t.Errorf("expected %q, got %q", expected, sql)
		}
	})

	t.Run("CTE with columns", func(t *testing.T) {
		builder := NewCTEBuilder()
		builder.AddWithColumns("user_stats", []string{"user_id", "order_count"},
			"SELECT user_id, COUNT(*) FROM orders GROUP BY user_id")

		sql, _ := builder.Build()
		expected := "WITH user_stats (user_id, order_count) AS (SELECT user_id, COUNT(*) FROM orders GROUP BY user_id)"
		if sql != expected {
			t.Errorf("expected %q, got %q", expected, sql)
		}
	})

	t.Run("multiple CTEs", func(t *testing.T) {
		builder := NewCTEBuilder()
		builder.Add("active_users", "SELECT * FROM users WHERE active = true")
		builder.Add("recent_orders", "SELECT * FROM orders WHERE created_at > NOW() - INTERVAL '7 days'")

		sql, _ := builder.Build()
		expected := "WITH active_users AS (SELECT * FROM users WHERE active = true), recent_orders AS (SELECT * FROM orders WHERE created_at > NOW() - INTERVAL '7 days')"
		if sql != expected {
			t.Errorf("expected %q, got %q", expected, sql)
		}
	})
}

func TestRecursiveCTE(t *testing.T) {
	t.Run("simple recursive CTE", func(t *testing.T) {
		cte := NewRecursiveCTE("employee_hierarchy", []string{"id", "name", "manager_id", "level"}).
			BaseCase("SELECT id, name, manager_id, 1 FROM employees WHERE manager_id IS NULL").
			RecursiveCase("SELECT e.id, e.name, e.manager_id, eh.level + 1 FROM employees e JOIN employee_hierarchy eh ON e.manager_id = eh.id")

		sql, _ := cte.Build()
		expected := "WITH RECURSIVE employee_hierarchy (id, name, manager_id, level) AS (SELECT id, name, manager_id, 1 FROM employees WHERE manager_id IS NULL UNION ALL SELECT e.id, e.name, e.manager_id, eh.level + 1 FROM employees e JOIN employee_hierarchy eh ON e.manager_id = eh.id)"
		if sql != expected {
			t.Errorf("expected %q, got %q", expected, sql)
		}
	})
}

func TestJSONBPath(t *testing.T) {
	tests := []struct {
		name     string
		column   string
		path     []string
		expected string
	}{
		{
			name:     "single path",
			column:   "data",
			path:     []string{"user"},
			expected: "data->'user'",
		},
		{
			name:     "nested path",
			column:   "data",
			path:     []string{"user", "address", "city"},
			expected: "data->'user'->'address'->'city'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JSONBPath(tt.column, tt.path...)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestJSONBPathText(t *testing.T) {
	tests := []struct {
		name     string
		column   string
		path     []string
		expected string
	}{
		{
			name:     "single path as text",
			column:   "data",
			path:     []string{"email"},
			expected: "data->>'email'",
		},
		{
			name:     "nested path as text",
			column:   "data",
			path:     []string{"user", "email"},
			expected: "data->'user'->>'email'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JSONBPathText(tt.column, tt.path...)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPostgreSQLFunctions(t *testing.T) {
	tests := []struct {
		name     string
		function string
		expected string
	}{
		{
			name:     "ArrayAgg",
			function: ArrayAgg("user_id"),
			expected: "array_agg(user_id)",
		},
		{
			name:     "JSONBAgg",
			function: JSONBAgg("data"),
			expected: "jsonb_agg(data)",
		},
		{
			name:     "StringAgg",
			function: StringAgg("name", ", "),
			expected: "string_agg(name, ', ')",
		},
		{
			name:     "DateTrunc",
			function: DateTrunc("day", "created_at"),
			expected: "date_trunc('day', created_at)",
		},
		{
			name:     "ArrayLength",
			function: ArrayLength("tags", 1),
			expected: "array_length(tags, 1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.function != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.function)
			}
		})
	}
}

func TestCompositeTypes(t *testing.T) {
	t.Run("FormatComposite", func(t *testing.T) {
		result := schema.FormatComposite("123 Main St", "New York", "10001")
		expected := `("123 Main St","New York","10001")`
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})

	t.Run("ParseComposite", func(t *testing.T) {
		input := `("123 Main St","New York","10001")`
		result := schema.ParseComposite(input)
		expected := []string{"123 Main St", "New York", "10001"}

		if len(result) != len(expected) {
			t.Errorf("expected %d parts, got %d", len(expected), len(result))
			return
		}

		for i, exp := range expected {
			if result[i] != exp {
				t.Errorf("part %d: expected %q, got %q", i, exp, result[i])
			}
		}
	})

	t.Run("Point type", func(t *testing.T) {
		point := schema.Point{X: 1.5, Y: 2.5}
		sql := point.SQLValue()
		expected := "(1.5,2.5)"
		if sql != expected {
			t.Errorf("expected %q, got %q", expected, sql)
		}
	})
}
