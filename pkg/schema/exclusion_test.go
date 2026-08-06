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

func TestParseExtensionFromComment(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"// extension: uuid-ossp", "uuid-ossp"},
		{`// extension: "pg_trgm"`, "pg_trgm"},
		{"//extension:btree_gist", "btree_gist"},
		{"// EXTENSION: pgcrypto", "pgcrypto"},
		{"// check: x >= 0", ""},
		{"// this extension: is prose", ""},
	} {
		if got := ParseExtensionFromComment(tc.in); got != tc.want {
			t.Errorf("ParseExtensionFromComment(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseDomainFromComment(t *testing.T) {
	d := ParseDomainFromComment(`// domain: email_address AS text CHECK (VALUE ~* '^[^@]+@[^@]+$')`)
	if d == nil {
		t.Fatal("expected a domain, got nil")
	}
	if d.Name != "email_address" || d.BaseType != "text" {
		t.Errorf("name/base: got %q / %q", d.Name, d.BaseType)
	}
	if d.Check != `CHECK (VALUE ~* '^[^@]+@[^@]+$')` {
		t.Errorf("check: got %q", d.Check)
	}

	d2 := ParseDomainFromComment("// domain: positive_int AS integer")
	if d2 == nil || d2.Name != "positive_int" || d2.BaseType != "integer" || d2.Check != "" {
		t.Fatalf("no-check domain: got %+v", d2)
	}

	if ParseDomainFromComment("// this domain: is prose") != nil {
		t.Error("prose comment should not parse as domain")
	}
}

func TestGetSQLTypeDomain(t *testing.T) {
	opts, err := ParseTag("email,domain(email_address),notNull")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := opts.GetSQLType(); got != "email_address" {
		t.Errorf("GetSQLType = %q, want email_address", got)
	}
}

func TestParseUniqueFromComment(t *testing.T) {
	c := ParseUniqueFromComment("// unique: uq_team_member (team_id, user_id)")
	if c == nil {
		t.Fatal("expected a constraint, got nil")
	}
	if c.Name != "uq_team_member" || c.Type != UniqueConstraint {
		t.Errorf("name/type: got %q / %q", c.Name, c.Type)
	}
	if len(c.Columns) != 2 || c.Columns[0] != "team_id" || c.Columns[1] != "user_id" {
		t.Errorf("columns: got %v", c.Columns)
	}
	if ParseUniqueFromComment("// index: idx_x ON (a)") != nil {
		t.Error("non-unique comment should return nil")
	}
	if ParseUniqueFromComment("// this is unique: prose") != nil {
		t.Error("prose comment should not parse as unique")
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
