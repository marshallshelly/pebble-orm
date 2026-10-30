package builder

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// fakeRows serves each row's values to Scan in result order. A nil row leaves
// the targets alone, for measuring the scanner without any decoding.
type fakeRows struct {
	pgx.Rows
	columns      []string
	row          []any
	descriptions int
}

func (f *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	f.descriptions++
	out := make([]pgconn.FieldDescription, len(f.columns))
	for i, name := range f.columns {
		out[i].Name = name
	}
	return out
}

func (f *fakeRows) Scan(dest ...any) error {
	if f.row == nil {
		return nil
	}
	if len(dest) != len(f.row) {
		return fmt.Errorf("%d targets for %d columns", len(dest), len(f.row))
	}
	for i, d := range dest {
		if s, ok := d.(sql.Scanner); ok {
			if err := s.Scan(f.row[i]); err != nil {
				return err
			}
			continue
		}
		if strs, ok := d.(*[]string); ok && cap(*strs) >= len(f.row[i].([]string)) {
			// Decode into existing storage, the worst a driver may do.
			*strs = (*strs)[:len(f.row[i].([]string))]
			copy(*strs, f.row[i].([]string))
			continue
		}
		if f.row[i] != nil {
			reflect.ValueOf(d).Elem().Set(reflect.ValueOf(f.row[i]))
		}
	}
	return nil
}

type scanAudit struct {
	CreatedBy string
}

type scanSettings struct {
	Theme string `json:"theme"`
}

type scanModel struct {
	scanAudit
	ID       int
	Name     string
	Tags     schema.StringArray
	Settings *scanSettings
	hidden   string
}

var scanTable = &schema.TableMetadata{Name: "models", Columns: []schema.ColumnMetadata{
	{Name: "id", GoField: "ID"},
	{Name: "name", GoField: "Name"},
	{Name: "tags", GoField: "Tags", SQLType: "text[]"},
	{Name: "settings", GoField: "Settings", IsJSONB: true},
	{Name: "created_by", GoField: "CreatedBy"},
	{Name: "hidden", GoField: "hidden"},
	{Name: "not_selected", GoField: "Name"},
}}

func TestRowScannerPlansOncePerResult(t *testing.T) {
	rows := &fakeRows{columns: []string{"id", "name"}}
	scanner := newRowScanner(scanTable)
	for i := range 3 {
		rows.row = []any{i, fmt.Sprint("row ", i)}
		var m scanModel
		if err := scanner.scan(rows, &m); err != nil {
			t.Fatal(err)
		}
		if m.ID != i || m.Name != fmt.Sprint("row ", i) {
			t.Errorf("row %d: %+v", i, m)
		}
	}
	if rows.descriptions != 1 {
		t.Errorf("columns were resolved %d times, want once", rows.descriptions)
	}
}

func TestRowScannerColumnMapping(t *testing.T) {
	rows := &fakeRows{
		columns: []string{"id", "extra", "name", "name", "created_by", "hidden"},
		row:     []any{1, "ignored", "first", "last", "ann", "secret"},
	}
	var m scanModel
	if err := newRowScanner(scanTable).scan(rows, &m); err != nil {
		t.Fatal(err)
	}
	if m.ID != 1 || m.Name != "last" {
		t.Errorf("a repeated column takes the last value: %+v", m)
	}
	if m.CreatedBy != "ann" {
		t.Errorf("a field promoted from an embedded struct is set: %+v", m)
	}
	if m.hidden != "" {
		t.Errorf("an unexported field stays unset: %+v", m)
	}
}

func TestRowScannerKeepsRowsApart(t *testing.T) {
	rows := &fakeRows{columns: []string{"tags", "settings"}}
	scanner := newRowScanner(scanTable)

	first := []string{"a", "b"}
	rows.row = []any{first, []byte(`{"theme":"dark"}`)}
	var a scanModel
	if err := scanner.scan(rows, &a); err != nil {
		t.Fatal(err)
	}

	rows.row = []any{[]string{"c"}, nil}
	var b scanModel
	if err := scanner.scan(rows, &b); err != nil {
		t.Fatal(err)
	}

	if a.Settings == nil || a.Settings.Theme != "dark" {
		t.Errorf("first row settings: %+v", a.Settings)
	}
	if b.Settings != nil {
		t.Errorf("a NULL JSONB column inherited the previous row: %+v", b.Settings)
	}
	if !reflect.DeepEqual([]string(a.Tags), []string{"a", "b"}) || !reflect.DeepEqual([]string(b.Tags), []string{"c"}) {
		t.Errorf("array columns: %v, %v", a.Tags, b.Tags)
	}
}

func TestRowScannerJSONBError(t *testing.T) {
	rows := &fakeRows{columns: []string{"settings"}, row: []any{[]byte(`{`)}}
	var m scanModel
	err := newRowScanner(scanTable).scan(rows, &m)
	var syntax *json.SyntaxError
	if !errors.As(err, &syntax) {
		t.Errorf("malformed JSONB: %v", err)
	}
}

func TestRowScannerRejectsNonStruct(t *testing.T) {
	rows := &fakeRows{columns: []string{"id"}, row: []any{1}}
	var n int
	for _, dest := range []any{scanModel{}, &n} {
		if err := newRowScanner(scanTable).scan(rows, dest); err == nil {
			t.Errorf("%T should be refused", dest)
		}
	}
}

// After the first row, a plain struct scans with no allocation of its own.
func TestRowScannerDoesNotAllocatePerRow(t *testing.T) {
	rows := &fakeRows{columns: []string{"id", "name", "created_by"}}
	scanner := newRowScanner(scanTable)
	var m scanModel
	if err := scanner.scan(rows, &m); err != nil {
		t.Fatal(err)
	}
	if n := testing.AllocsPerRun(100, func() {
		if err := scanner.scan(rows, &m); err != nil {
			t.Fatal(err)
		}
	}); n != 0 {
		t.Errorf("%v allocations per row", n)
	}
}

func BenchmarkRowScanner(b *testing.B) {
	rows := &fakeRows{columns: []string{"id", "name", "created_by"}}
	scanner := newRowScanner(scanTable)
	var m scanModel
	b.ReportAllocs()
	for b.Loop() {
		if err := scanner.scan(rows, &m); err != nil {
			b.Fatal(err)
		}
	}
}
