package builder

import (
	"context"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
)

// User model for transaction tests
type TxUser struct {
	ID   int    `po:"id,primaryKey,serial"`
	Name string `po:"name,varchar(100),notNull"`
	Age  int    `po:"age,integer"`
}

func TestTransactionBasics(t *testing.T) {
	// Register the model
	err := registry.Register(TxUser{})
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	// Test that Tx type exists and can create query builders
	// We can't test actual Begin/Commit/Rollback without a real DB connection
	tx := &Tx{
		tx:  nil,
		ctx: context.Background(),
	}

	// Verify that the transaction can create query builders
	selectQuery := tx.Select(TxUser{})
	if selectQuery == nil {
		t.Error("Expected Select to return a query")
	}

	insertQuery := tx.Insert(TxUser{})
	if insertQuery == nil {
		t.Error("Expected Insert to return a query")
	}

	updateQuery := tx.Update(TxUser{})
	if updateQuery == nil {
		t.Error("Expected Update to return a query")
	}

	deleteQuery := tx.Delete(TxUser{})
	if deleteQuery == nil {
		t.Error("Expected Delete to return a query")
	}
}

func TestTransactionQueries(t *testing.T) {
	// Register the model and get table metadata
	table, err := registry.GetOrRegister(TxUser{})
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	// Create a mock transaction for testing query building
	// We can't actually execute without a real DB, but we can test the API
	tx := &Tx{
		tx:  nil,
		ctx: context.Background(),
	}

	// Test Select query building
	selectQuery := tx.Select(TxUser{}).
		Where(Eq("age", 25)).
		OrderBy("name", Asc).
		Limit(10)

	if selectQuery.table.Name != table.Name {
		t.Errorf("Expected table name %s, got %s", table.Name, selectQuery.table.Name)
	}

	if len(selectQuery.where) != 1 {
		t.Errorf("Expected 1 where condition, got %d", len(selectQuery.where))
	}

	if selectQuery.limit == nil || *selectQuery.limit != 10 {
		t.Error("Expected limit to be 10")
	}

	// Test Insert query building
	insertQuery := tx.Insert(TxUser{}).
		Values(TxUser{Name: "Alice", Age: 30}).
		Returning("id", "name")

	if insertQuery.table.Name != table.Name {
		t.Errorf("Expected table name %s, got %s", table.Name, insertQuery.table.Name)
	}

	if len(insertQuery.values) != 1 {
		t.Errorf("Expected 1 value, got %d", len(insertQuery.values))
	}

	if len(insertQuery.returning) != 2 {
		t.Errorf("Expected 2 returning columns, got %d", len(insertQuery.returning))
	}

	// Test Update query building
	updateQuery := tx.Update(TxUser{}).
		Set("age", 31).
		Where(Eq("id", 1)).
		Returning("id", "age")

	if updateQuery.table.Name != table.Name {
		t.Errorf("Expected table name %s, got %s", table.Name, updateQuery.table.Name)
	}

	if len(updateQuery.sets) != 1 {
		t.Errorf("Expected 1 set, got %d", len(updateQuery.sets))
	}

	if len(updateQuery.where) != 1 {
		t.Errorf("Expected 1 where condition, got %d", len(updateQuery.where))
	}

	// Test Delete query building
	deleteQuery := tx.Delete(TxUser{}).
		Where(Eq("id", 1)).
		Returning("id")

	if deleteQuery.table.Name != table.Name {
		t.Errorf("Expected table name %s, got %s", table.Name, deleteQuery.table.Name)
	}

	if len(deleteQuery.where) != 1 {
		t.Errorf("Expected 1 where condition, got %d", len(deleteQuery.where))
	}

	if len(deleteQuery.returning) != 1 {
		t.Errorf("Expected 1 returning column, got %d", len(deleteQuery.returning))
	}
}

