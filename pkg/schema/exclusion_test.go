package schema

import "testing"

func TestParseExclusionFromComment(t *testing.T) {
	c := ParseExclusionFromComment("// exclude: no_overlap USING gist (room_id WITH =, period WITH &&)")
	if c == nil {
		t.Fatal("expected a constraint, got nil")
	}
	if c.Name != "no_overlap" {
		t.Errorf("name: got %q", c.Name)
	}
	if c.Type != ExclusionConstraint {
		t.Errorf("type: got %q", c.Type)
	}
	if c.Expression != "USING gist (room_id WITH =, period WITH &&)" {
		t.Errorf("expression: got %q", c.Expression)
	}
	if ParseExclusionFromComment("// index: idx_x ON (a)") != nil {
		t.Error("non-exclusion comment should return nil")
	}
}

func TestParseCheckFromComment(t *testing.T) {
	c := ParseCheckFromComment("// check: positive balance >= 0")
	if c == nil || c.Name != "positive" || c.Type != CheckConstraint || c.Expression != "(balance >= 0)" {
		t.Fatalf("got %+v", c)
	}
	if ParseCheckFromComment("// exclude: x USING gist (a WITH =)") != nil {
		t.Error("exclusion comment should not parse as check")
	}
	if ParseCheckFromComment("// we should check: this later") != nil {
		t.Error("prose comment should not parse as check")
	}
}

func TestCheckConstraintsFor(t *testing.T) {
	cols := []ColumnMetadata{{Name: "age", Check: "age >= 0"}, {Name: "name"}}
	got := CheckConstraintsFor("users", cols)
	if len(got) != 1 {
		t.Fatalf("expected 1 check, got %d", len(got))
	}
	if got[0].Name != "users_age_check" || got[0].Expression != "(age >= 0)" || got[0].Type != CheckConstraint {
		t.Errorf("got %+v", got[0])
	}
}
