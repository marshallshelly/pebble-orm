package registry

import (
	"reflect"
	"testing"
)

type User struct {
	ID    string `po:"id,primaryKey,uuid"`
	Name  string `po:"name,varchar(255),notNull"`
	Email string `po:"email,varchar(320),unique,notNull"`
}

type Product struct {
	ID    int64  `po:"id,primaryKey,bigserial"`
	Title string `po:"title,text,notNull"`
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	t.Run("register new model", func(t *testing.T) {
		err := registry.Register(User{})
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		if !registry.Has(reflect.TypeOf(User{})) {
			t.Error("expected model to be registered")
		}
	})

	t.Run("register duplicate model", func(t *testing.T) {
		err := registry.Register(User{})
		if err != nil {
			t.Fatalf("First register failed: %v", err)
		}

		// Should not error on duplicate registration
		err = registry.Register(User{})
		if err != nil {
			t.Errorf("Duplicate register failed: %v", err)
		}
	})

	t.Run("register pointer model", func(t *testing.T) {
		err := registry.Register(&User{})
		if err != nil {
			t.Fatalf("Register with pointer failed: %v", err)
		}

		// Should dereference and register the underlying type
		if !registry.Has(reflect.TypeOf(User{})) {
			t.Error("expected model to be registered")
		}
	})

	t.Run("register invalid type", func(t *testing.T) {
		err := registry.Register("not a struct")
		if err == nil {
			t.Error("expected error for non-struct type")
		}
	})
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()

	t.Run("get registered model", func(t *testing.T) {
		if err := registry.Register(User{}); err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		table, err := registry.Get(reflect.TypeOf(User{}))
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if table.Name != "user" {
			t.Errorf("expected table name 'user', got '%s'", table.Name)
		}
	})

	t.Run("get unregistered model", func(t *testing.T) {
		_, err := registry.Get(reflect.TypeOf(Product{}))
		if err == nil {
			t.Error("expected error for unregistered model")
		}
	})

	t.Run("get with pointer type", func(t *testing.T) {
		if err := registry.Register(User{}); err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		table, err := registry.Get(reflect.TypeOf(&User{}))
		if err != nil {
			t.Fatalf("Get with pointer failed: %v", err)
		}

		if table.Name != "user" {
			t.Errorf("expected table name 'user', got '%s'", table.Name)
		}
	})
}

func TestRegistry_GetByName(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(User{}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	t.Run("get by existing name", func(t *testing.T) {
		table, err := registry.GetByName("user")
		if err != nil {
			t.Fatalf("GetByName failed: %v", err)
		}

		if table.Name != "user" {
			t.Errorf("expected table name 'user', got '%s'", table.Name)
		}
	})

	t.Run("get by non-existing name", func(t *testing.T) {
		_, err := registry.GetByName("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent table")
		}
	})
}

func TestRegistry_GetOrRegister(t *testing.T) {
	registry := NewRegistry()

	t.Run("get or register unregistered model", func(t *testing.T) {
		table, err := registry.GetOrRegister(User{})
		if err != nil {
			t.Fatalf("GetOrRegister failed: %v", err)
		}

		if table.Name != "user" {
			t.Errorf("expected table name 'user', got '%s'", table.Name)
		}

		if !registry.Has(reflect.TypeOf(User{})) {
			t.Error("expected model to be registered")
		}
	})

	t.Run("get or register already registered model", func(t *testing.T) {
		if err := registry.Register(Product{}); err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		table1, _ := registry.GetOrRegister(Product{})
		table2, _ := registry.GetOrRegister(Product{})

		// Should return the same instance
		if table1 != table2 {
			t.Error("expected same table instance")
		}
	})
}

func TestRegistry_All(t *testing.T) {
	registry := NewRegistry()

	t.Run("empty registry", func(t *testing.T) {
		tables := registry.All()
		if len(tables) != 0 {
			t.Errorf("expected 0 tables, got %d", len(tables))
		}
	})

	t.Run("with registered models", func(t *testing.T) {
		if err := registry.Register(User{}); err != nil {
			t.Fatalf("Register User failed: %v", err)
		}
		if err := registry.Register(Product{}); err != nil {
			t.Fatalf("Register Product failed: %v", err)
		}

		tables := registry.All()
		if len(tables) != 2 {
			t.Errorf("expected 2 tables, got %d", len(tables))
		}
	})
}

func TestRegistry_AllNames(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(User{}); err != nil {
		t.Fatalf("Register User failed: %v", err)
	}
	if err := registry.Register(Product{}); err != nil {
		t.Fatalf("Register Product failed: %v", err)
	}

	names := registry.AllNames()
	if len(names) != 2 {
		t.Errorf("expected 2 table names, got %d", len(names))
	}

	// Check that both names are present
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	if !nameMap["user"] {
		t.Error("expected 'user' table name")
	}

	if !nameMap["product"] {
		t.Error("expected 'product' table name")
	}
}

func TestRegistry_Clear(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(User{}); err != nil {
		t.Fatalf("Register User failed: %v", err)
	}
	if err := registry.Register(Product{}); err != nil {
		t.Fatalf("Register Product failed: %v", err)
	}

	if len(registry.All()) != 2 {
		t.Fatal("expected 2 registered models")
	}

	registry.Clear()

	if len(registry.All()) != 0 {
		t.Error("expected 0 models after clear")
	}

	if registry.Has(reflect.TypeOf(User{})) {
		t.Error("expected user model to be cleared")
	}
}

func TestRegistry_Has(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(User{}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !registry.Has(reflect.TypeOf(User{})) {
		t.Error("expected Has to return true for registered model")
	}

	if registry.Has(reflect.TypeOf(Product{})) {
		t.Error("expected Has to return false for unregistered model")
	}
}

func TestRegistry_HasTable(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(User{}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !registry.HasTable("user") {
		t.Error("expected HasTable to return true for registered table")
	}

	if registry.HasTable("product") {
		t.Error("expected HasTable to return false for unregistered table")
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Clear global registry first
	Clear()

	t.Run("global register", func(t *testing.T) {
		err := Register(User{})
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		table, err := Get(reflect.TypeOf(User{}))
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if table.Name != "user" {
			t.Errorf("expected table name 'user', got '%s'", table.Name)
		}
	})

	t.Run("global get by name", func(t *testing.T) {
		table, err := GetByName("user")
		if err != nil {
			t.Fatalf("GetByName failed: %v", err)
		}

		if table.Name != "user" {
			t.Errorf("expected table name 'user', got '%s'", table.Name)
		}
	})

	t.Run("global all", func(t *testing.T) {
		tables := All()
		if len(tables) == 0 {
			t.Error("expected at least one table")
		}
	})

	// Clean up
	Clear()
}
