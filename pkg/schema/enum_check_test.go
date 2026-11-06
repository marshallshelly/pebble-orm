package schema

import (
	"strings"
	"testing"
)

func TestCheckEnumTypes(t *testing.T) {
	enumCol := func(name, typ string, values ...string) ColumnMetadata {
		return ColumnMetadata{Name: name, EnumType: typ, EnumValues: values}
	}
	tables := func(a, b ColumnMetadata) map[string]*TableMetadata {
		return map[string]*TableMetadata{
			"ticket": {Name: "ticket", Columns: []ColumnMetadata{b}},
			"order":  {Name: "order", Columns: []ColumnMetadata{{Name: "id"}, a}},
		}
	}

	err := CheckEnumTypes(tables(enumCol("status", "string", "pending", "paid"), enumCol("priority", "string", "low", "high")))
	if err == nil || !strings.Contains(err.Error(), `"string" is used by order.status (pending, paid) and ticket.priority (low, high)`) {
		t.Errorf("plain string enums with different values: got %v", err)
	}

	if err := CheckEnumTypes(tables(enumCol("status", "status", "a", "b"), enumCol("status", "status", "a", "b"))); err != nil {
		t.Errorf("one named type on two columns: %v", err)
	}
	if err := CheckEnumTypes(tables(enumCol("status", "order_status", "a"), enumCol("priority", "priority", "b"))); err != nil {
		t.Errorf("distinct types: %v", err)
	}
}
