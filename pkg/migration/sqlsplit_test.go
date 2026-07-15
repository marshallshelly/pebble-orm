package migration

import (
	"reflect"
	"testing"
)

func TestSplitSQLStatements(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []string
	}{
		{"simple", "CREATE TABLE a (id int); CREATE TABLE b (id int);",
			[]string{"CREATE TABLE a (id int)", "CREATE TABLE b (id int)"}},
		{"semicolon in string literal", "INSERT INTO t(v) VALUES ('a;b'); SELECT 1;",
			[]string{"INSERT INTO t(v) VALUES ('a;b')", "SELECT 1"}},
		{"dollar quoted function body",
			"CREATE FUNCTION f() RETURNS trigger AS $$ BEGIN NEW.x := 1; RETURN NEW; END; $$ LANGUAGE plpgsql; SELECT 2;",
			[]string{"CREATE FUNCTION f() RETURNS trigger AS $$ BEGIN NEW.x := 1; RETURN NEW; END; $$ LANGUAGE plpgsql", "SELECT 2"}},
		{"named dollar tag", "DO $body$ BEGIN PERFORM 1; END $body$; SELECT 3;",
			[]string{"DO $body$ BEGIN PERFORM 1; END $body$", "SELECT 3"}},
		{"line comment stripped", "-- a comment\nCREATE TABLE a (id int);\n-- trailing",
			[]string{"CREATE TABLE a (id int)"}},
		{"comment inside string kept", "INSERT INTO t(v) VALUES ('-- not a comment');",
			[]string{"INSERT INTO t(v) VALUES ('-- not a comment')"}},
		{"positional params not tags", "UPDATE t SET a = $1 WHERE id = $2;",
			[]string{"UPDATE t SET a = $1 WHERE id = $2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitSQLStatements(tt.sql)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got  %#v\nwant %#v", got, tt.want)
			}
		})
	}
}
