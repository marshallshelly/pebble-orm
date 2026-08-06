package builder

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// A generated column must never appear in an INSERT column list — PostgreSQL
// rejects a value for a GENERATED ALWAYS column (SQLSTATE 428C9).
func TestGeneratedColumnOmittedFromInsert(t *testing.T) {
	table := &schema.TableMetadata{
		Name: "people",
		Columns: []schema.ColumnMetadata{
			{Name: "id", GoField: "ID", SQLType: "bigserial", AutoIncrement: true},
			{Name: "first_name", GoField: "FirstName", SQLType: "varchar(100)"},
			{Name: "last_name", GoField: "LastName", SQLType: "varchar(100)"},
			{Name: "full_name", GoField: "FullName", SQLType: "varchar(255)",
				Generated: &schema.GeneratedColumn{Expression: "first_name || ' ' || last_name", Type: schema.GeneratedStored}},
		},
		PrimaryKey: &schema.PrimaryKeyMetadata{Name: "people_pkey", Columns: []string{"id"}},
	}
	type Person struct {
		ID        int64
		FirstName string
		LastName  string
		FullName  string
	}

	cols, _, err := structToValues(Person{FirstName: "Ada", LastName: "Lovelace"}, table, true)
	if err != nil {
		t.Fatalf("structToValues: %v", err)
	}
	for _, c := range cols {
		if c == "full_name" {
			t.Errorf("generated column full_name must be omitted from INSERT, got columns %v", cols)
		}
	}
}