func TestTransactionSelectMethods(t *testing.T) {
	err := registry.Register(TxUser{})
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	tx := &Tx{
		tx:  nil,
		ctx: context.Background(),
	}

	// Test method chaining
	query := tx.Select(TxUser{}).
		Columns("id", "name").
		Where(Gt("age", 18)).
		InnerJoin("posts", "posts.user_id = users.id").
		GroupBy("id", "name").
		Having(Gt("count(*)", 5)).
		OrderBy("name", Asc).
		Limit(10).
		Offset(5)

	if len(query.columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(query.columns))
	}

	if len(query.where) != 1 {
		t.Errorf("Expected 1 where condition, got %d", len(query.where))
	}

	if len(query.joins) != 1 {
		t.Errorf("Expected 1 join, got %d", len(query.joins))
	}

	if len(query.groupBy) != 2 {
		t.Errorf("Expected 2 group by columns, got %d", len(query.groupBy))
	}

	if len(query.having) != 1 {
		t.Errorf("Expected 1 having condition, got %d", len(query.having))
	}

	if len(query.orderBy) != 1 {
		t.Errorf("Expected 1 order by, got %d", len(query.orderBy))
	}

	if query.limit == nil || *query.limit != 10 {
		t.Error("Expected limit to be 10")
	}

	if query.offset == nil || *query.offset != 5 {
		t.Error("Expected offset to be 5")
	}
}

func TestTransactionInsertMethods(t *testing.T) {
	err := registry.Register(TxUser{})
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	tx := &Tx{
		tx:  nil,
		ctx: context.Background(),
	}

	// Test bulk insert
	query := tx.Insert(TxUser{}).
		Values(
			TxUser{Name: "Alice", Age: 30},
			TxUser{Name: "Bob", Age: 25},
		).
		Returning("id", "name")

	if len(query.values) != 2 {
		t.Errorf("Expected 2 values, got %d", len(query.values))
	}

	if len(query.returning) != 2 {
		t.Errorf("Expected 2 returning columns, got %d", len(query.returning))
	}

	// Test ON CONFLICT DO NOTHING
	queryWithConflict := tx.Insert(TxUser{}).
		Values(TxUser{Name: "Charlie", Age: 35}).
		OnConflictDoNothing("email")

	if queryWithConflict.onConflict == nil {
		t.Error("Expected onConflict to be set")
	}

	if queryWithConflict.onConflict.Action != DoNothing {
		t.Errorf("Expected DoNothing action, got %v", queryWithConflict.onConflict.Action)
	}
}

func TestTransactionUpdateMethods(t *testing.T) {
	err := registry.Register(TxUser{})
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	tx := &Tx{
		tx:  nil,
		ctx: context.Background(),
	}

	// Test Set
	query := tx.Update(TxUser{}).
		Set("name", "Updated Name").
		Set("age", 40).
		Where(Eq("id", 1))

	if len(query.sets) != 2 {
		t.Errorf("Expected 2 sets, got %d", len(query.sets))
	}

	if query.sets["name"] != "Updated Name" {
		t.Errorf("Expected name to be 'Updated Name', got %v", query.sets["name"])
	}

	if query.sets["age"] != 40 {
		t.Errorf("Expected age to be 40, got %v", query.sets["age"])
	}

	// Test SetMap
	queryWithMap := tx.Update(TxUser{}).
		SetMap(map[string]interface{}{
			"name": "Alice Updated",
			"age":  31,
		}).
		Where(Eq("id", 2))

	if len(queryWithMap.sets) != 2 {
		t.Errorf("Expected 2 sets, got %d", len(queryWithMap.sets))
	}
}

func TestTransactionDeleteMethods(t *testing.T) {
	err := registry.Register(TxUser{})
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	tx := &Tx{
		tx:  nil,
		ctx: context.Background(),
	}

	// Test Delete with multiple conditions
	query := tx.Delete(TxUser{}).
		Where(Eq("age", 25)).
		Where(Lt("created_at", "2024-01-01")).
		Returning("id", "name")

	if len(query.where) != 2 {
		t.Errorf("Expected 2 where conditions, got %d", len(query.where))
	}

	if len(query.returning) != 2 {
		t.Errorf("Expected 2 returning columns, got %d", len(query.returning))
	}
}
