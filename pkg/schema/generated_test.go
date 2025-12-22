package schema

import (
	"reflect"
	"testing"
)

func TestGeneratedColumns(t *testing.T) {
	type Person struct {
		FirstName string  `po:"first_name"`
		LastName  string  `po:"last_name"`
		FullName  string  `po:"full_name,generated:first_name || ' ' || last_name,stored"`
		HeightCm  float64 `po:"height_cm"`
		HeightIn  float64 `po:"height_in,generated:height_cm / 2.54,stored"`
	}

	parser := NewParser()
	table, err := parser.Parse(reflect.TypeOf(Person{}))
	if err != nil {
		t.Fatalf("Failed to parse struct: %v", err)
	}

	// Check full_name generated column
	fullNameCol := findColumn(table.Columns, "full_name")
	if fullNameCol == nil {
		t.Fatal("full_name column not found")
	}

	if fullNameCol.Generated == nil {
		t.Fatal("full_name should be a generated column")
	}

	if fullNameCol.Generated.Expression != "first_name || ' ' || last_name" {
		t.Errorf("Expected expression 'first_name || ' ' || last_name', got '%s'",
			fullNameCol.Generated.Expression)
	}

	if fullNameCol.Generated.Type != GeneratedStored {
		t.Errorf("Expected STORED type, got %s", fullNameCol.Generated.Type)
	}

	// Check height_in generated column
	heightInCol := findColumn(table.Columns, "height_in")
	if heightInCol == nil {
		t.Fatal("height_in column not found")
	}

	if heightInCol.Generated == nil {
		t.Fatal("height_in should be a generated column")
	}

	if heightInCol.Generated.Expression != "height_cm / 2.54" {
		t.Errorf("Expected expression 'height_cm / 2.54', got '%s'",
			heightInCol.Generated.Expression)
	}

	// Check non-generated columns
	firstNameCol := findColumn(table.Columns, "first_name")
	if firstNameCol == nil {
		t.Fatal("first_name column not found")
	}

	if firstNameCol.Generated != nil {
		t.Error("first_name should not be a generated column")
	}
}

func TestGeneratedColumnVirtual(t *testing.T) {
	type Product struct {
		Price        float64 `po:"price"`
		TaxRate      float64 `po:"tax_rate"`
		PriceWithTax float64 `po:"price_with_tax,generated:price * tax_rate,virtual"`
	}

	parser := NewParser()
	table, err := parser.Parse(reflect.TypeOf(Product{}))
	if err != nil {
		t.Fatalf("Failed to parse struct: %v", err)
	}

	priceWithTaxCol := findColumn(table.Columns, "price_with_tax")
	if priceWithTaxCol == nil {
		t.Fatal("price_with_tax column not found")
	}

	if priceWithTaxCol.Generated == nil {
		t.Fatal("price_with_tax should be a generated column")
	}

	if priceWithTaxCol.Generated.Type != GeneratedVirtual {
		t.Errorf("Expected VIRTUAL type, got %s", priceWithTaxCol.Generated.Type)
	}
}

func TestGeneratedColumnDefaultsToStored(t *testing.T) {
	type Test struct {
		A int `po:"a"`
		B int `po:"b,generated:a * 2"` // No explicit stored/virtual
	}

	parser := NewParser()
	table, err := parser.Parse(reflect.TypeOf(Test{}))
	if err != nil {
		t.Fatalf("Failed to parse struct: %v", err)
	}

	bCol := findColumn(table.Columns, "b")
	if bCol == nil {
		t.Fatal("b column not found")
	}

	if bCol.Generated == nil {
		t.Fatal("b should be a generated column")
	}

	if bCol.Generated.Type != GeneratedStored {
		t.Errorf("Expected default type to be STORED, got %s", bCol.Generated.Type)
	}
}

// Helper function to find a column by name
func findColumn(columns []ColumnMetadata, name string) *ColumnMetadata {
	for i := range columns {
		if columns[i].Name == name {
			return &columns[i]
		}
	}
	return nil
}
