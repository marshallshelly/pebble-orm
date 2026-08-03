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
