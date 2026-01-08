package schema

import (
	"reflect"
	"testing"
)

// Test column-level index parsing

func TestParseColumnIndex_Simple(t *testing.T) {
	type TestModel struct {
		ID    int    `po:"id,primaryKey,serial"`
		Email string `po:"email,varchar(255),notNull,index"`
	}

	parser := NewParser()
	table, err := parser.Parse(reflect.TypeOf(TestModel{}))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(table.Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(table.Indexes))
	}

	idx := table.Indexes[0]
	if idx.Name != "idx_test_model_email" {
		t.Errorf("expected index name 'idx_test_model_email', got '%s'", idx.Name)
	}
	if len(idx.Columns) != 1 || idx.Columns[0] != "email" {
		t.Errorf("expected columns ['email'], got %v", idx.Columns)
	}
	if idx.Type != "btree" {
		t.Errorf("expected type 'btree', got '%s'", idx.Type)
	}
}

func TestParseColumnIndex_Named(t *testing.T) {
	type TestModel struct {
		ID    int    `po:"id,primaryKey,serial"`
		Email string `po:"email,varchar(255),notNull,index(idx_custom_email)"`
	}

	parser := NewParser()
	table, err := parser.Parse(reflect.TypeOf(TestModel{}))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(table.Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(table.Indexes))
	}

	idx := table.Indexes[0]
	if idx.Name != "idx_custom_email" {
		t.Errorf("expected index name 'idx_custom_email', got '%s'", idx.Name)
	}
}

func TestParseColumnIndex_WithType(t *testing.T) {
	type TestModel struct {
		ID   int      `po:"id,primaryKey,serial"`
		Tags []string `po:"tags,text[],index(idx_tags,gin)"`
	}

	parser := NewParser()
	table, err := parser.Parse(reflect.TypeOf(TestModel{}))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(table.Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(table.Indexes))
	}

	idx := table.Indexes[0]
	if idx.Name != "idx_tags" {
		t.Errorf("expected index name 'idx_tags', got '%s'", idx.Name)
	}
	if idx.Type != "gin" {
		t.Errorf("expected type 'gin', got '%s'", idx.Type)
	}
}

func TestParseColumnIndex_WithOrdering(t *testing.T) {
	type TestModel struct {
		ID        int    `po:"id,primaryKey,serial"`
		CreatedAt string `po:"created_at,timestamptz,notNull,index(idx_created,btree,desc)"`
	}

	parser := NewParser()
	table, err := parser.Parse(reflect.TypeOf(TestModel{}))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(table.Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(table.Indexes))
	}

	idx := table.Indexes[0]
	if idx.Name != "idx_created" {
		t.Errorf("expected index name 'idx_created', got '%s'", idx.Name)
	}
	if len(idx.ColumnOrdering) != 1 {
		t.Fatalf("expected 1 column ordering, got %d", len(idx.ColumnOrdering))
	}
	if idx.ColumnOrdering[0].Direction != Descending {
		t.Errorf("expected direction DESC, got %v", idx.ColumnOrdering[0].Direction)
	}
}

func TestParseColumnIndex_MultipleIndexes(t *testing.T) {
	type TestModel struct {
		ID       int    `po:"id,primaryKey,serial"`
		Email    string `po:"email,varchar(255),notNull,index"`
		Username string `po:"username,varchar(100),notNull,index"`
	}

	parser := NewParser()
	table, err := parser.Parse(reflect.TypeOf(TestModel{}))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(table.Indexes) != 2 {
		t.Fatalf("expected 2 indexes, got %d", len(table.Indexes))
	}
}

// Test table-level index comment parsing

func TestParseIndexFromComment_Simple(t *testing.T) {
	comment := "// index: idx_email ON (email)"
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	if idx.Name != "idx_email" {
		t.Errorf("expected name 'idx_email', got '%s'", idx.Name)
	}
	if len(idx.Columns) != 1 || idx.Columns[0] != "email" {
		t.Errorf("expected columns ['email'], got %v", idx.Columns)
	}
	if idx.Type != "btree" {
		t.Errorf("expected type 'btree', got '%s'", idx.Type)
	}
}

