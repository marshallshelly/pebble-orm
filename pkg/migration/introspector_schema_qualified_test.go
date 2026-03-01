package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseIndexDefinitionWithSchemaQualifier tests parsing index definitions
// that include schema qualifiers (e.g., "ON public.table_name")
func TestParseIndexDefinitionWithSchemaQualifier(t *testing.T) {
	introspector := &Introspector{}

	tests := []struct {
		name         string
		indexDef     string
		tableName    string
		indexName    string
		indexType    string
		isUnique     bool
		isExpression bool
		expectedCols []string
	}{
		{
			name:         "schema-qualified table name (public.refresh_tokens)",
			indexDef:     "CREATE INDEX idx_refresh_tokens_user ON public.refresh_tokens USING btree (user_id)",
			tableName:    "refresh_tokens",
			indexName:    "idx_refresh_tokens_user",
			indexType:    "btree",
			isUnique:     false,
			isExpression: false,
			expectedCols: []string{"user_id"},
		},
		{
			name:         "unqualified table name (refresh_tokens)",
			indexDef:     "CREATE INDEX idx_refresh_tokens_user ON refresh_tokens USING btree (user_id)",
			tableName:    "refresh_tokens",
			indexName:    "idx_refresh_tokens_user",
			indexType:    "btree",
			isUnique:     false,
			isExpression: false,
			expectedCols: []string{"user_id"},
		},
		{
			name:         "schema-qualified with token_hash column",
			indexDef:     "CREATE INDEX idx_refresh_tokens_token ON public.refresh_tokens USING btree (token_hash)",
			tableName:    "refresh_tokens",
			indexName:    "idx_refresh_tokens_token",
			indexType:    "btree",
			isUnique:     false,
			isExpression: false,
			expectedCols: []string{"token_hash"},
		},
		{
			name:         "schema-qualified with expires_at column",
			indexDef:     "CREATE INDEX idx_refresh_tokens_expires ON public.refresh_tokens USING btree (expires_at)",
			tableName:    "refresh_tokens",
			indexName:    "idx_refresh_tokens_expires",
			indexType:    "btree",
			isUnique:     false,
			isExpression: false,
			expectedCols: []string{"expires_at"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, err := introspector.parseIndexDefinition(
				tt.tableName,
				tt.indexName,
				tt.indexType,
				tt.isUnique,
				tt.indexDef,
				nil,
				tt.isExpression,
			)

			assert.NoError(t, err)
			assert.NotNil(t, idx)
			assert.Equal(t, tt.indexName, idx.Name)
			assert.Equal(t, tt.indexType, idx.Type)
			assert.Equal(t, tt.isUnique, idx.Unique)
			assert.Equal(t, tt.expectedCols, idx.Columns, "Columns should be correctly extracted from index definition")
		})
	}
}
