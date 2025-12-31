package schema

import (
	"reflect"
	"testing"
)

type TestUser struct {
	ID          string  `po:"id,primaryKey,uuid,default(gen_random_uuid())"`
	Name        string  `po:"name,varchar(255),notNull"`
	Email       string  `po:"email,varchar(320),unique,notNull"`
	Age         int     `po:"age,smallint,notNull"`
	BankBalance float32 `po:"bank_balance,numeric(8,2),default(0),notNull"`
}

type TestProduct struct {
	ID    int64   `po:"id,primaryKey,bigserial"`
	Title string  `po:"title,text,notNull"`
	Price float64 `po:"price,numeric(10,2)"`
}

func TestParser_Parse(t *testing.T) {
	parser := NewParser()

	t.Run("basic struct parsing", func(t *testing.T) {
		table, err := parser.Parse(reflect.TypeOf(TestUser{}))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		if table.Name != "test_user" {
			t.Errorf("expected table name 'test_user', got '%s'", table.Name)
		}

		if len(table.Columns) != 5 {
			t.Errorf("expected 5 columns, got %d", len(table.Columns))
		}

		if table.PrimaryKey == nil {
			t.Fatal("expected primary key to be set")
		}

		if len(table.PrimaryKey.Columns) != 1 || table.PrimaryKey.Columns[0] != "id" {
			t.Errorf("expected primary key column 'id', got %v", table.PrimaryKey.Columns)
		}
	})

	t.Run("column metadata", func(t *testing.T) {
		table, err := parser.Parse(reflect.TypeOf(TestUser{}))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		// Check ID column
		idCol := table.GetColumnByName("id")
		if idCol == nil {
			t.Fatal("id column not found")
		}

		if idCol.SQLType != "uuid" {
			t.Errorf("expected uuid type, got '%s'", idCol.SQLType)
		}

		if idCol.Default == nil || *idCol.Default != "gen_random_uuid()" {
			t.Errorf("expected default gen_random_uuid(), got %v", idCol.Default)
		}

		// Check name column
		nameCol := table.GetColumnByName("name")
		if nameCol == nil {
			t.Fatal("name column not found")
		}

		if nameCol.SQLType != "varchar(255)" {
			t.Errorf("expected varchar(255), got '%s'", nameCol.SQLType)
		}

		if nameCol.Nullable {
			t.Error("expected name to be not null")
		}

		// Check email column
		emailCol := table.GetColumnByName("email")
		if emailCol == nil {
			t.Fatal("email column not found")
		}

		if !emailCol.Unique {
			t.Error("expected email to be unique")
		}

		// Check age column
		ageCol := table.GetColumnByName("age")
		if ageCol == nil {
			t.Fatal("age column not found")
		}

		if ageCol.SQLType != "smallint" {
			t.Errorf("expected smallint, got '%s'", ageCol.SQLType)
		}

		// Check bank_balance column
		balanceCol := table.GetColumnByName("bank_balance")
		if balanceCol == nil {
			t.Fatal("bank_balance column not found")
		}

		if balanceCol.SQLType != "numeric(8,2)" {
			t.Errorf("expected numeric(8,2), got '%s'", balanceCol.SQLType)
		}

		if balanceCol.Default == nil || *balanceCol.Default != "0" {
			t.Errorf("expected default 0, got %v", balanceCol.Default)
		}
	})

	t.Run("unique column without separate index", func(t *testing.T) {
		table, err := parser.Parse(reflect.TypeOf(TestUser{}))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		// UNIQUE columns should NOT create separate indexes
		// PostgreSQL creates them implicitly when the table is created
		// So parser should not add them to table.Indexes
		for _, idx := range table.Indexes {
			if len(idx.Columns) == 1 && idx.Columns[0] == "email" && idx.Unique {
				t.Error("Parser should not create separate indexes for UNIQUE columns - PostgreSQL creates them implicitly")
			}
		}

		// Verify the email column is still marked as unique
		emailCol := findColumn(table.Columns, "email")
		if emailCol == nil {
			t.Fatal("email column not found")
		}
		if !emailCol.Unique {
			t.Error("email column should be marked as unique")
		}
	})

	t.Run("cache test", func(t *testing.T) {
		table1, _ := parser.Parse(reflect.TypeOf(TestUser{}))
		table2, _ := parser.Parse(reflect.TypeOf(TestUser{}))

		if table1 != table2 {
			t.Error("expected cached result to be the same instance")
		}
	})
}

