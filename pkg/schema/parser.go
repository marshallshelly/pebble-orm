package schema

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

const (
	// StructTagKey is the key used in struct tags (e.g., `po:"..."`).
	StructTagKey = "po"
)

// Parser parses struct definitions to extract table metadata.
type Parser struct {
	typeMapper *TypeMapper
	cache      map[reflect.Type]*TableMetadata
}

// NewParser creates a new Parser instance.
func NewParser() *Parser {
	return &Parser{
		typeMapper: DefaultTypeMapper,
		cache:      make(map[reflect.Type]*TableMetadata),
	}
}

// Global table name registry for compile-time table names.
// Can be populated by generated code from `pebble generate metadata`.
var customTableNames = make(map[string]string) // Struct name → table name

// RegisterTableName registers a custom table name for a struct type.
// This is called by generated code to provide table names from comments.
//
// Example generated code:
//
//	func init() {
//	    schema.RegisterTableName("Tenant", "tenants")
//	    schema.RegisterTableName("TenantUser", "tenant_users")
//	}
func RegisterTableName(structName, tableName string) {
	customTableNames[structName] = tableName
}

// Parse extracts TableMetadata from a Go struct type.
func (p *Parser) Parse(modelType reflect.Type) (*TableMetadata, error) {
	// Dereference pointer types
	for modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}
	if modelType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("model must be a struct, got %s", modelType.Kind())
	}
	// Check cache
	if cached, ok := p.cache[modelType]; ok {
		return cached, nil
	}
	table := &TableMetadata{
		Name:        p.extractTableName(modelType),
		GoType:      modelType,
		Columns:     make([]ColumnMetadata, 0),
		ForeignKeys: make([]ForeignKeyMetadata, 0),
		Indexes:     make([]IndexMetadata, 0),
		Constraints: make([]ConstraintMetadata, 0),
	}
	// Parse fields
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}
		// Get tag value
		tagValue := field.Tag.Get(StructTagKey)
		if tagValue == "" {
			// Skip fields without po tag
			continue
		}
		// Parse tag
		tagOpts, err := p.parseTag(tagValue)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tag for field %s: %w", field.Name, err)
		}
		// Check if this is a relationship field
		if p.isRelationshipTag(tagOpts) {
			// Handle relationships separately (will be implemented in relationships.go)
			continue
		}
		// Create column metadata
		column := p.createColumnMetadata(field, tagOpts, i)
		// Handle primary key
		if tagOpts.Has("primaryKey") {
			if table.PrimaryKey == nil {
				table.PrimaryKey = &PrimaryKeyMetadata{
					Columns: []string{column.Name},
					Name:    table.Name + "_pkey",
				}
			} else {
				table.PrimaryKey.Columns = append(table.PrimaryKey.Columns, column.Name)
			}
		}
		// Note: UNIQUE columns automatically create indexes in PostgreSQL
		// No need to explicitly create separate UNIQUE indexes - they're implicit
		table.Columns = append(table.Columns, column)
	}

	// Parse foreign keys from tags
	if err := p.parseForeignKeys(modelType, table); err != nil {
		return nil, fmt.Errorf("failed to parse foreign keys: %w", err)
	}

	// Parse relationships
	if err := p.ParseRelationships(modelType, table); err != nil {
		return nil, fmt.Errorf("failed to parse relationships: %w", err)
	}
	// Cache the result
	p.cache[modelType] = table
	return table, nil
}

// extractTableName extracts the table name from struct type.
// Priority order:
// 1. Global registry (populated by generated code from `pebble generate metadata`)
// 2. Comment directive (development only, when source files exist)
// 3. snake_case conversion (default fallback)
func (p *Parser) extractTableName(modelType reflect.Type) string {
	structName := modelType.Name()

	// Priority 1: Check global registry (from generated code)
	if tableName, ok := customTableNames[structName]; ok {
		return tableName
	}

	// Priority 2: Try to extract from source file comments (development only)
	if customName := p.extractTableNameFromSource(modelType); customName != "" {
		return customName
	}

	// Priority 3: Default to struct name converted to snake_case
	return toSnakeCase(structName)
}

// extractTableNameFromSource attempts to extract table name from source file comments.
// It looks for a comment directive like: // table_name: custom_table_name
func (p *Parser) extractTableNameFromSource(modelType reflect.Type) string {
	// Get the package path and struct name
	pkgPath := modelType.PkgPath()
	structName := modelType.Name()
	if pkgPath == "" || structName == "" {
		return ""
	}
	// Find the source file
	// Use runtime.Caller to get potential source file locations
	sourceFile, err := findSourceFile(pkgPath, structName)
	if err != nil {
		return "" // Silently fail - not critical
	}
	// Parse the source file
	tableName, err := extractTableNameFromFile(sourceFile, structName)
	if err != nil {
		return "" // Silently fail - not critical
	}
	return tableName
}

