package builder

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
)

func TestDeleteQuery_ToSQL(t *testing.T) {
	if err := registry.Register(TestUser{}); err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	db := New(nil)

	tests := []struct {
		name       string
		setupQuery func() *DeleteQuery[TestUser]
		wantSQL    string
		wantArgLen int
		wantErr    bool
	}{
		{
			name: "delete with WHERE",
			setupQuery: func() *DeleteQuery[TestUser] {
				return Delete[TestUser](db).Where(Eq("id", "123"))
			},
			wantSQL:    "DELETE FROM test_user WHERE id = $1",
			wantArgLen: 1,
		},
		{
			name: "delete with multiple WHERE",
			setupQuery: func() *DeleteQuery[TestUser] {
				return Delete[TestUser](db).
					Where(Eq("age", 25)).
					And(Eq("name", "John"))
			},
			wantSQL:    "DELETE FROM test_user WHERE age = $1 AND name = $2",
			wantArgLen: 2,
		},
		{
			name: "delete with OR condition",
			setupQuery: func() *DeleteQuery[TestUser] {
				return Delete[TestUser](db).
					Where(Eq("status", "inactive")).
					Or(Eq("status", "deleted"))
			},
			wantSQL:    "DELETE FROM test_user WHERE status = $1 OR status = $2",
			wantArgLen: 2,
		},
		{
			name: "delete with RETURNING",
			setupQuery: func() *DeleteQuery[TestUser] {
				return Delete[TestUser](db).
					Where(Eq("id", "123")).
					Returning("id", "name")
			},
			wantSQL:    "DELETE FROM test_user WHERE id = $1 RETURNING id, name",
			wantArgLen: 1,
		},
		{
			name: "delete with complex WHERE",
			setupQuery: func() *DeleteQuery[TestUser] {
				return Delete[TestUser](db).
					Where(Lt("age", 18)).
					Or(IsNull("email"))
			},
			wantSQL:    "DELETE FROM test_user WHERE age < $1 OR email IS NULL",
			wantArgLen: 1,
		},
		{
			name: "delete with IN condition",
			setupQuery: func() *DeleteQuery[TestUser] {
				return Delete[TestUser](db).
					Where(In("status", "deleted", "suspended", "banned"))
			},
			wantSQL:    "DELETE FROM test_user WHERE status IN ($1, $2, $3)",
			wantArgLen: 3,
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

func TestDeleteQuery_Methods(t *testing.T) {
	if err := registry.Register(TestUser{}); err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	db := New(nil)

	t.Run("Where method", func(t *testing.T) {
		query := Delete[TestUser](db).Where(Eq("id", "123"))

		if len(query.where) != 1 {
			t.Errorf("Expected 1 where condition, got %d", len(query.where))
		}
	})

	t.Run("And method", func(t *testing.T) {
		query := Delete[TestUser](db).
			Where(Eq("age", 25)).
			And(Eq("name", "John"))

		if len(query.where) != 2 {
			t.Errorf("Expected 2 where conditions, got %d", len(query.where))
		}

		if query.where[1].Logic != LogicAnd {
			t.Error("And() did not set correct logic operator")
		}
	})

	t.Run("Or method", func(t *testing.T) {
		query := Delete[TestUser](db).
			Where(Eq("status", "inactive")).
			Or(Eq("status", "deleted"))

		if len(query.where) != 2 {
			t.Errorf("Expected 2 where conditions, got %d", len(query.where))
		}

		if query.where[1].Logic != LogicOr {
			t.Error("Or() did not set correct logic operator")
		}
	})

	t.Run("Returning method", func(t *testing.T) {
		query := Delete[TestUser](db).Returning("id", "name")

		if len(query.returning) != 2 {
			t.Errorf("Expected 2 returning columns, got %d", len(query.returning))
		}
	})

	t.Run("Chaining methods", func(t *testing.T) {
		query := Delete[TestUser](db).
			Where(Eq("status", "deleted")).
			And(Lt("age", 18)).
			Returning("id")

		if len(query.where) != 2 {
			t.Errorf("Expected 2 where conditions, got %d", len(query.where))
		}

		if len(query.returning) != 1 {
			t.Errorf("Expected 1 returning column, got %d", len(query.returning))
		}
	})
}
