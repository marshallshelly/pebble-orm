package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func reconstructCols(t *testing.T, colDef string) []string {
	t.Helper()
	dir := t.TempDir()
	sql := "CREATE TABLE IF NOT EXISTS probe (\n    id uuid NOT NULL PRIMARY KEY,\n    " + colDef + "\n);\n"
	if err := os.WriteFile(filepath.Join(dir, "0001_x.up.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	tables, err := ReconstructSchemaFromMigrations(dir)
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
	tbl, ok := tables["probe"]
	if !ok {
		t.Fatal("probe table not reconstructed")
	}
	names := make([]string, 0, len(tbl.Columns))
	for _, c := range tbl.Columns {
		names = append(names, c.Name)
	}
	return names
}

// A column whose name begins with a constraint keyword must not be mistaken for
// a table-level constraint and dropped. Dropping it made every offline generate
// re-emit ADD COLUMN, and the matching down migration DROP it.
func TestReconstructKeepsColumnsNamedLikeKeywords(t *testing.T) {
	tests := []struct {
		name   string
		colDef string
		want   string
	}{
		{"check prefix", "checked_on date NOT NULL", "checked_on"},
		{"check prefix, no underscore", "checksum text", "checksum"},
		{"checking", "checking boolean", "checking"},
		{"unique prefix", "unique_ref text", "unique_ref"},
		{"uniquely", "uniquely text", "uniquely"},
		{"constraint prefix", "constraint_name text", "constraint_name"},
		{"foreign prefix", "foreign_key_id uuid", "foreign_key_id"},
		{"primary prefix", "primary_contact text", "primary_contact"},
		{"references prefix", "referenced_id uuid", "referenced_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reconstructCols(t, tt.colDef)
			if len(got) != 2 || got[1] != tt.want {
				t.Errorf("columns = %v, want [id %s]", got, tt.want)
			}
		})
	}
}

// Real table-level constraints must still be recognised and not become columns.
func TestReconstructStillSkipsRealConstraints(t *testing.T) {
	tests := []struct{ name, clause string }{
		{"bare CHECK", "CHECK (id IS NOT NULL)"},
		{"bare CHECK no space", "CHECK(id IS NOT NULL)"},
		{"bare UNIQUE", "UNIQUE (id)"},
		{"bare FOREIGN KEY", "FOREIGN KEY (id) REFERENCES other (id)"},
		{"named CONSTRAINT CHECK", "CONSTRAINT probe_id_check CHECK (id IS NOT NULL)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reconstructCols(t, tt.clause)
			if len(got) != 1 || got[0] != "id" {
				t.Errorf("columns = %v, want just [id]: a constraint must not become a column", got)
			}
		})
	}
}
