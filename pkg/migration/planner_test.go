package migration

import (
	"strings"
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

func TestGenerateCreateTable(t *testing.T) {
	planner := NewPlanner()

	table := &schema.TableMetadata{
		Name: "users",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "serial", Nullable: false},
			{Name: "email", SQLType: "varchar(255)", Nullable: false, Unique: true},
			{Name: "name", SQLType: "varchar(100)", Nullable: true},
		},
		PrimaryKey: &schema.PrimaryKeyMetadata{
			Name:    "users_pkey",
			Columns: []string{"id"},
		},
	}

	sql := planner.generateCreateTable(table)

	// Check for table name
	if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS users") {
		t.Errorf("Expected CREATE TABLE IF NOT EXISTS users, got: %s", sql)
	}

	// Check for columns
	if !strings.Contains(sql, "id serial NOT NULL PRIMARY KEY") {
		t.Errorf("Expected inline PRIMARY KEY after id column, got: %s", sql)
	}
	if !strings.Contains(sql, "email varchar(255) NOT NULL UNIQUE") {
		t.Errorf("Expected email column definition, got: %s", sql)
	}
	if !strings.Contains(sql, "name varchar(100)") {
		t.Errorf("Expected name column definition, got: %s", sql)
	}
}

func TestGenerateCreateTableWithIndexes(t *testing.T) {
	planner := NewPlanner()

	table := &schema.TableMetadata{
		Name: "posts",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "serial", Nullable: false},
			{Name: "user_id", SQLType: "integer", Nullable: false},
			{Name: "title", SQLType: "varchar(255)", Nullable: false},
		},
		PrimaryKey: &schema.PrimaryKeyMetadata{
			Name:    "posts_pkey",
			Columns: []string{"id"},
		},
		Indexes: []schema.IndexMetadata{
			{Name: "idx_posts_user_id", Columns: []string{"user_id"}, Unique: false, Type: "btree"},
			{Name: "idx_posts_title", Columns: []string{"title"}, Unique: true, Type: "btree"},
		},
	}

	sql := planner.generateCreateTable(table)

	// Check for indexes
	if !strings.Contains(sql, "CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts (user_id)") {
		t.Errorf("Expected index on user_id, got: %s", sql)
	}
	if !strings.Contains(sql, "CREATE UNIQUE INDEX IF NOT EXISTS idx_posts_title ON posts (title)") {
		t.Errorf("Expected unique index on title, got: %s", sql)
	}
}

func TestGenerateCreateTableWithForeignKeys(t *testing.T) {
	planner := NewPlanner()

	table := &schema.TableMetadata{
		Name: "posts",
		Columns: []schema.ColumnMetadata{
			{Name: "id", SQLType: "serial", Nullable: false},
			{Name: "user_id", SQLType: "integer", Nullable: false},
		},
		PrimaryKey: &schema.PrimaryKeyMetadata{
			Name:    "posts_pkey",
			Columns: []string{"id"},
		},
		ForeignKeys: []schema.ForeignKeyMetadata{
			{
				Name:              "fk_posts_user_id",
				Columns:           []string{"user_id"},
				ReferencedTable:   "users",
				ReferencedColumns: []string{"id"},
				OnDelete:          schema.Cascade,
				OnUpdate:          schema.NoAction,
			},
		},
	}

	sql := planner.generateCreateTable(table)

	// Check for foreign key
	if !strings.Contains(sql, "CONSTRAINT fk_posts_user_id FOREIGN KEY (user_id)") {
		t.Errorf("Expected foreign key constraint, got: %s", sql)
	}
	if !strings.Contains(sql, "REFERENCES users (id)") {
		t.Errorf("Expected foreign key reference, got: %s", sql)
	}
	if !strings.Contains(sql, "ON DELETE CASCADE") {
		t.Errorf("Expected ON DELETE CASCADE, got: %s", sql)
	}
}

func TestGenerateDropTable(t *testing.T) {
	planner := NewPlanner()
	sql := planner.generateDropTable("users")

	expected := `DROP TABLE IF EXISTS "users";`
	if sql != expected {
		t.Errorf("Expected %s, got: %s", expected, sql)
	}
}

