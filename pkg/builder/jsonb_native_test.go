package builder

import (
	"context"
	"testing"
	"time"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/runtime"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Test models for JSONB native scanning

type TaskMetadata struct {
	Priority string   `json:"priority"`
	Tags     []string `json:"tags"`
	DueDate  string   `json:"dueDate,omitempty"`
}

// table_name: document_with_native_jsonb
type DocumentWithNativeJSONB struct {
	ID       int           `po:"id,primaryKey,serial"`
	Title    string        `po:"title,varchar(255),notNull"`
	Metadata *TaskMetadata `po:"metadata,jsonb"` // Use pointer to handle NULL
}

// table_name: document_with_jsonb_slice
type DocumentWithJSONBSlice struct {
	ID    int      `po:"id,primaryKey,serial"`
	Title string   `po:"title,varchar(255),notNull"`
	Items []string `po:"items,jsonb"` // JSON array of strings
}

// table_name: document_with_jsonb_map
type DocumentWithJSONBMap struct {
	ID    int                    `po:"id,primaryKey,serial"`
	Title string                 `po:"title,varchar(255),notNull"`
	Data  map[string]interface{} `po:"data,jsonb"` // Generic map
}

// Helper to setup test DB
func setupJSONBTestDB(t *testing.T) (*postgres.PostgresContainer, *runtime.DB, func()) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	runtimeDB, err := runtime.ConnectWithURL(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	cleanup := func() {
		runtimeDB.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Errorf("failed to terminate container: %v", err)
		}
	}

	return pgContainer, runtimeDB, cleanup
}

