package loader_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/loader"
	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

type IgnoredWidget struct {
	ID         int    `po:"id,primaryKey,integer"`
	Name       string `po:"name,varchar(120),notNull"`
	CarerCount int    `po:"-" json:"carerCount"`
	Note       string `po:"-,varchar(20)"`
	OwnerID    string `po:"-,uuid,fk:users(id),index"`
}

const ignoredSource = `package models

// table_name: ignored_widgets
type IgnoredWidget struct {
	ID         int    ` + "`po:\"id,primaryKey,integer\"`" + `
	Name       string ` + "`po:\"name,varchar(120),notNull\"`" + `
	CarerCount int    ` + "`po:\"-\" json:\"carerCount\"`" + `
	Note       string ` + "`po:\"-,varchar(20)\"`" + `
	OwnerID    string ` + "`po:\"-,uuid,fk:users(id),index\"`" + `
}
`

func loadSource(t *testing.T, source string) (*captureRegistrar, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "model.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &captureRegistrar{tables: map[string]*schema.TableMetadata{}}
	_, err := loader.LoadModelsFromPath(dir, c)
	return c, err
}

// The runtime parser and the CLI's loader must agree on what a model's columns
// are: migrations come from one and queries from the other.
func TestIgnoredFieldParity(t *testing.T) {
	registry.Clear()
	schema.RegisterTableName("IgnoredWidget", "ignored_widgets")
	t.Cleanup(func() { registry.Clear() })
	if err := registry.Register(IgnoredWidget{}); err != nil {
		t.Fatal(err)
	}
	reflected, err := registry.GetByName("ignored_widgets")
	if err != nil {
		t.Fatal(err)
	}

	c, err := loadSource(t, ignoredSource)
	if err != nil {
		t.Fatal(err)
	}
	loaded := c.tables["ignored_widgets"]
	if loaded == nil {
		t.Fatal("the loader produced no table")
	}

	want := []string{"id", "name"}
	if got := columnNames(reflected); !reflect.DeepEqual(got, want) {
		t.Errorf("runtime columns = %q, want %q", got, want)
	}
	if got := columnNames(loaded); !reflect.DeepEqual(got, want) {
		t.Errorf("loader columns = %q, want %q", got, want)
	}
	for name, table := range map[string]*schema.TableMetadata{"runtime": reflected, "loader": loaded} {
		if len(table.ForeignKeys) != 0 || len(table.Indexes) != 0 {
			t.Errorf("%s: an excluded field produced %+v, %+v", name, table.ForeignKeys, table.Indexes)
		}
	}
}

func TestLoaderRejectsMissingColumnName(t *testing.T) {
	_, err := loadSource(t, `package models

type Widget struct {
	ID   int    `+"`po:\"id,primaryKey,integer\"`"+`
	Note string `+"`po:\",varchar(20)\"`"+`
}
`)
	if err == nil {
		t.Fatal("a tag with no column name should stop the load")
	}
	for _, want := range []string{"Widget", "Note"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should name %s: %v", want, err)
		}
	}
}

func TestStructWithoutColumnsIsNotATable(t *testing.T) {
	c, err := loadSource(t, `package models

type Stats struct {
	Count int `+"`po:\"-\"`"+`
}

type Parent struct {
	Children []Child `+"`po:\"-,hasMany,foreignKey(parent_id),references(id)\"`"+`
}

type Child struct {
	ID int `+"`po:\"id,primaryKey,integer\"`"+`
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.tables) != 1 || c.tables["child"] == nil {
		t.Errorf("tables = %v, want only child", c.tables)
	}
}

func TestLoaderRejectsMalformedTag(t *testing.T) {
	_, err := loadSource(t, `package models

type Widget struct {
	ID   int    `+"`po:\"id,primaryKey,integer\"`"+`
	Note string `+"`po:\"note,varchar(20\"`"+`
}
`)
	if err == nil {
		t.Fatal("a malformed tag should stop the load, not drop the column")
	}
	for _, want := range []string{"Widget", "Note"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should name %s: %v", want, err)
		}
	}
}
