package migration

import (
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func TestGeneratedColumnSQL(t *testing.T) {
	planner := NewPlanner()

	tests := []struct {
		name     string
		column   schema.ColumnMetadata
		expected string
	}{
		{
			name: "STORED generated column",
			column: schema.ColumnMetadata{
				Name:    "full_name",
				SQLType: "varchar(255)",
				Generated: &schema.GeneratedColumn{
					Expression: "first_name || ' ' || last_name",
					Type:       schema.GeneratedStored,
				},
			},
			expected: "full_name varchar(255) GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED",
		},
		{
			name: "VIRTUAL generated column",
			column: schema.ColumnMetadata{
				Name:    "price_with_tax",
				SQLType: "numeric",
				Generated: &schema.GeneratedColumn{
					Expression: "price * 1.2",
					Type:       schema.GeneratedVirtual,
				},
			},
			expected: "price_with_tax numeric GENERATED ALWAYS AS (price * 1.2) VIRTUAL",
		},
		{
			name: "Regular column (not generated)",
			column: schema.ColumnMetadata{
				Name:     "email",
				SQLType:  "varchar(255)",
				Nullable: false,
				Unique:   true,
			},
			expected: "email varchar(255) NOT NULL UNIQUE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := planner.generateColumnDefinition(tt.column)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestGeneratedColumnInCreateTable(t *testing.T) {
	planner := NewPlanner()

	table := &schema.TableMetadata{
		Name: "people",
		Columns: []schema.ColumnMetadata{
			{
				Name:     "id",
				SQLType:  "serial",
				Nullable: false,
			},
			{
				Name:    "first_name",
				SQLType: "varchar(100)",
			},
			{
				Name:    "last_name",
				SQLType: "varchar(100)",
			},
			{
				Name:    "full_name",
				SQLType: "varchar(255)",
				Generated: &schema.GeneratedColumn{
					Expression: "first_name || ' ' || last_name",
					Type:       schema.GeneratedStored,
				},
			},
			{
				Name:    "height_cm",
				SQLType: "numeric",
			},
			{
				Name:    "height_in",
				SQLType: "numeric",
				Generated: &schema.GeneratedColumn{
					Expression: "height_cm / 2.54",
					Type:       schema.GeneratedStored,
				},
			},
		},
		PrimaryKey: &schema.PrimaryKeyMetadata{
			Name:    "people_pkey",
			Columns: []string{"id"},
		},
	}

	sql := planner.generateCreateTable(table)

	// Check that generated columns are present
	if !strings.Contains(sql, "GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED") {
		t.Error("Expected full_name generated column in SQL")
	}

	if !strings.Contains(sql, "GENERATED ALWAYS AS (height_cm / 2.54) STORED") {
		t.Error("Expected height_in generated column in SQL")
	}

	// Check that regular columns are still present
	if !strings.Contains(sql, "first_name varchar(100)") {
		t.Error("Expected first_name column in SQL")
	}

	t.Logf("Generated SQL:\n%s", sql)
}

func TestGeneratedColumnMigration(t *testing.T) {
	planner := NewPlanner()

	diff := &SchemaDiff{
		TablesAdded: []schema.TableMetadata{
			{
				Name: "products",
				Columns: []schema.ColumnMetadata{
					{
						Name:     "id",
						SQLType:  "serial",
						Nullable: false,
					},
					{
						Name:    "price",
						SQLType: "numeric",
					},
					{
						Name:    "tax_rate",
						SQLType: "numeric",
					},
					{
						Name:    "price_with_tax",
						SQLType: "numeric",
						Generated: &schema.GeneratedColumn{
							Expression: "price * (1 + tax_rate)",
							Type:       schema.GeneratedStored,
						},
					},
				},
				PrimaryKey: &schema.PrimaryKeyMetadata{
					Name:    "products_pkey",
					Columns: []string{"id"},
				},
			},
		},
	}

	upSQL, _ := planner.GenerateMigration(diff)

	if !strings.Contains(upSQL, "CREATE TABLE IF NOT EXISTS products") {
		t.Error("Expected CREATE TABLE statement")
	}

	if !strings.Contains(upSQL, "GENERATED ALWAYS AS (price * (1 + tax_rate)) STORED") {
		t.Error("Expected generated column in migration SQL")
	}

	t.Logf("Migration SQL:\n%s", upSQL)
}
