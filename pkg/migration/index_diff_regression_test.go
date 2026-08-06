package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// A partial index authored as "read = false" must compare equal to PostgreSQL's
// stored form "(read = false)" so it does not phantom-diff.
func TestIsSameIndex_PartialPredicateParens(t *testing.T) {
	d := NewDiffer()
	code := schema.IndexMetadata{Name: "idx", Columns: []string{"user_id"}, Type: "btree", Where: "read = false"}
	db := schema.IndexMetadata{Name: "idx", Columns: []string{"user_id"}, Type: "btree", Where: "(read = false)"}
	if !d.isSameIndex(code, db) {
		t.Errorf("partial index predicate should be equal across paren-wrapping")
	}
	if d.isSameIndex(code, schema.IndexMetadata{Name: "idx", Columns: []string{"user_id"}, Type: "btree", Where: "read = true"}) {
		t.Errorf("genuinely different predicate must not compare equal")
	}
}

// A modified index (same name in both dropped and added) must emit DROP before
// CREATE in the up migration, else the recreate is immediately dropped.
func TestGenerateAlterTable_IndexModifyOrdering(t *testing.T) {
	p := NewPlanner()
	diff := TableDiff{
		TableName:      "notifications",
		IndexesDropped: []schema.IndexMetadata{{Name: "idx_n", Columns: []string{"user_id"}, Type: "btree", Where: "(read = false)"}},
		IndexesAdded:   []schema.IndexMetadata{{Name: "idx_n", Columns: []string{"user_id"}, Type: "btree", Where: "read = true"}},
	}
	up, down := p.generateAlterTable(diff)
	upJoined := strings.Join(up, "\n")
	dropAt := strings.Index(upJoined, "DROP INDEX IF EXISTS idx_n")
	createAt := strings.Index(upJoined, "CREATE INDEX")
	if dropAt < 0 || createAt < 0 || dropAt > createAt {
		t.Errorf("up must DROP before CREATE for a same-name index replace:\n%s", upJoined)
	}
	// Down reverses: DROP the added, then recreate the dropped.
	downJoined := strings.Join(down, "\n")
	if !strings.Contains(downJoined, "DROP INDEX IF EXISTS idx_n") || !strings.Contains(downJoined, "CREATE INDEX") {
		t.Errorf("down must both drop and recreate:\n%s", downJoined)
	}
}

// The offline reconstruct must replay CREATE INDEX (columns, ordering, USING,
// partial WHERE) and ALTER COLUMN SET DEFAULT so it does not phantom-diff.
func TestReconstruct_IndexesAndDefault(t *testing.T) {
	dir := t.TempDir()
	sql := `CREATE TABLE labels (
    id serial NOT NULL PRIMARY KEY,
    user_id integer NOT NULL,
    color varchar(7) NOT NULL DEFAULT '#cccccc',
    at timestamptz NOT NULL
);
CREATE INDEX idx_labels_user ON labels (user_id);
CREATE INDEX idx_labels_at ON labels (at DESC);
CREATE INDEX idx_labels_partial ON labels (user_id) WHERE color = '#000000';
ALTER TABLE labels ALTER COLUMN color SET DEFAULT '#dddddd';`
	if err := os.WriteFile(filepath.Join(dir, "0001_init.up.sql"), []byte(sql), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tables, err := ReconstructSchemaFromMigrations(dir)
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
	labels, ok := tables["labels"]
	if !ok {
		t.Fatalf("labels table not reconstructed")
	}

	if len(labels.Indexes) != 3 {
		t.Fatalf("expected 3 indexes, got %d: %+v", len(labels.Indexes), labels.Indexes)
	}
	byName := map[string]schema.IndexMetadata{}
	for _, idx := range labels.Indexes {
		byName[idx.Name] = idx
	}
	if got := byName["idx_labels_at"]; len(got.ColumnOrdering) != 1 || got.ColumnOrdering[0].Direction != schema.Descending {
		t.Errorf("idx_labels_at should carry DESC ordering: %+v", got)
	}
	if got := byName["idx_labels_partial"]; strings.TrimSpace(got.Where) == "" {
		t.Errorf("idx_labels_partial should carry a WHERE predicate: %+v", got)
	}

	col := labels.GetColumnByName("color")
	if col == nil || col.Default == nil || !strings.Contains(*col.Default, "#dddddd") {
		t.Errorf("color default should be replayed to #dddddd: %+v", col)
	}

	// A DROP INDEX later must remove it from the reconstructed schema.
	sql2 := sql + "\nDROP INDEX idx_labels_partial;"
	if err := os.WriteFile(filepath.Join(dir, "0001_init.up.sql"), []byte(sql2), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	tables2, err := ReconstructSchemaFromMigrations(dir)
	if err != nil {
		t.Fatalf("reconstruct 2: %v", err)
	}
	if len(tables2["labels"].Indexes) != 2 {
		t.Errorf("expected 2 indexes after DROP INDEX, got %d", len(tables2["labels"].Indexes))
	}
}
