package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// The stored forms are pg_get_constraintdef output from PostgreSQL 18.6 with the
// leading "CHECK " stripped, as the introspector returns them. Columns: kind
// varchar(30), s text, n integer, m numeric(10,2), d date, b boolean, e varchar.
func TestNormalizeCheckSQL_MatchesStoredForm(t *testing.T) {
	cases := []struct{ authored, stored string }{
		{`(kind IN ('consent','photo','capacity_assessment'))`, `(((kind)::text = ANY ((ARRAY['consent'::character varying, 'photo'::character varying, 'capacity_assessment'::character varying])::text[])))`},
		{`(kind NOT IN ('a','b'))`, `(((kind)::text <> ALL ((ARRAY['a'::character varying, 'b'::character varying])::text[])))`},
		{`(kind IN ('only'))`, `(((kind)::text = 'only'::text))`},
		{`(s IN ('x','y'))`, `((s = ANY (ARRAY['x'::text, 'y'::text])))`},
		{`(n BETWEEN 1 AND 5)`, `(((n >= 1) AND (n <= 5)))`},
		{`(e LIKE '%@%')`, `(((e)::text ~~ '%@%'::text))`},
		{`(e ILIKE '%@%')`, `(((e)::text ~~* '%@%'::text))`},
		{`(n != 0)`, `((n <> 0))`},
		{`(n > -1)`, `((n > '-1'::integer))`},
		{`(m >= 0)`, `((m >= (0)::numeric))`},
		{`(n IN (1,2,3))`, `((n = ANY (ARRAY[1, 2, 3])))`},
		{`(n > 0 AND n < 100 OR n IS NULL)`, `((((n > 0) AND (n < 100)) OR (n IS NULL)))`},
		{`(char_length(kind) > 0)`, `((char_length((kind)::text) > 0))`},
		{`(e ~* '^[^@]+@[^@]+$')`, `(((e)::text ~* '^[^@]+@[^@]+$'::text))`},
		{`(d <= CURRENT_DATE)`, `((d <= CURRENT_DATE))`},
		{`(s <> '')`, `((s <> ''::text))`},
		{`(m > 0.5)`, `((m > 0.5))`},
		{`(e NOT LIKE '% %')`, `(((e)::text !~~ '% %'::text))`},
		{`(b = true)`, `((b = true))`},
		{`(s IN ('It''s','b'))`, `((s = ANY (ARRAY['It''s'::text, 'b'::text])))`},
	}
	for _, c := range cases {
		if a, s := normalizeCheckSQL(c.authored), normalizeCheckSQL(c.stored); a != s {
			t.Errorf("%s\n  authored -> %q\n  stored   -> %q", c.authored, a, s)
		}
	}
}

func TestNormalizeCheckSQL_DetectsChanges(t *testing.T) {
	cases := []struct{ old, new string }{
		{`(((kind)::text = ANY ((ARRAY['consent'::character varying, 'capacity_assessment'::character varying])::text[])))`, `(kind IN ('consent','photo','capacity_assessment'))`},
		{`(s IN ('Active','b'))`, `(s IN ('active','b'))`},
		{`(n > 1)`, `(n > 2)`},
		{`(kind IN ('a','b'))`, `(kind NOT IN ('a','b'))`},
		{`(s IN ('a(b)'))`, `(s IN ('ab'))`},
	}
	for _, c := range cases {
		if normalizeCheckSQL(c.old) == normalizeCheckSQL(c.new) {
			t.Errorf("%s and %s normalize equal", c.old, c.new)
		}
	}
}

// A CHECK whose expression changed under the same name is dropped and re-added,
// drop first, and the down migration restores the old expression.
func TestCheckExpressionChangeIsPlanned(t *testing.T) {
	table := func(expr string) map[string]*schema.TableMetadata {
		return map[string]*schema.TableMetadata{"client_consents": {
			Name:        "client_consents",
			Columns:     []schema.ColumnMetadata{{Name: "kind", SQLType: "varchar(30)"}},
			Constraints: []schema.ConstraintMetadata{{Name: "client_consents_kind_check", Type: schema.CheckConstraint, Expression: expr}},
		}}
	}
	code := table(`(kind IN ('consent','photo'))`)
	db := table(`(((kind)::text = ANY ((ARRAY['consent'::character varying])::text[])))`)

	up, down := NewPlanner().GenerateMigration(NewDiffer().Compare(code, db))
	drop := "ALTER TABLE client_consents DROP CONSTRAINT IF EXISTS client_consents_kind_check;"
	if i, j := strings.Index(up, drop), strings.Index(up, "'photo'"); i < 0 || j < 0 || i > j {
		t.Fatalf("up must drop then re-add the CHECK:\n%s", up)
	}
	if i, j := strings.Index(down, drop), strings.Index(down, "ARRAY['consent'"); i < 0 || j < 0 || i > j {
		t.Fatalf("down must drop then restore the old CHECK:\n%s", down)
	}

	if diff := NewDiffer().Compare(code, table(`(kind IN ('consent','photo'))`)); diff.HasChanges() {
		t.Fatalf("an unchanged CHECK produced a diff: %+v", diff)
	}
}

// A migration written before pebble truncated generated names spells them in
// full; PostgreSQL stored them cut to 63 bytes, which is what the models now
// generate. Both must key the same, and a later DROP of the cut name must remove
// the full-name constraint from the reconstructed schema.
func TestLegacyFullLengthNamesMatchTruncated(t *testing.T) {
	const full = "fk_client_schedule_blocks_generation_id_client_schedule_generations"
	dir := t.TempDir()
	writeMigration(t, dir, "001_init.up.sql", `CREATE TABLE client_schedule_generations (id uuid PRIMARY KEY);
CREATE TABLE client_schedule_blocks (
    id uuid PRIMARY KEY,
    generation_id uuid,
    CONSTRAINT `+full+` FOREIGN KEY (generation_id) REFERENCES client_schedule_generations (id) ON DELETE CASCADE
);`)

	rebuilt, err := ReconstructSchemaFromMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	code := map[string]*schema.TableMetadata{}
	for name, tbl := range rebuilt {
		cp := *tbl
		cp.ForeignKeys = append([]schema.ForeignKeyMetadata(nil), tbl.ForeignKeys...)
		for i := range cp.ForeignKeys {
			cp.ForeignKeys[i].Name = schema.TruncateIdent(cp.ForeignKeys[i].Name)
		}
		code[name] = &cp
	}
	if diff := NewDiffer().Compare(code, rebuilt); diff.HasChanges() {
		t.Fatalf("truncated and full-length names diffed: %+v", diff.TablesModified)
	}

	writeMigration(t, dir, "002_drop.up.sql",
		"ALTER TABLE client_schedule_blocks DROP CONSTRAINT IF EXISTS "+schema.TruncateIdent(full)+";")
	rebuilt, err = ReconstructSchemaFromMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fks := rebuilt["client_schedule_blocks"].ForeignKeys; len(fks) != 0 {
		t.Fatalf("DROP CONSTRAINT of the truncated name left %v", fks)
	}
}

func writeMigration(t *testing.T, dir, name, sql string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
}
