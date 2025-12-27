package migration

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// TestSerialColumnNoDefaultChange verifies that serial columns don't trigger DROP DEFAULT
func TestSerialColumnNoDefaultChange(t *testing.T) {
	differ := NewDiffer()

	// Code has serial column (no explicit default)
	codeCol := schema.ColumnMetadata{
		Name:          "id",
		SQLType:       "serial",
		Nullable:      false,
		AutoIncrement: true,
	}

	// Database has integer with sequence default
	seqDefault := "nextval('table_id_seq'::regclass)"
	dbCol := schema.ColumnMetadata{
		Name:     "id",
		SQLType:  "integer",
		Nullable: false,
		Default:  &seqDefault,
	}

	colDiff := differ.compareColumn(codeCol, dbCol)

	if colDiff.DefaultChanged {
		t.Errorf("Serial column should not trigger default change, but got DefaultChanged=true")
	}
}

// TestBigSerialColumnNoDefaultChange tests bigserial columns
func TestBigSerialColumnNoDefaultChange(t *testing.T) {
	differ := NewDiffer()

	codeCol := schema.ColumnMetadata{
		Name:          "id",
		SQLType:       "bigserial",
		Nullable:      false,
		AutoIncrement: true,
	}

	seqDefault := "nextval('table_id_seq'::regclass)"
	dbCol := schema.ColumnMetadata{
		Name:     "id",
		SQLType:  "bigint",
		Nullable: false,
		Default:  &seqDefault,
	}

	colDiff := differ.compareColumn(codeCol, dbCol)

	if colDiff.DefaultChanged {
		t.Errorf("BigSerial column should not trigger default change")
	}
}

// TestSmallSerialColumnNoDefaultChange tests smallserial columns
func TestSmallSerialColumnNoDefaultChange(t *testing.T) {
	differ := NewDiffer()

	codeCol := schema.ColumnMetadata{
		Name:     "id",
		SQLType:  "smallserial",
		Nullable: false,
	}

	seqDefault := "nextval('table_id_seq'::regclass)"
	dbCol := schema.ColumnMetadata{
		Name:     "id",
		SQLType:  "smallint",
		Nullable: false,
		Default:  &seqDefault,
	}

	colDiff := differ.compareColumn(codeCol, dbCol)

	if colDiff.DefaultChanged {
		t.Errorf("SmallSerial column should not trigger default change")
	}
}

// TestSerialVariations tests different nextval formats
func TestSerialVariations(t *testing.T) {
	differ := NewDiffer()

	codeCol := schema.ColumnMetadata{
		Name:     "id",
		SQLType:  "serial",
		Nullable: false,
	}

	testCases := []struct {
		name        string
		dbDefault   string
		shouldMatch bool
	}{
		{
			name:        "standard nextval with regclass",
			dbDefault:   "nextval('table_id_seq'::regclass)",
			shouldMatch: true,
		},
		{
			name:        "nextval without regclass",
			dbDefault:   "nextval('table_id_seq')",
			shouldMatch: true,
		},
		{
			name:        "quoted sequence name",
			dbDefault:   `nextval('"table_id_seq"'::regclass)`,
			shouldMatch: true,
		},
		{
			name:        "uppercase NEXTVAL",
			dbDefault:   "NEXTVAL('table_id_seq'::regclass)",
			shouldMatch: true,
		},
		{
			name:        "non-sequence default",
			dbDefault:   "0",
			shouldMatch: false,
		},
		{
			name:        "string default",
			dbDefault:   "'default_value'",
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dbCol := schema.ColumnMetadata{
				Name:     "id",
				SQLType:  "integer",
				Nullable: false,
				Default:  &tc.dbDefault,
			}

			colDiff := differ.compareColumn(codeCol, dbCol)

			if tc.shouldMatch && colDiff.DefaultChanged {
				t.Errorf("Expected no default change for %q, but got DefaultChanged=true", tc.dbDefault)
			}
			if !tc.shouldMatch && !colDiff.DefaultChanged {
				t.Errorf("Expected default change for %q, but got DefaultChanged=false", tc.dbDefault)
			}
		})
	}
}

