package migration

import (
	"slices"
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func replay(t *testing.T, migrations ...string) map[string]*schema.TableMetadata {
	t.Helper()
	dir := t.TempDir()
	for i, sql := range migrations {
		writeMigration(t, dir, string(rune('a'+i))+".up.sql", sql)
	}
	tables, err := ReconstructSchemaFromMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	return tables
}

func enumValues(tables map[string]*schema.TableMetadata, name string) []string {
	return NewDiffer().collectEnumTypes(tables)[name].Values
}

const teamsTable = "CREATE TABLE teams (id integer PRIMARY KEY, tier tier NOT NULL);"

func TestReplayCreateType(t *testing.T) {
	tables := replay(t, `CREATE TYPE tier AS ENUM ('free', 'paid');`+teamsTable)
	if got := enumValues(tables, "tier"); !slices.Equal(got, []string{"free", "paid"}) {
		t.Errorf("values = %q", got)
	}
}

func TestReplayEnumValuesKeepTheirPunctuation(t *testing.T) {
	tables := replay(t, `CREATE TYPE note AS ENUM ('a, b', 'it''s', 'x (y)', '');`+teamsTable)
	if got := enumValues(tables, "note"); !slices.Equal(got, []string{"a, b", "it's", "x (y)", ""}) {
		t.Errorf("values = %q", got)
	}
}

func TestReplayIgnoresTypesThatAreNotEnums(t *testing.T) {
	tables := replay(t, `CREATE TYPE point2 AS (x integer, y integer);
CREATE TYPE span AS RANGE (subtype = integer);`+teamsTable)
	if got := NewDiffer().collectEnumTypes(tables); len(got) != 0 {
		t.Errorf("enums = %v", got)
	}
}

func TestReplayAddValue(t *testing.T) {
	tables := replay(t,
		`CREATE TYPE tier AS ENUM ('free');`+teamsTable,
		`ALTER TYPE tier ADD VALUE IF NOT EXISTS 'paid';
ALTER TYPE tier ADD VALUE IF NOT EXISTS 'paid';
ALTER TYPE tier ADD VALUE 'it''s' AFTER 'free';`)
	if got := enumValues(tables, "tier"); !slices.Equal(got, []string{"free", "paid", "it's"}) {
		t.Errorf("values = %q", got)
	}
}

func TestReplayDropType(t *testing.T) {
	tables := replay(t,
		`CREATE TYPE tier AS ENUM ('free');
CREATE TYPE gone AS ENUM ('x');`+teamsTable,
		`DROP TYPE IF EXISTS gone;`)
	enums := NewDiffer().collectEnumTypes(tables)
	if _, ok := enums["gone"]; ok || len(enums) != 1 {
		t.Errorf("enums = %v", enums)
	}
}

func TestReplayEnumAfterLeadingComment(t *testing.T) {
	tables := replay(t, "-- Migration: tiers\n-- Written by hand\n\nCREATE TYPE tier AS ENUM ('free');\n"+teamsTable)
	if got := enumValues(tables, "tier"); !slices.Equal(got, []string{"free"}) {
		t.Errorf("values = %q", got)
	}
}

func enumCode(values ...string) map[string]*schema.TableMetadata {
	return map[string]*schema.TableMetadata{"teams": {
		Name:       "teams",
		PrimaryKey: &schema.PrimaryKeyMetadata{Name: "teams_pkey", Columns: []string{"id"}},
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "integer"},
			{Name: "tier", SQLType: "tier", EnumType: "tier", EnumValues: values, Position: 1},
		},
		EnumTypes: []schema.EnumType{{Name: "tier", Values: values}},
	}}
}

// Generating from the models, then replaying that migration, must leave nothing
// to generate, however many times the type has changed.
func TestEnumRoundTripOffline(t *testing.T) {
	first, _ := NewPlanner().GenerateMigration(NewDiffer().Compare(enumCode("free", "paid"), map[string]*schema.TableMetadata{}))
	if diff := NewDiffer().Compare(enumCode("free", "paid"), replay(t, first)); diff.HasChanges() {
		t.Fatalf("unchanged models generated: %+v", diff.EnumTypesAdded)
	}

	second, _ := NewPlanner().GenerateMigration(NewDiffer().Compare(enumCode("free", "paid", "team"), replay(t, first)))
	if strings.Contains(second, "CREATE TYPE") || !strings.Contains(second, "ADD VALUE IF NOT EXISTS 'team'") {
		t.Fatalf("adding a value should only add that value:\n%s", second)
	}
	if diff := NewDiffer().Compare(enumCode("free", "paid", "team"), replay(t, first, second)); diff.HasChanges() {
		t.Fatalf("the added value was not replayed: %+v", diff.EnumTypesModified)
	}
}

func TestRemovedEnumIsDroppedAndRestored(t *testing.T) {
	first, _ := NewPlanner().GenerateMigration(NewDiffer().Compare(enumCode("free", "paid"), map[string]*schema.TableMetadata{}))

	code := enumCode("free")
	code["teams"].EnumTypes = nil
	code["teams"].Columns = code["teams"].Columns[:1]
	up, down := NewPlanner().GenerateMigration(NewDiffer().Compare(code, replay(t, first)))

	inOrder(t, up, "DROP COLUMN IF EXISTS tier", "DROP TYPE IF EXISTS tier")
	inOrder(t, down, "CREATE TYPE tier AS ENUM ('free', 'paid')", "ADD COLUMN tier tier")
}
