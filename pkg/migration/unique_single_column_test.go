package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// invoicesTable has one column made unique by the tag (number) and one by a
// `// unique:` directive (seq), the two ways a single column becomes unique.
func invoicesTable() *schema.TableMetadata {
	cols := []schema.ColumnMetadata{
		{Name: "id", SQLType: "integer", Nullable: false},
		{Name: "number", SQLType: "varchar(40)", Nullable: false, Unique: true},
		{Name: "seq", SQLType: "integer", Nullable: true},
	}
	t := &schema.TableMetadata{
		Name:       "invoices",
		Columns:    cols,
		PrimaryKey: &schema.PrimaryKeyMetadata{Name: "invoices_pkey", Columns: []string{"id"}},
	}
	t.Constraints = append(schema.UniqueConstraintsFor(t.Name, cols),
		*schema.ParseUniqueFromComment("// unique: uq_invoices_seq (seq)"))
	return t
}

func TestCreateTableSingleColumnUnique(t *testing.T) {
	sql := NewPlanner().generateCreateTable(invoicesTable())
	if !strings.Contains(sql, "CONSTRAINT uq_invoices_seq UNIQUE (seq)") {
		t.Errorf("a single-column // unique: directive must be declared:\n%s", sql)
	}
	if strings.Contains(sql, "invoices_number_key") || strings.Count(sql, "UNIQUE") != 2 {
		t.Errorf("a tag-unique column is declared once, inline:\n%s", sql)
	}
}

// Generating from the models, then replaying that migration offline, must
// leave nothing to generate.
func TestSingleColumnUniqueRoundTrip(t *testing.T) {
	code := map[string]*schema.TableMetadata{"invoices": invoicesTable()}
	up, _ := NewPlanner().GenerateMigration(NewDiffer().Compare(code, map[string]*schema.TableMetadata{}))

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0001_init.up.sql"), []byte(up), 0o644); err != nil {
		t.Fatal(err)
	}
	replayed, err := ReconstructSchemaFromMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if diff := NewDiffer().Compare(code, replayed); diff.HasChanges() {
		t.Fatalf("replaying the generated migration left changes: %+v\n%s", diff.TablesModified, up)
	}
}

// A differently named single-column UNIQUE added later does not mark the column
// unique, so recreating the table declares the constraint rather than an
// unnamed inline UNIQUE; dropping it leaves a tag-unique column unique.
func TestReconstructNamedSingleColumnUnique(t *testing.T) {
	dir := t.TempDir()
	sql := `CREATE TABLE invoices (
    id integer NOT NULL PRIMARY KEY,
    number varchar(40) NOT NULL UNIQUE,
    seq integer
);
ALTER TABLE invoices ADD CONSTRAINT uq_invoices_seq UNIQUE (seq);
ALTER TABLE invoices ADD CONSTRAINT uq_invoices_number UNIQUE (number);
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS uq_invoices_number;`
	if err := os.WriteFile(filepath.Join(dir, "0001_init.up.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	tables, err := ReconstructSchemaFromMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	unique := map[string]bool{}
	for _, c := range tables["invoices"].Columns {
		unique[c.Name] = c.Unique
	}
	if unique["seq"] || !unique["number"] {
		t.Errorf("column unique flags: %v", unique)
	}
	if got := NewPlanner().generateCreateTable(tables["invoices"]); !strings.Contains(got, "CONSTRAINT uq_invoices_seq UNIQUE (seq)") {
		t.Errorf("recreating the table must keep the named constraint:\n%s", got)
	}
}
