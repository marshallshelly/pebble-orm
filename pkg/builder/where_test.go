package builder

import (
	"strings"
	"testing"
)

func TestWhereBuilder_Build(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []Condition
		expectedSQL    string
		expectedArgLen int
	}{
		{
			name:           "empty conditions",
			conditions:     []Condition{},
			expectedSQL:    "",
			expectedArgLen: 0,
		},
		{
			name: "single equality condition",
			conditions: []Condition{
				Eq("age", 25),
			},
			expectedSQL:    "WHERE age = $1",
			expectedArgLen: 1,
		},
		{
			name: "multiple AND conditions",
			conditions: []Condition{
				Eq("age", 25),
				Eq("name", "John"),
			},
			expectedSQL:    "WHERE age = $1 AND name = $2",
			expectedArgLen: 2,
		},
		{
			name: "OR condition",
			conditions: []Condition{
				Eq("age", 25),
				Or(Eq("age", 30)),
			},
			expectedSQL:    "WHERE age = $1 OR age = $2",
			expectedArgLen: 2,
		},
		{
			name: "IN condition",
			conditions: []Condition{
				In("status", "active", "pending", "completed"),
			},
			expectedSQL:    "WHERE status IN ($1, $2, $3)",
			expectedArgLen: 3,
		},
		{
			name: "IS NULL condition",
			conditions: []Condition{
				IsNull("deleted_at"),
			},
			expectedSQL:    "WHERE deleted_at IS NULL",
			expectedArgLen: 0,
		},
		{
			name: "IS NOT NULL condition",
			conditions: []Condition{
				IsNotNull("email"),
			},
			expectedSQL:    "WHERE email IS NOT NULL",
			expectedArgLen: 0,
		},
		{
			name: "LIKE condition",
			conditions: []Condition{
				Like("name", "%John%"),
			},
			expectedSQL:    "WHERE name LIKE $1",
			expectedArgLen: 1,
		},
		{
			name: "BETWEEN condition",
			conditions: []Condition{
				Between("age", 18, 65),
			},
			expectedSQL:    "WHERE age BETWEEN $1 AND $2",
			expectedArgLen: 2,
		},
		{
			name: "NOT condition",
			conditions: []Condition{
				Not(Eq("status", "deleted")),
			},
			expectedSQL:    "WHERE NOT (status = $1)",
			expectedArgLen: 1,
		},
		{
			name: "complex mixed conditions",
			conditions: []Condition{
				Eq("active", true),
				Gt("age", 18),
				Or(Like("name", "%admin%")),
			},
			expectedSQL:    "WHERE active = $1 AND age > $2 OR name LIKE $3",
			expectedArgLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wb := NewWhereBuilder()
			for _, cond := range tt.conditions {
				wb.Add(cond)
			}

			sql, args, err := wb.Build()
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			if sql != tt.expectedSQL {
				t.Errorf("Build() sql = %v, want %v", sql, tt.expectedSQL)
			}

			if len(args) != tt.expectedArgLen {
				t.Errorf("Build() args length = %v, want %v", len(args), tt.expectedArgLen)
			}
		})
	}
}

func TestConditionHelpers(t *testing.T) {
	t.Run("Eq creates equality condition", func(t *testing.T) {
		cond := Eq("name", "John")
		if cond.Column != "name" || cond.Operator != OpEqual || cond.Value != "John" {
			t.Error("Eq() did not create correct condition")
		}
	})

	t.Run("NotEq creates not-equal condition", func(t *testing.T) {
		cond := NotEq("status", "deleted")
		if cond.Operator != OpNotEqual {
			t.Error("NotEq() did not set correct operator")
		}
	})

	t.Run("Gt creates greater-than condition", func(t *testing.T) {
		cond := Gt("age", 18)
		if cond.Operator != OpGreaterThan {
			t.Error("Gt() did not set correct operator")
		}
	})

	t.Run("Gte creates greater-than-or-equal condition", func(t *testing.T) {
		cond := Gte("score", 100)
		if cond.Operator != OpGreaterThanOrEqual {
			t.Error("Gte() did not set correct operator")
		}
	})

	t.Run("Lt creates less-than condition", func(t *testing.T) {
		cond := Lt("price", 50)
		if cond.Operator != OpLessThan {
			t.Error("Lt() did not set correct operator")
		}
	})

	t.Run("Lte creates less-than-or-equal condition", func(t *testing.T) {
		cond := Lte("quantity", 10)
		if cond.Operator != OpLessThanOrEqual {
			t.Error("Lte() did not set correct operator")
		}
	})

	t.Run("In creates IN condition", func(t *testing.T) {
		cond := In("status", "active", "pending")
		if cond.Operator != OpIn {
			t.Error("In() did not set correct operator")
		}
	})

	t.Run("Like creates LIKE condition", func(t *testing.T) {
		cond := Like("name", "%test%")
		if cond.Operator != OpLike {
			t.Error("Like() did not set correct operator")
		}
	})

	t.Run("ILike creates ILIKE condition", func(t *testing.T) {
		cond := ILike("email", "%@example.com")
		if cond.Operator != OpILike {
			t.Error("ILike() did not set correct operator")
		}
	})

	t.Run("Or sets OR logic", func(t *testing.T) {
		cond := Or(Eq("age", 25))
		if cond.Logic != LogicOr {
			t.Error("Or() did not set correct logic operator")
		}
	})

	t.Run("Not negates condition", func(t *testing.T) {
		cond := Not(Eq("deleted", true))
		if !cond.Not {
			t.Error("Not() did not set Not flag")
		}
	})

	t.Run("Between creates BETWEEN condition", func(t *testing.T) {
		cond := Between("age", 18, 65)
		if cond.Operator != OpBetween {
			t.Error("Between() did not set correct operator")
		}

		values, ok := cond.Value.([]interface{})
		if !ok || len(values) != 2 {
			t.Error("Between() did not create correct value array")
		}
	})
}

func TestGroupedConditions(t *testing.T) {
	t.Run("grouped conditions with AND", func(t *testing.T) {
		wb := NewWhereBuilder()
		wb.Add(Eq("active", true))
		wb.Add(Group(
			Eq("role", "admin"),
			Or(Eq("role", "moderator")),
		))

		sql, args, err := wb.Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}

		if !strings.Contains(sql, "(") || !strings.Contains(sql, ")") {
			t.Error("Expected parentheses in grouped condition SQL")
		}

		if len(args) != 3 {
			t.Errorf("Expected 3 args, got %d", len(args))
		}
	})
}
