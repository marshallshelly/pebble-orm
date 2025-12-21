// Package registry provides a central schema registry for table metadata.
package registry

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// Registry is a thread-safe registry for table metadata.
type Registry struct {
	mu     sync.RWMutex
	parser *schema.Parser
	tables map[reflect.Type]*schema.TableMetadata
	names  map[string]*schema.TableMetadata
}

// NewRegistry creates a new Registry instance.
func NewRegistry() *Registry {
	return &Registry{
		parser: schema.NewParser(),
		tables: make(map[reflect.Type]*schema.TableMetadata),
		names:  make(map[string]*schema.TableMetadata),
	}
}

// Register registers a model type and extracts its metadata.
func (r *Registry) Register(model interface{}) error {
	modelType := reflect.TypeOf(model)

	// Dereference pointer
	for modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	if modelType.Kind() != reflect.Struct {
		return fmt.Errorf("model must be a struct, got %s", modelType.Kind())
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already registered
	if _, ok := r.tables[modelType]; ok {
		return nil // Already registered
	}

	// Parse the model
	table, err := r.parser.Parse(modelType)
	if err != nil {
		return fmt.Errorf("failed to parse model %s: %w", modelType.Name(), err)
	}

	// Store in registry
	r.tables[modelType] = table
	r.names[table.Name] = table

	return nil
}

// Get retrieves TableMetadata by Go type.
func (r *Registry) Get(modelType reflect.Type) (*schema.TableMetadata, error) {
	// Dereference pointer
	for modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	r.mu.RLock()
	table, ok := r.tables[modelType]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("model type %s not registered", modelType.Name())
	}

	return table, nil
}

// GetByName retrieves TableMetadata by table name.
func (r *Registry) GetByName(tableName string) (*schema.TableMetadata, error) {
	r.mu.RLock()
	table, ok := r.names[tableName]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("table %s not registered", tableName)
	}

	return table, nil
}

// GetOrRegister retrieves TableMetadata or registers it if not found.
func (r *Registry) GetOrRegister(model interface{}) (*schema.TableMetadata, error) {
	modelType := reflect.TypeOf(model)

	// Dereference pointer
	for modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	// Try to get first
	r.mu.RLock()
	table, ok := r.tables[modelType]
	r.mu.RUnlock()

	if ok {
		return table, nil
	}

	// Register if not found
	if err := r.Register(model); err != nil {
		return nil, err
	}

	// Get again
	return r.Get(modelType)
}

// All returns all registered table metadata.
func (r *Registry) All() []*schema.TableMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tables := make([]*schema.TableMetadata, 0, len(r.tables))
	for _, table := range r.tables {
		tables = append(tables, table)
	}

	return tables
}

// AllNames returns all registered table names.
func (r *Registry) AllNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.names))
	for name := range r.names {
		names = append(names, name)
	}

	return names
}

// Clear removes all registered models.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tables = make(map[reflect.Type]*schema.TableMetadata)
	r.names = make(map[string]*schema.TableMetadata)
}

// Has checks if a model type is registered.
func (r *Registry) Has(modelType reflect.Type) bool {
	// Dereference pointer
	for modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	r.mu.RLock()
	_, ok := r.tables[modelType]
	r.mu.RUnlock()

	return ok
}

// HasTable checks if a table name is registered.
func (r *Registry) HasTable(tableName string) bool {
	r.mu.RLock()
	_, ok := r.names[tableName]
	r.mu.RUnlock()

	return ok
}

// globalRegistry is the default global registry instance.
var globalRegistry = NewRegistry()

// Register registers a model in the global registry.
func Register(model interface{}) error {
	return globalRegistry.Register(model)
}

// Get retrieves TableMetadata from the global registry.
func Get(modelType reflect.Type) (*schema.TableMetadata, error) {
	return globalRegistry.Get(modelType)
}

// GetByName retrieves TableMetadata by name from the global registry.
func GetByName(tableName string) (*schema.TableMetadata, error) {
	return globalRegistry.GetByName(tableName)
}

// GetOrRegister retrieves or registers a model in the global registry.
func GetOrRegister(model interface{}) (*schema.TableMetadata, error) {
	return globalRegistry.GetOrRegister(model)
}

// GetAllTables retrieves all registered tables from the global registry.
func GetAllTables() map[string]*schema.TableMetadata {
	return globalRegistry.GetAllTables()
}

// All returns all registered tables from the global registry.
func All() []*schema.TableMetadata {
	return globalRegistry.All()
}

// Clear clears the global registry.
func Clear() {
	globalRegistry.Clear()
}

// GetAllTables returns all registered tables as a map[tableName]*TableMetadata.
// This is useful for migration generation.
func (r *Registry) GetAllTables() map[string]*schema.TableMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tables := make(map[string]*schema.TableMetadata)
	for name, table := range r.names {
		tables[name] = table
	}

	return tables
}

// AllTables returns all registered tables from the global registry as a map.
func AllTables() map[string]*schema.TableMetadata {
	return globalRegistry.GetAllTables()
}
