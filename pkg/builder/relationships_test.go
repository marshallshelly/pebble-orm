package builder

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// Test models for relationships

type Author struct {
	ID    int      `po:"id,primaryKey,serial"`
	Name  string   `po:"name,varchar(100),notNull"`
	Books []Book   `po:"-,hasMany,foreignKey(author_id),references(id)"`
	Posts []Post   `po:"-,hasMany,foreignKey(author_id),references(id)"`
}

type Book struct {
	ID       int     `po:"id,primaryKey,serial"`
	Title    string  `po:"title,varchar(255),notNull"`
	AuthorID int     `po:"author_id,integer,notNull"`
	Author   *Author `po:"-,belongsTo,foreignKey(author_id),references(id)"`
}

type Post struct {
	ID       int     `po:"id,primaryKey,serial"`
	Title    string  `po:"title,varchar(255),notNull"`
	AuthorID int     `po:"author_id,integer,notNull"`
	Author   *Author `po:"-,belongsTo,foreignKey(author_id),references(id)"`
}

type Profile struct {
	ID     int    `po:"id,primaryKey,serial"`
	Bio    string `po:"bio,text"`
	UserID int    `po:"user_id,integer,notNull,unique"`
	User   *User  `po:"-,belongsTo,foreignKey(user_id),references(id)"`
}

type User struct {
	ID      int      `po:"id,primaryKey,serial"`
	Name    string   `po:"name,varchar(100),notNull"`
	Profile *Profile `po:"-,hasOne,foreignKey(user_id),references(id)"`
	Roles   []Role   `po:"-,manyToMany,joinTable(user_roles),foreignKey(user_id),references(id)"`
}

type Role struct {
	ID    int    `po:"id,primaryKey,serial"`
	Name  string `po:"name,varchar(50),notNull"`
	Users []User `po:"-,manyToMany,joinTable(user_roles),foreignKey(role_id),references(id)"`
}

func TestRelationshipParsing_BelongsTo(t *testing.T) {
	// Register the model
	table, err := registry.GetOrRegister(Book{})
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	// Check that relationship was parsed
	if !table.HasRelationships() {
		t.Fatal("Expected table to have relationships")
	}

	// Get the Author relationship
	rel := table.GetRelationship("Author")
	if rel == nil {
		t.Fatal("Expected Author relationship to exist")
	}

	// Verify relationship type
	if rel.Type != schema.BelongsTo {
		t.Errorf("Expected BelongsTo relationship, got %s", rel.Type)
	}

	// Verify foreign key
	if rel.ForeignKey != "author_id" {
		t.Errorf("Expected foreign key 'author_id', got %s", rel.ForeignKey)
	}

	// Verify references
	if rel.References != "id" {
		t.Errorf("Expected references 'id', got %s", rel.References)
	}

	// Verify target table
	if rel.TargetTable != "author" {
		t.Errorf("Expected target table 'author', got %s", rel.TargetTable)
	}
}

func TestRelationshipParsing_HasOne(t *testing.T) {
	// Register the model
	table, err := registry.GetOrRegister(User{})
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	// Get the Profile relationship
	rel := table.GetRelationship("Profile")
	if rel == nil {
		t.Fatal("Expected Profile relationship to exist")
	}

	// Verify relationship type
	if rel.Type != schema.HasOne {
		t.Errorf("Expected HasOne relationship, got %s", rel.Type)
	}

	// Verify foreign key
	if rel.ForeignKey != "user_id" {
		t.Errorf("Expected foreign key 'user_id', got %s", rel.ForeignKey)
	}

	// Verify references
	if rel.References != "id" {
		t.Errorf("Expected references 'id', got %s", rel.References)
	}
}

func TestRelationshipParsing_HasMany(t *testing.T) {
	// Register the model
	table, err := registry.GetOrRegister(Author{})
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	// Get the Books relationship
	rel := table.GetRelationship("Books")
	if rel == nil {
		t.Fatal("Expected Books relationship to exist")
	}

	// Verify relationship type
	if rel.Type != schema.HasMany {
		t.Errorf("Expected HasMany relationship, got %s", rel.Type)
	}

	// Verify foreign key
	if rel.ForeignKey != "author_id" {
		t.Errorf("Expected foreign key 'author_id', got %s", rel.ForeignKey)
	}

	// Get the Posts relationship
	postsRel := table.GetRelationship("Posts")
	if postsRel == nil {
		t.Fatal("Expected Posts relationship to exist")
	}

	// Verify it's also HasMany
	if postsRel.Type != schema.HasMany {
		t.Errorf("Expected HasMany relationship, got %s", postsRel.Type)
	}
}

func TestRelationshipParsing_ManyToMany(t *testing.T) {
	// Register the model
	table, err := registry.GetOrRegister(User{})
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	// Get the Roles relationship
	rel := table.GetRelationship("Roles")
	if rel == nil {
		t.Fatal("Expected Roles relationship to exist")
	}

	// Verify relationship type
	if rel.Type != schema.ManyToMany {
		t.Errorf("Expected ManyToMany relationship, got %s", rel.Type)
	}

	// Verify junction table
	if rel.JoinTable == nil {
		t.Fatal("Expected junction table to be set")
	}

	if *rel.JoinTable != "user_roles" {
		t.Errorf("Expected junction table 'user_roles', got %s", *rel.JoinTable)
	}

	// Verify references
	if rel.References != "id" {
		t.Errorf("Expected references 'id', got %s", rel.References)
	}
}

