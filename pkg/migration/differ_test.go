package migration

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func TestCompareTablesAdded(t *testing.T) {
	differ := NewDiffer()

	codeSchema := map[string]*schema.TableMetadata{
		"users": {
			Name: "users",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "serial"},
				{Name: "email", SQLType: "varchar(255)"},
			},
		},
	}

	dbSchema := map[string]*schema.TableMetadata{}

	diff := differ.Compare(codeSchema, dbSchema)

	if len(diff.TablesAdded) != 1 {
		t.Errorf("Expected 1 table added, got %d", len(diff.TablesAdded))
	}
	if diff.TablesAdded[0].Name != "users" {
		t.Errorf("Expected users table, got %s", diff.TablesAdded[0].Name)
	}
	if len(diff.TablesDropped) != 0 {
		t.Errorf("Expected 0 tables dropped, got %d", len(diff.TablesDropped))
	}
	if len(diff.TablesModified) != 0 {
		t.Errorf("Expected 0 tables modified, got %d", len(diff.TablesModified))
	}
}

func TestCompareTablesDropped(t *testing.T) {
	differ := NewDiffer()

	codeSchema := map[string]*schema.TableMetadata{}

	dbSchema := map[string]*schema.TableMetadata{
		"old_users": {
			Name: "old_users",
			Columns: []schema.ColumnMetadata{
				{Name: "id", SQLType: "serial"},
			},
		},
	}

	diff := differ.Compare(codeSchema, dbSchema)

	if len(diff.TablesAdded) != 0 {
		t.Errorf("Expected 0 tables added, got %d", len(diff.TablesAdded))
	}
	if len(diff.TablesDropped) != 1 {
		t.Errorf("Expected 1 table dropped, got %d", len(diff.TablesDropped))
	}
	if diff.TablesDropped[0] != "old_users" {
		t.Errorf("Expected old_users table, got %s", diff.TablesDropped[0])
	}
	if len(diff.TablesModified) != 0 {
		t.Errorf("Expected 0 tables modified, got %d", len(diff.TablesModified))
	}
}

func TestCompareColumnsAdded(t *testing.T) {
	differ := NewDiffer()

	codeTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "serial"},
			{Name: "email", SQLType: "varchar(255)"},
			{Name: "phone", SQLType: "varchar(20)"}, // New column
		},
	}

	dbTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "serial"},
			{Name: "email", SQLType: "varchar(255)"},
		},
	}

	diff := differ.compareTable(codeTable, dbTable)

	if len(diff.ColumnsAdded) != 1 {
		t.Fatalf("Expected 1 column added, got %d", len(diff.ColumnsAdded))
	}
	if diff.ColumnsAdded[0].Name != "phone" {
		t.Errorf("Expected phone column, got %s", diff.ColumnsAdded[0].Name)
	}
}

func TestCompareColumnsDropped(t *testing.T) {
	differ := NewDiffer()

	codeTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "serial"},
			{Name: "email", SQLType: "varchar(255)"},
		},
	}

	dbTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "serial"},
			{Name: "email", SQLType: "varchar(255)"},
			{Name: "phone", SQLType: "varchar(20)"}, // To be dropped
		},
	}

	diff := differ.compareTable(codeTable, dbTable)

	if len(diff.ColumnsDropped) != 1 {
		t.Fatalf("Expected 1 column dropped, got %d", len(diff.ColumnsDropped))
	}
	if diff.ColumnsDropped[0] != "phone" {
		t.Errorf("Expected phone column, got %s", diff.ColumnsDropped[0])
	}
}

func TestCompareColumnTypeChanged(t *testing.T) {
	differ := NewDiffer()

	codeTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "email", SQLType: "varchar(255)"},
		},
	}

	dbTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "email", SQLType: "varchar(100)"},
		},
	}

	diff := differ.compareTable(codeTable, dbTable)

	if len(diff.ColumnsModified) != 1 {
		t.Fatalf("Expected 1 column modified, got %d", len(diff.ColumnsModified))
	}
	if !diff.ColumnsModified[0].TypeChanged {
		t.Errorf("Expected TypeChanged to be true")
	}
	if diff.ColumnsModified[0].ColumnName != "email" {
		t.Errorf("Expected email column, got %s", diff.ColumnsModified[0].ColumnName)
	}
}

func TestCompareColumnNullabilityChanged(t *testing.T) {
	differ := NewDiffer()

	codeTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "name", SQLType: "varchar(100)", Nullable: false},
		},
	}

	dbTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "name", SQLType: "varchar(100)", Nullable: true},
		},
	}

	diff := differ.compareTable(codeTable, dbTable)

	if len(diff.ColumnsModified) != 1 {
		t.Fatalf("Expected 1 column modified, got %d", len(diff.ColumnsModified))
	}
	if !diff.ColumnsModified[0].NullChanged {
		t.Errorf("Expected NullChanged to be true")
	}
}