// TestActualDefaultChange verifies that real default changes are still detected
func TestActualDefaultChange(t *testing.T) {
	differ := NewDiffer()

	activeDefault := "'active'"
	codeCol := schema.ColumnMetadata{
		Name:     "status",
		SQLType:  "varchar",
		Nullable: true,
		Default:  &activeDefault,
	}

	pendingDefault := "'pending'"
	dbCol := schema.ColumnMetadata{
		Name:     "status",
		SQLType:  "varchar",
		Nullable: true,
		Default:  &pendingDefault,
	}

	colDiff := differ.compareColumn(codeCol, dbCol)

	if !colDiff.DefaultChanged {
		t.Errorf("Different defaults should be detected, but got DefaultChanged=false")
	}
}

// TestNonSerialIntegerWithDefault ensures we don't break regular integer columns
func TestNonSerialIntegerWithDefault(t *testing.T) {
	differ := NewDiffer()

	zeroDefault := "0"
	codeCol := schema.ColumnMetadata{
		Name:     "count",
		SQLType:  "integer",
		Nullable: false,
		Default:  &zeroDefault,
	}

	oneDefault := "1"
	dbCol := schema.ColumnMetadata{
		Name:     "count",
		SQLType:  "integer",
		Nullable: false,
		Default:  &oneDefault,
	}

	colDiff := differ.compareColumn(codeCol, dbCol)

	if !colDiff.DefaultChanged {
		t.Errorf("Different integer defaults should be detected")
	}
}

// TestIsAutoIncrementColumn tests the helper function
func TestIsAutoIncrementColumn(t *testing.T) {
	differ := NewDiffer()

	testCases := []struct {
		name     string
		col      schema.ColumnMetadata
		expected bool
	}{
		{
			name:     "serial type",
			col:      schema.ColumnMetadata{SQLType: "serial"},
			expected: true,
		},
		{
			name:     "bigserial type",
			col:      schema.ColumnMetadata{SQLType: "bigserial"},
			expected: true,
		},
		{
			name:     "serial4 type",
			col:      schema.ColumnMetadata{SQLType: "serial4"},
			expected: true,
		},
		{
			name:     "AutoIncrement flag",
			col:      schema.ColumnMetadata{SQLType: "integer", AutoIncrement: true},
			expected: true,
		},
		{
			name:     "regular integer",
			col:      schema.ColumnMetadata{SQLType: "integer"},
			expected: false,
		},
		{
			name:     "varchar",
			col:      schema.ColumnMetadata{SQLType: "varchar"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := differ.isAutoIncrementColumn(tc.col)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v for column type %q", tc.expected, result, tc.col.SQLType)
			}
		})
	}
}

// TestIsSequenceDefault tests the helper function
func TestIsSequenceDefault(t *testing.T) {
	differ := NewDiffer()

	testCases := []struct {
		name       string
		defaultVal *string
		expected   bool
	}{
		{
			name:       "nil default",
			defaultVal: nil,
			expected:   false,
		},
		{
			name:       "empty string",
			defaultVal: stringPtr(""),
			expected:   false,
		},
		{
			name:       "standard nextval",
			defaultVal: stringPtr("nextval('table_id_seq'::regclass)"),
			expected:   true,
		},
		{
			name:       "uppercase NEXTVAL",
			defaultVal: stringPtr("NEXTVAL('TABLE_ID_SEQ'::REGCLASS)"),
			expected:   true,
		},
		{
			name:       "numeric default",
			defaultVal: stringPtr("0"),
			expected:   false,
		},
		{
			name:       "CURRENT_TIMESTAMP",
			defaultVal: stringPtr("CURRENT_TIMESTAMP"),
			expected:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := differ.isSequenceDefault(tc.defaultVal)
			if result != tc.expected {
				if tc.defaultVal == nil {
					t.Errorf("Expected %v, got %v for nil default", tc.expected, result)
				} else {
					t.Errorf("Expected %v, got %v for default %q", tc.expected, result, *tc.defaultVal)
				}
			}
		})
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