// findSourceFile attempts to locate the source file containing the struct definition.
func findSourceFile(pkgPath, structName string) (string, error) {
	// Get GOPATH
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(os.Getenv("HOME"), "go")
	}

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	// Build list of possible paths to search
	possiblePaths := []string{
		wd, // First, try current working directory (handles main package and local packages)
	}

	// For non-main packages, add traditional Go paths
	if pkgPath != "" && pkgPath != "main" {
		possiblePaths = append(possiblePaths,
			filepath.Join(gopath, "src", pkgPath),
			pkgPath,                    // Try as absolute path
			filepath.Join(wd, pkgPath), // Try as relative to working directory
		)
	}

	// Search for the struct in .go files
	for _, pkgDir := range possiblePaths {
		files, err := filepath.Glob(filepath.Join(pkgDir, "*.go"))
		if err != nil {
			continue
		}
		for _, file := range files {
			// Quick check: does the file contain the struct name?
			content, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			if strings.Contains(string(content), "type "+structName+" struct") {
				return file, nil
			}
		}
	}
	return "", fmt.Errorf("source file not found for %s.%s", pkgPath, structName)
}

// extractTableNameFromFile parses a Go source file and extracts the table name from comments.
func extractTableNameFromFile(filename, structName string) (string, error) {
	fset := token.NewFileSet()
	// Parse the file
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("failed to parse file: %w", err)
	}
	// Find the struct declaration
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		// Check each spec in the declaration
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != structName {
				continue
			}
			// Check if it's a struct type
			if _, ok := typeSpec.Type.(*ast.StructType); !ok {
				continue
			}
			// Found the struct! Now check for table_name comment
			if genDecl.Doc != nil {
				for _, comment := range genDecl.Doc.List {
					tableName := ParseTableNameFromComment(comment.Text)
					if tableName != "" {
						return tableName, nil
					}
				}
			}
			// Also check line comments
			if typeSpec.Comment != nil {
				for _, comment := range typeSpec.Comment.List {
					tableName := ParseTableNameFromComment(comment.Text)
					if tableName != "" {
						return tableName, nil
					}
				}
			}
		}
	}
	return "", nil // No custom table name found
}

// createColumnMetadata creates a ColumnMetadata from a struct field.
func (p *Parser) createColumnMetadata(field reflect.StructField, opts *TagOptions, position int) ColumnMetadata {
	column := ColumnMetadata{
		Name:     opts.Name,
		GoField:  field.Name,
		GoType:   field.Type,
		Position: position,
	}
	// Determine SQL type
	if sqlType := opts.GetSQLType(); sqlType != "" {
		column.SQLType = sqlType
	} else {
		column.SQLType = p.typeMapper.GoTypeToPostgreSQL(field.Type)
	}
	// Set nullability
	column.Nullable = !opts.Has("notNull") && !opts.Has("primaryKey")
	if IsNullable(field.Type) {
		column.Nullable = true
	}
	// Set default value
	if defaultVal := opts.Get("default"); defaultVal != "" {
		column.Default = &defaultVal
	}
	// Set unique constraint
	column.Unique = opts.Has("unique")
	// Set auto-increment (legacy serial)
	column.AutoIncrement = opts.Has("autoIncrement") || opts.Has("serial")

	// Parse identity column (modern SQL standard, PostgreSQL 10+)
	// identity or identityAlways -> GENERATED ALWAYS AS IDENTITY
	// identityByDefault -> GENERATED BY DEFAULT AS IDENTITY
	if opts.Has("identity") || opts.Has("identityAlways") {
		column.Identity = &IdentityColumn{
			Generation: IdentityAlways,
		}
	} else if opts.Has("identityByDefault") {
		column.Identity = &IdentityColumn{
			Generation: IdentityByDefault,
		}
	}

	// Parse generated column
	if generatedExpr := opts.Get("generated"); generatedExpr != "" {
		genType := GeneratedStored // Default to STORED
		if opts.Has("virtual") {
			genType = GeneratedVirtual
		}

		column.Generated = &GeneratedColumn{
			Expression: generatedExpr,
			Type:       genType,
		}
	}

	return column
}

// isRelationshipTag checks if tag options indicate a relationship field.
func (p *Parser) isRelationshipTag(opts *TagOptions) bool {
	return opts.Has("belongsTo") || opts.Has("hasOne") ||
		opts.Has("hasMany") || opts.Has("manyToMany")
}

// TagOptions represents parsed tag options.
type TagOptions struct {
	Name    string            // Column name (first element)
	Options map[string]string // Other options
}

// parseTag parses a struct tag value into TagOptions.
// Format: "column_name,option1,option2(value),option3"
func (p *Parser) parseTag(tag string) (*TagOptions, error) {
	parts := splitTag(tag)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty tag value")
	}
	opts := &TagOptions{
		Name:    parts[0],
		Options: make(map[string]string),
	}
	// Parse remaining options
	for i := 1; i < len(parts); i++ {
		opt := parts[i]
		// Check if option has a value: option(value) or option:value
		if idx := strings.Index(opt, "("); idx != -1 {
			if !strings.HasSuffix(opt, ")") {
				return nil, fmt.Errorf("invalid option format: %s", opt)
			}
			key := opt[:idx]
			value := opt[idx+1 : len(opt)-1]
			opts.Options[key] = value
		} else if idx := strings.Index(opt, ":"); idx != -1 {
			// Support colon format: key:value
			key := opt[:idx]
			value := opt[idx+1:]
			opts.Options[key] = value
		} else {
			// Boolean option
			opts.Options[opt] = ""
		}
	}
	return opts, nil
}

