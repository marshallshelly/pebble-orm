package migration

import (
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// TestTimestampDefaultSQLSyntax verifies that DEFAULT CURRENT_TIMESTAMP
// is generated correctly without spaces
func TestTimestampDefaultSQLSyntax(t *testing.T) {
	planner := NewPlanner()

	tests := []struct {
		name           string
		col            schema.ColumnMetadata
		expectedSQL    string
		shouldNotMatch string
	}{
		{
			name: "CURRENT_TIMESTAMP",
			col: schema.ColumnMetadata{
				Name:     "created_at",
				SQLType:  "timestamp",
				Nullable: false,
				Default:  strPtr("CURRENT_TIMESTAMP"),
			},
			expectedSQL:    "created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP",
			shouldNotMatch: "DEFAULT CURRENT TIMESTAMP", // Bug: space between words
		},
		{
			name: "NOW()",
			col: schema.ColumnMetadata{
				Name:     "updated_at",
				SQLType:  "timestamptz",
				Nullable: false,
				Default:  strPtr("NOW()"),
			},
			expectedSQL:    "updated_at timestamptz NOT NULL DEFAULT NOW()",
			shouldNotMatch: "",
		},
		{
			name: "gen_random_uuid()",
			col: schema.ColumnMetadata{
				Name:     "id",
				SQLType:  "uuid",
				Nullable: false,
				Default:  strPtr("gen_random_uuid()"),
			},
			expectedSQL:    "id uuid NOT NULL DEFAULT gen_random_uuid()",
			shouldNotMatch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := planner.generateColumnDefinition(tt.col)

			if sql != tt.expectedSQL {
				t.Errorf("Expected SQL:\n  %s\nGot:\n  %s", tt.expectedSQL, sql)
			}

			if tt.shouldNotMatch != "" && strings.Contains(sql, tt.shouldNotMatch) {
				t.Errorf("SQL should NOT contain '%s', but got: %s", tt.shouldNotMatch, sql)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
