package schema

import (
	"reflect"
	"testing"
	"time"
)

// TestTimestampParsing tests that timestamp fields are correctly parsed
func TestTimestampParsing(t *testing.T) {
	type TestModel struct {
		ID        int       `po:"id,primaryKey,serial"`
		CreatedAt time.Time `po:"created_at,timestamp,default(NOW()),notNull"`
		UpdatedAt time.Time `po:"updated_at,timestamptz,default(NOW()),notNull"`
	}

	parser := NewParser()
	metadata, err := parser.Parse(reflect.TypeFor[TestModel]())
	if err != nil {
		t.Fatalf("Failed to parse struct: %v", err)
	}

	// Check CreatedAt column
	createdAt := metadata.GetColumnByName("created_at")
	if createdAt == nil {
		t.Fatal("created_at column not found")
	}

	if createdAt.SQLType != "timestamp" {
		t.Errorf("Expected SQLType 'timestamp', got '%s'", createdAt.SQLType)
	}

	if createdAt.Nullable {
		t.Error("Expected created_at to be NOT NULL")
	}

	if createdAt.Default == nil || *createdAt.Default != "NOW()" {
		t.Errorf("Expected default NOW(), got %v", createdAt.Default)
	}

	// Check UpdatedAt column
	updatedAt := metadata.GetColumnByName("updated_at")
	if updatedAt == nil {
		t.Fatal("updated_at column not found")
	}

	if updatedAt.SQLType != "timestamptz" {
		t.Errorf("Expected SQLType 'timestamptz', got '%s'", updatedAt.SQLType)
	}
}

// TestTimeTypeMapping tests that time.Time maps to correct PostgreSQL types
func TestTimeTypeMapping(t *testing.T) {
	type TestModel struct {
		SimpleTime  time.Time  `po:"simple_time"`
		WithTZ      time.Time  `po:"with_tz,timestamptz"`
		WithoutTZ   time.Time  `po:"without_tz,timestamp"`
		NullablePtr *time.Time `po:"nullable_ptr"`
	}

	parser := NewParser()
	metadata, err := parser.Parse(reflect.TypeFor[TestModel]())
	if err != nil {
		t.Fatalf("Failed to parse struct: %v", err)
	}

	tests := []struct {
		columnName   string
		expectedType string
		nullable     bool
	}{
		{"simple_time", "timestamp with time zone", true}, // time.Time is nullable by default
		{"with_tz", "timestamptz", true},                  // unless notNull is specified
		{"without_tz", "timestamp", true},
		{"nullable_ptr", "timestamp with time zone", true},
	}

	for _, tt := range tests {
		col := metadata.GetColumnByName(tt.columnName)
		if col == nil {
			t.Errorf("Column %s not found", tt.columnName)
			continue
		}

		if col.SQLType != tt.expectedType {
			t.Errorf("Column %s: expected type '%s', got '%s'",
				tt.columnName, tt.expectedType, col.SQLType)
		}

		if col.Nullable != tt.nullable {
			t.Errorf("Column %s: expected nullable=%v, got %v",
				tt.columnName, tt.nullable, col.Nullable)
		}
	}
}

// TestTimestampDefaults tests various timestamp default values
func TestTimestampDefaults(t *testing.T) {
	type TestModel struct {
		DefaultNow       time.Time `po:"default_now,timestamptz,default(NOW())"`
		DefaultCurrentTS time.Time `po:"default_current,timestamptz,default(CURRENT_TIMESTAMP)"`
		NoDefault        time.Time `po:"no_default,timestamptz"`
	}

	parser := NewParser()
	metadata, err := parser.Parse(reflect.TypeFor[TestModel]())
	if err != nil {
		t.Fatalf("Failed to parse struct: %v", err)
	}

	// Test NOW() default
	nowCol := metadata.GetColumnByName("default_now")
	if nowCol == nil {
		t.Fatal("default_now column not found")
	}
	if nowCol.Default == nil || *nowCol.Default != "NOW()" {
		t.Errorf("Expected NOW() default, got %v", nowCol.Default)
	}

	// Test CURRENT_TIMESTAMP default
	currentCol := metadata.GetColumnByName("default_current")
	if currentCol == nil {
		t.Fatal("default_current column not found")
	}
	if currentCol.Default == nil || *currentCol.Default != "CURRENT_TIMESTAMP" {
		t.Errorf("Expected CURRENT_TIMESTAMP default, got %v", currentCol.Default)
	}

	// Test no default
	noDefaultCol := metadata.GetColumnByName("no_default")
	if noDefaultCol == nil {
		t.Fatal("no_default column not found")
	}
	if noDefaultCol.Default != nil {
		t.Errorf("Expected nil default, got %v", noDefaultCol.Default)
	}
}
