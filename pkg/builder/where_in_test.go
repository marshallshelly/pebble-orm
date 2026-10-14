package builder

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func buildWhere(t *testing.T, conds ...Condition) (string, []interface{}) {
	t.Helper()
	wb := NewWhereBuilder()
	for _, c := range conds {
		wb.Add(c)
	}
	sql, args, err := wb.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return sql, args
}

func TestInExpandsASingleSlice(t *testing.T) {
	tests := []struct {
		name     string
		cond     Condition
		wantSQL  string
		wantArgs []interface{}
	}{
		{"variadic scalars", In("status", "active", "suspended"),
			"WHERE status IN ($1, $2)", []interface{}{"active", "suspended"}},
		{"[]any", In("status", []interface{}{"active", "suspended"}),
			"WHERE status IN ($1, $2)", []interface{}{"active", "suspended"}},
		{"[]string", In("status", []string{"active", "suspended"}),
			"WHERE status IN ($1, $2)", []interface{}{"active", "suspended"}},
		{"[]int", In("id", []int{1, 2, 3}),
			"WHERE id IN ($1, $2, $3)", []interface{}{1, 2, 3}},
		{"single scalar", In("id", 7),
			"WHERE id IN ($1)", []interface{}{7}},
		{"one-element slice", In("id", []string{"only"}),
			"WHERE id IN ($1)", []interface{}{"only"}},
		{"NotIn []string", NotIn("status", []string{"a", "b"}),
			"WHERE status NOT IN ($1, $2)", []interface{}{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := buildWhere(t, tt.cond)
			if sql != tt.wantSQL {
				t.Errorf("sql = %q, want %q", sql, tt.wantSQL)
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("args = %#v, want %#v", args, tt.wantArgs)
			}
			for i := range args {
				if args[i] != tt.wantArgs[i] {
					t.Errorf("args[%d] = %#v, want %#v", i, args[i], tt.wantArgs[i])
				}
			}
		})
	}
}

// A slice that represents one database value must not be expanded.
func TestInDoesNotExpandSingleValueSlices(t *testing.T) {
	tests := []struct {
		name string
		cond Condition
	}{
		{"[]byte is a bytea value", In("data", []byte("abc"))},
		{"driver.Valuer declares its own encoding", In("tags", schema.StringArray{"a", "b"})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := buildWhere(t, tt.cond)
			if sql != "WHERE "+tt.cond.Column+" IN ($1)" {
				t.Errorf("sql = %q, want a single placeholder", sql)
			}
			if len(args) != 1 {
				t.Errorf("args = %#v, want exactly one", args)
			}
		})
	}
}

func TestEmptyInIsFalseAndEmptyNotInIsTrue(t *testing.T) {
	var noIDs []interface{} // only []interface{} can spread into a ...interface{} parameter

	tests := []struct {
		name    string
		cond    Condition
		wantSQL string
	}{
		{"In with no values", In("id"), "WHERE FALSE"},
		{"NotIn with no values", NotIn("id"), "WHERE TRUE"},
		{"In with a spread empty slice", In("id", noIDs...), "WHERE FALSE"},
		{"In with an empty slice argument", In("id", []string{}), "WHERE FALSE"},
		{"NotIn with an empty slice argument", NotIn("id", []string{}), "WHERE TRUE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := buildWhere(t, tt.cond)
			if sql != tt.wantSQL {
				t.Errorf("sql = %q, want %q", sql, tt.wantSQL)
			}
			if len(args) != 0 {
				t.Errorf("args = %#v, want none", args)
			}
		})
	}
}

// An empty IN binds nothing, so it must not consume a placeholder number.
func TestEmptyInDoesNotConsumeAPlaceholder(t *testing.T) {
	tests := []struct {
		name     string
		conds    []Condition
		wantSQL  string
		wantArgs int
	}{
		{"empty IN after a bound condition",
			[]Condition{Eq("a", 1), In("id")},
			"WHERE a = $1 AND FALSE", 1},
		{"empty IN before a bound condition",
			[]Condition{In("id"), Eq("b", 2)},
			"WHERE FALSE AND b = $1", 1},
		{"empty IN between bound conditions",
			[]Condition{Eq("a", 1), In("id"), Eq("b", 2)},
			"WHERE a = $1 AND FALSE AND b = $2", 2},
		{"expanded IN keeps later numbering correct",
			[]Condition{Eq("a", 1), In("id", []int{7, 8}), Eq("b", 2)},
			"WHERE a = $1 AND id IN ($2, $3) AND b = $4", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := buildWhere(t, tt.conds...)
			if sql != tt.wantSQL {
				t.Errorf("sql  = %q\nwant = %q", sql, tt.wantSQL)
			}
			if len(args) != tt.wantArgs {
				t.Errorf("len(args) = %d, want %d (%#v)", len(args), tt.wantArgs, args)
			}
		})
	}
}

// InSubquery also carries OpIn, but as a Raw condition holding SQL rather than
// a value list. It must not go through IN-value expansion.
func TestInSubqueryIsUnaffectedByExpansion(t *testing.T) {
	sub := NewSubquery("SELECT user_id FROM orders WHERE status = $1", "paid")
	sql, args := buildWhere(t, Eq("active", true), InSubquery("id", sub))

	want := "WHERE active = $1 AND id IN (SELECT user_id FROM orders WHERE status = $2)"
	if sql != want {
		t.Errorf("sql  = %q\nwant = %q", sql, want)
	}
	if len(args) != 2 || args[0] != true || args[1] != "paid" {
		t.Errorf("args = %#v, want [true paid]", args)
	}
}
