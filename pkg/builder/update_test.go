package builder

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
)

func TestUpdateQuery_ToSQL(t *testing.T) {
	if err := registry.Register(TestUser{}); err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	db := New(nil)

	tests := []struct {
		name       string
		setupQuery func() *UpdateQuery[TestUser]
		wantSQL    string
		wantArgLen int
		wantErr    bool
	}{
		{
			name: "update single column",
			setupQuery: func() *UpdateQuery[TestUser] {
				return Update[TestUser](db).Set("name", "John Updated")
			},
			wantSQL:    "UPDATE test_user SET name = $1",
			wantArgLen: 1,
		},
		{
			name: "update with WHERE",
			setupQuery: func() *UpdateQuery[TestUser] {
				return Update[TestUser](db).
					Set("name", "John Updated").
					Where(Eq("id", "123"))
			},
			wantSQL:    "UPDATE test_user SET name = $1 WHERE id = $2",
			wantArgLen: 2,
		},
		{
			name: "update multiple columns",
			setupQuery: func() *UpdateQuery[TestUser] {
				return Update[TestUser](db).
					Set("name", "John").
					Set("age", 30).
					Where(Eq("id", "123"))
			},
			wantSQL:    "UPDATE test_user SET", // Partial match due to map iteration order
			wantArgLen: 3,
		},
		{
			name: "update with RETURNING",
			setupQuery: func() *UpdateQuery[TestUser] {
				return Update[TestUser](db).
					Set("name", "John Updated").
					Where(Eq("id", "123")).
					Returning("id", "name", "age")
			},
			wantSQL:    "UPDATE test_user SET name = $1 WHERE id = $2 RETURNING id, name, age",
			wantArgLen: 2,
		},
		{
			name: "update with SetMap",
			setupQuery: func() *UpdateQuery[TestUser] {
				return Update[TestUser](db).
					SetMap(map[string]interface{}{
						"name": "John",
						"age":  30,
					}).
					Where(Eq("id", "123"))
			},
			wantSQL:    "UPDATE test_user SET", // Partial match
			wantArgLen: 3,
		},
		{
			name: "update with complex WHERE",
			setupQuery: func() *UpdateQuery[TestUser] {
				return Update[TestUser](db).
					Set("name", "Updated").
					Where(Gt("age", 18)).
					And(Like("email", "%@example.com"))
			},
			wantSQL:    "UPDATE test_user SET name = $1 WHERE age > $2 AND email LIKE $3",
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
				// For tests with map updates, just check that it starts correctly
				// since map iteration order is not guaranteed
				if len(tt.setupQuery().sets) > 1 {
					if sql[:len(tt.wantSQL)] != tt.wantSQL {
						t.Errorf("ToSQL() sql does not start with %v, got %v", tt.wantSQL, sql)
					}
				} else {
					if sql != tt.wantSQL {
						t.Errorf("ToSQL() sql = %v, want %v", sql, tt.wantSQL)
					}
				}

				if len(args) != tt.wantArgLen {
					t.Errorf("ToSQL() args length = %v, want %v", len(args), tt.wantArgLen)
				}
			}
		})
	}
}

func TestUpdateQuery_Methods(t *testing.T) {
	if err := registry.Register(TestUser{}); err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	db := New(nil)

	t.Run("Set method", func(t *testing.T) {
		query := Update[TestUser](db).Set("name", "John")

		if len(query.sets) != 1 {
			t.Errorf("Expected 1 set, got %d", len(query.sets))
		}

		if query.sets["name"] != "John" {
			t.Error("Set did not store correct value")
		}
	})

	t.Run("SetMap method", func(t *testing.T) {
		query := Update[TestUser](db).SetMap(map[string]interface{}{
			"name": "John",
			"age":  30,
		})

		if len(query.sets) != 2 {
			t.Errorf("Expected 2 sets, got %d", len(query.sets))
		}
	})

	t.Run("Where method", func(t *testing.T) {
		query := Update[TestUser](db).Where(Eq("id", "123"))

		if len(query.where) != 1 {
			t.Errorf("Expected 1 where condition, got %d", len(query.where))
		}
	})

	t.Run("Returning method", func(t *testing.T) {
		query := Update[TestUser](db).Returning("id", "name")

		if len(query.returning) != 2 {
			t.Errorf("Expected 2 returning columns, got %d", len(query.returning))
		}
	})

	t.Run("And method", func(t *testing.T) {
		query := Update[TestUser](db).
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
		query := Update[TestUser](db).
			Where(Eq("age", 25)).
			Or(Eq("age", 30))

		if len(query.where) != 2 {
			t.Errorf("Expected 2 where conditions, got %d", len(query.where))
		}

		if query.where[1].Logic != LogicOr {
			t.Error("Or() did not set correct logic operator")
		}
	})
}
