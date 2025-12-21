package models

import "github.com/marshallshelly/pebble-orm/pkg/registry"

// RegisterAll registers all models with Pebble ORM
func RegisterAll() error {
	models := []interface{}{
		Product{},
		Category{},
	}

	for _, model := range models {
		if err := registry.Register(model); err != nil {
			return err
		}
	}

	return nil
}

// RegisterV2 registers the updated schema for migration demo
func RegisterV2() error {
	models := []interface{}{
		ProductV2{},
		Category{},
	}

	for _, model := range models {
		if err := registry.Register(model); err != nil {
			return err
		}
	}

	return nil
}