func TestParseTag(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		tag      string
		expected *TagOptions
		wantErr  bool
	}{
		{
			name: "simple column name",
			tag:  "id",
			expected: &TagOptions{
				Name:    "id",
				Options: map[string]string{},
			},
		},
		{
			name: "column with single option",
			tag:  "id,primaryKey",
			expected: &TagOptions{
				Name: "id",
				Options: map[string]string{
					"primaryKey": "",
				},
			},
		},
		{
			name: "column with value option",
			tag:  "name,varchar(255)",
			expected: &TagOptions{
				Name: "name",
				Options: map[string]string{
					"varchar": "255",
				},
			},
		},
		{
			name: "complex tag",
			tag:  "email,varchar(320),unique,notNull",
			expected: &TagOptions{
				Name: "email",
				Options: map[string]string{
					"varchar": "320",
					"unique":  "",
					"notNull": "",
				},
			},
		},
		{
			name: "default with value",
			tag:  "balance,numeric(8,2),default(0),notNull",
			expected: &TagOptions{
				Name: "balance",
				Options: map[string]string{
					"numeric": "8,2",
					"default": "0",
					"notNull": "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.parseTag(tt.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result.Name != tt.expected.Name {
					t.Errorf("expected name '%s', got '%s'", tt.expected.Name, result.Name)
				}

				if len(result.Options) != len(tt.expected.Options) {
					t.Errorf("expected %d options, got %d", len(tt.expected.Options), len(result.Options))
				}

				for key, expectedVal := range tt.expected.Options {
					if actualVal, ok := result.Options[key]; !ok || actualVal != expectedVal {
						t.Errorf("option '%s': expected '%s', got '%s'", key, expectedVal, actualVal)
					}
				}
			}
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"User", "user"},
		{"TestUser", "test_user"},
		{"BankAccount", "bank_account"},
		{"HTTPResponse", "h_t_t_p_response"},
		{"ID", "i_d"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("toSnakeCase(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSplitTag(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected []string
	}{
		{
			name:     "simple split",
			tag:      "id,primaryKey,uuid",
			expected: []string{"id", "primaryKey", "uuid"},
		},
		{
			name:     "with parentheses",
			tag:      "name,varchar(255),notNull",
			expected: []string{"name", "varchar(255)", "notNull"},
		},
		{
			name:     "nested parentheses",
			tag:      "balance,numeric(8,2),default(0)",
			expected: []string{"balance", "numeric(8,2)", "default(0)"},
		},
		{
			name:     "complex nested",
			tag:      "data,jsonb,default({})",
			expected: []string{"data", "jsonb", "default({})"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitTag(tt.tag)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d parts, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("part %d: expected '%s', got '%s'", i, expected, result[i])
				}
			}
		})
	}
}

func TestEnumParsing(t *testing.T) {
	parser := NewParser()

	type OrderStatus string

	type TestOrder struct {
		ID     int         `po:"id,primaryKey,serial"`
		Status OrderStatus `po:"status,enum(pending,active,completed),notNull"`
	}

	t.Run("enum column parsing", func(t *testing.T) {
		table, err := parser.Parse(reflect.TypeOf(TestOrder{}))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		// Check status column
		statusCol := table.GetColumnByName("status")
		if statusCol == nil {
			t.Fatal("status column not found")
		}

		// Should have enum type set
		if statusCol.EnumType == "" {
			t.Error("expected EnumType to be set")
		}

		// Enum type name should be derived from Go type name (OrderStatus -> order_status)
		if statusCol.EnumType != "order_status" {
			t.Errorf("expected EnumType 'order_status', got '%s'", statusCol.EnumType)
		}

		// SQL type should be the enum type
		if statusCol.SQLType != "order_status" {
			t.Errorf("expected SQLType 'order_status', got '%s'", statusCol.SQLType)
		}

		// Should have enum values
		if len(statusCol.EnumValues) != 3 {
			t.Fatalf("expected 3 enum values, got %d", len(statusCol.EnumValues))
		}

		expectedValues := []string{"pending", "active", "completed"}
		for i, expected := range expectedValues {
			if statusCol.EnumValues[i] != expected {
				t.Errorf("enum value[%d]: expected '%s', got '%s'", i, expected, statusCol.EnumValues[i])
			}
		}
	})

	t.Run("enum types collected at table level", func(t *testing.T) {
		table, err := parser.Parse(reflect.TypeOf(TestOrder{}))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		// Table should have enum types
		if len(table.EnumTypes) != 1 {
			t.Fatalf("expected 1 enum type at table level, got %d", len(table.EnumTypes))
		}

		enumType := table.EnumTypes[0]
		if enumType.Name != "order_status" {
			t.Errorf("expected enum name 'order_status', got '%s'", enumType.Name)
		}

		if len(enumType.Values) != 3 {
			t.Fatalf("expected 3 enum values, got %d", len(enumType.Values))
		}

		expectedValues := []string{"pending", "active", "completed"}
		for i, expected := range expectedValues {
			if enumType.Values[i] != expected {
				t.Errorf("enum value[%d]: expected '%s', got '%s'", i, expected, enumType.Values[i])
			}
		}
	})

	t.Run("multiple columns with same enum type", func(t *testing.T) {
		type TestShipment struct {
			ID            int         `po:"id,primaryKey,serial"`
			CurrentStatus OrderStatus `po:"current_status,enum(pending,active,completed)"`
			PrevStatus    OrderStatus `po:"prev_status,enum(pending,active,completed)"`
		}

		table, err := parser.Parse(reflect.TypeOf(TestShipment{}))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		// Both columns should use the same enum type
		currentCol := table.GetColumnByName("current_status")
		prevCol := table.GetColumnByName("prev_status")

		if currentCol.EnumType != "order_status" {
			t.Errorf("current_status: expected enum type 'order_status', got '%s'", currentCol.EnumType)
		}

		if prevCol.EnumType != "order_status" {
			t.Errorf("prev_status: expected enum type 'order_status', got '%s'", prevCol.EnumType)
		}

		// Enum type should only be registered once at table level
		if len(table.EnumTypes) != 1 {
			t.Errorf("expected enum type to be deduplicated, got %d types", len(table.EnumTypes))
		}
	})
}
