package migration

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func TestExtractBalancedParens(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantContent string
		wantRemaining string
	}{
		{
			name:      "simple",
			input:     "email)",
			wantContent: "email",
			wantRemaining: "",
		},
		{
			name:      "nested parentheses",
			input:     "lower(email))",
			wantContent: "lower(email)",
			wantRemaining: "",
		},
		{
			name:      "with remaining",
			input:     "email) INCLUDE (name)",
			wantContent: "email",
			wantRemaining: " INCLUDE (name)",
		},
		{
			name:      "nested with remaining",
			input:     "lower(email)) WHERE active = true",
			wantContent: "lower(email)",
			wantRemaining: " WHERE active = true",
		},
		{
			name:      "multiple nested",
			input:     "COALESCE(lower(email), 'default'))",
			wantContent: "COALESCE(lower(email), 'default')",
			wantRemaining: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotContent, gotRemaining := extractBalancedParens(tt.input)
			if gotContent != tt.wantContent {
				t.Errorf("extractBalancedParens() content = %q, want %q", gotContent, tt.wantContent)
			}
			if gotRemaining != tt.wantRemaining {
				t.Errorf("extractBalancedParens() remaining = %q, want %q", gotRemaining, tt.wantRemaining)
			}
		})
	}
}

func TestSplitRespectingParens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		delim rune
		want  []string
	}{
		{
			name:  "simple columns",
			input: "email, name, created_at",
			delim: ',',
			want:  []string{"email", " name", " created_at"},
		},
		{
			name:  "with function",
			input: "lower(email), name",
			delim: ',',
			want:  []string{"lower(email)", " name"},
		},
		{
			name:  "nested function",
			input: "COALESCE(lower(email), 'default'), name",
			delim: ',',
			want:  []string{"COALESCE(lower(email), 'default')", " name"},
		},
		{
			name:  "single column",
			input: "email",
			delim: ',',
			want:  []string{"email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitRespectingParens(tt.input, tt.delim)
			if len(got) != len(tt.want) {
				t.Errorf("splitRespectingParens() got %d parts, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitRespectingParens() part[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTokenizeIndexColumn(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple column",
			input: "email",
			want:  []string{"email"},
		},
		{
			name:  "column with DESC",
			input: "email DESC",
			want:  []string{"email", "DESC"},
		},
		{
			name:  "column with operator class",
			input: "email varchar_pattern_ops",
			want:  []string{"email", "varchar_pattern_ops"},
		},
		{
			name:  "column with collation",
			input: `email COLLATE "en_US"`,
			want:  []string{"email", "COLLATE", `"en_US"`},
		},
		{
			name:  "column with all modifiers",
			input: `email varchar_pattern_ops COLLATE "C" DESC NULLS LAST`,
			want:  []string{"email", "varchar_pattern_ops", "COLLATE", `"C"`, "DESC", "NULLS", "LAST"},
		},
		{
			name:  "single quoted collation",
			input: `name COLLATE 'en_US'`,
			want:  []string{"name", "COLLATE", `'en_US'`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenizeIndexColumn(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("tokenizeIndexColumn() got %d tokens, want %d\nGot: %v\nWant: %v", len(got), len(tt.want), got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("tokenizeIndexColumn() token[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseIndexColumn(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantColumn  string
		wantOrdering schema.ColumnOrder
	}{
		{
			name:       "simple column",
			input:      "email",
			wantColumn: "email",
			wantOrdering: schema.ColumnOrder{
				Column: "email",
			},
		},
		{
			name:       "column with DESC",
			input:      "email DESC",
			wantColumn: "email",
			wantOrdering: schema.ColumnOrder{
				Column:    "email",
				Direction: schema.Descending,
			},
		},
		{
			name:       "column with ASC",
			input:      "email ASC",
			wantColumn: "email",
			wantOrdering: schema.ColumnOrder{
				Column:    "email",
				Direction: schema.Ascending,
			},
		},
		{
			name:       "column with NULLS FIRST",
			input:      "priority NULLS FIRST",
			wantColumn: "priority",
			wantOrdering: schema.ColumnOrder{
				Column: "priority",
				Nulls:  schema.NullsFirst,
			},
		},
		{
			name:       "column with DESC NULLS LAST",
			input:      "created_at DESC NULLS LAST",
			wantColumn: "created_at",
			wantOrdering: schema.ColumnOrder{
				Column:    "created_at",
				Direction: schema.Descending,
				Nulls:     schema.NullsLast,
			},
		},
		{
			name:       "column with operator class",
			input:      "email varchar_pattern_ops",
			wantColumn: "email",
			wantOrdering: schema.ColumnOrder{
				Column:  "email",
				OpClass: "varchar_pattern_ops",
			},
		},
		{
			name:       "column with collation",
			input:      `name COLLATE "en_US"`,
			wantColumn: "name",
			wantOrdering: schema.ColumnOrder{
				Column:    "name",
				Collation: "en_US", // Quotes removed
			},
		},
		{
			name:       "column with all modifiers",
			input:      `email varchar_pattern_ops COLLATE "C" DESC NULLS LAST`,
			wantColumn: "email",
			wantOrdering: schema.ColumnOrder{
				Column:    "email",
				OpClass:   "varchar_pattern_ops",
				Collation: "C",
				Direction: schema.Descending,
				Nulls:     schema.NullsLast,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotColumn, gotOrdering := parseIndexColumn(tt.input)
			if gotColumn != tt.wantColumn {
				t.Errorf("parseIndexColumn() column = %q, want %q", gotColumn, tt.wantColumn)
			}
			if gotOrdering.Column != tt.wantOrdering.Column {
				t.Errorf("parseIndexColumn() ordering.Column = %q, want %q", gotOrdering.Column, tt.wantOrdering.Column)
			}
			if gotOrdering.Direction != tt.wantOrdering.Direction {
				t.Errorf("parseIndexColumn() ordering.Direction = %q, want %q", gotOrdering.Direction, tt.wantOrdering.Direction)
			}
			if gotOrdering.Nulls != tt.wantOrdering.Nulls {
				t.Errorf("parseIndexColumn() ordering.Nulls = %q, want %q", gotOrdering.Nulls, tt.wantOrdering.Nulls)
			}
			if gotOrdering.OpClass != tt.wantOrdering.OpClass {
				t.Errorf("parseIndexColumn() ordering.OpClass = %q, want %q", gotOrdering.OpClass, tt.wantOrdering.OpClass)
			}
			if gotOrdering.Collation != tt.wantOrdering.Collation {
				t.Errorf("parseIndexColumn() ordering.Collation = %q, want %q", gotOrdering.Collation, tt.wantOrdering.Collation)
			}
		})
	}
}

func TestParseIndexColumnList(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantColumns   []string
		wantOrderings []schema.ColumnOrder
	}{
		{
			name:          "simple columns",
			input:         "email, name",
			wantColumns:   []string{"email", "name"},
			wantOrderings: []schema.ColumnOrder{}, // No non-default orderings
		},
		{
			name:        "columns with DESC",
			input:       "email, created_at DESC",
			wantColumns: []string{"email", "created_at"},
			wantOrderings: []schema.ColumnOrder{
				{
					Column:    "created_at",
					Direction: schema.Descending,
				},
			},
		},
		{
			name:        "columns with mixed ordering",
			input:       "tenant_id, created_at DESC NULLS LAST",
			wantColumns: []string{"tenant_id", "created_at"},
			wantOrderings: []schema.ColumnOrder{
				{
					Column:    "created_at",
					Direction: schema.Descending,
					Nulls:     schema.NullsLast,
				},
			},
		},
		{
			name:        "column with operator class",
			input:       "email varchar_pattern_ops",
			wantColumns: []string{"email"},
			wantOrderings: []schema.ColumnOrder{
				{
					Column:  "email",
					OpClass: "varchar_pattern_ops",
				},
			},
		},
		{
			name:        "column with collation",
			input:       `name COLLATE "en_US"`,
			wantColumns: []string{"name"},
			wantOrderings: []schema.ColumnOrder{
				{
					Column:    "name",
					Collation: "en_US",
				},
			},
		},
		{
			name:        "complex multi-column",
			input:       `email varchar_pattern_ops COLLATE "C" DESC, name, created_at DESC NULLS LAST`,
			wantColumns: []string{"email", "name", "created_at"},
			wantOrderings: []schema.ColumnOrder{
				{
					Column:    "email",
					OpClass:   "varchar_pattern_ops",
					Collation: "C",
					Direction: schema.Descending,
				},
				{
					Column:    "created_at",
					Direction: schema.Descending,
					Nulls:     schema.NullsLast,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotColumns, gotOrderings := parseIndexColumnList(tt.input)

			// Check columns
			if len(gotColumns) != len(tt.wantColumns) {
				t.Errorf("parseIndexColumnList() got %d columns, want %d", len(gotColumns), len(tt.wantColumns))
				return
			}
			for i := range gotColumns {
				if gotColumns[i] != tt.wantColumns[i] {
					t.Errorf("parseIndexColumnList() column[%d] = %q, want %q", i, gotColumns[i], tt.wantColumns[i])
				}
			}

			// Check orderings
			if len(gotOrderings) != len(tt.wantOrderings) {
				t.Errorf("parseIndexColumnList() got %d orderings, want %d\nGot: %+v\nWant: %+v",
					len(gotOrderings), len(tt.wantOrderings), gotOrderings, tt.wantOrderings)
				return
			}
			for i := range gotOrderings {
				got := gotOrderings[i]
				want := tt.wantOrderings[i]

				if got.Column != want.Column {
					t.Errorf("parseIndexColumnList() ordering[%d].Column = %q, want %q", i, got.Column, want.Column)
				}
				if got.Direction != want.Direction {
					t.Errorf("parseIndexColumnList() ordering[%d].Direction = %q, want %q", i, got.Direction, want.Direction)
				}
				if got.Nulls != want.Nulls {
					t.Errorf("parseIndexColumnList() ordering[%d].Nulls = %q, want %q", i, got.Nulls, want.Nulls)
				}
				if got.OpClass != want.OpClass {
					t.Errorf("parseIndexColumnList() ordering[%d].OpClass = %q, want %q", i, got.OpClass, want.OpClass)
				}
				if got.Collation != want.Collation {
					t.Errorf("parseIndexColumnList() ordering[%d].Collation = %q, want %q", i, got.Collation, want.Collation)
				}
			}
		})
	}
}

func TestIsReservedIndexKeyword(t *testing.T) {
	tests := []struct {
		token string
		want  bool
	}{
		{"ASC", true},
		{"DESC", true},
		{"NULLS", true},
		{"FIRST", true},
		{"LAST", true},
		{"COLLATE", true},
		{"USING", true},
		{"WHERE", true},
		{"INCLUDE", true},
		{"asc", true},    // Case insensitive
		{"desc", true},
		{"varchar_pattern_ops", false},
		{"text_pattern_ops", false},
		{"btree", false},
		{"gin", false},
		{"email", false},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			got := isReservedIndexKeyword(tt.token)
			if got != tt.want {
				t.Errorf("isReservedIndexKeyword(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestParseIndexDefinition(t *testing.T) {
	i := &Introspector{}

	tests := []struct {
		name         string
		tableName    string
		indexName    string
		indexType    string
		isUnique     bool
		indexDef     string
		predicate    *string
		isExpression bool
		want         *schema.IndexMetadata
	}{
		{
			name:      "simple index",
			tableName: "users",
			indexName: "idx_email",
			indexType: "btree",
			isUnique:  false,
			indexDef:  "CREATE INDEX idx_email ON users USING btree (email)",
			want: &schema.IndexMetadata{
				Name:    "idx_email",
				Type:    "btree",
				Unique:  false,
				Columns: []string{"email"},
			},
		},
		{
			name:      "unique index",
			tableName: "users",
			indexName: "idx_email_unique",
			indexType: "btree",
			isUnique:  true,
			indexDef:  "CREATE UNIQUE INDEX idx_email_unique ON users USING btree (email)",
			want: &schema.IndexMetadata{
				Name:    "idx_email_unique",
				Type:    "btree",
				Unique:  true,
				Columns: []string{"email"},
			},
		},
		{
			name:      "gin index",
			tableName: "posts",
			indexName: "idx_tags",
			indexType: "gin",
			isUnique:  false,
			indexDef:  "CREATE INDEX idx_tags ON posts USING gin (tags)",
			want: &schema.IndexMetadata{
				Name:    "idx_tags",
				Type:    "gin",
				Unique:  false,
				Columns: []string{"tags"},
			},
		},
		{
			name:      "index with DESC",
			tableName: "posts",
			indexName: "idx_created",
			indexType: "btree",
			isUnique:  false,
			indexDef:  "CREATE INDEX idx_created ON posts USING btree (created_at DESC)",
			want: &schema.IndexMetadata{
				Name:    "idx_created",
				Type:    "btree",
				Unique:  false,
				Columns: []string{"created_at"},
				ColumnOrdering: []schema.ColumnOrder{
					{
						Column:    "created_at",
						Direction: schema.Descending,
					},
				},
			},
		},
		{
			name:      "index with operator class",
			tableName: "users",
			indexName: "idx_email_pattern",
			indexType: "btree",
			isUnique:  false,
			indexDef:  "CREATE INDEX idx_email_pattern ON users USING btree (email varchar_pattern_ops)",
			want: &schema.IndexMetadata{
				Name:    "idx_email_pattern",
				Type:    "btree",
				Unique:  false,
				Columns: []string{"email"},
				ColumnOrdering: []schema.ColumnOrder{
					{
						Column:  "email",
						OpClass: "varchar_pattern_ops",
					},
				},
			},
		},
		{
			name:      "index with collation",
			tableName: "users",
			indexName: "idx_name_ci",
			indexType: "btree",
			isUnique:  false,
			indexDef:  `CREATE INDEX idx_name_ci ON users USING btree (name COLLATE "en_US")`,
			want: &schema.IndexMetadata{
				Name:    "idx_name_ci",
				Type:    "btree",
				Unique:  false,
				Columns: []string{"name"},
				ColumnOrdering: []schema.ColumnOrder{
					{
						Column:    "name",
						Collation: "en_US",
					},
				},
			},
		},
		{
			name:         "expression index",
			tableName:    "users",
			indexName:    "idx_lower_email",
			indexType:    "btree",
			isUnique:     false,
			indexDef:     "CREATE INDEX idx_lower_email ON users USING btree (lower(email))",
			isExpression: true,
			want: &schema.IndexMetadata{
				Name:       "idx_lower_email",
				Type:       "btree",
				Unique:     false,
				Expression: "lower(email)",
			},
		},
		{
			name:      "index with INCLUDE",
			tableName: "users",
			indexName: "idx_email_covering",
			indexType: "btree",
			isUnique:  false,
			indexDef:  "CREATE INDEX idx_email_covering ON users USING btree (email) INCLUDE (name, created_at)",
			want: &schema.IndexMetadata{
				Name:    "idx_email_covering",
				Type:    "btree",
				Unique:  false,
				Columns: []string{"email"},
				Include: []string{"name", "created_at"},
			},
		},
		{
			name:      "complex index",
			tableName: "users",
			indexName: "idx_complex",
			indexType: "btree",
			isUnique:  false,
			indexDef:  `CREATE INDEX idx_complex ON users USING btree (email varchar_pattern_ops COLLATE "C" DESC NULLS LAST, name) INCLUDE (created_at)`,
			want: &schema.IndexMetadata{
				Name:    "idx_complex",
				Type:    "btree",
				Unique:  false,
				Columns: []string{"email", "name"},
				Include: []string{"created_at"},
				ColumnOrdering: []schema.ColumnOrder{
					{
						Column:    "email",
						OpClass:   "varchar_pattern_ops",
						Collation: "C",
						Direction: schema.Descending,
						Nulls:     schema.NullsLast,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := i.parseIndexDefinition(nil, tt.tableName, tt.indexName, tt.indexType, tt.isUnique, tt.indexDef, tt.predicate, tt.isExpression)
			if err != nil {
				t.Fatalf("parseIndexDefinition() error = %v", err)
			}

			// Compare basic fields
			if got.Name != tt.want.Name {
				t.Errorf("parseIndexDefinition() Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Type != tt.want.Type {
				t.Errorf("parseIndexDefinition() Type = %q, want %q", got.Type, tt.want.Type)
			}
			if got.Unique != tt.want.Unique {
				t.Errorf("parseIndexDefinition() Unique = %v, want %v", got.Unique, tt.want.Unique)
			}
			if got.Expression != tt.want.Expression {
				t.Errorf("parseIndexDefinition() Expression = %q, want %q", got.Expression, tt.want.Expression)
			}

			// Compare columns
			if len(got.Columns) != len(tt.want.Columns) {
				t.Errorf("parseIndexDefinition() got %d columns, want %d", len(got.Columns), len(tt.want.Columns))
			} else {
				for j := range got.Columns {
					if got.Columns[j] != tt.want.Columns[j] {
						t.Errorf("parseIndexDefinition() Columns[%d] = %q, want %q", j, got.Columns[j], tt.want.Columns[j])
					}
				}
			}

			// Compare include columns
			if len(got.Include) != len(tt.want.Include) {
				t.Errorf("parseIndexDefinition() got %d include columns, want %d", len(got.Include), len(tt.want.Include))
			} else {
				for j := range got.Include {
					if got.Include[j] != tt.want.Include[j] {
						t.Errorf("parseIndexDefinition() Include[%d] = %q, want %q", j, got.Include[j], tt.want.Include[j])
					}
				}
			}

			// Compare column ordering
			if len(got.ColumnOrdering) != len(tt.want.ColumnOrdering) {
				t.Errorf("parseIndexDefinition() got %d column orderings, want %d\nGot: %+v\nWant: %+v",
					len(got.ColumnOrdering), len(tt.want.ColumnOrdering), got.ColumnOrdering, tt.want.ColumnOrdering)
			} else {
				for j := range got.ColumnOrdering {
					gotOrd := got.ColumnOrdering[j]
					wantOrd := tt.want.ColumnOrdering[j]

					if gotOrd.Column != wantOrd.Column {
						t.Errorf("parseIndexDefinition() ColumnOrdering[%d].Column = %q, want %q", j, gotOrd.Column, wantOrd.Column)
					}
					if gotOrd.Direction != wantOrd.Direction {
						t.Errorf("parseIndexDefinition() ColumnOrdering[%d].Direction = %q, want %q", j, gotOrd.Direction, wantOrd.Direction)
					}
					if gotOrd.Nulls != wantOrd.Nulls {
						t.Errorf("parseIndexDefinition() ColumnOrdering[%d].Nulls = %q, want %q", j, gotOrd.Nulls, wantOrd.Nulls)
					}
					if gotOrd.OpClass != wantOrd.OpClass {
						t.Errorf("parseIndexDefinition() ColumnOrdering[%d].OpClass = %q, want %q", j, gotOrd.OpClass, wantOrd.OpClass)
					}
					if gotOrd.Collation != wantOrd.Collation {
						t.Errorf("parseIndexDefinition() ColumnOrdering[%d].Collation = %q, want %q", j, gotOrd.Collation, wantOrd.Collation)
					}
				}
			}
		})
	}
}