// Has checks if an option exists.
func (t *TagOptions) Has(key string) bool {
	_, ok := t.Options[key]
	return ok
}

// Get returns the value of an option.
func (t *TagOptions) Get(key string) string {
	return t.Options[key]
}

// GetSQLType returns the SQL type from tag options.
// Checks for: uuid, varchar(n), text, numeric(p,s), smallint, integer, bigint, etc.
func (t *TagOptions) GetSQLType() string {
	// Check common PostgreSQL types
	pgTypes := []string{
		"uuid", "varchar", "text", "char",
		"smallint", "integer", "bigint", "serial", "bigserial",
		"numeric", "decimal", "real", "double precision",
		"boolean", "bool",
		"date", "time", "timestamp", "timestamptz", "interval",
		"json", "jsonb",
		"bytea",
		"inet", "cidr", "macaddr",
		"point", "line", "lseg", "box", "path", "polygon", "circle",
		"tsvector", "tsquery",
	}
	for _, pgType := range pgTypes {
		if t.Has(pgType) {
			// If the type has a parameter (e.g., varchar(255))
			if value := t.Get(pgType); value != "" {
				return fmt.Sprintf("%s(%s)", pgType, value)
			}
			return pgType
		}
	}
	return ""
}

// splitTag splits a tag value by commas, handling nested parentheses.
func splitTag(tag string) []string {
	var parts []string
	var current strings.Builder
	depth := 0
	for _, ch := range tag {
		switch ch {
		case '(':
			depth++
			current.WriteRune(ch)
		case ')':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}
	return parts
}

// toSnakeCase converts a string from PascalCase to snake_case.
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

// ParseTableNameFromComment extracts table name from a comment.
// Format: // table_name: custom_table_name
func ParseTableNameFromComment(comment string) string {
	re := regexp.MustCompile(`table_name:\s*([a-zA-Z0-9_]+)`)
	matches := re.FindStringSubmatch(comment)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ParseSourceFile attempts to extract table name from source file comments.
// This requires reading the source file, which is more complex.
func ParseSourceFile(_ string, _ string) (string, error) {
	// TODO: Implement source file parsing to extract table_name from comments
	// This would involve:
	// 1. Reading the source file
	// 2. Finding the struct definition
	// 3. Looking for the comment above it
	// 4. Parsing the table_name directive
	return "", fmt.Errorf("source file parsing not implemented")
}

// parseForeignKeys extracts foreign key constraints from struct tags.
func (p *Parser) parseForeignKeys(modelType reflect.Type, table *TableMetadata) error {
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		tag := field.Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}

		opts, err := p.parseTag(tag)
		if err != nil {
			continue
		}

		// Check for foreign key definition: fk:table.column or fk:table(column)
		fkStr := opts.Get("fk")
		if fkStr == "" {
			continue
		}

		// Parse foreign key reference
		var refTable, refColumn string

		// Support both "table.column" and "table(column)" formats
		if strings.Contains(fkStr, ".") {
			parts := strings.SplitN(fkStr, ".", 2)
			if len(parts) == 2 {
				refTable = parts[0]
				refColumn = parts[1]
			}
		} else if strings.Contains(fkStr, "(") {
			// Parse "table(column)" format
			idx := strings.Index(fkStr, "(")
			if idx > 0 && strings.HasSuffix(fkStr, ")") {
				refTable = fkStr[:idx]
				refColumn = fkStr[idx+1 : len(fkStr)-1]
			}
		}

		if refTable == "" || refColumn == "" {
			continue // Invalid format, skip
		}

		// Get column name
		columnName := opts.Name

		// Create foreign key metadata
		fk := ForeignKeyMetadata{
			Name:              fmt.Sprintf("fk_%s_%s_%s", table.Name, columnName, refTable),
			Columns:           []string{columnName},
			ReferencedTable:   refTable,
			ReferencedColumns: []string{refColumn},
			OnDelete:          parseReferenceAction(opts.Get("onDelete")),
			OnUpdate:          parseReferenceAction(opts.Get("onUpdate")),
		}

		table.ForeignKeys = append(table.ForeignKeys, fk)
	}

	return nil
}

// parseReferenceAction converts a string to ReferenceAction.
func parseReferenceAction(action string) ReferenceAction {
	if action == "" {
		return NoAction
	}

	switch strings.ToUpper(strings.TrimSpace(action)) {
	case "CASCADE":
		return Cascade
	case "RESTRICT":
		return Restrict
	case "SETNULL", "SET NULL":
		return SetNull
	case "SETDEFAULT", "SET DEFAULT":
		return SetDefault
	case "NOACTION", "NO ACTION":
		return NoAction
	default:
		return NoAction
	}
}