func TestJSONBNativeStructScanning(t *testing.T) {
	_, runtimeDB, cleanup := setupJSONBTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Register the model
	reg := registry.NewRegistry()
	reg.Register(DocumentWithNativeJSONB{})

	// Create table
	_, err := runtimeDB.Pool().Exec(ctx, `
		CREATE TABLE document_with_native_jsonb (
			id SERIAL PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			metadata JSONB
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	db := New(runtimeDB)

	// Test 1: Insert and retrieve a document with JSONB struct
	t.Run("insert and select JSONB struct", func(t *testing.T) {
		doc := DocumentWithNativeJSONB{
			Title: "Test Document",
			Metadata: &TaskMetadata{
				Priority: "high",
				Tags:     []string{"urgent", "bug-fix"},
				DueDate:  "2026-01-15",
			},
		}

		// Insert
		inserted, err := Insert[DocumentWithNativeJSONB](db).
			Values(doc).
			Returning("id", "title", "metadata").
			ExecReturning(ctx)
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}

		if len(inserted) != 1 {
			t.Fatalf("expected 1 row, got %d", len(inserted))
		}

		// Verify the inserted data
		if inserted[0].Title != "Test Document" {
			t.Errorf("expected title 'Test Document', got '%s'", inserted[0].Title)
		}

		if inserted[0].Metadata.Priority != "high" {
			t.Errorf("expected priority 'high', got '%s'", inserted[0].Metadata.Priority)
		}

		if len(inserted[0].Metadata.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(inserted[0].Metadata.Tags))
		}

		// Select back
		docs, err := Select[DocumentWithNativeJSONB](db).
			Where(Eq(Col[DocumentWithNativeJSONB]("ID"), inserted[0].ID)).
			All(ctx)
		if err != nil {
			t.Fatalf("failed to select: %v", err)
		}

		if len(docs) != 1 {
			t.Fatalf("expected 1 document, got %d", len(docs))
		}

		retrieved := docs[0]
		if retrieved.Metadata == nil {
			t.Fatal("expected non-nil metadata")
		}
		if retrieved.Metadata.Priority != "high" {
			t.Errorf("expected priority 'high', got '%s'", retrieved.Metadata.Priority)
		}

		if len(retrieved.Metadata.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(retrieved.Metadata.Tags))
		}

		if retrieved.Metadata.Tags[0] != "urgent" {
			t.Errorf("expected first tag 'urgent', got '%s'", retrieved.Metadata.Tags[0])
		}
	})

	// Test 2: NULL JSONB handling
	t.Run("handle NULL JSONB", func(t *testing.T) {
		_, err := runtimeDB.Pool().Exec(ctx,
			"INSERT INTO document_with_native_jsonb (title, metadata) VALUES ($1, NULL)",
			"Document with NULL metadata")
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}

		docs, err := Select[DocumentWithNativeJSONB](db).
			Where(Eq(Col[DocumentWithNativeJSONB]("Title"), "Document with NULL metadata")).
			All(ctx)
		if err != nil {
			t.Fatalf("failed to select: %v", err)
		}

		if len(docs) != 1 {
			t.Fatalf("expected 1 document, got %d", len(docs))
		}

		// NULL should be scanned as nil pointer
		retrieved := docs[0]
		if retrieved.Metadata != nil {
			t.Errorf("expected nil metadata for NULL JSONB, got %+v", retrieved.Metadata)
		}
	})
}

func TestJSONBNativeSliceScanning(t *testing.T) {
	_, runtimeDB, cleanup := setupJSONBTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Register the model
	reg := registry.NewRegistry()
	reg.Register(DocumentWithJSONBSlice{})

	// Create table
	_, err := runtimeDB.Pool().Exec(ctx, `
		CREATE TABLE document_with_jsonb_slice (
			id SERIAL PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			items JSONB
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	db := New(runtimeDB)

	t.Run("insert and select JSONB array", func(t *testing.T) {
		doc := DocumentWithJSONBSlice{
			Title: "Shopping List",
			Items: []string{"milk", "bread", "eggs"},
		}

		inserted, err := Insert[DocumentWithJSONBSlice](db).
			Values(doc).
			Returning("id", "title", "items").
			ExecReturning(ctx)
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}

		if len(inserted) != 1 {
			t.Fatalf("expected 1 row, got %d", len(inserted))
		}

		if len(inserted[0].Items) != 3 {
			t.Errorf("expected 3 items, got %d", len(inserted[0].Items))
		}

		// Select back
		docs, err := Select[DocumentWithJSONBSlice](db).
			Where(Eq(Col[DocumentWithJSONBSlice]("ID"), inserted[0].ID)).
			All(ctx)
		if err != nil {
			t.Fatalf("failed to select: %v", err)
		}

		if len(docs) != 1 {
			t.Fatalf("expected 1 document, got %d", len(docs))
		}

		retrieved := docs[0]
		if len(retrieved.Items) != 3 {
			t.Errorf("expected 3 items, got %d", len(retrieved.Items))
		}

		if retrieved.Items[0] != "milk" {
			t.Errorf("expected first item 'milk', got '%s'", retrieved.Items[0])
		}
	})
}

func TestJSONBNativeMapScanning(t *testing.T) {
	_, runtimeDB, cleanup := setupJSONBTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Register the model
	reg := registry.NewRegistry()
	reg.Register(DocumentWithJSONBMap{})

	// Create table
	_, err := runtimeDB.Pool().Exec(ctx, `
		CREATE TABLE document_with_jsonb_map (
			id SERIAL PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			data JSONB
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	db := New(runtimeDB)

	t.Run("insert and select JSONB map", func(t *testing.T) {
		doc := DocumentWithJSONBMap{
			Title: "Configuration",
			Data: map[string]interface{}{
				"timeout":  30,
				"enabled":  true,
				"hostname": "localhost",
			},
		}

		inserted, err := Insert[DocumentWithJSONBMap](db).
			Values(doc).
			Returning("id", "title", "data").
			ExecReturning(ctx)
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}

		if len(inserted) != 1 {
			t.Fatalf("expected 1 row, got %d", len(inserted))
		}

		if len(inserted[0].Data) != 3 {
			t.Errorf("expected 3 data entries, got %d", len(inserted[0].Data))
		}

		// Select back
		docs, err := Select[DocumentWithJSONBMap](db).
			Where(Eq(Col[DocumentWithJSONBMap]("ID"), inserted[0].ID)).
			All(ctx)
		if err != nil {
			t.Fatalf("failed to select: %v", err)
		}

		if len(docs) != 1 {
			t.Fatalf("expected 1 document, got %d", len(docs))
		}

		retrieved := docs[0]
		if len(retrieved.Data) != 3 {
			t.Errorf("expected 3 data entries, got %d", len(retrieved.Data))
		}

		// Note: JSON numbers are unmarshaled as float64
		if retrieved.Data["timeout"] != float64(30) {
			t.Errorf("expected timeout 30, got %v", retrieved.Data["timeout"])
		}

		if retrieved.Data["enabled"] != true {
			t.Errorf("expected enabled true, got %v", retrieved.Data["enabled"])
		}
	})
}

// =====================================================
// Test for custom type aliases WITHOUT Value()/Scan()
// This is the key test for native JSONB auto-marshaling
// =====================================================

// ActivityTasks is a type alias that does NOT implement driver.Valuer or sql.Scanner
// Pebble ORM should handle marshaling/unmarshaling automatically
type ActivityTasks map[string]interface{}

// table_name: client_activity_groups_test
type ClientActivityGroupTest struct {
	ID        int            `po:"id,primaryKey,serial"`
	ClientID  string         `po:"client_id,varchar(100),notNull"`
	GroupName string         `po:"group_name,varchar(255),notNull"`
	Tasks     *ActivityTasks `po:"tasks,jsonb"` // Native JSONB - no Value()/Scan() needed!
	IsActive  bool           `po:"is_active,boolean,default(true)"`
}

func TestJSONBNativeCustomTypeAlias(t *testing.T) {
	_, runtimeDB, cleanup := setupJSONBTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Register the model
	reg := registry.NewRegistry()
	reg.Register(ClientActivityGroupTest{})

	// Create table
	_, err := runtimeDB.Pool().Exec(ctx, `
		CREATE TABLE client_activity_groups_test (
			id SERIAL PRIMARY KEY,
			client_id VARCHAR(100) NOT NULL,
			group_name VARCHAR(255) NOT NULL,
			tasks JSONB,
			is_active BOOLEAN DEFAULT true
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	db := New(runtimeDB)

	t.Run("insert and select custom type alias without Value/Scan", func(t *testing.T) {
		// This is the exact pattern used in wecare-server's create-client.go
		tasks := ActivityTasks{
			"Assist with washing":  true,
			"Assist with dressing": true,
			"Assist with grooming": true,
			"Apply creams/lotions": map[string]interface{}{
				"required": true,
				"notes":    "Use hypoallergenic products",
			},
		}

		group := ClientActivityGroupTest{
			ClientID:  "test-client-123",
			GroupName: "Personal Care",
			Tasks:     &tasks, // Pointer to map - Pebble should auto-marshal this!
			IsActive:  true,
		}

		// INSERT - this should auto-marshal the ActivityTasks to JSON
		inserted, err := Insert[ClientActivityGroupTest](db).
			Values(group).
			Returning("id", "group_name", "tasks").
			ExecReturning(ctx)
		if err != nil {
			t.Fatalf("failed to insert (auto-marshal should work): %v", err)
		}

		if len(inserted) != 1 {
			t.Fatalf("expected 1 row, got %d", len(inserted))
		}

		t.Logf("✅ INSERT worked! ID: %d, GroupName: %s", inserted[0].ID, inserted[0].GroupName)
		t.Logf("   Tasks: %v", *inserted[0].Tasks)

		// Verify returned tasks
		if inserted[0].Tasks == nil {
			t.Fatal("expected non-nil tasks in returned data")
		}
		if len(*inserted[0].Tasks) != 4 {
			t.Errorf("expected 4 tasks, got %d", len(*inserted[0].Tasks))
		}

		// SELECT - this should auto-unmarshal the JSON back to ActivityTasks
		groups, err := Select[ClientActivityGroupTest](db).
			Where(Eq(Col[ClientActivityGroupTest]("GroupName"), "Personal Care")).
			All(ctx)
		if err != nil {
			t.Fatalf("failed to select (auto-unmarshal should work): %v", err)
		}

		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}

		retrieved := groups[0]
		t.Logf("✅ SELECT worked! Retrieved: %s", retrieved.GroupName)
		t.Logf("   Tasks: %v", *retrieved.Tasks)

		if retrieved.Tasks == nil {
			t.Fatal("expected non-nil tasks after SELECT")
		}

		if len(*retrieved.Tasks) != 4 {
			t.Errorf("expected 4 tasks, got %d", len(*retrieved.Tasks))
		}

		// Verify specific task values
		if (*retrieved.Tasks)["Assist with washing"] != true {
			t.Errorf("expected 'Assist with washing' to be true")
		}

		// Verify nested object was preserved
		creamsTask, ok := (*retrieved.Tasks)["Apply creams/lotions"].(map[string]interface{})
		if !ok {
			t.Errorf("expected 'Apply creams/lotions' to be a map")
		} else {
			if creamsTask["notes"] != "Use hypoallergenic products" {
				t.Errorf("expected notes to be preserved, got: %v", creamsTask["notes"])
			}
		}
	})

	t.Run("insert multiple activity groups", func(t *testing.T) {
		// Test inserting multiple groups (like wecare-server does)
		groups := []struct {
			name  string
			tasks map[string]interface{}
		}{
			{
				name: "Medication Management",
				tasks: map[string]interface{}{
					"Prompt client to take medication":      true,
					"Assist with medication administration": true,
				},
			},
			{
				name: "Mobility Assistance",
				tasks: map[string]interface{}{
					"Assist with getting out of bed": true,
					"Assist with transfers":          true,
				},
			},
		}

		for _, g := range groups {
			tasks := ActivityTasks(g.tasks)
			group := ClientActivityGroupTest{
				ClientID:  "test-client-456",
				GroupName: g.name,
				Tasks:     &tasks,
				IsActive:  true,
			}

			_, err := Insert[ClientActivityGroupTest](db).
				Values(group).
				Exec(ctx)
			if err != nil {
				t.Fatalf("failed to insert %s: %v", g.name, err)
			}
			t.Logf("✅ Inserted: %s", g.name)
		}

		// Verify all were inserted
		all, err := Select[ClientActivityGroupTest](db).
			Where(Eq(Col[ClientActivityGroupTest]("ClientID"), "test-client-456")).
			All(ctx)
		if err != nil {
			t.Fatalf("failed to select all: %v", err)
		}

		if len(all) != 2 {
			t.Errorf("expected 2 groups, got %d", len(all))
		}

		for _, group := range all {
			t.Logf("   - %s: %d tasks", group.GroupName, len(*group.Tasks))
		}
	})
}
