// Package loader provides utilities to load and register models from Go source files.
package loader

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// ModelRegistrar is an interface for registering table metadata
type ModelRegistrar interface {
	RegisterMetadata(table *schema.TableMetadata) error
}

// LoadModelsFromPath scans a file or directory for Go structs with pebble tags
// and registers them using the provided registrar.
// Supports:
// - Single .go file
// - Directory (scans all .go files recursively)
// - Custom table names from // table_name: comments
func LoadModelsFromPath(path string, registrar ModelRegistrar) (int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to stat path: %w", err)
	}

	var filesToParse []string

	if info.IsDir() {
		// Walk directory and find all .go files
		err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), "_test.go") {
				filesToParse = append(filesToParse, p)
			}
			return nil
		})
		if err != nil {
			return 0, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		// Single file
		if !strings.HasSuffix(path, ".go") {
			return 0, fmt.Errorf("file must have .go extension")
		}
		filesToParse = append(filesToParse, path)
	}

	if len(filesToParse) == 0 {
		return 0, fmt.Errorf("no .go files found in %s", path)
	}

	// Parse all files and collect struct definitions
	modelsRegistered := 0

	for _, file := range filesToParse {
		count, err := loadModelsFromFile(file, registrar)
		if err != nil {
			return modelsRegistered, fmt.Errorf("failed to load models from %s: %w", file, err)
		}
		modelsRegistered += count
	}

	return modelsRegistered, nil
}

// loadModelsFromFile parses a single Go file and registers structs with pebble tags
func loadModelsFromFile(filename string, registrar ModelRegistrar) (int, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return 0, fmt.Errorf("failed to parse file: %w", err)
	}

	modelsRegistered := 0

	// Iterate through declarations
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Check if struct has pebble tags
			if !hasPebbleTags(structType) {
				continue
			}

			structName := typeSpec.Name.Name

			// Extract custom table name from comment if present
			tableName := toSnakeCase(structName) // Default
			if genDecl.Doc != nil {
				for _, comment := range genDecl.Doc.List {
					if customName := schema.ParseTableNameFromComment(comment.Text); customName != "" {
						tableName = customName
						break
					}
				}
			}

			// Build TableMetadata directly from AST
			table := buildTableMetadataFromAST(tableName, structType)

			if err := registrar.RegisterMetadata(table); err != nil {
				return modelsRegistered, fmt.Errorf("failed to register %s: %w", structName, err)
			}

			modelsRegistered++
		}
	}

	return modelsRegistered, nil
}

