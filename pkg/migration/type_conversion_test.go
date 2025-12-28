package migration

import (
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// TestTypeConversionUSINGClause verifies that incompatible type conversions
// generate appropriate USING clauses.
//
// Issue: PostgreSQL cannot automatically cast between certain types (e.g., text → text[]),
// causing ERROR: column "languages_spoken" cannot be cast automatically to type text[]
func TestTypeConversionTextToArray(t *testing.T) {
	planner := NewPlanner()

	// Database has: languages_spoken text
	// Code changes to: languages_spoken text[]
	diff := TableDiff{
		TableName: "teams",
		ColumnsModified: []ColumnDiff{
			{
				ColumnName:  "languages_spoken",
				TypeChanged: true,
				OldColumn: schema.ColumnMetadata{
					Name:    "languages_spoken",
					SQLType: "text",
				},
				NewColumn: schema.ColumnMetadata{
					Name:    "languages_spoken",
					SQLType: "text[]",
				},
			},
		},
	}

	upSQL, _ := planner.generateAlterTable(diff)

	// Should generate USING clause for text → text[] conversion
	if len(upSQL) == 0 {
		t.Fatal("Expected ALTER TABLE statement, got none")
	}

	sql := upSQL[0]
	if !strings.Contains(sql, "USING") {
		t.Errorf("Expected USING clause for text → text[] conversion\nGot: %s", sql)
	}

	// Should handle NULL and empty string cases
	if !strings.Contains(sql, "CASE") {
		t.Errorf("Expected CASE statement in USING clause for safety\nGot: %s", sql)
	}

	// Should cast to array
	if !strings.Contains(sql, "ARRAY[") {
		t.Errorf("Expected ARRAY constructor in USING clause\nGot: %s", sql)
	}

	t.Logf("Generated SQL with USING clause:\n%s", sql)
}

// TestTypeConversionTextToJSONB verifies text → jsonb conversion
func TestTypeConversionTextToJSONB(t *testing.T) {
	planner := NewPlanner()

	diff := TableDiff{
		TableName: "users",
		ColumnsModified: []ColumnDiff{
			{
				ColumnName:  "metadata",
				TypeChanged: true,
				OldColumn: schema.ColumnMetadata{
					Name:    "metadata",
					SQLType: "text",
				},
				NewColumn: schema.ColumnMetadata{
					Name:    "metadata",
					SQLType: "jsonb",
				},
			},
		},
	}

	upSQL, _ := planner.generateAlterTable(diff)

	if len(upSQL) == 0 {
		t.Fatal("Expected ALTER TABLE statement, got none")
	}

	sql := upSQL[0]
	if !strings.Contains(sql, "USING") {
		t.Errorf("Expected USING clause for text → jsonb conversion\nGot: %s", sql)
	}

	// Should handle empty string → '{}' conversion
	if !strings.Contains(sql, "'{}'::jsonb") {
		t.Errorf("Expected empty object conversion in USING clause\nGot: %s", sql)
	}

	t.Logf("Generated SQL:\n%s", sql)
}

// TestTypeConversionManualIntervention verifies that unsupported conversions
// generate commented-out statements requiring manual intervention
func TestTypeConversionManualIntervention(t *testing.T) {
	planner := NewPlanner()

	// Conversion that we don't have a safe automatic conversion for
	diff := TableDiff{
		TableName: "products",
		ColumnsModified: []ColumnDiff{
			{
				ColumnName:  "price",
				TypeChanged: true,
				OldColumn: schema.ColumnMetadata{
					Name:    "price",
					SQLType: "integer",
				},
				NewColumn: schema.ColumnMetadata{
					Name:    "price",
					SQLType: "jsonb",
				},
			},
		},
	}

	upSQL, _ := planner.generateAlterTable(diff)

	if len(upSQL) == 0 {
		t.Fatal("Expected migration statements, got none")
	}

	// Should contain manual migration comment
	hasManualComment := false
	for _, sql := range upSQL {
		if strings.Contains(sql, "MANUAL MIGRATION REQUIRED") {
			hasManualComment = true
			break
		}
	}

	if !hasManualComment {
		t.Errorf("Expected manual migration comment for integer → jsonb\nGot: %v", upSQL)
	}

	t.Logf("Generated manual migration comments:\n%s", strings.Join(upSQL, "\n"))
}

// TestTypeConversionImplicitCast verifies that simple conversions
// still work without USING clauses
func TestTypeConversionImplicitCast(t *testing.T) {
	planner := NewPlanner()

	// integer → bigint is implicit
	diff := TableDiff{
		TableName: "orders",
		ColumnsModified: []ColumnDiff{
			{
				ColumnName:  "total",
				TypeChanged: true,
				OldColumn: schema.ColumnMetadata{
					Name:    "total",
					SQLType: "integer",
				},
				NewColumn: schema.ColumnMetadata{
					Name:    "total",
					SQLType: "bigint",
				},
			},
		},
	}

	upSQL, _ := planner.generateAlterTable(diff)

	if len(upSQL) == 0 {
		t.Fatal("Expected ALTER TABLE statement, got none")
	}

	sql := upSQL[0]

	// Should NOT have USING clause for implicit cast
	if strings.Contains(sql, "USING") {
		t.Errorf("Should not generate USING clause for implicit integer → bigint cast\nGot: %s", sql)
	}

	// Should be simple ALTER TYPE
	expected := "ALTER TABLE orders ALTER COLUMN total TYPE bigint;"
	if sql != expected {
		t.Errorf("Expected simple type change\nExpected: %s\nGot: %s", expected, sql)
	}
}

// TestRequiresUsingClause tests the detection logic
func TestRequiresUsingClause(t *testing.T) {
	tests := []struct {
		name     string
		fromType string
		toType   string
		expected bool
	}{
		{"text to text[] requires USING", "text", "text[]", true},
		{"text to jsonb requires USING", "text", "jsonb", true},
		{"text to json requires USING", "text", "json", true},
		{"varchar to text[] requires USING", "varchar(255)", "text[]", true},
		{"integer to bigint is implicit", "integer", "bigint", false},
		{"timestamp to date is implicit", "timestamp", "date", false},
		{"same type no USING", "text", "text", false},
		{"varchar to text is implicit", "varchar(100)", "text", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := requiresUsingClause(tt.fromType, tt.toType)
			if result != tt.expected {
				t.Errorf("requiresUsingClause(%s, %s) = %v, expected %v",
					tt.fromType, tt.toType, result, tt.expected)
			}
		})
	}
}
