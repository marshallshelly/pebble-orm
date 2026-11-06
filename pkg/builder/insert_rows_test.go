package builder

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
)

type batchLine struct {
	ID     int             `po:"id,primaryKey,serial"`
	Name   string          `po:"name,text,notNull"`
	VAT    float64         `po:"vat,numeric(5,2),notNull,default(0)"`
	Status string          `po:"status,text,notNull,default('open')"`
	Detail json.RawMessage `po:"detail,jsonb"`
}

func batchTable(t *testing.T) insertSpec {
	t.Helper()
	table, err := registry.GetOrRegister(batchLine{})
	if err != nil {
		t.Fatal(err)
	}
	return insertSpec{table: table}
}

// A row left zero on a defaulted column must not decide the column list for
// the rows after it: they keep their values and it gets DEFAULT.
func TestBuildInsertSQL_RowsGetTheirOwnDefaults(t *testing.T) {
	s := batchTable(t)
	s.rows = []interface{}{
		batchLine{Name: "exempt"},
		batchLine{Name: "taxed", VAT: 20, Status: "paid"},
		&batchLine{Name: "pointer", Detail: json.RawMessage(`{"a":1}`)},
	}
	sql, args, err := buildInsertSQL(s)
	if err != nil {
		t.Fatal(err)
	}
	// detail has no default, so it is always sent; a nil RawMessage is NULL.
	want := "INSERT INTO batch_line (name, vat, status, detail) VALUES ($1, DEFAULT, DEFAULT, $2), ($3, $4, $5, $6), ($7, DEFAULT, DEFAULT, $8)"
	if sql != want {
		t.Errorf("sql:\n got %s\nwant %s", sql, want)
	}
	wantArgs := []interface{}{"exempt", nil, "taxed", 20.0, "paid", nil, "pointer", `{"a":1}`}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Errorf("args = %#v\nwant   %#v", args, wantArgs)
	}

	// The same rows in the other order give the same values per row.
	s.rows[0], s.rows[1] = s.rows[1], s.rows[0]
	sql, _, err = buildInsertSQL(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "VALUES ($1, $2, $3, $4), ($5, DEFAULT, DEFAULT, $6)") {
		t.Errorf("reordered sql: %s", sql)
	}
}

// TxInsert takes ...interface{}; a row of another type used to be read field
// by field, writing whatever happened to share a name.
func TestBuildInsertSQL_RejectsOtherTypes(t *testing.T) {
	s := batchTable(t)
	type other struct{ Name string }
	s.rows = []interface{}{batchLine{Name: "ok"}, other{Name: "wrong"}}
	if _, _, err := buildInsertSQL(s); err == nil || !strings.Contains(err.Error(), "row 1 is builder.other") {
		t.Fatalf("err = %v", err)
	}
	s.rows = []interface{}{nil}
	if _, _, err := buildInsertSQL(s); err == nil {
		t.Fatal("nil row accepted")
	}
}

// A nil RawMessage, slice or map is SQL NULL, like a nil pointer.
func TestMarshalJSONB_NilIsNull(t *testing.T) {
	var raw json.RawMessage
	var list []string
	var m map[string]int
	for name, v := range map[string]interface{}{"RawMessage": raw, "slice": list, "map": m} {
		got, err := marshalJSONB(v)
		if err != nil || got != nil {
			t.Errorf("%s: got %v, %v; want nil", name, got, err)
		}
	}
	got, err := marshalJSONB([]string{})
	if err != nil || got != "[]" {
		t.Errorf("empty slice: got %v, %v; want []", got, err)
	}
}

// One SetMap call shape must be one statement text.
func TestBuildUpdateSQL_StableText(t *testing.T) {
	s := batchTable(t)
	sets := map[string]interface{}{"name": "a", "vat": 1.5, "status": "x", "detail": nil}
	first, _, err := buildUpdateSQL(updateSpec{table: s.table, sets: sets})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		got, args, _ := buildUpdateSQL(updateSpec{table: s.table, sets: sets})
		if got != first {
			t.Fatalf("text changed:\n%s\n%s", first, got)
		}
		if args[0] != nil || args[1] != "a" || args[2] != "x" || args[3] != 1.5 {
			t.Fatalf("args out of order: %v", args)
		}
	}
	if want := "UPDATE batch_line SET detail = $1, name = $2, status = $3, vat = $4"; first != want {
		t.Errorf("got %s", first)
	}
}