func TestGenerateAlterTableAddColumn(t *testing.T) {
	planner := NewPlanner()

	diff := TableDiff{
		TableName: "users",
		ColumnsAdded: []schema.ColumnMetadata{
			{Name: "phone", SQLType: "varchar(20)", Nullable: true},
		},
	}

	upSQL, downSQL := planner.generateAlterTable(diff)

	// Check up migration
	if len(upSQL) != 1 {
		t.Fatalf("Expected 1 up statement, got %d", len(upSQL))
	}
	if !strings.Contains(upSQL[0], "ALTER TABLE users ADD COLUMN phone varchar(20)") {
		t.Errorf("Expected ADD COLUMN statement, got: %s", upSQL[0])
	}

	// Check down migration
	if len(downSQL) != 1 {
		t.Fatalf("Expected 1 down statement, got %d", len(downSQL))
	}
	if !strings.Contains(downSQL[0], "ALTER TABLE users DROP COLUMN IF EXISTS phone") {
		t.Errorf("Expected DROP COLUMN statement, got: %s", downSQL[0])
	}
}

func TestGenerateAlterTableDropColumn(t *testing.T) {
	planner := NewPlanner()

	diff := TableDiff{
		TableName: "users",
		ColumnsDropped: []schema.ColumnMetadata{
			{Name: "phone", SQLType: "varchar(20)", Nullable: true},
		},
	}

	upSQL, downSQL := planner.generateAlterTable(diff)

	// Check up migration
	if len(upSQL) != 1 {
		t.Fatalf("Expected 1 up statement, got %d", len(upSQL))
	}
	if !strings.Contains(upSQL[0], "ALTER TABLE users DROP COLUMN IF EXISTS phone") {
		t.Errorf("Expected DROP COLUMN statement, got: %s", upSQL[0])
	}

	// Check down migration (should ADD COLUMN back with original definition)
	if len(downSQL) != 1 {
		t.Fatalf("Expected 1 down statement, got %d", len(downSQL))
	}
	if !strings.Contains(downSQL[0], "ALTER TABLE users ADD COLUMN phone varchar(20)") {
		t.Errorf("Expected ADD COLUMN statement, got: %s", downSQL[0])
	}
}

func TestGenerateAlterTableModifyColumn(t *testing.T) {
	planner := NewPlanner()

	diff := TableDiff{
		TableName: "users",
		ColumnsModified: []ColumnDiff{
			{
				ColumnName:  "email",
				TypeChanged: true,
				OldColumn:   schema.ColumnMetadata{Name: "email", SQLType: "varchar(100)"},
				NewColumn:   schema.ColumnMetadata{Name: "email", SQLType: "varchar(255)"},
			},
		},
	}

	upSQL, downSQL := planner.generateAlterTable(diff)

	// Check up migration
	if len(upSQL) != 1 {
		t.Fatalf("Expected 1 up statement, got %d", len(upSQL))
	}
	if !strings.Contains(upSQL[0], "ALTER TABLE users ALTER COLUMN email TYPE varchar(255)") {
		t.Errorf("Expected ALTER COLUMN TYPE statement, got: %s", upSQL[0])
	}

	// Check down migration
	if len(downSQL) != 1 {
		t.Fatalf("Expected 1 down statement, got %d", len(downSQL))
	}
	if !strings.Contains(downSQL[0], "ALTER TABLE users ALTER COLUMN email TYPE varchar(100)") {
		t.Errorf("Expected ALTER COLUMN TYPE statement, got: %s", downSQL[0])
	}
}

func TestGenerateAlterTableNullability(t *testing.T) {
	planner := NewPlanner()

	diff := TableDiff{
		TableName: "users",
		ColumnsModified: []ColumnDiff{
			{
				ColumnName:  "name",
				NullChanged: true,
				OldColumn:   schema.ColumnMetadata{Name: "name", Nullable: true},
				NewColumn:   schema.ColumnMetadata{Name: "name", Nullable: false},
			},
		},
	}

	upSQL, downSQL := planner.generateAlterTable(diff)

	// Check up migration (set NOT NULL)
	if len(upSQL) != 1 {
		t.Fatalf("Expected 1 up statement, got %d", len(upSQL))
	}
	if !strings.Contains(upSQL[0], "ALTER TABLE users ALTER COLUMN name SET NOT NULL") {
		t.Errorf("Expected SET NOT NULL statement, got: %s", upSQL[0])
	}

	// Check down migration (drop NOT NULL)
	if len(downSQL) != 1 {
		t.Fatalf("Expected 1 down statement, got %d", len(downSQL))
	}
	if !strings.Contains(downSQL[0], "ALTER TABLE users ALTER COLUMN name DROP NOT NULL") {
		t.Errorf("Expected DROP NOT NULL statement, got: %s", downSQL[0])
	}
}