func TestParseIndexFromComment_Expression(t *testing.T) {
	comment := "// index: idx_email_lower ON (lower(email))"
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	if idx.Name != "idx_email_lower" {
		t.Errorf("expected name 'idx_email_lower', got '%s'", idx.Name)
	}
	if idx.Expression != "lower(email)" {
		t.Errorf("expected expression 'lower(email)', got '%s'", idx.Expression)
	}
	if len(idx.Columns) != 0 {
		t.Errorf("expected no columns for expression index, got %v", idx.Columns)
	}
}

func TestParseIndexFromComment_WithType(t *testing.T) {
	comment := "// index: idx_tags ON (tags) USING gin"
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	if idx.Type != "gin" {
		t.Errorf("expected type 'gin', got '%s'", idx.Type)
	}
}

func TestParseIndexFromComment_Partial(t *testing.T) {
	comment := "// index: idx_active ON (email) WHERE deleted_at IS NULL"
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	if idx.Where != "deleted_at IS NULL" {
		t.Errorf("expected where 'deleted_at IS NULL', got '%s'", idx.Where)
	}
}

func TestParseIndexFromComment_WithInclude(t *testing.T) {
	comment := "// index: idx_email ON (email) INCLUDE (name, created_at)"
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	if len(idx.Include) != 2 {
		t.Fatalf("expected 2 include columns, got %d", len(idx.Include))
	}
	if idx.Include[0] != "name" || idx.Include[1] != "created_at" {
		t.Errorf("expected include ['name', 'created_at'], got %v", idx.Include)
	}
}

func TestParseIndexFromComment_MultiColumn(t *testing.T) {
	comment := "// index: idx_multi ON (tenant_id, status, created_at)"
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	if len(idx.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(idx.Columns))
	}
	expectedCols := []string{"tenant_id", "status", "created_at"}
	for i, col := range expectedCols {
		if idx.Columns[i] != col {
			t.Errorf("expected column %d to be '%s', got '%s'", i, col, idx.Columns[i])
		}
	}
}

func TestParseIndexFromComment_WithOrdering(t *testing.T) {
	comment := "// index: idx_ordered ON (tenant_id, created_at DESC)"
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	if len(idx.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(idx.Columns))
	}

	if len(idx.ColumnOrdering) != 1 {
		t.Fatalf("expected 1 column ordering, got %d", len(idx.ColumnOrdering))
	}

	order := idx.ColumnOrdering[0]
	if order.Column != "created_at" {
		t.Errorf("expected ordering for 'created_at', got '%s'", order.Column)
	}
	if order.Direction != Descending {
		t.Errorf("expected DESC direction, got %v", order.Direction)
	}
}

func TestParseIndexFromComment_Complex(t *testing.T) {
	comment := "// index: idx_complex ON (tenant_id, status) USING btree INCLUDE (name) WHERE active = true"
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	if idx.Name != "idx_complex" {
		t.Errorf("expected name 'idx_complex', got '%s'", idx.Name)
	}
	if len(idx.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(idx.Columns))
	}
	if idx.Type != "btree" {
		t.Errorf("expected type 'btree', got '%s'", idx.Type)
	}
	if len(idx.Include) != 1 || idx.Include[0] != "name" {
		t.Errorf("expected include ['name'], got %v", idx.Include)
	}
	if idx.Where != "active = true" {
		t.Errorf("expected where 'active = true', got '%s'", idx.Where)
	}
}

func TestParseIndexFromComment_Invalid(t *testing.T) {
	testCases := []string{
		"// some random comment",
		"// table_name: users",
		"// index: invalid format",
		"",
	}

	for _, comment := range testCases {
		idx := ParseIndexFromComment(comment)
		if idx != nil {
			t.Errorf("expected nil for invalid comment '%s', got %+v", comment, idx)
		}
	}
}

// Test parseIndexColumns helper function

func TestParseIndexColumns_Simple(t *testing.T) {
	cols, ordering := parseIndexColumns("email")

	if len(cols) != 1 || cols[0] != "email" {
		t.Errorf("expected ['email'], got %v", cols)
	}
	if len(ordering) != 0 {
		t.Errorf("expected no ordering, got %v", ordering)
	}
}

func TestParseIndexColumns_Multiple(t *testing.T) {
	cols, ordering := parseIndexColumns("email, name, created_at")

	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	expected := []string{"email", "name", "created_at"}
	for i, col := range expected {
		if cols[i] != col {
			t.Errorf("expected column %d to be '%s', got '%s'", i, col, cols[i])
		}
	}
	if len(ordering) != 0 {
		t.Errorf("expected no ordering, got %v", ordering)
	}
}

