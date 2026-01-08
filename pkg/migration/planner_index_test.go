package migration

import (
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func TestGenerateCreateIndex_Simple(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_users_email",
		Columns: []string{"email"},
		Type:    "btree",
	}

	sql := planner.generateCreateIndex("users", idx)

	expected := "CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);"
	if sql != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, sql)
	}
}

func TestGenerateCreateIndex_Unique(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_users_email_unique",
		Columns: []string{"email"},
		Type:    "btree",
		Unique:  true,
	}

	sql := planner.generateCreateIndex("users", idx)

	if !strings.Contains(sql, "CREATE UNIQUE INDEX") {
		t.Errorf("Expected UNIQUE INDEX in:\n%s", sql)
	}
}

func TestGenerateCreateIndex_WithType(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_tags",
		Columns: []string{"tags"},
		Type:    "gin",
	}

	sql := planner.generateCreateIndex("posts", idx)

	if !strings.Contains(sql, "USING gin") {
		t.Errorf("Expected USING gin in:\n%s", sql)
	}
}

func TestGenerateCreateIndex_Expression(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:       "idx_email_lower",
		Expression: "lower(email)",
		Type:       "btree",
	}

	sql := planner.generateCreateIndex("users", idx)

	expected := "CREATE INDEX IF NOT EXISTS idx_email_lower ON users (lower(email));"
	if sql != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, sql)
	}
}

func TestGenerateCreateIndex_Partial(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_active_users",
		Columns: []string{"email"},
		Type:    "btree",
		Where:   "deleted_at IS NULL",
	}

	sql := planner.generateCreateIndex("users", idx)

	if !strings.Contains(sql, "WHERE deleted_at IS NULL") {
		t.Errorf("Expected WHERE clause in:\n%s", sql)
	}
}

func TestGenerateCreateIndex_WithInclude(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_email_covering",
		Columns: []string{"email"},
		Type:    "btree",
		Include: []string{"name", "created_at"},
	}

	sql := planner.generateCreateIndex("users", idx)

	if !strings.Contains(sql, "INCLUDE (name, created_at)") {
		t.Errorf("Expected INCLUDE clause in:\n%s", sql)
	}
}

func TestGenerateCreateIndex_MultiColumn(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_tenant_user",
		Columns: []string{"tenant_id", "user_id"},
		Type:    "btree",
	}

	sql := planner.generateCreateIndex("posts", idx)

	expected := "CREATE INDEX IF NOT EXISTS idx_tenant_user ON posts (tenant_id, user_id);"
	if sql != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, sql)
	}
}

func TestGenerateCreateIndex_WithOrdering(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_created",
		Columns: []string{"created_at"},
		Type:    "btree",
		ColumnOrdering: []schema.ColumnOrder{
			{
				Column:    "created_at",
				Direction: schema.Descending,
			},
		},
	}

	sql := planner.generateCreateIndex("posts", idx)

	if !strings.Contains(sql, "created_at DESC") {
		t.Errorf("Expected DESC ordering in:\n%s", sql)
	}
}

func TestGenerateCreateIndex_WithNullsOrdering(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_priority",
		Columns: []string{"priority"},
		Type:    "btree",
		ColumnOrdering: []schema.ColumnOrder{
			{
				Column:    "priority",
				Direction: schema.Descending,
				Nulls:     schema.NullsLast,
			},
		},
	}

	sql := planner.generateCreateIndex("tasks", idx)

	if !strings.Contains(sql, "priority DESC NULLS LAST") {
		t.Errorf("Expected DESC NULLS LAST in:\n%s", sql)
	}
}

func TestGenerateCreateIndex_MultiColumnWithMixedOrdering(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_tenant_created",
		Columns: []string{"tenant_id", "created_at"},
		Type:    "btree",
		ColumnOrdering: []schema.ColumnOrder{
			// tenant_id has default ASC, not specified
			{
				Column:    "created_at",
				Direction: schema.Descending,
			},
		},
	}

	sql := planner.generateCreateIndex("posts", idx)

	// tenant_id should not have DESC, created_at should
	if !strings.Contains(sql, "tenant_id, created_at DESC") {
		t.Errorf("Expected mixed ordering in:\n%s", sql)
	}
}

func TestGenerateCreateIndex_Concurrent(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:       "idx_email",
		Columns:    []string{"email"},
		Type:       "btree",
		Concurrent: true,
	}

	sql := planner.generateCreateIndex("users", idx)

	if !strings.Contains(sql, "CONCURRENTLY") {
		t.Errorf("Expected CONCURRENTLY in:\n%s", sql)
	}
}

