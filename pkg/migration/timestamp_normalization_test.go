package migration

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// TestTimestampTypeNormalization tests that timestamp types are correctly normalized
func TestTimestampTypeNormalization(t *testing.T) {
	differ := NewDiffer()

	testCases := []struct {
		name        string
		codeType    string
		dbType      string
		shouldMatch bool
		description string
	}{
		{
			name:        "timestamp matches timestamp without time zone",
			codeType:    "timestamp",
			dbType:      "timestamp without time zone",
			shouldMatch: true,
			description: "Code 'timestamp' should equal DB 'timestamp without time zone'",
		},
		{
			name:        "timestamp matches timestamp",
			codeType:    "timestamp",
			dbType:      "timestamp",
			shouldMatch: true,
			description: "Same types should match",
		},
		{
			name:        "timestamptz matches timestamp with time zone",
			codeType:    "timestamptz",
			dbType:      "timestamp with time zone",
			shouldMatch: true,
			description: "Code 'timestamptz' should equal DB 'timestamp with time zone'",
		},
		{
			name:        "timestamp does NOT match timestamptz",
			codeType:    "timestamp",
			dbType:      "timestamptz",
			shouldMatch: false,
			description: "timestamp and timestamptz are different types",
		},
		{
			name:        "timestamp without time zone does NOT match timestamp with time zone",
			codeType:    "timestamp without time zone",
			dbType:      "timestamp with time zone",
			shouldMatch: false,
			description: "Timestamp variants with different zones should not match",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := differ.isSameType(tc.codeType, tc.dbType)
			if result != tc.shouldMatch {
				t.Errorf("%s\nExpected match: %v, got: %v\nCode type: %s\nDB type: %s",
					tc.description, tc.shouldMatch, result, tc.codeType, tc.dbType)
			}
		})
	}
}

// TestTimestampColumnNoPhantomAlter verifies no phantom ALTER when timestamp types match
func TestTimestampColumnNoPhantomAlter(t *testing.T) {
	differ := NewDiffer()

	// Code column: timestamp (from Go time.Time with `po:"created_at,timestamp"`)
	codeCol := schema.ColumnMetadata{
		Name:     "created_at",
		SQLType:  "timestamp",
		Nullable: false,
		Default:  strPtr("NOW()"),
	}

	// DB column: timestamp without time zone (introspected from PostgreSQL)
	dbCol := schema.ColumnMetadata{
		Name:     "created_at",
		SQLType:  "timestamp without time zone",
		Nullable: false,
		Default:  strPtr("now()"),
	}

	colDiff := differ.compareColumn(codeCol, dbCol)

	if colDiff.TypeChanged {
		t.Errorf("Type should NOT be marked as changed!\nCode type: %s\nDB type: %s\nNormalized code: %s\nNormalized DB: %s",
			codeCol.SQLType, dbCol.SQLType,
			differ.normalizeType(codeCol.SQLType),
			differ.normalizeType(dbCol.SQLType))
	}

	if colDiff.hasChanges() {
		t.Errorf("Column should have NO changes, but found:\n"+
			"TypeChanged: %v\nNullChanged: %v\nDefaultChanged: %v",
			colDiff.TypeChanged, colDiff.NullChanged, colDiff.DefaultChanged)
	}

	t.Log("✅ timestamp and 'timestamp without time zone' correctly treated as same type")
}

// TestTimestampTzColumnNoPhantomAlter verifies no phantom ALTER for timestamptz
func TestTimestampTzColumnNoPhantomAlter(t *testing.T) {
	differ := NewDiffer()

	// Code column: timestamptz
	codeCol := schema.ColumnMetadata{
		Name:     "updated_at",
		SQLType:  "timestamptz",
		Nullable: false,
		Default:  strPtr("NOW()"),
	}

	// DB column: timestamp with time zone (introspected)
	dbCol := schema.ColumnMetadata{
		Name:     "updated_at",
		SQLType:  "timestamp with time zone",
		Nullable: false,
		Default:  strPtr("now()"),
	}

	colDiff := differ.compareColumn(codeCol, dbCol)

	if colDiff.TypeChanged {
		t.Errorf("Type should NOT be marked as changed!\nCode type: %s\nDB type: %s",
			codeCol.SQLType, dbCol.SQLType)
	}

	if colDiff.hasChanges() {
		t.Errorf("Column should have NO changes")
	}

	t.Log("✅ timestamptz and 'timestamp with time zone' correctly treated as same type")
}

// TestFullTableWithTimestampNoPhantomAlter tests complete table comparison
func TestFullTableWithTimestampNoPhantomAlter(t *testing.T) {
	differ := NewDiffer()

	// Code table (from models)
	codeTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{
				Name:          "id",
				SQLType:       "serial",
				Nullable:      false,
				AutoIncrement: true,
			},
			{
				Name:     "created_at",
				SQLType:  "timestamp",
				Nullable: false,
				Default:  strPtr("NOW()"),
			},
		},
	}

	// DB table (introspected)
	dbTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{
				Name:          "id",
				SQLType:       "integer",
				Nullable:      false,
				Default:       strPtr("nextval('users_id_seq'::regclass)"),
				AutoIncrement: true,
			},
			{
				Name:     "created_at",
				SQLType:  "timestamp without time zone",
				Nullable: false,
				Default:  strPtr("now()"),
			},
		},
	}

	tableDiff := differ.compareTable(codeTable, dbTable)

	if len(tableDiff.ColumnsModified) > 0 {
		t.Errorf("Should have NO modified columns, but found %d:\n", len(tableDiff.ColumnsModified))
		for _, cd := range tableDiff.ColumnsModified {
			t.Logf("  - %s: TypeChanged=%v, NullChanged=%v, DefaultChanged=%v",
				cd.ColumnName, cd.TypeChanged, cd.NullChanged, cd.DefaultChanged)
		}
	}

	if tableDiff.HasChanges() {
		t.Error("Table should have NO changes - timestamp types should match!")
	}

	t.Log("✅ No phantom ALTERs for tables with timestamp columns")
}
