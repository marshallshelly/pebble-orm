package builder

import (
	"testing"
	"time"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// TestSmartDefaultDetection verifies that zero-valued fields with database defaults
// are automatically omitted from INSERT statements.
func TestSmartDefaultDetection(t *testing.T) {
	// Define table metadata with defaults
	userTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{
				Name:     "id",
				GoField:  "ID",
				SQLType:  "uuid",
				Nullable: false,
				Default:  strPtr("gen_random_uuid()"),
			},
			{
				Name:     "email",
				GoField:  "Email",
				SQLType:  "varchar(255)",
				Nullable: false,
			},
			{
				Name:     "created_at",
				GoField:  "CreatedAt",
				SQLType:  "timestamptz",
				Nullable: false,
				Default:  strPtr("NOW()"),
			},
		},
		PrimaryKey: &schema.PrimaryKeyMetadata{
			Name:    "users_pkey",
			Columns: []string{"id"},
		},
	}

	// Define test user struct
	type User struct {
		ID        string
		Email     string
		CreatedAt time.Time
	}

	tests := []struct {
		name      string
		user      User
		wantCols  []string
		wantCount int
	}{
		{
			name: "zero ID and CreatedAt - both omitted (have defaults)",
			user: User{
				ID:        "", // zero value, has default → omit
				Email:     "test@test.com",
				CreatedAt: time.Time{}, // zero value, has default → omit
			},
			wantCols:  []string{"email"},
			wantCount: 1,
		},
		{
			name: "explicit ID - included even with default",
			user: User{
				ID:        "custom-uuid",
				Email:     "test@test.com",
				CreatedAt: time.Time{}, // still zero, omit
			},
			wantCols:  []string{"id", "email"},
			wantCount: 2,
		},
		{
			name: "all explicit values - all included",
			user: User{
				ID:        "custom-uuid",
				Email:     "test@test.com",
				CreatedAt: time.Now(),
			},
			wantCols:  []string{"id", "email", "created_at"},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, vals, err := structToValues(tt.user, userTable, false)
			if err != nil {
				t.Fatalf("structToValues() error = %v", err)
			}

			if len(cols) != tt.wantCount {
				t.Errorf("got %d columns, want %d\nColumns: %v", len(cols), tt.wantCount, cols)
			}

			// Check that we got the expected columns
			for _, wantCol := range tt.wantCols {
				found := false
				for _, gotCol := range cols {
					if gotCol == wantCol {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected column %s not found in %v", wantCol, cols)
				}
			}

			// Verify values match columns
			if len(vals) != len(cols) {
				t.Errorf("values count mismatch: got %d values for %d columns", len(vals), len(cols))
			}
		})
	}
}

// TestSmartDefaultWithIdentityColumns verifies identity columns are also handled
func TestSmartDefaultWithIdentityColumns(t *testing.T) {
	productTable := &schema.TableMetadata{
		Name: "products",
		Columns: []schema.ColumnMetadata{
			{
				Name:     "id",
				GoField:  "ID",
				SQLType:  "bigint",
				Nullable: false,
				Identity: &schema.IdentityColumn{
					Generation: schema.IdentityAlways,
				},
			},
			{
				Name:     "name",
				GoField:  "Name",
				SQLType:  "text",
				Nullable: false,
			},
		},
		PrimaryKey: &schema.PrimaryKeyMetadata{
			Name:    "products_pkey",
			Columns: []string{"id"},
		},
	}

	type Product struct {
		ID   int64
		Name string
	}

	tests := []struct {
		name      string
		product   Product
		wantCols  []string
		wantCount int
	}{
		{
			name: "zero ID with identity - omitted",
			product: Product{
				ID:   0, // zero value, has identity → omit
				Name: "Widget",
			},
			wantCols:  []string{"name"},
			wantCount: 1,
		},
		{
			name: "explicit ID with identity - included",
			product: Product{
				ID:   42,
				Name: "Widget",
			},
			wantCols:  []string{"id", "name"},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, vals, err := structToValues(tt.product, productTable, false)
			if err != nil {
				t.Fatalf("structToValues() error = %v", err)
			}

			if len(cols) != tt.wantCount {
				t.Errorf("got %d columns, want %d\nColumns: %v", len(cols), tt.wantCount, cols)
			}

			for _, wantCol := range tt.wantCols {
				found := false
				for _, gotCol := range cols {
					if gotCol == wantCol {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected column %s not found in %v", wantCol, cols)
				}
			}

			if len(vals) != len(cols) {
				t.Errorf("values count mismatch: got %d values for %d columns", len(vals), len(cols))
			}
		})
	}
}