func TestGenerateCreateIndex_Complex(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_complex",
		Columns: []string{"tenant_id", "status", "created_at"},
		Type:    "btree",
		ColumnOrdering: []schema.ColumnOrder{
			{
				Column:    "created_at",
				Direction: schema.Descending,
				Nulls:     schema.NullsLast,
			},
		},
		Include: []string{"user_id", "amount"},
		Where:   "deleted_at IS NULL AND active = true",
	}

	sql := planner.generateCreateIndex("orders", idx)

	// Check all components are present
	checks := []string{
		"CREATE INDEX IF NOT EXISTS idx_complex ON orders",
		"tenant_id, status, created_at DESC NULLS LAST",
		"INCLUDE (user_id, amount)",
		"WHERE deleted_at IS NULL AND active = true",
	}

	for _, check := range checks {
		if !strings.Contains(sql, check) {
			t.Errorf("Expected '%s' in:\n%s", check, sql)
		}
	}
}

func TestFormatColumnsWithOrdering_NoOrdering(t *testing.T) {
	planner := NewPlanner()

	columns := []string{"col1", "col2", "col3"}
	result := planner.formatColumnsWithOrdering(columns, nil)

	expected := "col1, col2, col3"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestFormatColumnsWithOrdering_WithOrdering(t *testing.T) {
	planner := NewPlanner()

	columns := []string{"col1", "col2", "col3"}
	ordering := []schema.ColumnOrder{
		{
			Column:    "col2",
			Direction: schema.Descending,
		},
		{
			Column:    "col3",
			Direction: schema.Ascending,
			Nulls:     schema.NullsFirst,
		},
	}

	result := planner.formatColumnsWithOrdering(columns, ordering)

	expected := "col1, col2 DESC, col3 NULLS FIRST"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestGenerateCreateIndex_DefaultBtreeNotShown(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_email",
		Columns: []string{"email"},
		Type:    "btree",
	}

	sql := planner.generateCreateIndex("users", idx)

	// USING btree should not be in the output (it's the default)
	if strings.Contains(sql, "USING btree") {
		t.Errorf("Expected no 'USING btree' (default) in:\n%s", sql)
	}
}

func TestGenerateCreateIndex_WithoutIfNotExists(t *testing.T) {
	planner := NewPlannerWithOptions(PlannerOptions{
		IfNotExists: false,
	})

	idx := schema.IndexMetadata{
		Name:    "idx_email",
		Columns: []string{"email"},
		Type:    "btree",
	}

	sql := planner.generateCreateIndex("users", idx)

	if strings.Contains(sql, "IF NOT EXISTS") {
		t.Errorf("Expected no 'IF NOT EXISTS' in:\n%s", sql)
	}

	expected := "CREATE INDEX idx_email ON users (email);"
	if sql != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, sql)
	}
}

// Tests for advanced features (operator classes, collations, CONCURRENTLY)

func TestGenerateCreateIndex_WithOpClass(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_email_pattern",
		Columns: []string{"email"},
		Type:    "btree",
		ColumnOrdering: []schema.ColumnOrder{
			{
				Column:  "email",
				OpClass: "varchar_pattern_ops",
			},
		},
	}

	sql := planner.generateCreateIndex("users", idx)

	if !strings.Contains(sql, "email varchar_pattern_ops") {
		t.Errorf("Expected 'email varchar_pattern_ops' in:\n%s", sql)
	}
}

func TestGenerateCreateIndex_WithCollation(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_name_ci",
		Columns: []string{"name"},
		Type:    "btree",
		ColumnOrdering: []schema.ColumnOrder{
			{
				Column:    "name",
				Collation: "en_US",
			},
		},
	}

	sql := planner.generateCreateIndex("users", idx)

	if !strings.Contains(sql, `COLLATE "en_US"`) {
		t.Errorf("Expected 'COLLATE \"en_US\"' in:\n%s", sql)
	}
}

func TestGenerateCreateIndex_WithOpClassAndCollation(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_email_advanced",
		Columns: []string{"email"},
		Type:    "btree",
		ColumnOrdering: []schema.ColumnOrder{
			{
				Column:    "email",
				OpClass:   "varchar_pattern_ops",
				Collation: "C",
			},
		},
	}

	sql := planner.generateCreateIndex("users", idx)

	if !strings.Contains(sql, "varchar_pattern_ops") {
		t.Errorf("Expected operator class in:\n%s", sql)
	}
	if !strings.Contains(sql, `COLLATE "C"`) {
		t.Errorf("Expected collation in:\n%s", sql)
	}
}

