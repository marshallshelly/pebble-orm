package loader_test

import (
	stdsql "database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/loader"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

type tagCollector struct{ tables []*schema.TableMetadata }

func (c *tagCollector) RegisterMetadata(t *schema.TableMetadata) error {
	c.tables = append(c.tables, t)
	return nil
}

func loadOneTable(t *testing.T, src string) *schema.TableMetadata {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "models.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	c := &tagCollector{}
	if _, err := loader.LoadModelsFromPath(dir, c); err != nil {
		t.Fatalf("LoadModelsFromPath: %v", err)
	}
	if len(c.tables) != 1 {
		t.Fatalf("registered %d tables, want 1", len(c.tables))
	}
	return c.tables[0]
}

func columnNames(table *schema.TableMetadata) []string {
	out := make([]string, 0, len(table.Columns))
	for _, c := range table.Columns {
		out = append(out, c.Name)
	}
	return out
}

// A struct tag may be any string literal. A double-quoted one used to be
// invisible to the AST loader, which dropped the column from generated SQL.
type DoubleQuoted struct {
	A string `po:"a,primaryKey"`
	B string "po:\"b,varchar(10)\""
}

func TestParity_DoubleQuotedTagLiteral(t *testing.T) {
	src := "package models\n\n" +
		"// table_name: double_quoteds\n" +
		"type DoubleQuoted struct {\n" +
		"\tA string `po:\"a,primaryKey\"`\n" +
		"\tB string \"po:\\\"b,varchar(10)\\\"\"\n" +
		"}\n"

	fromAST := loadOneTable(t, src)

	fromReflect, err := schema.NewParser().Parse(reflect.TypeOf(DoubleQuoted{}))
	if err != nil {
		t.Fatalf("reflection parse: %v", err)
	}

	gotAST, gotReflect := columnNames(fromAST), columnNames(fromReflect)
	if !reflect.DeepEqual(gotAST, gotReflect) {
		t.Errorf("columns differ\n  AST        = %v\n  reflection = %v", gotAST, gotReflect)
	}
	if len(gotAST) != 2 {
		t.Errorf("columns = %v, want both a and b", gotAST)
	}

	for i := range fromAST.Columns {
		if a, r := fromAST.Columns[i].SQLType, fromReflect.Columns[i].SQLType; a != r {
			t.Errorf("column %q SQLType: AST=%q reflection=%q", fromAST.Columns[i].Name, a, r)
		}
	}
}

// astNullable matched the package qualifier by name, so an aliased
// database/sql import defeated it. Only observable when the tag also says
// notNull, since a column is otherwise nullable by default.
type AliasedNull struct {
	ID string            `po:"id,primaryKey"`
	D  stdsql.NullString `po:"d,text,notNull"`
}

func TestParity_AliasedSQLImportNullability(t *testing.T) {
	src := "package models\n\n" +
		"import stdsql \"database/sql\"\n\n" +
		"// table_name: aliased_nulls\n" +
		"type AliasedNull struct {\n" +
		"\tID string            `po:\"id,primaryKey\"`\n" +
		"\tD  stdsql.NullString `po:\"d,text,notNull\"`\n" +
		"}\n"

	fromAST := loadOneTable(t, src)

	fromReflect, err := schema.NewParser().Parse(reflect.TypeOf(AliasedNull{}))
	if err != nil {
		t.Fatalf("reflection parse: %v", err)
	}

	for i := range fromAST.Columns {
		name := fromAST.Columns[i].Name
		if a, r := fromAST.Columns[i].Nullable, fromReflect.Columns[i].Nullable; a != r {
			t.Errorf("column %q Nullable: AST=%t reflection=%t", name, a, r)
		}
	}
}

// The unaliased form must keep working.
func TestParity_UnaliasedSQLImportNullability(t *testing.T) {
	src := "package models\n\n" +
		"import \"database/sql\"\n\n" +
		"// table_name: plain_nulls\n" +
		"type PlainNull struct {\n" +
		"\tID string         `po:\"id,primaryKey\"`\n" +
		"\tD  sql.NullString `po:\"d,text,notNull\"`\n" +
		"}\n"

	table := loadOneTable(t, src)
	for _, c := range table.Columns {
		if c.Name == "d" && !c.Nullable {
			t.Error("sql.NullString with notNull should still be nullable")
		}
	}
}

// A dot-imported sql.NullString arrives as a bare identifier, not a selector.
type DotNull struct {
	ID string            `po:"id,primaryKey"`
	D  stdsql.NullString `po:"d,text,notNull"`
}

func TestParity_DotImportedSQLNullability(t *testing.T) {
	src := "package models\n\n" +
		"import . \"database/sql\"\n\n" +
		"// table_name: dot_nulls\n" +
		"type DotNull struct {\n" +
		"\tID string     `po:\"id,primaryKey\"`\n" +
		"\tD  NullString `po:\"d,text,notNull\"`\n" +
		"}\n"

	fromAST := loadOneTable(t, src)

	fromReflect, err := schema.NewParser().Parse(reflect.TypeOf(DotNull{}))
	if err != nil {
		t.Fatalf("reflection parse: %v", err)
	}

	for i := range fromAST.Columns {
		name := fromAST.Columns[i].Name
		if a, r := fromAST.Columns[i].Nullable, fromReflect.Columns[i].Nullable; a != r {
			t.Errorf("column %q Nullable: AST=%t reflection=%t", name, a, r)
		}
	}
}

// Matching bare identifiers on a "Null" prefix would catch a user's own type.
// Only the real database/sql names may be treated as nullable.
type Nullish string

type CollidingName struct {
	ID string  `po:"id,primaryKey"`
	N  Nullish `po:"n,text,notNull"`
}

func TestParity_DotImportDoesNotMatchUserTypes(t *testing.T) {
	src := "package models\n\n" +
		"import . \"database/sql\"\n\n" +
		"type Nullish string\n\n" +
		"// table_name: colliding_names\n" +
		"type CollidingName struct {\n" +
		"\tID string  `po:\"id,primaryKey\"`\n" +
		"\tN  Nullish `po:\"n,text,notNull\"`\n" +
		"}\n"

	fromAST := loadOneTable(t, src)

	fromReflect, err := schema.NewParser().Parse(reflect.TypeOf(CollidingName{}))
	if err != nil {
		t.Fatalf("reflection parse: %v", err)
	}

	for i := range fromAST.Columns {
		name := fromAST.Columns[i].Name
		if a, r := fromAST.Columns[i].Nullable, fromReflect.Columns[i].Nullable; a != r {
			t.Errorf("column %q Nullable: AST=%t reflection=%t (a user type must not match)", name, a, r)
		}
	}
	for _, c := range fromAST.Columns {
		if c.Name == "n" && c.Nullable {
			t.Error("Nullish is a user type, not a sql.Null*; notNull must stand")
		}
	}
}

// A blank import puts nothing in scope.
func TestParity_BlankImportIsNotDotImport(t *testing.T) {
	src := "package models\n\n" +
		"import _ \"database/sql\"\n\n" +
		"type NullString string\n\n" +
		"// table_name: blanks\n" +
		"type Blank struct {\n" +
		"\tID string     `po:\"id,primaryKey\"`\n" +
		"\tD  NullString `po:\"d,text,notNull\"`\n" +
		"}\n"

	for _, c := range loadOneTable(t, src).Columns {
		if c.Name == "d" && c.Nullable {
			t.Error("a blank import puts nothing in scope; the local NullString must not be treated as sql.NullString")
		}
	}
}

// hasPebbleTags gates whether a struct is processed, and it tests for the
// substring "po:" — which a double-quoted tag still contains. So a struct whose
// tags are ALL double-quoted used to pass the gate and then lose every field,
// producing CREATE TABLE x () — valid SQL that applies and leaves a useless table.
type OnlyDoubleQuoted struct {
	ID   string "po:\"id,primaryKey\""
	Name string "po:\"name,varchar(50)\""
}

func TestParity_StructWithOnlyDoubleQuotedTags(t *testing.T) {
	src := "package models\n\n" +
		"// table_name: only_double_quoteds\n" +
		"type OnlyDoubleQuoted struct {\n" +
		"\tID   string \"po:\\\"id,primaryKey\\\"\"\n" +
		"\tName string \"po:\\\"name,varchar(50)\\\"\"\n" +
		"}\n"

	fromAST := loadOneTable(t, src)
	if len(fromAST.Columns) == 0 {
		t.Fatal("no columns: the table would generate as CREATE TABLE x (), which Postgres accepts")
	}

	fromReflect, err := schema.NewParser().Parse(reflect.TypeOf(OnlyDoubleQuoted{}))
	if err != nil {
		t.Fatalf("reflection parse: %v", err)
	}

	gotAST, gotReflect := columnNames(fromAST), columnNames(fromReflect)
	if !reflect.DeepEqual(gotAST, gotReflect) {
		t.Errorf("columns differ\n  AST        = %v\n  reflection = %v", gotAST, gotReflect)
	}
}