func TestParseIndexColumns_WithOrdering(t *testing.T) {
	cols, ordering := parseIndexColumns("tenant_id, created_at DESC, name ASC")

	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}

	if len(ordering) != 2 {
		t.Fatalf("expected 2 orderings, got %d", len(ordering))
	}

	// Check created_at DESC
	if ordering[0].Column != "created_at" {
		t.Errorf("expected first ordering column 'created_at', got '%s'", ordering[0].Column)
	}
	if ordering[0].Direction != Descending {
		t.Errorf("expected first ordering DESC, got %v", ordering[0].Direction)
	}

	// Check name ASC
	if ordering[1].Column != "name" {
		t.Errorf("expected second ordering column 'name', got '%s'", ordering[1].Column)
	}
	if ordering[1].Direction != Ascending {
		t.Errorf("expected second ordering ASC, got %v", ordering[1].Direction)
	}
}

func TestParseIndexColumns_WithNulls(t *testing.T) {
	cols, ordering := parseIndexColumns("created_at DESC NULLS FIRST")

	if len(cols) != 1 || cols[0] != "created_at" {
		t.Fatalf("expected ['created_at'], got %v", cols)
	}

	if len(ordering) != 1 {
		t.Fatalf("expected 1 ordering, got %d", len(ordering))
	}

	if ordering[0].Direction != Descending {
		t.Errorf("expected DESC, got %v", ordering[0].Direction)
	}
	if ordering[0].Nulls != NullsFirst {
		t.Errorf("expected NULLS FIRST, got %v", ordering[0].Nulls)
	}
}

// Test operator class and collation parsing

func TestParseIndexColumns_WithOpClass(t *testing.T) {
	cols, ordering := parseIndexColumns("email varchar_pattern_ops")

	if len(cols) != 1 || cols[0] != "email" {
		t.Fatalf("expected ['email'], got %v", cols)
	}

	if len(ordering) != 1 {
		t.Fatalf("expected 1 ordering, got %d", len(ordering))
	}

	if ordering[0].OpClass != "varchar_pattern_ops" {
		t.Errorf("expected OpClass 'varchar_pattern_ops', got '%s'", ordering[0].OpClass)
	}
}

func TestParseIndexColumns_WithCollation(t *testing.T) {
	cols, ordering := parseIndexColumns(`name COLLATE "en_US"`)

	if len(cols) != 1 || cols[0] != "name" {
		t.Fatalf("expected ['name'], got %v", cols)
	}

	if len(ordering) != 1 {
		t.Fatalf("expected 1 ordering, got %d", len(ordering))
	}

	if ordering[0].Collation != "en_US" {
		t.Errorf("expected Collation 'en_US', got '%s'", ordering[0].Collation)
	}
}

func TestParseIndexColumns_WithOpClassAndCollation(t *testing.T) {
	cols, ordering := parseIndexColumns(`email varchar_pattern_ops COLLATE "C"`)

	if len(cols) != 1 || cols[0] != "email" {
		t.Fatalf("expected ['email'], got %v", cols)
	}

	if len(ordering) != 1 {
		t.Fatalf("expected 1 ordering, got %d", len(ordering))
	}

	if ordering[0].OpClass != "varchar_pattern_ops" {
		t.Errorf("expected OpClass 'varchar_pattern_ops', got '%s'", ordering[0].OpClass)
	}
	if ordering[0].Collation != "C" {
		t.Errorf("expected Collation 'C', got '%s'", ordering[0].Collation)
	}
}

func TestParseIndexColumns_Complete(t *testing.T) {
	cols, ordering := parseIndexColumns(`email varchar_pattern_ops COLLATE "C" DESC NULLS LAST`)

	if len(cols) != 1 || cols[0] != "email" {
		t.Fatalf("expected ['email'], got %v", cols)
	}

	if len(ordering) != 1 {
		t.Fatalf("expected 1 ordering, got %d", len(ordering))
	}

	order := ordering[0]
	if order.OpClass != "varchar_pattern_ops" {
		t.Errorf("expected OpClass 'varchar_pattern_ops', got '%s'", order.OpClass)
	}
	if order.Collation != "C" {
		t.Errorf("expected Collation 'C', got '%s'", order.Collation)
	}
	if order.Direction != Descending {
		t.Errorf("expected DESC, got %v", order.Direction)
	}
	if order.Nulls != NullsLast {
		t.Errorf("expected NULLS LAST, got %v", order.Nulls)
	}
}