func TestGenerateAlterTableAddIndex(t *testing.T) {
	planner := NewPlanner()

	diff := TableDiff{
		TableName: "users",
		IndexesAdded: []schema.IndexMetadata{
			{Name: "idx_users_email", Columns: []string{"email"}, Unique: true, Type: "btree"},
		},
	}

	upSQL, downSQL := planner.generateAlterTable(diff)

	// Check up migration
	if len(upSQL) != 1 {
		t.Fatalf("Expected 1 up statement, got %d", len(upSQL))
	}
	if !strings.Contains(upSQL[0], "CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email)") {
		t.Errorf("Expected CREATE INDEX IF NOT EXISTS statement, got: %s", upSQL[0])
	}

	// Check down migration
	if len(downSQL) != 1 {
		t.Fatalf("Expected 1 down statement, got %d", len(downSQL))
	}
	if !strings.Contains(downSQL[0], "DROP INDEX IF EXISTS idx_users_email") {
		t.Errorf("Expected DROP INDEX statement, got: %s", downSQL[0])
	}
}

func TestGenerateMigration(t *testing.T) {
	planner := NewPlanner()

	diff := &SchemaDiff{
		TablesAdded: []schema.TableMetadata{
			{
				Name: "users",
				Columns: []schema.ColumnMetadata{
					{Name: "id", SQLType: "serial", Nullable: false},
					{Name: "email", SQLType: "varchar(255)", Nullable: false},
				},
				PrimaryKey: &schema.PrimaryKeyMetadata{
					Name:    "users_pkey",
					Columns: []string{"id"},
				},
			},
		},
		TablesDropped: []schema.TableMetadata{
			{
				Name: "old_table",
				Columns: []schema.ColumnMetadata{
					{Name: "id", SQLType: "serial", Nullable: false},
				},
			},
		},
		TablesModified: []TableDiff{
			{
				TableName: "posts",
				ColumnsAdded: []schema.ColumnMetadata{
					{Name: "status", SQLType: "varchar(20)", Nullable: false},
				},
			},
		},
	}

	upSQL, downSQL := planner.GenerateMigration(diff)

	// Check up migration has all components
	if !strings.Contains(upSQL, "CREATE TABLE IF NOT EXISTS users") {
		t.Errorf("Expected CREATE TABLE in up migration, got: %s", upSQL)
	}
	if !strings.Contains(upSQL, `DROP TABLE IF EXISTS "old_table"`) {
		t.Errorf("Expected DROP TABLE in up migration, got: %s", upSQL)
	}
	if !strings.Contains(upSQL, "ALTER TABLE posts ADD COLUMN status") {
		t.Errorf("Expected ALTER TABLE in up migration, got: %s", upSQL)
	}

	// Check down migration has all components
	if !strings.Contains(downSQL, `DROP TABLE IF EXISTS "users"`) {
		t.Errorf("Expected DROP TABLE in down migration, got: %s", downSQL)
	}
	if !strings.Contains(downSQL, "CREATE TABLE IF NOT EXISTS old_table") {
		t.Errorf("Expected CREATE TABLE in down migration, got: %s", downSQL)
	}
	if !strings.Contains(downSQL, "ALTER TABLE posts DROP COLUMN IF EXISTS status") {
		t.Errorf("Expected ALTER TABLE in down migration, got: %s", downSQL)
	}
}

func TestGeneratePrimaryKeyChange(t *testing.T) {
	planner := NewPlanner()

	pkChange := &PrimaryKeyChange{
		Old: &schema.PrimaryKeyMetadata{
			Name:    "users_pkey_old",
			Columns: []string{"id"},
		},
		New: &schema.PrimaryKeyMetadata{
			Name:    "users_pkey_new",
			Columns: []string{"id", "tenant_id"},
		},
	}

	upSQL, downSQL := planner.generatePrimaryKeyChange("users", pkChange)

	// Check up migration
	if len(upSQL) != 2 {
		t.Fatalf("Expected 2 up statements, got %d", len(upSQL))
	}
	if !strings.Contains(upSQL[0], "DROP CONSTRAINT IF EXISTS users_pkey_old") {
		t.Errorf("Expected DROP CONSTRAINT, got: %s", upSQL[0])
	}
	if !strings.Contains(upSQL[1], "ADD CONSTRAINT users_pkey_new PRIMARY KEY (id, tenant_id)") {
		t.Errorf("Expected ADD CONSTRAINT, got: %s", upSQL[1])
	}

	// Check down migration (reverse)
	if len(downSQL) != 2 {
		t.Fatalf("Expected 2 down statements, got %d", len(downSQL))
	}
	if !strings.Contains(downSQL[0], "DROP CONSTRAINT IF EXISTS users_pkey_new") {
		t.Errorf("Expected DROP CONSTRAINT, got: %s", downSQL[0])
	}
	if !strings.Contains(downSQL[1], "ADD CONSTRAINT users_pkey_old PRIMARY KEY (id)") {
		t.Errorf("Expected ADD CONSTRAINT, got: %s", downSQL[1])
	}
}
