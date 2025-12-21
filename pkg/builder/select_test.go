package builder

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/runtime"
)

type TestUser struct {
	ID    string `po:"id,primaryKey,uuid"`
	Name  string `po:"name,varchar(255),notNull"`
	Email string `po:"email,varchar(320),unique,notNull"`
	Age   int    `po:"age,integer"`
}

func TestSelectQuery_ToSQL(t *testing.T) {
	// Register the test model
	if err := registry.Register(TestUser{}); err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	db := New(nil) // Nil runtime DB for SQL generation tests

	tests := []struct {
		name        string
		setupQuery  func() *SelectQuery[TestUser]
		wantSQL     string
		wantArgLen  int
		wantErr     bool
	}{
		{
			name: "simple select all",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db)
			},
			wantSQL:    "SELECT * FROM test_user",
			wantArgLen: 0,
		},
		{
			name: "select specific columns",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).Columns("id", "name")
			},
			wantSQL:    "SELECT id, name FROM test_user",
			wantArgLen: 0,
		},
		{
			name: "select with WHERE",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).Where(Eq("age", 25))
			},
			wantSQL:    "SELECT * FROM test_user WHERE age = $1",
			wantArgLen: 1,
		},
		{
			name: "select with multiple WHERE",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).
					Where(Eq("age", 25)).
					And(Eq("name", "John"))
			},
			wantSQL:    "SELECT * FROM test_user WHERE age = $1 AND name = $2",
			wantArgLen: 2,
		},
		{
			name: "select with ORDER BY",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).OrderByAsc("name")
			},
			wantSQL:    "SELECT * FROM test_user ORDER BY name ASC",
			wantArgLen: 0,
		},
		{
			name: "select with multiple ORDER BY",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).
					OrderByDesc("age").
					OrderByAsc("name")
			},
			wantSQL:    "SELECT * FROM test_user ORDER BY age DESC, name ASC",
			wantArgLen: 0,
		},
		{
			name: "select with LIMIT",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).Limit(10)
			},
			wantSQL:    "SELECT * FROM test_user LIMIT 10",
			wantArgLen: 0,
		},
		{
			name: "select with LIMIT and OFFSET",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).Limit(10).Offset(20)
			},
			wantSQL:    "SELECT * FROM test_user LIMIT 10 OFFSET 20",
			wantArgLen: 0,
		},
		{
			name: "select with DISTINCT",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).Distinct().Columns("name")
			},
			wantSQL:    "SELECT DISTINCT name FROM test_user",
			wantArgLen: 0,
		},
		{
			name: "select with FOR UPDATE",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).ForUpdate()
			},
			wantSQL:    "SELECT * FROM test_user FOR UPDATE",
			wantArgLen: 0,
		},
		{
			name: "select with GROUP BY",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).
					Columns("age", "COUNT(*) as count").
					GroupBy("age")
			},
			wantSQL:    "SELECT age, COUNT(*) as count FROM test_user GROUP BY age",
			wantArgLen: 0,
		},
		{
			name: "select with GROUP BY and HAVING",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).
					Columns("age", "COUNT(*) as count").
					GroupBy("age").
					Having(Gt("COUNT(*)", 5))
			},
			wantSQL:    "SELECT age, COUNT(*) as count FROM test_user GROUP BY age HAVING COUNT(*) > $1",
			wantArgLen: 1,
		},
		{
			name: "select with INNER JOIN",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).
					InnerJoin("posts", "posts.user_id = test_user.id")
			},
			wantSQL:    "SELECT * FROM test_user INNER JOIN posts ON posts.user_id = test_user.id",
			wantArgLen: 0,
		},
		{
			name: "select with LEFT JOIN",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).
					LeftJoin("posts", "posts.user_id = test_user.id")
			},
			wantSQL:    "SELECT * FROM test_user LEFT JOIN posts ON posts.user_id = test_user.id",
			wantArgLen: 0,
		},
		{
			name: "complex query",
			setupQuery: func() *SelectQuery[TestUser] {
				return Select[TestUser](db).
					Columns("id", "name", "email").
					Where(Gt("age", 18)).
					And(Like("email", "%@example.com")).
					OrderByDesc("age").
					Limit(50).
					Offset(100)
			},
			wantSQL:    "SELECT id, name, email FROM test_user WHERE age > $1 AND email LIKE $2 ORDER BY age DESC LIMIT 50 OFFSET 100",
			wantArgLen: 2,
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

func TestSelectQuery_Methods(t *testing.T) {
	if err := registry.Register(TestUser{}); err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	db := New(nil)

	t.Run("Columns method", func(t *testing.T) {
		query := Select[TestUser](db).Columns("id", "name", "email")
		if len(query.columns) != 3 {
			t.Errorf("Expected 3 columns, got %d", len(query.columns))
		}
	})

	t.Run("Where method chains", func(t *testing.T) {
		query := Select[TestUser](db).Where(Eq("age", 25))
		if len(query.where) != 1 {
			t.Errorf("Expected 1 where condition, got %d", len(query.where))
		}
	})

	t.Run("OrderBy method", func(t *testing.T) {
		query := Select[TestUser](db).OrderBy("name", Asc)
		if len(query.orderBy) != 1 || query.orderBy[0].Direction != Asc {
			t.Error("OrderBy did not set correct direction")
		}
	})

	t.Run("Limit method", func(t *testing.T) {
		query := Select[TestUser](db).Limit(10)
		if query.limit == nil || *query.limit != 10 {
			t.Error("Limit did not set correct value")
		}
	})

	t.Run("Offset method", func(t *testing.T) {
		query := Select[TestUser](db).Offset(20)
		if query.offset == nil || *query.offset != 20 {
			t.Error("Offset did not set correct value")
		}
	})

	t.Run("Distinct method", func(t *testing.T) {
		query := Select[TestUser](db).Distinct()
		if !query.distinct {
			t.Error("Distinct did not set distinct flag")
		}
	})

	t.Run("ForUpdate method", func(t *testing.T) {
		query := Select[TestUser](db).ForUpdate()
		if !query.forUpdate {
			t.Error("ForUpdate did not set forUpdate flag")
		}
	})

	t.Run("GroupBy method", func(t *testing.T) {
		query := Select[TestUser](db).GroupBy("age", "name")
		if len(query.groupBy) != 2 {
			t.Errorf("Expected 2 group by columns, got %d", len(query.groupBy))
		}
	})

	t.Run("Having method", func(t *testing.T) {
		query := Select[TestUser](db).Having(Gt("COUNT(*)", 5))
		if len(query.having) != 1 {
			t.Errorf("Expected 1 having condition, got %d", len(query.having))
		}
	})
}

func TestSelectQuery_Joins(t *testing.T) {
	if err := registry.Register(TestUser{}); err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	db := New(nil)

	t.Run("InnerJoin", func(t *testing.T) {
		query := Select[TestUser](db).InnerJoin("posts", "posts.user_id = test_user.id")
		if len(query.joins) != 1 || query.joins[0].Type != InnerJoin {
			t.Error("InnerJoin did not add join correctly")
		}
	})

	t.Run("LeftJoin", func(t *testing.T) {
		query := Select[TestUser](db).LeftJoin("posts", "posts.user_id = test_user.id")
		if len(query.joins) != 1 || query.joins[0].Type != LeftJoin {
			t.Error("LeftJoin did not add join correctly")
		}
	})

	t.Run("RightJoin", func(t *testing.T) {
		query := Select[TestUser](db).RightJoin("posts", "posts.user_id = test_user.id")
		if len(query.joins) != 1 || query.joins[0].Type != RightJoin {
			t.Error("RightJoin did not add join correctly")
		}
	})

	t.Run("FullJoin", func(t *testing.T) {
		query := Select[TestUser](db).FullJoin("posts", "posts.user_id = test_user.id")
		if len(query.joins) != 1 || query.joins[0].Type != FullJoin {
			t.Error("FullJoin did not add join correctly")
		}
	})

	t.Run("Multiple joins", func(t *testing.T) {
		query := Select[TestUser](db).
			InnerJoin("posts", "posts.user_id = test_user.id").
			LeftJoin("comments", "comments.post_id = posts.id")

		if len(query.joins) != 2 {
			t.Errorf("Expected 2 joins, got %d", len(query.joins))
		}
	})
}

func TestDB_Select(t *testing.T) {
	if err := registry.Register(TestUser{}); err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	t.Run("creates SelectQuery", func(t *testing.T) {
		db := New(&runtime.DB{})
		query := Select[TestUser](db)

		if query == nil {
			t.Fatal("Select() returned nil")
		}

		if query.table == nil {
			t.Error("SelectQuery table is nil")
		}

		if query.table.Name != "test_user" {
			t.Errorf("Expected table name 'test_user', got '%s'", query.table.Name)
		}
	})
}