func TestGenerateCreateIndex_CompleteAdvanced(t *testing.T) {
	planner := NewPlanner()

	idx := schema.IndexMetadata{
		Name:    "idx_advanced",
		Columns: []string{"email", "name"},
		Type:    "btree",
		ColumnOrdering: []schema.ColumnOrder{
			{
				Column:    "email",
				OpClass:   "varchar_pattern_ops",
				Collation: "C",
				Direction: schema.Descending,
				Nulls:     schema.NullsLast,
			},
			{
				Column:    "name",
				OpClass:   "text_pattern_ops",
				Direction: schema.Ascending,
			},
		},
		Include:    []string{"created_at"},
		Where:      "active = true",
		Concurrent: true,
	}

	sql := planner.generateCreateIndex("users", idx)

	// Check all components
	checks := []string{
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_advanced ON users",
		"email varchar_pattern_ops",
		`COLLATE "C"`,
		"DESC NULLS LAST",
		"name text_pattern_ops",
		"INCLUDE (created_at)",
		"WHERE active = true",
	}

	for _, check := range checks {
		if !strings.Contains(sql, check) {
			t.Errorf("Expected '%s' in:\n%s", check, sql)
		}
	}
}

func TestFormatColumnsWithOrdering_WithOpClass(t *testing.T) {
	planner := NewPlanner()

	columns := []string{"email"}
	ordering := []schema.ColumnOrder{
		{
			Column:  "email",
			OpClass: "varchar_pattern_ops",
		},
	}

	result := planner.formatColumnsWithOrdering(columns, ordering)

	expected := "email varchar_pattern_ops"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestFormatColumnsWithOrdering_WithCollation(t *testing.T) {
	planner := NewPlanner()

	columns := []string{"name"}
	ordering := []schema.ColumnOrder{
		{
			Column:    "name",
			Collation: "en_US",
		},
	}

	result := planner.formatColumnsWithOrdering(columns, ordering)

	expected := `name COLLATE "en_US"`
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestFormatColumnsWithOrdering_CompleteModifiers(t *testing.T) {
	planner := NewPlanner()

	columns := []string{"email"}
	ordering := []schema.ColumnOrder{
		{
			Column:    "email",
			OpClass:   "varchar_pattern_ops",
			Collation: "C",
			Direction: schema.Descending,
			Nulls:     schema.NullsLast,
		},
	}

	result := planner.formatColumnsWithOrdering(columns, ordering)

	expected := `email varchar_pattern_ops COLLATE "C" DESC NULLS LAST`
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

// Integration test with full migration generation

func TestFullMigrationWithIndexes(t *testing.T) {
	planner := NewPlanner()

	codeSchema := map[string]*schema.TableMetadata{
		"users": {
			Name: "users",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "serial", Nullable: false},
				{Name: "email", SQLType: "varchar(255)", Nullable: false},
				{Name: "name", SQLType: "varchar(255)", Nullable: false},
				{Name: "created_at", SQLType: "timestamptz", Nullable: false},
				{Name: "deleted_at", SQLType: "timestamptz", Nullable: true},
			},
			PrimaryKey: &schema.PrimaryKeyMetadata{
				Name:    "users_pkey",
				Columns: []string{"id"},
			},
			Indexes: []schema.IndexMetadata{
				{
					Name:    "idx_email",
					Columns: []string{"email"},
					Type:    "btree",
				},
				{
					Name:       "idx_email_lower",
					Expression: "lower(email)",
					Type:       "btree",
				},
				{
					Name:    "idx_active_users",
					Columns: []string{"email"},
					Type:    "btree",
					Where:   "deleted_at IS NULL",
				},
			},
		},
	}

	dbSchema := map[string]*schema.TableMetadata{}

	differ := NewDiffer()
	diff := differ.Compare(codeSchema, dbSchema)

	upSQL, _ := planner.GenerateMigration(diff)

	// Check that all indexes are generated
	expectedIndexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_email ON users (email)",
		"CREATE INDEX IF NOT EXISTS idx_email_lower ON users (lower(email))",
		"CREATE INDEX IF NOT EXISTS idx_active_users ON users (email)",
		"WHERE deleted_at IS NULL",
	}

	for _, expected := range expectedIndexes {
		if !strings.Contains(upSQL, expected) {
			t.Errorf("Expected '%s' in migration:\n%s", expected, upSQL)
		}
	}
}
