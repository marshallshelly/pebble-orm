package builder

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
)

type TestFieldUser struct {
	ID        int64  `po:"id,primaryKey,serial"`
	Name      string `po:"name"`
	Email     string `po:"email,unique"`
	Age       int    `po:"age"`
	CreatedAt string `po:"created_at"`
}

func TestCol(t *testing.T) {
	// Register the test model
	if err := registry.Register(TestFieldUser{}); err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	tests := []struct {
		name        string
		goFieldName string
		expectedCol string
	}{
		{
			name:        "simple field",
			goFieldName: "Email",
			expectedCol: "email",
		},
		{
			name:        "snake_case field",
			goFieldName: "CreatedAt",
			expectedCol: "created_at",
		},
		{
			name:        "Age field",
			goFieldName: "Age",
			expectedCol: "age",
		},
		{
			name:        "ID field",
			goFieldName: "ID",
			expectedCol: "id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Col[TestFieldUser](tt.goFieldName)
			if result != tt.expectedCol {
				t.Errorf("Col[TestFieldUser](%q) = %q, want %q", tt.goFieldName, result, tt.expectedCol)
			}
		})
	}
}

func TestCol_NotRegistered(t *testing.T) {
	type UnregisteredModel struct {
		Field string `po:"field"`
	}

	// Should return the field name as-is when model not registered
	result := Col[UnregisteredModel]("Field")
	if result != "Field" {
		t.Errorf("Expected fallback to field name, got %q", result)
	}
}

func TestCol_InvalidField(t *testing.T) {
	if err := registry.Register(TestFieldUser{}); err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	// Should return the field name as-is when field doesn't exist
	result := Col[TestFieldUser]("NonExistent")
	if result != "NonExistent" {
		t.Errorf("Expected fallback to field name for non-existent field")
	}
}
