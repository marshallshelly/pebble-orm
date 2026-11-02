package migration

import (
	"slices"
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func orderTable(name string, refs ...string) *schema.TableMetadata {
	t := &schema.TableMetadata{
		Name:       name,
		PrimaryKey: &schema.PrimaryKeyMetadata{Name: name + "_pkey", Columns: []string{"id"}},
		Columns:    []schema.ColumnMetadata{{Name: "id", SQLType: "integer", Position: 0}},
	}
	for i, ref := range refs {
		col := ref + "_id"
		t.Columns = append(t.Columns, schema.ColumnMetadata{Name: col, SQLType: "integer", Nullable: true, Position: i + 1})
		t.ForeignKeys = append(t.ForeignKeys, schema.ForeignKeyMetadata{
			Name: "fk_" + name + "_" + col, Columns: []string{col},
			ReferencedTable: ref, ReferencedColumns: []string{"id"},
		})
	}
	return t
}

// richSchemas differ in every way a diff can report, each several times over.
func richSchemas() (code, db map[string]*schema.TableMetadata) {
	code, db = map[string]*schema.TableMetadata{}, map[string]*schema.TableMetadata{}
	for _, name := range []string{"zebra", "yak", "xerus", "wolf", "vole", "urchin"} {
		code[name] = orderTable(name)
		code[name].EnumTypes = []schema.EnumType{{Name: name + "_kind", Values: []string{"a"}}}
		code[name].Domains = []schema.DomainType{{Name: name + "_dom", BaseType: "text"}}
		db["old_"+name] = orderTable("old_" + name)
		db["old_"+name].EnumTypes = []schema.EnumType{{Name: "old_" + name + "_kind", Values: []string{"a"}}}
		db["old_"+name].Domains = []schema.DomainType{{Name: "old_" + name + "_dom", BaseType: "text"}}
	}
	for _, name := range []string{"users", "teams", "orders", "items"} {
		codeT, dbT := orderTable(name), orderTable(name)
		for i, c := range []string{"zip", "yard", "xray", "web", "van"} {
			codeT.Columns = append(codeT.Columns, schema.ColumnMetadata{Name: c, SQLType: "text", Nullable: true, Position: i + 1})
			dbT.Columns = append(dbT.Columns, schema.ColumnMetadata{Name: "old_" + c, SQLType: "text", Nullable: true, Position: i + 1})
			codeT.Indexes = append(codeT.Indexes, schema.IndexMetadata{Name: "idx_" + name + "_" + c, Columns: []string{c}})
			dbT.Indexes = append(dbT.Indexes, schema.IndexMetadata{Name: "idx_old_" + name + "_" + c, Columns: []string{"old_" + c}})
			codeT.ForeignKeys = append(codeT.ForeignKeys, schema.ForeignKeyMetadata{Name: "fk_" + name + "_" + c, Columns: []string{c}, ReferencedTable: "users", ReferencedColumns: []string{"id"}})
			dbT.ForeignKeys = append(dbT.ForeignKeys, schema.ForeignKeyMetadata{Name: "fk_old_" + name + "_" + c, Columns: []string{"old_" + c}, ReferencedTable: "users", ReferencedColumns: []string{"id"}})
			codeT.Constraints = append(codeT.Constraints, schema.ConstraintMetadata{Name: name + "_" + c + "_check", Type: schema.CheckConstraint, Expression: "(" + c + " <> '')"})
			dbT.Constraints = append(dbT.Constraints, schema.ConstraintMetadata{Name: "old_" + name + "_" + c + "_check", Type: schema.CheckConstraint, Expression: "(old_" + c + " <> '')"})
		}
		code[name], db[name] = codeT, dbT
	}
	return code, db
}

func TestMigrationOutputIsStable(t *testing.T) {
	code, db := richSchemas()
	wantUp, wantDown := NewPlanner().GenerateMigration(NewDiffer().Compare(code, db))
	if !strings.Contains(wantUp, "ADD CONSTRAINT") || !strings.Contains(wantUp, "CREATE TYPE") || !strings.Contains(wantUp, "CREATE DOMAIN") {
		t.Fatalf("the fixture should exercise every kind of change:\n%s", wantUp)
	}
	for range 50 {
		up, down := NewPlanner().GenerateMigration(NewDiffer().Compare(code, db))
		if up != wantUp || down != wantDown {
			t.Fatal("the same schemas generated a different migration")
		}
	}
}

func TestNewColumnsKeepDeclaredOrder(t *testing.T) {
	code, db := richSchemas()
	diff := NewDiffer().Compare(code, db)
	var added []string
	for _, c := range diff.TablesModified[0].ColumnsAdded {
		added = append(added, c.Name)
	}
	if want := []string{"zip", "yard", "xray", "web", "van"}; !slices.Equal(added, want) {
		t.Errorf("columns added = %q, want %q", added, want)
	}
}

// A referenced table has to outlive the tables that point at it, whatever the
// names sort to.
func TestDroppedTablesFollowForeignKeys(t *testing.T) {
	db := map[string]*schema.TableMetadata{
		"accounts": orderTable("accounts"),
		"zones":    orderTable("zones", "accounts"),
		"banks":    orderTable("banks", "zones"),
	}
	up, down := NewPlanner().GenerateMigration(NewDiffer().Compare(map[string]*schema.TableMetadata{}, db))

	order := func(sql string, names ...string) {
		t.Helper()
		last := -1
		for _, name := range names {
			at := strings.Index(sql, name)
			if at < last {
				t.Errorf("%s out of order in:\n%s", name, sql)
			}
			last = at
		}
	}
	order(up, `"banks"`, `"zones"`, `"accounts"`)
	order(down, "EXISTS accounts", "EXISTS zones", "EXISTS banks")
}
