package migration

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// TestSerialTypesMapToInteger tests that serial pseudotypes are correctly
// mapped to their underlying integer types.
func TestSerialTypesMapToInteger(t *testing.T) {
	differ := NewDiffer()

	tests := []struct {
		name     string
		type1    string
		type2    string
		expected bool
	}{
		{
			name:     "serial equals integer",
			type1:    "serial",
			type2:    "integer",
			expected: true, // serial IS integer + sequence
		},
		{
			name:     "serial4 equals integer",
			type1:    "serial4",
			type2:    "integer",
			expected: true,
		},
		{
			name:     "bigserial equals bigint",
			type1:    "bigserial",
			type2:    "bigint",
			expected: true, // bigserial IS bigint + sequence
		},
		{
			name:     "serial8 equals bigint",
			type1:    "serial8",
			type2:    "bigint",
			expected: true,
		},
		{
			name:     "smallserial equals smallint",
			type1:    "smallserial",
			type2:    "smallint",
			expected: true, // smallserial IS smallint + sequence
		},
		{
			name:     "serial2 equals smallint",
			type1:    "serial2",
			type2:    "smallint",
			expected: true,
		},
		{
			name:     "serial does NOT equal bigint",
			type1:    "serial",
			type2:    "bigint",
			expected: false, // serial is integer, not bigint
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := differ.isSameType(tt.type1, tt.type2)
			if result != tt.expected {
				t.Errorf("isSameType(%s, %s) = %v, expected %v",
					tt.type1, tt.type2, result, tt.expected)
			}
		})
	}
}

// TestSerialDoesNotTriggerTypeChange tests that changing between serial and
// integer does NOT trigger a type change (since they're the same underlying type).
func TestSerialDoesNotTriggerTypeChange(t *testing.T) {
	differ := NewDiffer()

	// Scenario: Code has `serial`, database introspects to `integer`
	// This should NOT trigger a type change
	codeTable := &schema.TableMetadata{
		Name: "tenants",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "serial", Nullable: false}, // Code: serial
		},
	}

	dbTable := &schema.TableMetadata{
		Name: "tenants",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "integer", Nullable: false}, // DB: integer
		},
	}

	tableDiff := differ.compareTable(codeTable, dbTable)

	// Should NOT have any type changes
	if len(tableDiff.ColumnsModified) > 0 {
		for _, colDiff := range tableDiff.ColumnsModified {
			if colDiff.TypeChanged {
				t.Errorf("serial vs integer should NOT trigger type change!")
				t.Errorf("Column: %s, Old: %s, New: %s",
					colDiff.ColumnName, colDiff.OldColumn.SQLType, colDiff.NewColumn.SQLType)
			}
		}
	}

	t.Log("✅ serial and integer are correctly treated as the same type")
}

// TestSerialInBothCodeAndDB tests that serial in both places is recognized as no change.
func TestSerialInBothCodeAndDB(t *testing.T) {
	differ := NewDiffer()

	codeTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "serial", Nullable: false},
		},
	}

	dbTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "serial", Nullable: false},
		},
	}

	tableDiff := differ.compareTable(codeTable, dbTable)

	if len(tableDiff.ColumnsModified) > 0 {
		t.Errorf("Expected no column modifications, got %d", len(tableDiff.ColumnsModified))
	}
}

// TestBigSerialMapping tests bigserial <-> bigint equivalence.
func TestBigSerialMapping(t *testing.T) {
	differ := NewDiffer()

	codeTable := &schema.TableMetadata{
		Name: "large_table",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "bigserial", Nullable: false},
		},
	}

	dbTable := &schema.TableMetadata{
		Name: "large_table",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "bigint", Nullable: false}, // DB shows bigint
		},
	}

	tableDiff := differ.compareTable(codeTable, dbTable)

	// Should NOT trigger type change
	for _, colDiff := range tableDiff.ColumnsModified {
		if colDiff.TypeChanged {
			t.Errorf("bigserial vs bigint should NOT trigger type change!")
		}
	}

	t.Log("✅ bigserial and bigint are correctly treated as the same type")
}
