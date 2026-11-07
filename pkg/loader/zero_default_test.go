package loader_test

import (
	stdsql "database/sql"
	"reflect"
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

type ZeroDefaults struct {
	ID      int              `po:"id,primaryKey,serial"`
	Active  bool             `po:"active,boolean,notNull,default(true)"`
	Grace   int              `po:"grace,integer,notNull,default(30)"`
	Percent float64          `po:"percent,decimal(5,2),notNull,default(100)"`
	Cast    int              `po:"cast_col,integer,default('7'::integer)"`
	Ptr     *bool            `po:"ptr,boolean,default(true)"`
	Need    *bool            `po:"need,boolean,notNull,default(true)"`
	Null    stdsql.NullInt64 `po:"null_int,bigint,default(5)"`
	Off     bool             `po:"off,boolean,notNull,default(false)"`
	Zero    float64          `po:"zero,numeric(5,2),notNull,default(0.00)"`
	Status  string           `po:"status,varchar(20),default('draft')"`
	Seq     int64            `po:"seq,bigint,default(nextval('s'))"`
}

const zeroDefaultsSrc = "package models\n\nimport stdsql \"database/sql\"\n\n" +
	"// table_name: zero_defaults\n" +
	"type ZeroDefaults struct {\n" +
	"\tID      int              `po:\"id,primaryKey,serial\"`\n" +
	"\tActive  bool             `po:\"active,boolean,notNull,default(true)\"`\n" +
	"\tGrace   int              `po:\"grace,integer,notNull,default(30)\"`\n" +
	"\tPercent float64          `po:\"percent,decimal(5,2),notNull,default(100)\"`\n" +
	"\tCast    int              `po:\"cast_col,integer,default('7'::integer)\"`\n" +
	"\tPtr     *bool            `po:\"ptr,boolean,default(true)\"`\n" +
	"\tNeed    *bool            `po:\"need,boolean,notNull,default(true)\"`\n" +
	"\tNull    stdsql.NullInt64 `po:\"null_int,bigint,default(5)\"`\n" +
	"\tOff     bool             `po:\"off,boolean,notNull,default(false)\"`\n" +
	"\tZero    float64          `po:\"zero,numeric(5,2),notNull,default(0.00)\"`\n" +
	"\tStatus  string           `po:\"status,varchar(20),default('draft')\"`\n" +
	"\tSeq     int64            `po:\"seq,bigint,default(nextval('s'))\"`\n" +
	"}\n"

// A bool with default(true) or a number with a non-zero default can't be
// inserted as its zero value; both model paths must report the same columns.
func TestCheckZeroDefaults(t *testing.T) {
	fromAST := loadOneTable(t, zeroDefaultsSrc)
	fromReflect, err := schema.NewParser().Parse(reflect.TypeOf(ZeroDefaults{}))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := []string{
		"  zero_defaults.active (field Active, default true)",
		"  zero_defaults.grace (field Grace, default 30)",
		"  zero_defaults.percent (field Percent, default 100)",
		"  zero_defaults.cast_col (field Cast, default '7'::integer)",
	}
	for name, table := range map[string]*schema.TableMetadata{"ast": fromAST, "reflect": fromReflect} {
		err := schema.CheckZeroDefaults(map[string]*schema.TableMetadata{"zero_defaults": table})
		if err == nil {
			t.Fatalf("%s: no error", name)
		}
		lines := strings.Split(err.Error(), "\n")[1:]
		if !reflect.DeepEqual(lines, want) {
			t.Errorf("%s: reported\n%s\nwant\n%s", name, strings.Join(lines, "\n"), strings.Join(want, "\n"))
		}
	}
}

// A pointer field is still NOT NULL when its tag says notNull.
func TestPointerKeepsNotNull(t *testing.T) {
	fromAST := loadOneTable(t, zeroDefaultsSrc)
	fromReflect, err := schema.NewParser().Parse(reflect.TypeOf(ZeroDefaults{}))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := map[string]bool{"need": false, "ptr": true}
	for name, table := range map[string]*schema.TableMetadata{"ast": fromAST, "reflect": fromReflect} {
		for _, c := range table.Columns {
			if exp, ok := want[c.Name]; ok && c.Nullable != exp {
				t.Errorf("%s: %s Nullable = %t, want %t", name, c.Name, c.Nullable, exp)
			}
		}
	}
}