// TestSmartDefaultWithPointers verifies that pointers still work (nil is also zero)
func TestSmartDefaultWithPointers(t *testing.T) {
	userTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{
				Name:     "id",
				GoField:  "ID",
				SQLType:  "uuid",
				Nullable: false,
				Default:  strPtr("gen_random_uuid()"),
			},
			{
				Name:     "email",
				GoField:  "Email",
				SQLType:  "varchar(255)",
				Nullable: false,
			},
		},
		PrimaryKey: &schema.PrimaryKeyMetadata{
			Name:    "users_pkey",
			Columns: []string{"id"},
		},
	}

	type User struct {
		ID    *string
		Email string
	}

	tests := []struct {
		name      string
		user      User
		wantCols  []string
		wantCount int
	}{
		{
			name: "nil pointer - omitted (zero value)",
			user: User{
				ID:    nil, // nil is zero, has default → omit
				Email: "test@test.com",
			},
			wantCols:  []string{"email"},
			wantCount: 1,
		},
		{
			name: "empty string pointer - omitted (zero value of pointed type)",
			user: User{
				ID:    strPtr(""), // pointer to empty string is still zero for the string
				Email: "test@test.com",
			},
			wantCols:  []string{"id", "email"}, // Included because pointer is non-nil
			wantCount: 2,
		},
		{
			name: "explicit value pointer - included",
			user: User{
				ID:    strPtr("custom-uuid"),
				Email: "test@test.com",
			},
			wantCols:  []string{"id", "email"},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, vals, err := structToValues(tt.user, userTable, false)
			if err != nil {
				t.Fatalf("structToValues() error = %v", err)
			}

			if len(cols) != tt.wantCount {
				t.Errorf("got %d columns, want %d\nColumns: %v", len(cols), tt.wantCount, cols)
			}

			for _, wantCol := range tt.wantCols {
				found := false
				for _, gotCol := range cols {
					if gotCol == wantCol {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected column %s not found in %v", wantCol, cols)
				}
			}

			if len(vals) != len(cols) {
				t.Errorf("values count mismatch: got %d values for %d columns", len(vals), len(cols))
			}
		})
	}
}

// TestFieldWithoutDefault verifies fields without defaults are always included
func TestFieldWithoutDefault(t *testing.T) {
	userTable := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{
				Name:     "email",
				GoField:  "Email",
				SQLType:  "varchar(255)",
				Nullable: false,
				// No Default
			},
			{
				Name:     "age",
				GoField:  "Age",
				SQLType:  "integer",
				Nullable: true,
				// No Default
			},
		},
	}

	type User struct {
		Email string
		Age   int
	}

	user := User{
		Email: "", // zero value, but NO default → should be included
		Age:   0,  // zero value, but NO default → should be included
	}

	cols, vals, err := structToValues(user, userTable, false)
	if err != nil {
		t.Fatalf("structToValues() error = %v", err)
	}

	// Both columns should be included even though values are zero
	// because they don't have database defaults
	if len(cols) != 2 {
		t.Errorf("got %d columns, want 2\nColumns: %v", len(cols), cols)
	}

	expectedCols := map[string]bool{"email": true, "age": true}
	for _, col := range cols {
		if !expectedCols[col] {
			t.Errorf("unexpected column: %s", col)
		}
	}

	if len(vals) != len(cols) {
		t.Errorf("values count mismatch: got %d values for %d columns", len(vals), len(cols))
	}
}

func strPtr(s string) *string {
	return &s
}
