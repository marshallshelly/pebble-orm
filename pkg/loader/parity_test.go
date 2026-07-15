package loader_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/loader"
	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// The model used for the parity check. It exercises the tag options that used
// to diverge between the reflection parser and the AST loader: explicit types,
// serial/identity, defaults, unique, enum, generated, column index, and fk.
type Membership struct {
	ID        int64   `po:"id,primaryKey,identityAlways"`
	OrgID     int     `po:"org_id,integer,notNull,fk:orgs(id),onDelete:CASCADE,index"`
	Email     string  `po:"email,varchar(320),unique,notNull"`
	Role      MemRole `po:"role,enum(owner,admin,member),notNull"`
	FullName  string  `po:"full_name,text,generated(first_name || ' ' || last_name)"`
	Nickname  *string `po:"nickname,varchar(50)"`
	CreatedAt string  `po:"created_at,timestamptz,default(NOW()),notNull"`
}

type MemRole string

const paritySource = `package models

type MemRole string

// table_name: memberships
type Membership struct {
	ID        int64   ` + "`po:\"id,primaryKey,identityAlways\"`" + `
	OrgID     int     ` + "`po:\"org_id,integer,notNull,fk:orgs(id),onDelete:CASCADE,index\"`" + `
	Email     string  ` + "`po:\"email,varchar(320),unique,notNull\"`" + `
	Role      MemRole ` + "`po:\"role,enum(owner,admin,member),notNull\"`" + `
	FullName  string  ` + "`po:\"full_name,text,generated(first_name || ' ' || last_name)\"`" + `
	Nickname  *string ` + "`po:\"nickname,varchar(50)\"`" + `
	CreatedAt string  ` + "`po:\"created_at,timestamptz,default(NOW()),notNull\"`" + `
}
`

type captureRegistrar struct {
	tables map[string]*schema.TableMetadata
}

func (c *captureRegistrar) RegisterMetadata(t *schema.TableMetadata) error {
	c.tables[t.Name] = t
	return nil
}

// TestParserLoaderParity asserts the reflection parser and the AST loader
// produce identical TableMetadata for the same model, so the two code paths
// can never silently drift (the class of bug that dropped index/enum/generated
// from the CLI loader).
func TestParserLoaderParity(t *testing.T) {
	// Reflection path.
	registry.Clear()
	schema.RegisterTableName("Membership", "memberships")
	t.Cleanup(func() { registry.Clear() })
	if err := registry.Register(Membership{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	reflected, err := registry.GetByName("memberships")
	if err != nil {
		t.Fatalf("get reflected: %v", err)
	}

	// AST path.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "membership.go"), []byte(paritySource), 0644); err != nil {
		t.Fatal(err)
	}
	cap := &captureRegistrar{tables: map[string]*schema.TableMetadata{}}
	if _, err := loader.LoadModelsFromPath(dir, cap); err != nil {
		t.Fatalf("load: %v", err)
	}
	loaded := cap.tables["memberships"]
	if loaded == nil {
		t.Fatal("AST loader did not produce the memberships table")
	}

	// Guard against a false pass where both sides are equally empty: the model
	// deliberately has an FK, an index, an enum, a generated column and a
	// unique constraint, so each must actually be present.
	if len(reflected.ForeignKeys) != 1 {
		t.Errorf("expected 1 foreign key from reflection, got %d", len(reflected.ForeignKeys))
	}
	if len(reflected.Indexes) != 1 {
		t.Errorf("expected 1 index from reflection, got %d", len(reflected.Indexes))
	}
	if len(reflected.EnumTypes) != 1 {
		t.Errorf("expected 1 enum type from reflection, got %d", len(reflected.EnumTypes))
	}
	var hasGenerated bool
	for _, c := range reflected.Columns {
		if c.Generated != nil {
			hasGenerated = true
		}
	}
	if !hasGenerated {
		t.Error("expected a generated column from reflection")
	}

	assertColumnsEqual(t, reflected.Columns, loaded.Columns)
	assertPKEqual(t, reflected.PrimaryKey, loaded.PrimaryKey)
	assertFKsEqual(t, reflected.ForeignKeys, loaded.ForeignKeys)
	assertIndexesEqual(t, reflected.Indexes, loaded.Indexes)
	assertConstraintsEqual(t, reflected.Constraints, loaded.Constraints)
	assertEnumsEqual(t, reflected.EnumTypes, loaded.EnumTypes)
}

func assertColumnsEqual(t *testing.T, a, b []schema.ColumnMetadata) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("column count: reflected %d, loaded %d", len(a), len(b))
	}
	for i := range a {
		// GoType is only available via reflection; compare everything else.
		ca, cb := a[i], b[i]
		ca.GoType, cb.GoType = nil, nil
		if !reflect.DeepEqual(ca, cb) {
			t.Errorf("column %q differs:\n reflected %+v\n loaded    %+v", ca.Name, ca, cb)
		}
	}
}

func assertPKEqual(t *testing.T, a, b *schema.PrimaryKeyMetadata) {
	t.Helper()
	if !reflect.DeepEqual(a, b) {
		t.Errorf("primary key differs:\n reflected %+v\n loaded    %+v", a, b)
	}
}

func assertFKsEqual(t *testing.T, a, b []schema.ForeignKeyMetadata) {
	t.Helper()
	if !reflect.DeepEqual(a, b) {
		t.Errorf("foreign keys differ:\n reflected %+v\n loaded    %+v", a, b)
	}
}

func assertIndexesEqual(t *testing.T, a, b []schema.IndexMetadata) {
	t.Helper()
	sortIdx := func(s []schema.IndexMetadata) { sort.Slice(s, func(i, j int) bool { return s[i].Name < s[j].Name }) }
	sortIdx(a)
	sortIdx(b)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("indexes differ:\n reflected %+v\n loaded    %+v", a, b)
	}
}

func assertConstraintsEqual(t *testing.T, a, b []schema.ConstraintMetadata) {
	t.Helper()
	sortC := func(s []schema.ConstraintMetadata) {
		sort.Slice(s, func(i, j int) bool { return s[i].Name < s[j].Name })
	}
	sortC(a)
	sortC(b)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("constraints differ:\n reflected %+v\n loaded    %+v", a, b)
	}
}

func assertEnumsEqual(t *testing.T, a, b []schema.EnumType) {
	t.Helper()
	sortE := func(s []schema.EnumType) { sort.Slice(s, func(i, j int) bool { return s[i].Name < s[j].Name }) }
	sortE(a)
	sortE(b)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("enum types differ:\n reflected %+v\n loaded    %+v", a, b)
	}
}
