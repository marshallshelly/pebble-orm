package builder

import (
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
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

// TestPostgreSQLOperatorsToSQL verifies PG-specific operators and Raw
// conditions actually build SQL instead of returning "unknown operator".
func TestPostgreSQLOperatorsToSQL(t *testing.T) {
	tests := []struct {
		name     string
		cond     Condition
		wantSQL  string
		wantArgs int
	}{
		{"JSONBContains", JSONBContains("prefs", `{"a":1}`), "WHERE prefs @> $1", 1},
		{"JSONBHasKey", JSONBHasKey("prefs", "a"), "WHERE prefs ? $1", 1},
		{"JSONBHasAnyKey", JSONBHasAnyKey("prefs", []string{"a", "b"}), "WHERE prefs ?| $1", 1},
		{"ArrayContains", ArrayContains("tags", []string{"go"}), "WHERE tags @> $1", 1},
		{"ArrayOverlap", ArrayOverlap("tags", []string{"go"}), "WHERE tags && $1", 1},
		{"RegexpMatch", RegexpMatch("email", "^a"), "WHERE email ~ $1", 1},
		{"RegexpMatchInsensitive", RegexpMatchInsensitive("email", "^a"), "WHERE email ~* $1", 1},
		{"TSMatch", TSMatch("bio", "word"), "WHERE to_tsvector(bio) @@ to_tsquery($1)", 1},
		{"InSubquery", InSubquery("id", NewSubquery("SELECT id FROM t")), "WHERE id IN (SELECT id FROM t)", 0},
		{"ExistsSubquery", ExistsSubquery(NewSubquery("SELECT 1")), "WHERE EXISTS (SELECT 1)", 0},
		{"NotExistsSubquery", NotExistsSubquery(NewSubquery("SELECT 1")), "WHERE NOT EXISTS (SELECT 1)", 0},
		{"GtSubquery", GtSubquery("age", NewSubquery("SELECT AVG(age) FROM t")), "WHERE age > (SELECT AVG(age) FROM t)", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wb := NewWhereBuilder()
			wb.Add(tt.cond)
			sql, args, err := wb.Build()
			if err != nil {
				t.Fatalf("Build() error: %v", err)
			}
			if sql != tt.wantSQL {
				t.Errorf("SQL: got %q, want %q", sql, tt.wantSQL)
			}
			if len(args) != tt.wantArgs {
				t.Errorf("args: got %d, want %d", len(args), tt.wantArgs)
			}
		})
	}
}

// TestQuoteReservedIdentInBuilders verifies reserved-word table names are quoted.
func TestQuoteReservedIdentInBuilders(t *testing.T) {
	type ReservedUser struct {
		ID   int    `po:"id,primaryKey,serial"`
		Name string `po:"name,text"`
	}
	if err := registry.Register(ReservedUser{}); err != nil {
		t.Fatal(err)
	}
	// reserved_user is not reserved — check the helper directly plus a real
	// reserved name through schema metadata.
	if got := schema.QuoteReservedIdent("user"); got != `"user"` {
		t.Errorf(`QuoteReservedIdent("user") = %s, want "user" quoted`, got)
	}
	if got := schema.QuoteReservedIdent("users"); got != "users" {
		t.Errorf(`QuoteReservedIdent("users") = %s, want unquoted`, got)
	}
	if got := schema.QuoteReservedIdent("order"); got != `"order"` {
		t.Errorf(`QuoteReservedIdent("order") = %s, want quoted`, got)
	}
}

// TestTSMatchInjection verifies the full-text query is bound as a parameter,
// so a single-quote payload cannot break out of the SQL. Regression for the
// v1.17.0 injection (fixed in v1.17.1).
func TestTSMatchInjection(t *testing.T) {
	payload := `x') OR 1=1 --`
	wb := NewWhereBuilder()
	wb.Add(TSMatch("body", payload))
	sql, args, err := wb.Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if sql != "WHERE to_tsvector(body) @@ to_tsquery($1)" {
		t.Errorf("SQL: got %q, want parameterized to_tsquery($1)", sql)
	}
	if len(args) != 1 || args[0] != payload {
		t.Errorf("args: got %v, want the raw payload bound as one parameter", args)
	}
	if strings.Contains(sql, "OR 1=1") {
		t.Errorf("payload leaked into SQL text: %q", sql)
	}
}

func TestJSONBPathEscaping(t *testing.T) {
	// A single quote in a path segment must be escaped, not break the literal.
	got := JSONBPathText("data", "a'b")
	if got != "data->>'a''b'" {
		t.Errorf("got %q, want single quote doubled", got)
	}
	if got := ToTSQuery("a'b"); got != "to_tsquery('a''b')" {
		t.Errorf("ToTSQuery: got %q, want single quote doubled", got)
	}
}
