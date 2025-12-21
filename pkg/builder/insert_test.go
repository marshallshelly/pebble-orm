package builder

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
)

func TestInsertQuery_ToSQL(t *testing.T) {
	if err := registry.Register(TestUser{}); err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	db := New(nil)

	tests := []struct {
		name       string
		setupQuery func() *InsertQuery[TestUser]
		wantSQL    string
		wantArgLen int
		wantErr    bool
	}{
		{
			name: "single row insert",
			setupQuery: func() *InsertQuery[TestUser] {
				user := TestUser{
					ID:    "123",
					Name:  "John",
					Email: "john@example.com",
					Age:   25,
				}
				return Insert[TestUser](db).Values(user)
			},
			wantSQL:    "INSERT INTO test_user (id, name, email, age) VALUES ($1, $2, $3, $4)",
			wantArgLen: 4,
		},
		{
			name: "insert with RETURNING",
			setupQuery: func() *InsertQuery[TestUser] {
				user := TestUser{
					ID:    "456",
					Name:  "John",
					Email: "john@example.com",
					Age:   25,
				}
				return Insert[TestUser](db).Values(user).Returning("id", "name")
			},
			wantSQL:    "INSERT INTO test_user (id, name, email, age) VALUES ($1, $2, $3, $4) RETURNING id, name",
			wantArgLen: 4,
		},
		{
			name: "insert with ON CONFLICT DO NOTHING",
			setupQuery: func() *InsertQuery[TestUser] {
				user := TestUser{
					ID:    "789",
					Name:  "John",
					Email: "john@example.com",
					Age:   25,
				}
				return Insert[TestUser](db).
					Values(user).
					OnConflictDoNothing("email")
			},
			wantSQL:    "INSERT INTO test_user (id, name, email, age) VALUES ($1, $2, $3, $4) ON CONFLICT (email) DO NOTHING",
			wantArgLen: 4,
		},
		{
			name: "insert with ON CONFLICT DO UPDATE",
			setupQuery: func() *InsertQuery[TestUser] {
				user := TestUser{
					ID:    "abc",
					Name:  "John",
					Email: "john@example.com",
					Age:   25,
				}
				return Insert[TestUser](db).
					Values(user).
					OnConflictDoUpdate(
						[]string{"email"},
						map[string]interface{}{"name": "John Updated"},
					)
			},
			wantSQL:    "INSERT INTO test_user (id, name, email, age) VALUES ($1, $2, $3, $4) ON CONFLICT (email) DO UPDATE SET name = $5",
			wantArgLen: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := tt.setupQuery()
			sql, args, err := query.ToSQL()

			if (err != nil) != tt.wantErr {
				t.Errorf("ToSQL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if sql != tt.wantSQL {
					t.Errorf("ToSQL() sql = %v, want %v", sql, tt.wantSQL)
				}

				if len(args) != tt.wantArgLen {
					t.Errorf("ToSQL() args length = %v, want %v", len(args), tt.wantArgLen)
				}
			}
		})
	}
}

func TestInsertQuery_Methods(t *testing.T) {
	if err := registry.Register(TestUser{}); err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	db := New(nil)

	t.Run("Values method", func(t *testing.T) {
		user := TestUser{Name: "John"}
		query := Insert[TestUser](db).Values(user)

		if len(query.values) != 1 {
			t.Errorf("Expected 1 value, got %d", len(query.values))
		}
	})

	t.Run("Multiple values", func(t *testing.T) {
		user1 := TestUser{Name: "John"}
		user2 := TestUser{Name: "Jane"}
		query := Insert[TestUser](db).Values(user1, user2)

		if len(query.values) != 2 {
			t.Errorf("Expected 2 values, got %d", len(query.values))
		}
	})

	t.Run("Returning method", func(t *testing.T) {
		query := Insert[TestUser](db).Returning("id", "name")

		if len(query.returning) != 2 {
			t.Errorf("Expected 2 returning columns, got %d", len(query.returning))
		}
	})

	t.Run("OnConflictDoNothing method", func(t *testing.T) {
		query := Insert[TestUser](db).OnConflictDoNothing("email")

		if query.onConflict == nil {
			t.Fatal("OnConflict is nil")
		}

		if query.onConflict.Action != DoNothing {
			t.Error("OnConflict action is not DO NOTHING")
		}
	})

	t.Run("OnConflictDoUpdate method", func(t *testing.T) {
		query := Insert[TestUser](db).
			OnConflictDoUpdate(
				[]string{"email"},
				map[string]interface{}{"name": "Updated"},
			)

		if query.onConflict == nil {
			t.Fatal("OnConflict is nil")
		}

		if query.onConflict.Action != DoUpdate {
			t.Error("OnConflict action is not DO UPDATE")
		}

		if len(query.onConflict.Updates) != 1 {
			t.Errorf("Expected 1 update, got %d", len(query.onConflict.Updates))
		}
	})
}
