package schema

import (
	"reflect"
	"strings"
	"testing"
)

type ignoredPart struct {
	ID       int `po:"id,primaryKey,integer"`
	WidgetID int `po:"widget_id,integer"`
}

type ignoredWidget struct {
	ID         int           `po:"id,primaryKey,integer"`
	Name       string        `po:"name,varchar(120),notNull"`
	CarerCount int           `po:"-" json:"carerCount"`
	Note       string        `po:"-,varchar(20)"`
	OwnerID    string        `po:"-,uuid,fk:users(id),index"`
	Parts      []ignoredPart `po:"-,hasMany,foreignKey(widget_id),references(id)"`
}

func TestParseExcludesIgnoredFields(t *testing.T) {
	table, err := NewParser().Parse(reflect.TypeFor[ignoredWidget]())
	if err != nil {
		t.Fatal(err)
	}

	var columns []string
	for _, c := range table.Columns {
		columns = append(columns, c.Name)
	}
	if !reflect.DeepEqual(columns, []string{"id", "name"}) {
		t.Errorf("columns = %q, want id and name", columns)
	}
	if len(table.ForeignKeys) != 0 {
		t.Errorf("a field excluded with - produced foreign keys: %+v", table.ForeignKeys)
	}
	if len(table.Indexes) != 0 {
		t.Errorf("a field excluded with - produced indexes: %+v", table.Indexes)
	}
}

func TestParseKeepsRelationshipsNamedDash(t *testing.T) {
	table, err := NewParser().Parse(reflect.TypeFor[ignoredWidget]())
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Relationships) != 1 || table.Relationships[0].Type != HasMany {
		t.Errorf("relationships = %+v, want the one hasMany", table.Relationships)
	}
}

func TestParseRejectsMissingColumnName(t *testing.T) {
	type blank struct {
		ID   int    `po:"id,primaryKey,integer"`
		Note string `po:",varchar(20)"`
	}

	_, err := NewParser().Parse(reflect.TypeFor[blank]())
	if err == nil {
		t.Fatal("a tag with no column name should be refused")
	}
	if !strings.Contains(err.Error(), "Note") {
		t.Errorf("the error should name the field: %v", err)
	}
}

func TestIsColumnTag(t *testing.T) {
	cases := []struct {
		tag     string
		want    bool
		wantErr bool
	}{
		{tag: "name,text", want: true},
		{tag: "-"},
		{tag: "-,varchar(20)"},
		{tag: "-,hasMany,foreignKey(a),references(b)"},
		{tag: ",hasMany,foreignKey(a),references(b)"},
		{tag: ",text", wantErr: true},
		{tag: " ,text", wantErr: true},
	}
	for _, c := range cases {
		opts, err := ParseTag(c.tag)
		if err != nil {
			t.Fatalf("%q: %v", c.tag, err)
		}
		got, err := IsColumnTag(opts)
		if got != c.want || (err != nil) != c.wantErr {
			t.Errorf("IsColumnTag(%q) = %v, %v", c.tag, got, err)
		}
	}
}
