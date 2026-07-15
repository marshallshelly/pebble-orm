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

			// Table-level index directives from the struct's comments.
			for _, cg := range []*ast.CommentGroup{genDecl.Doc, typeSpec.Comment} {
				if cg == nil {
					continue
				}
				for _, comment := range cg.List {
					if idx := schema.ParseIndexFromComment(comment.Text); idx != nil {
						table.Indexes = append(table.Indexes, *idx)
					}
				}
			}

			if err := registrar.RegisterMetadata(table); err != nil {
				return modelsRegistered, fmt.Errorf("failed to register %s: %w", structName, err)
			}

			modelsRegistered++
		}
	}

	return modelsRegistered, nil
}

// buildTableMetadataFromAST creates TableMetadata by parsing the AST struct
// definition. Go-type facts are extracted from the AST expressions and the tag
// is interpreted through the shared schema helpers, so the AST loader produces
// the same metadata as the reflection parser (index/enum/generated/identity/FK
// included).
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
		if len(field.Names) == 0 || field.Tag == nil {
			continue // Embedded or untagged field
		}

		poTag := parseStructTag(strings.Trim(field.Tag.Value, "`")).Get("po")
		if poTag == "" || poTag == "-" {
			continue
		}
		opts, err := schema.ParseTag(poTag)
		if err != nil {
			continue
		}
		if schema.IsRelationshipTag(opts) {
			continue
		}

		for _, fieldName := range field.Names {
			fm := schema.FieldMeta{
				GoField:      fieldName.Name,
				TypeName:     astTypeName(field.Type),
				Nullable:     astNullable(field.Type),
				InferredType: astInferPGType(field.Type),
				Position:     position,
			}
			column := schema.BuildColumn(opts, fm)

			if opts.Has("primaryKey") {
				if table.PrimaryKey == nil {
					table.PrimaryKey = &schema.PrimaryKeyMetadata{
						Columns: []string{column.Name},
						Name:    tableName + "_pkey",
					}
				} else {
					table.PrimaryKey.Columns = append(table.PrimaryKey.Columns, column.Name)
				}
			}

			table.Columns = append(table.Columns, column)

			if idx, ok := schema.ColumnIndex(opts, tableName); ok {
				table.Indexes = append(table.Indexes, idx)
			}
			if fk, ok := schema.ColumnForeignKey(opts, tableName); ok {
				table.ForeignKeys = append(table.ForeignKeys, fk)
			}
			position++
		}
	}

	table.Constraints = append(table.Constraints, schema.UniqueConstraintsFor(tableName, table.Columns)...)
	table.EnumTypes = schema.CollectEnumTypes(table.Columns)

	return table
}

// astTypeName returns the base named type of an AST type expression, unwrapping
// pointers and slices (e.g. *OrderStatus -> "OrderStatus", []string -> "string",
// time.Time -> "Time"). Used for enum type naming.
func astTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return astTypeName(t.X)
	case *ast.ArrayType:
		return astTypeName(t.Elt)
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	default:
		return ""
	}
}

// astNullable reports whether an AST type expression is nullable (a pointer or
// a database/sql Null* type).
func astNullable(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return true
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok && x.Name == "sql" && strings.HasPrefix(t.Sel.Name, "Null") {
			return true
		}
	}
	return false
}

// astInferPGType infers the PostgreSQL type for an AST type expression, matching
// schema.DefaultTypeMapper for the common Go built-in and standard-library
// types. Returns "" for named or unknown types (the shared BuildColumn then
// falls back to the explicit tag type or text).
func astInferPGType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return astInferPGType(t.X)
	case *ast.ArrayType:
		if id, ok := t.Elt.(*ast.Ident); ok && (id.Name == "byte" || id.Name == "uint8") {
			return "bytea"
		}
		if elem := astInferPGType(t.Elt); elem != "" {
			return elem + "[]"
		}
		return ""
	case *ast.MapType:
		if k, ok := t.Key.(*ast.Ident); ok && k.Name == "string" {
			if _, ok := t.Value.(*ast.InterfaceType); ok {
				return "jsonb"
			}
		}
		return ""
	case *ast.SelectorExpr:
		x, ok := t.X.(*ast.Ident)
		if !ok {
			return ""
		}
		if x.Name == "time" && t.Sel.Name == "Time" {
			return "timestamp with time zone"
		}
		if x.Name == "sql" {
			switch t.Sel.Name {
			case "NullString":
				return "text"
			case "NullInt64":
				return "bigint"
			case "NullInt32":
				return "integer"
			case "NullFloat64":
				return "double precision"
			case "NullBool":
				return "boolean"
			case "NullTime":
				return "timestamp with time zone"
			}
		}
		return ""
	case *ast.Ident:
		switch t.Name {
		case "string":
			return "text"
		case "bool":
			return "boolean"
		case "int8", "int16", "uint8":
			return "smallint"
		case "int32", "int", "uint16":
			return "integer"
		case "int64", "uint32", "uint64":
			return "bigint"
		case "float32":
			return "real"
		case "float64":
			return "double precision"
		}
		return ""
	default:
		return ""
	}
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