// buildTableMetadataFromAST creates TableMetadata by parsing the AST struct definition
func buildTableMetadataFromAST(tableName string, structType *ast.StructType) *schema.TableMetadata {
	table := &schema.TableMetadata{
		Name:        tableName,
		GoType:      nil, // No actual Go type available from AST
		Columns:     make([]schema.ColumnMetadata, 0),
		ForeignKeys: make([]schema.ForeignKeyMetadata, 0),
		Indexes:     make([]schema.IndexMetadata, 0),
		Constraints: make([]schema.ConstraintMetadata, 0),
	}

	if structType.Fields == nil {
		return table
	}

	position := 0
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue //  Embedded field
		}

		for _, fieldName := range field.Names {
			// Get the tag
			if field.Tag == nil {
				continue
			}

			tagValue := strings.Trim(field.Tag.Value, "`")
			tag := parseStructTag(tagValue)

			poTag := tag.Get("po")
			if poTag == "" || poTag == "-" {
				continue // Not a database column
			}

			// Parse the po tag
			opts := parseTag(poTag)
			if opts == nil {
				continue
			}

			// Skip relationship fields
			if isRelationshipTag(opts) {
				continue
			}

			// Create column metadata
			column := schema.ColumnMetadata{
				Name:     opts.name,
				GoField:  fieldName.Name,
				GoType:   nil, // Can't determine from AST alone
				Position: position,
			}

			// Determine SQL type from tag
			column.SQLType = getSQLTypeFromOptions(opts)
			if column.SQLType == "" {
				column.SQLType = "text" // Default
			}

			// Set properties from tags
			column.Nullable = !hasOption(opts, "notNull") && !hasOption(opts, "primaryKey")
			column.Unique = hasOption(opts, "unique")
			column.AutoIncrement = hasOption(opts, "serial") || hasOption(opts, "bigserial") || hasOption(opts, "autoIncrement")

			// Handle identity columns
			if hasOption(opts, "identity") || hasOption(opts, "identityAlways") {
				column.Identity = &schema.IdentityColumn{
					Generation: schema.IdentityAlways,
				}
				column.Nullable = false // Identity columns are implicitly NOT NULL
			} else if hasOption(opts, "identityByDefault") {
				column.Identity = &schema.IdentityColumn{
					Generation: schema.IdentityByDefault,
				}
				column.Nullable = false
			}

			if defaultVal := getOptionValue(opts, "default"); defaultVal != "" {
				column.Default = &defaultVal
			}

			// Handle primary key
			if hasOption(opts, "primaryKey") {
				if table.PrimaryKey == nil {
					table.PrimaryKey = &schema.PrimaryKeyMetadata{
						Columns: []string{column.Name},
						Name:    tableName + "_pkey",
					}
				} else {
					table.PrimaryKey.Columns = append(table.PrimaryKey.Columns, column.Name)
				}
			}

			// Note: UNIQUE columns automatically create indexes in PostgreSQL
			// No need to explicitly create separate UNIQUE indexes - they're implicit

			table.Columns = append(table.Columns, column)
			position++
		}
	}

	// Create UNIQUE constraints for columns marked as unique
	// This allows the migration system to detect and manage UNIQUE constraints
	for _, col := range table.Columns {
		if col.Unique {
			constraint := schema.ConstraintMetadata{
				Name:    fmt.Sprintf("%s_%s_key", table.Name, col.Name),
				Type:    schema.UniqueConstraint,
				Columns: []string{col.Name},
			}
			table.Constraints = append(table.Constraints, constraint)
		}
	}

	// Parse foreign keys from column tags (fk:table(column) / onDelete:action)
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 || field.Tag == nil {
			continue
		}
		tagValue := strings.Trim(field.Tag.Value, "`")
		tag := parseStructTag(tagValue)
		poTag := tag.Get("po")
		if poTag == "" || poTag == "-" {
			continue
		}
		opts := parseTag(poTag)
		if opts == nil || isRelationshipTag(opts) {
			continue
		}

		fkStr := getColonValue(opts, "fk")
		if fkStr == "" {
			continue
		}

		// Parse "table(column)" format
		var refTable, refColumn string
		if idx := strings.Index(fkStr, "("); idx > 0 && strings.HasSuffix(fkStr, ")") {
			refTable = fkStr[:idx]
			refColumn = fkStr[idx+1 : len(fkStr)-1]
		} else if strings.Contains(fkStr, ".") {
			parts := strings.SplitN(fkStr, ".", 2)
			if len(parts) == 2 {
				refTable, refColumn = parts[0], parts[1]
			}
		}
		if refTable == "" || refColumn == "" {
			continue
		}

		fk := schema.ForeignKeyMetadata{
			Name:              fmt.Sprintf("fk_%s_%s_%s", tableName, opts.name, refTable),
			Columns:           []string{opts.name},
			ReferencedTable:   refTable,
			ReferencedColumns: []string{refColumn},
			OnDelete:          loaderParseReferenceAction(getColonValue(opts, "onDelete")),
			OnUpdate:          loaderParseReferenceAction(getColonValue(opts, "onUpdate")),
		}
		table.ForeignKeys = append(table.ForeignKeys, fk)
	}

	return table
}

// getColonValue extracts the value from a colon-format option like "fk:table(col)" -> "table(col)".
func getColonValue(opts *tagOptions, key string) string {
	prefix := key + ":"
	for _, opt := range opts.options {
		if strings.HasPrefix(opt, prefix) {
			return opt[len(prefix):]
		}
	}
	return ""
}

// loaderParseReferenceAction converts an onDelete/onUpdate string to a ReferenceAction.
func loaderParseReferenceAction(action string) schema.ReferenceAction {
	switch strings.ToUpper(strings.TrimSpace(action)) {
	case "CASCADE":
		return schema.Cascade
	case "RESTRICT":
		return schema.Restrict
	case "SETNULL", "SET NULL":
		return schema.SetNull
	case "SETDEFAULT", "SET DEFAULT":
		return schema.SetDefault
	default:
		return schema.NoAction
	}
}