func TestCompareColumnDefaultChanged(t *testing.T) {
	differ := NewDiffer()

	defaultVal1 := "NOW()"
	defaultVal2 := "CURRENT_TIMESTAMP"

	codeTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "created_at", SQLType: "timestamp", Default: &defaultVal1},
		},
	}

	dbTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "created_at", SQLType: "timestamp", Default: &defaultVal2},
		},
	}

	diff := differ.compareTable(codeTable, dbTable)

	if len(diff.ColumnsModified) != 1 {
		t.Fatalf("Expected 1 column modified, got %d", len(diff.ColumnsModified))
	}
	if !diff.ColumnsModified[0].DefaultChanged {
		t.Errorf("Expected DefaultChanged to be true")
	}
}

func TestNormalizeType(t *testing.T) {
	differ := NewDiffer()

	tests := []struct {
		input    string
		expected string
	}{
		{"int", "integer"},
		{"int4", "integer"},
		{"INT", "integer"},
		{"int2", "smallint"},
		{"int8", "bigint"},
		{"float4", "real"},
		{"float8", "double precision"},
		{"bool", "boolean"},
		{"serial", "integer"},       // serial maps to integer
		{"serial4", "integer"},      // serial4 maps to integer
		{"bigserial", "bigint"},     // bigserial maps to bigint
		{"serial8", "bigint"},       // serial8 maps to bigint
		{"smallserial", "smallint"}, // smallserial maps to smallint
		{"VARCHAR(255)", "varchar(255)"},
		{"  varchar(100)  ", "varchar(100)"},
	}

	for _, test := range tests {
		result := differ.normalizeType(test.input)
		if result != test.expected {
			t.Errorf("normalizeType(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestIsSameType(t *testing.T) {
	differ := NewDiffer()

	tests := []struct {
		type1    string
		type2    string
		expected bool
	}{
		{"int", "integer", true},
		{"int4", "integer", true},
		{"int", "int4", true},
		{"varchar(255)", "VARCHAR(255)", true},
		{"text", "TEXT", true},
		{"int", "bigint", false},
		{"varchar(100)", "varchar(255)", false},
	}

	for _, test := range tests {
		result := differ.isSameType(test.type1, test.type2)
		if result != test.expected {
			t.Errorf("isSameType(%s, %s) = %v, expected %v",
				test.type1, test.type2, result, test.expected)
		}
	}
}

func TestIsSameDefault(t *testing.T) {
	differ := NewDiffer()

	val1 := "NOW()"
	val2 := "NOW()"
	val3 := "CURRENT_TIMESTAMP"

	tests := []struct {
		default1 *string
		default2 *string
		expected bool
	}{
		{nil, nil, true},
		{&val1, &val2, true},
		{&val1, &val3, false},
		{&val1, nil, false},
		{nil, &val1, false},
	}

	for i, test := range tests {
		result := differ.isSameDefault(test.default1, test.default2)
		if result != test.expected {
			t.Errorf("Test %d: isSameDefault = %v, expected %v", i, result, test.expected)
		}
	}
}

func TestNormalizeDefault(t *testing.T) {
	differ := NewDiffer()

	tests := []struct {
		input    string
		expected string
	}{
		{"NOW()", "now()"},               // Lowercase for case-insensitive comparison
		{"(NOW())", "now()"},             // Remove parens and lowercase
		{"  NOW()  ", "now()"},           // Trim whitespace and lowercase
		{"'default'::text", "'default'"}, // Remove type cast
		{"true::boolean", "true"},        // Remove type cast
	}

	for _, test := range tests {
		result := differ.normalizeDefault(test.input)
		if result != test.expected {
			t.Errorf("normalizeDefault(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestComparePrimaryKey(t *testing.T) {
	differ := NewDiffer()

	tests := []struct {
		name     string
		codePK   *schema.PrimaryKeyMetadata
		dbPK     *schema.PrimaryKeyMetadata
		expected bool
	}{
		{
			name:     "both nil",
			codePK:   nil,
			dbPK:     nil,
			expected: false,
		},
		{
			name:     "code has PK, db doesn't",
			codePK:   &schema.PrimaryKeyMetadata{Name: "pk", Columns: []string{"id"}},
			dbPK:     nil,
			expected: true,
		},
		{
			name:     "db has PK, code doesn't",
			codePK:   nil,
			dbPK:     &schema.PrimaryKeyMetadata{Name: "pk", Columns: []string{"id"}},
			expected: true,
		},
		{
			name:     "same PK",
			codePK:   &schema.PrimaryKeyMetadata{Name: "pk", Columns: []string{"id"}},
			dbPK:     &schema.PrimaryKeyMetadata{Name: "pk", Columns: []string{"id"}},
			expected: false,
		},
		{
			name:     "different columns",
			codePK:   &schema.PrimaryKeyMetadata{Name: "pk", Columns: []string{"id", "tenant_id"}},
			dbPK:     &schema.PrimaryKeyMetadata{Name: "pk", Columns: []string{"id"}},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			codeTable := &schema.TableMetadata{Name: "test", PrimaryKey: test.codePK}
			dbTable := &schema.TableMetadata{Name: "test", PrimaryKey: test.dbPK}

			diff := &TableDiff{}
			differ.comparePrimaryKey(codeTable, dbTable, diff)

			hasChange := diff.PrimaryKeyChanged != nil
			if hasChange != test.expected {
				t.Errorf("Expected primary key changed = %v, got %v", test.expected, hasChange)
			}
		})
	}
}

func TestCompareIndexes(t *testing.T) {
	differ := NewDiffer()

	codeTable := &schema.TableMetadata{
		Name: "users",
		Indexes: []schema.IndexMetadata{
			{Name: "idx_email", Columns: []string{"email"}, Unique: true},
			{Name: "idx_name", Columns: []string{"name"}, Unique: false},
		},
	}

	dbTable := &schema.TableMetadata{
		Name: "users",
		Indexes: []schema.IndexMetadata{
			{Name: "idx_email", Columns: []string{"email"}, Unique: true},
			{Name: "idx_old", Columns: []string{"old_column"}, Unique: false},
		},
	}

	diff := &TableDiff{}
	differ.compareIndexes(codeTable, dbTable, diff)

	// Should add idx_name
	if len(diff.IndexesAdded) != 1 {
		t.Errorf("Expected 1 index added, got %d", len(diff.IndexesAdded))
	}
	if len(diff.IndexesAdded) > 0 && diff.IndexesAdded[0].Name != "idx_name" {
		t.Errorf("Expected idx_name to be added, got %s", diff.IndexesAdded[0].Name)
	}

	// Should drop idx_old
	if len(diff.IndexesDropped) != 1 {
		t.Errorf("Expected 1 index dropped, got %d", len(diff.IndexesDropped))
	}
	if len(diff.IndexesDropped) > 0 && diff.IndexesDropped[0] != "idx_old" {
		t.Errorf("Expected idx_old to be dropped, got %s", diff.IndexesDropped[0])
	}
}

func TestCompareForeignKeys(t *testing.T) {
	differ := NewDiffer()

	codeTable := &schema.TableMetadata{
		Name: "posts",
		ForeignKeys: []schema.ForeignKeyMetadata{
			{
				Name:              "fk_user",
				Columns:           []string{"user_id"},
				ReferencedTable:   "users",
				ReferencedColumns: []string{"id"},
			},
			{
				Name:              "fk_category",
				Columns:           []string{"category_id"},
				ReferencedTable:   "categories",
				ReferencedColumns: []string{"id"},
			},
		},
	}

	dbTable := &schema.TableMetadata{
		Name: "posts",
		ForeignKeys: []schema.ForeignKeyMetadata{
			{
				Name:              "fk_user",
				Columns:           []string{"user_id"},
				ReferencedTable:   "users",
				ReferencedColumns: []string{"id"},
			},
		},
	}

	diff := &TableDiff{}
	differ.compareForeignKeys(codeTable, dbTable, diff)

	// Should add fk_category
	if len(diff.ForeignKeysAdded) != 1 {
		t.Errorf("Expected 1 foreign key added, got %d", len(diff.ForeignKeysAdded))
	}
	if len(diff.ForeignKeysAdded) > 0 && diff.ForeignKeysAdded[0].Name != "fk_category" {
		t.Errorf("Expected fk_category to be added, got %s", diff.ForeignKeysAdded[0].Name)
	}
}

func TestHasChanges(t *testing.T) {
	// SchemaDiff with no changes
	diff1 := &SchemaDiff{}
	if diff1.HasChanges() {
		t.Errorf("Expected no changes in empty SchemaDiff")
	}

	// SchemaDiff with table added
	diff2 := &SchemaDiff{
		TablesAdded: []schema.TableMetadata{
			{Name: "users"},
		},
	}
	if !diff2.HasChanges() {
		t.Errorf("Expected changes when table added")
	}

	// TableDiff with no changes
	tableDiff1 := &TableDiff{}
	if tableDiff1.HasChanges() {
		t.Errorf("Expected no changes in empty TableDiff")
	}

	// TableDiff with column added
	tableDiff2 := &TableDiff{
		ColumnsAdded: []schema.ColumnMetadata{
			{Name: "email"},
		},
	}
	if !tableDiff2.HasChanges() {
		t.Errorf("Expected changes when column added")
	}
}
