package builder

import (
	"strings"
	"testing"
)

type ignoredFieldWidget struct {
	ID         string `po:"id,primaryKey,uuid,default(uuidv7())"`
	Name       string `po:"name,varchar(120),notNull"`
	CarerCount int    `po:"-" json:"carerCount"`
	Note       string `po:"-,varchar(20)"`
}

func TestInsertLeavesOutIgnoredFields(t *testing.T) {
	sql, args, err := Insert[ignoredFieldWidget](New(nil)).
		Values(ignoredFieldWidget{Name: "x", CarerCount: 4, Note: "n"}).
		Returning("*").ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "(name) VALUES ($1) RETURNING *") {
		t.Errorf("sql = %s", sql)
	}
	if len(args) != 1 || args[0] != "x" {
		t.Errorf("args = %v", args)
	}
}

func TestInsertManyLeavesOutIgnoredFields(t *testing.T) {
	sql, args, err := Insert[ignoredFieldWidget](New(nil)).
		Values(ignoredFieldWidget{Name: "x"}, ignoredFieldWidget{Name: "y", CarerCount: 2}).
		ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "(name) VALUES ($1), ($2)") {
		t.Errorf("sql = %s", sql)
	}
	if len(args) != 2 {
		t.Errorf("args = %v", args)
	}
}
