package migration

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
	"github.com/stretchr/testify/assert"
)

// TestIndexComparisonBug tests the bug where indexes are marked as both added and dropped
func TestIndexComparisonBug(t *testing.T) {
	differ := NewDiffer()

	// Simulate code schema (from Go struct tags)
	codeSchema := map[string]*schema.TableMetadata{
		"refresh_tokens": {
			Name: "refresh_tokens",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "uuid", Nullable: false},
				{Name: "user_id", SQLType: "integer", Nullable: false},
				{Name: "token_hash", SQLType: "varchar(64)", Nullable: false},
				{Name: "expires_at", SQLType: "timestamptz", Nullable: false},
			},
			Indexes: []schema.IndexMetadata{
				{
					Name:    "idx_refresh_tokens_user",
					Columns: []string{"user_id"},
					Type:    "btree",
					Unique:  false,
				},
				{
					Name:    "idx_refresh_tokens_token",
					Columns: []string{"token_hash"},
					Type:    "btree",
					Unique:  false,
				},
				{
					Name:    "idx_refresh_tokens_expires",
					Columns: []string{"expires_at"},
					Type:    "btree",
					Unique:  false,
				},
			},
		},
	}

	// Simulate DB schema (from introspection)
	// These should be identical to code schema
	dbSchema := map[string]*schema.TableMetadata{
		"refresh_tokens": {
			Name: "refresh_tokens",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "uuid", Nullable: false},
				{Name: "user_id", SQLType: "integer", Nullable: false},
				{Name: "token_hash", SQLType: "varchar(64)", Nullable: false},
				{Name: "expires_at", SQLType: "timestamptz", Nullable: false},
			},
			Indexes: []schema.IndexMetadata{
				{
					Name:    "idx_refresh_tokens_user",
					Columns: []string{"user_id"},
					Type:    "btree",
					Unique:  false,
				},
				{
					Name:    "idx_refresh_tokens_token",
					Columns: []string{"token_hash"},
					Type:    "btree",
					Unique:  false,
				},
				{
					Name:    "idx_refresh_tokens_expires",
					Columns: []string{"expires_at"},
					Type:    "btree",
					Unique:  false,
				},
			},
		},
	}

	// Compare schemas
	diff := differ.Compare(codeSchema, dbSchema)

	// Print debug information
	t.Logf("Tables Modified: %d", len(diff.TablesModified))
	if len(diff.TablesModified) > 0 {
		tableDiff := diff.TablesModified[0]
		t.Logf("Table: %s", tableDiff.TableName)
		t.Logf("Indexes Added: %d", len(tableDiff.IndexesAdded))
		for _, idx := range tableDiff.IndexesAdded {
			t.Logf("  - Added: %s (Type: %s, Columns: %v, Unique: %v, Expression: %s, Where: %s, Include: %v, ColumnOrdering: %v)",
				idx.Name, idx.Type, idx.Columns, idx.Unique, idx.Expression, idx.Where, idx.Include, idx.ColumnOrdering)
		}
		t.Logf("Indexes Dropped: %d", len(tableDiff.IndexesDropped))
		for _, idxName := range tableDiff.IndexesDropped {
			t.Logf("  - Dropped: %s", idxName)
		}
	}

	// The indexes are identical, so there should be NO changes
	assert.Equal(t, 0, len(diff.TablesModified), "Expected no table modifications")

	// If there are modifications, check that indexes are not both added and dropped
	if len(diff.TablesModified) > 0 {
		tableDiff := diff.TablesModified[0]

		// Check for indexes that appear in both added and dropped
		addedMap := make(map[string]bool)
		for _, idx := range tableDiff.IndexesAdded {
			addedMap[idx.Name] = true
		}

		for _, idxName := range tableDiff.IndexesDropped {
			if addedMap[idxName] {
				t.Errorf("BUG DETECTED: Index %s is in BOTH IndexesAdded and IndexesDropped", idxName)
			}
		}

		assert.Equal(t, 0, len(tableDiff.IndexesAdded), "Expected no indexes to be added")
		assert.Equal(t, 0, len(tableDiff.IndexesDropped), "Expected no indexes to be dropped")
	}
}

// TestIsSameIndex tests the isSameIndex function directly
func TestIsSameIndexDirectly(t *testing.T) {
	differ := NewDiffer()

	tests := []struct {
		name     string
		idx1     schema.IndexMetadata
		idx2     schema.IndexMetadata
		expected bool
	}{
		{
			name: "identical simple indexes",
			idx1: schema.IndexMetadata{
				Name:    "idx_user_id",
				Columns: []string{"user_id"},
				Type:    "btree",
				Unique:  false,
			},
			idx2: schema.IndexMetadata{
				Name:    "idx_user_id",
				Columns: []string{"user_id"},
				Type:    "btree",
				Unique:  false,
			},
			expected: true,
		},
		{
			name: "empty type should default to btree",
			idx1: schema.IndexMetadata{
				Name:    "idx_user_id",
				Columns: []string{"user_id"},
				Type:    "",
				Unique:  false,
			},
			idx2: schema.IndexMetadata{
				Name:    "idx_user_id",
				Columns: []string{"user_id"},
				Type:    "btree",
				Unique:  false,
			},
			expected: true,
		},
		{
			name: "different column ordering length",
			idx1: schema.IndexMetadata{
				Name:           "idx_user_id",
				Columns:        []string{"user_id"},
				Type:           "btree",
				Unique:         false,
				ColumnOrdering: []schema.ColumnOrder{},
			},
			idx2: schema.IndexMetadata{
				Name:    "idx_user_id",
				Columns: []string{"user_id"},
				Type:    "btree",
				Unique:  false,
				ColumnOrdering: []schema.ColumnOrder{
					{
						Column:    "user_id",
						Direction: schema.Ascending,
					},
				},
			},
			expected: true, // Empty ordering should be equivalent to default ASC
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := differ.isSameIndex(tt.idx1, tt.idx2)
			t.Logf("idx1: %+v", tt.idx1)
			t.Logf("idx2: %+v", tt.idx2)
			t.Logf("Result: %v", result)
			assert.Equal(t, tt.expected, result)
		})
	}
}
