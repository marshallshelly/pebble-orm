package loader_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/loader"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// One file dot-imports database/sql; a sibling declares its own NullString.
// If sqlImports leaked across files, the sibling's column would become nullable.
func TestImportsDoNotLeakAcrossFiles(t *testing.T) {
	dir := t.TempDir()

	dotted := "package models\n\n" +
		"import . \"database/sql\"\n\n" +
		"// table_name: dotted\n" +
		"type Dotted struct {\n" +
		"\tID string     `po:\"id,primaryKey\"`\n" +
		"\tD  NullString `po:\"d,text,notNull\"`\n" +
		"}\n"

	sibling := "package models\n\n" +
		"type NullString string\n\n" +
		"// table_name: siblings\n" +
		"type Sibling struct {\n" +
		"\tID string     `po:\"id,primaryKey\"`\n" +
		"\tS  NullString `po:\"s,text,notNull\"`\n" +
		"}\n"

	// a.go sorts before b.go, so the dotted file is parsed first
	if err := os.WriteFile(filepath.Join(dir, "a_dotted.go"), []byte(dotted), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b_sibling.go"), []byte(sibling), 0o644); err != nil {
		t.Fatal(err)
	}

	c := &tagCollector{}
	if _, err := loader.LoadModelsFromPath(dir, c); err != nil {
		t.Fatalf("LoadModelsFromPath: %v", err)
	}

	byName := map[string]*schema.TableMetadata{}
	for _, tbl := range c.tables {
		byName[tbl.Name] = tbl
	}

	want := map[string]map[string]bool{
		"dotted":   {"d": true},  // real sql.NullString -> nullable
		"siblings": {"s": false}, // the file's own type -> notNull stands
	}
	for table, cols := range want {
		tbl, ok := byName[table]
		if !ok {
			t.Fatalf("table %q not registered (got %v)", table, byName)
		}
		for _, col := range tbl.Columns {
			if exp, tracked := cols[col.Name]; tracked && col.Nullable != exp {
				t.Errorf("%s.%s Nullable = %t, want %t", table, col.Name, col.Nullable, exp)
			}
		}
	}
}