func TestRelationshipHelpers_GetRelationshipsByType(t *testing.T) {
	// Register the model
	table, err := registry.GetOrRegister(Author{})
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	// Get all HasMany relationships
	hasManyRels := table.GetRelationshipsByType(schema.HasMany)
	if len(hasManyRels) != 2 {
		t.Errorf("Expected 2 HasMany relationships, got %d", len(hasManyRels))
	}

	// Verify both Books and Posts are included
	foundBooks := false
	foundPosts := false
	for _, rel := range hasManyRels {
		if rel.SourceField == "Books" {
			foundBooks = true
		}
		if rel.SourceField == "Posts" {
			foundPosts = true
		}
	}

	if !foundBooks {
		t.Error("Expected to find Books relationship")
	}
	if !foundPosts {
		t.Error("Expected to find Posts relationship")
	}
}

func TestPreloadAPI(t *testing.T) {
	// Register models
	err := registry.Register(Author{})
	if err != nil {
		t.Fatalf("Failed to register Author: %v", err)
	}

	err = registry.Register(Book{})
	if err != nil {
		t.Fatalf("Failed to register Book: %v", err)
	}

	// Create a query with preloads
	// We can't actually execute without a DB, but we can test the API
	db := &DB{db: nil}
	query := Select[Author](db).
		Preload("Books").
		Preload("Posts")

	// Verify preloads were added
	if len(query.preloads) != 2 {
		t.Errorf("Expected 2 preloads, got %d", len(query.preloads))
	}

	if query.preloads[0] != "Books" {
		t.Errorf("Expected first preload to be 'Books', got %s", query.preloads[0])
	}

	if query.preloads[1] != "Posts" {
		t.Errorf("Expected second preload to be 'Posts', got %s", query.preloads[1])
	}
}

func TestHelperFunctions_ToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"author_id", "AuthorId"},
		{"user_profile_id", "UserProfileId"},
		{"id", "Id"},
		{"name", "Name"},
		{"", ""},
	}

	for _, tt := range tests {
		result := toPascalCase(tt.input)
		if result != tt.expected {
			t.Errorf("toPascalCase(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestHelperFunctions_ToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"AuthorId", "author_id"},
		{"UserProfileId", "user_profile_id"},
		{"Id", "id"},
		{"Name", "name"},
		{"", ""},
	}

	for _, tt := range tests {
		result := toSnakeCase(tt.input)
		if result != tt.expected {
			t.Errorf("toSnakeCase(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestHelperFunctions_IsZeroValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{"nil", nil, true},
		{"zero int", 0, true},
		{"non-zero int", 42, false},
		{"empty string", "", true},
		{"non-empty string", "hello", false},
		{"nil pointer", (*int)(nil), true},
		{"zero pointer", new(int), false},
		{"nil slice", []int(nil), true},
		{"empty slice", []int{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isZeroValue(tt.value)
			if result != tt.expected {
				t.Errorf("isZeroValue(%v) = %v, expected %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestJunctionTableNameGeneration(t *testing.T) {
	// Register User and Role models
	_, err := registry.GetOrRegister(User{})
	if err != nil {
		t.Fatalf("Failed to register User: %v", err)
	}

	roleTable, err := registry.GetOrRegister(Role{})
	if err != nil {
		t.Fatalf("Failed to register Role: %v", err)
	}

	// Get the Users relationship from Role
	rel := roleTable.GetRelationship("Users")
	if rel == nil {
		t.Fatal("Expected Users relationship to exist on Role")
	}

	// Verify junction table name is alphabetically sorted
	if rel.JoinTable == nil {
		t.Fatal("Expected junction table to be set")
	}

	// Should be "user_roles" (not "role_users") due to alphabetical sorting
	if *rel.JoinTable != "user_roles" {
		t.Errorf("Expected junction table 'user_roles', got %s", *rel.JoinTable)
	}
}

func TestRelationshipTargetType(t *testing.T) {
	// Clear registry to start fresh
	registry.Clear()

	// Register the Book model
	table, err := registry.GetOrRegister(Book{})
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	// Get the Author relationship
	rel := table.GetRelationship("Author")
	if rel == nil {
		t.Fatal("Expected Author relationship to exist")
	}

	// Verify TargetType is set correctly
	if rel.TargetType == nil {
		t.Fatal("Expected TargetType to be set")
	}

	// Verify it's the Author type (check type name to avoid reflect.Type comparison issues)
	if rel.TargetType.Name() != "Author" {
		t.Errorf("Expected TargetType name to be 'Author', got %s", rel.TargetType.Name())
	}

	// Verify TargetTable is still set (backward compatibility)
	if rel.TargetTable != "author" {
		t.Errorf("Expected TargetTable 'author', got %s", rel.TargetTable)
	}

	// Clean up
	registry.Clear()
}