// Simple tag option struct
type tagOptions struct {
	name    string
	options []string
}

// parseTag parses a po tag value
func parseTag(tag string) *tagOptions {
	var parts []string
	var buffer strings.Builder
	inParens := 0

	for _, r := range tag {
		switch r {
		case '(':
			inParens++
			buffer.WriteRune(r)
		case ')':
			inParens--
			buffer.WriteRune(r)
		case ',':
			if inParens == 0 {
				parts = append(parts, buffer.String())
				buffer.Reset()
			} else {
				buffer.WriteRune(r)
			}
		default:
			buffer.WriteRune(r)
		}
	}
	if buffer.Len() > 0 {
		parts = append(parts, buffer.String())
	}

	if len(parts) == 0 {
		return nil
	}

	opts := &tagOptions{
		name:    strings.TrimSpace(parts[0]),
		options: make([]string, 0),
	}

	for i := 1; i < len(parts); i++ {
		opts.options = append(opts.options, strings.TrimSpace(parts[i]))
	}

	return opts
}

// parseStructTag parses a complete struct tag
func parseStructTag(tag string) *structTag {
	return &structTag{value: tag}
}

type structTag struct {
	value string
}

func (t *structTag) Get(key string) string {
	// Simple tag parsing - look for `po:"..."`
	prefix := key + `:"`
	start := strings.Index(t.value, prefix)
	if start == -1 {
		return ""
	}

	start += len(prefix)
	end := strings.Index(t.value[start:], `"`)
	if end == -1 {
		return ""
	}

	return t.value[start : start+end]
}

// hasOption checks if an option exists
func hasOption(opts *tagOptions, option string) bool {
	for _, opt := range opts.options {
		if opt == option {
			return true
		}
		if strings.HasPrefix(opt, option+"(") {
			return true
		}
	}
	return false
}

// getOptionValue gets the value of an option like default(value)
func getOptionValue(opts *tagOptions, option string) string {
	for _, opt := range opts.options {
		if strings.HasPrefix(opt, option) {
			// Check for parentheses format: option(value)
			if idx := strings.Index(opt, "("); idx != -1 {
				if strings.HasSuffix(opt, ")") {
					return opt[idx+1 : len(opt)-1]
				}
			}
		}
	}
	return ""
}

// getSQLTypeFromOptions extracts SQL type from tag options
func getSQLTypeFromOptions(opts *tagOptions) string {
	// IMPORTANT: Order matters! More specific types must come before their prefixes.
	// - jsonb before json (jsonb starts with "json")
	// - timestamptz before timestamp (timestamptz starts with "timestamp")
	// - bigserial before serial (bigserial contains "serial")
	pgTypes := []string{
		"uuid", "varchar", "text", "char",
		"smallint", "integer", "bigint", "bigserial", "serial",
		"numeric", "decimal", "real", "double precision",
		"boolean", "bool",
		"timestamptz", "timestamp", "date", "time", "interval",
		"jsonb", "json",
		"bytea",
	}

	for _, opt := range opts.options {
		for _, pgType := range pgTypes {
			if strings.HasPrefix(opt, pgType) {
				// Check for type with size: varchar(255)
				if found := strings.Contains(opt, "("); found {
					return opt
				}
				return pgType
			}
		}
	}

	return ""
}

// isRelationshipTag checks if options indicate a relationship
func isRelationshipTag(opts *tagOptions) bool {
	relationships := []string{"belongsTo", "hasOne", "hasMany", "manyToMany"}
	for _, opt := range opts.options {
		if slices.Contains(relationships, opt) {
			return true
		}
	}
	return false
}

// toSnakeCase converts PascalCase to snake_case
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, ch := range s {
		if i > 0 && ch >= 'A' && ch <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(ch)
	}
	return strings.ToLower(result.String())
}

// hasPebbleTags checks if a struct has any fields with pebble tags
func hasPebbleTags(structType *ast.StructType) bool {
	if structType.Fields == nil {
		return false
	}

	for _, field := range structType.Fields.List {
		if field.Tag != nil {
			tagValue := field.Tag.Value
			// Remove backticks
			tagValue = strings.Trim(tagValue, "`")
			if strings.Contains(tagValue, schema.StructTagKey+":") {
				return true
			}
		}
	}

	return false
}
