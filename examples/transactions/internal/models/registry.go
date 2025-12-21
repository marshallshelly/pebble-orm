package models

import "github.com/marshallshelly/pebble-orm/pkg/registry"

// RegisterAll registers all models with Pebble ORM
func RegisterAll() error {
	models := []interface{}{
		Account{},
		Transaction{},
	}

	for _, model := range models {
		if err := registry.Register(model); err != nil {
			return err
		}
	}

	return nil
}