func TestParseIndexFromComment_WithOpClass(t *testing.T) {
	comment := "// index: idx_email_pattern ON (email varchar_pattern_ops)"
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	if len(idx.ColumnOrdering) != 1 {
		t.Fatalf("expected 1 column ordering, got %d", len(idx.ColumnOrdering))
	}

	if idx.ColumnOrdering[0].OpClass != "varchar_pattern_ops" {
		t.Errorf("expected OpClass 'varchar_pattern_ops', got '%s'", idx.ColumnOrdering[0].OpClass)
	}
}

func TestParseIndexFromComment_WithCollation(t *testing.T) {
	comment := `// index: idx_name_ci ON (name COLLATE "en_US")`
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	if len(idx.ColumnOrdering) != 1 {
		t.Fatalf("expected 1 column ordering, got %d", len(idx.ColumnOrdering))
	}

	if idx.ColumnOrdering[0].Collation != "en_US" {
		t.Errorf("expected Collation 'en_US', got '%s'", idx.ColumnOrdering[0].Collation)
	}
}

func TestParseIndexFromComment_Concurrently(t *testing.T) {
	comment := "// index: idx_email ON (email) CONCURRENTLY"
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	if !idx.Concurrent {
		t.Error("expected Concurrent to be true")
	}
}

func TestParseIndexFromComment_ConcurrentlyWithWhere(t *testing.T) {
	comment := "// index: idx_active ON (email) WHERE deleted_at IS NULL CONCURRENTLY"
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	if idx.Where != "deleted_at IS NULL" {
		t.Errorf("expected where 'deleted_at IS NULL', got '%s'", idx.Where)
	}

	if !idx.Concurrent {
		t.Error("expected Concurrent to be true")
	}
}

func TestParseIndexFromComment_AdvancedComplete(t *testing.T) {
	comment := `// index: idx_advanced ON (email varchar_pattern_ops COLLATE "C" DESC, name text_pattern_ops ASC) USING btree INCLUDE (created_at) WHERE active = true CONCURRENTLY`
	idx := ParseIndexFromComment(comment)

	if idx == nil {
		t.Fatal("expected index to be parsed, got nil")
	}

	// Check columns
	if len(idx.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(idx.Columns))
	}

	// Check first column (email)
	if len(idx.ColumnOrdering) < 1 {
		t.Fatal("expected column ordering for email")
	}
	emailOrder := idx.ColumnOrdering[0]
	if emailOrder.Column != "email" {
		t.Errorf("expected column 'email', got '%s'", emailOrder.Column)
	}
	if emailOrder.OpClass != "varchar_pattern_ops" {
		t.Errorf("expected OpClass 'varchar_pattern_ops', got '%s'", emailOrder.OpClass)
	}
	if emailOrder.Collation != "C" {
		t.Errorf("expected Collation 'C', got '%s'", emailOrder.Collation)
	}
	if emailOrder.Direction != Descending {
		t.Errorf("expected DESC, got %v", emailOrder.Direction)
	}

	// Check second column (name)
	if len(idx.ColumnOrdering) < 2 {
		t.Fatal("expected column ordering for name")
	}
	nameOrder := idx.ColumnOrdering[1]
	if nameOrder.Column != "name" {
		t.Errorf("expected column 'name', got '%s'", nameOrder.Column)
	}
	if nameOrder.OpClass != "text_pattern_ops" {
		t.Errorf("expected OpClass 'text_pattern_ops', got '%s'", nameOrder.OpClass)
	}

	// Check other fields
	if idx.Type != "btree" {
		t.Errorf("expected type 'btree', got '%s'", idx.Type)
	}
	if len(idx.Include) != 1 || idx.Include[0] != "created_at" {
		t.Errorf("expected include ['created_at'], got %v", idx.Include)
	}
	if idx.Where != "active = true" {
		t.Errorf("expected where 'active = true', got '%s'", idx.Where)
	}
	if !idx.Concurrent {
		t.Error("expected Concurrent to be true")
	}
}
