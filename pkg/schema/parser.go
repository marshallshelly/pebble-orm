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
	for modelType.Kind() == reflect.Pointer {
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
	for field := range modelType.Fields() {
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
		column := p.createColumnMetadata(field, tagOpts, field.Index[0])
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

	// Create UNIQUE constraints for columns marked as unique
	// This allows the migration system to detect and manage UNIQUE constraints
	for _, col := range table.Columns {
		if col.Unique {
			constraint := ConstraintMetadata{
				Name:    fmt.Sprintf("%s_%s_key", table.Name, col.Name),
				Type:    UniqueConstraint,
				Columns: []string{col.Name},
			}
			table.Constraints = append(table.Constraints, constraint)
		}
	}

	// Parse column-level indexes from tags
	if err := p.parseColumnIndexes(modelType, table); err != nil {
		return nil, fmt.Errorf("failed to parse column indexes: %w", err)
	}

	// Collect enum types used by this table
	// Build a map to deduplicate enum types (multiple columns can use same enum)
	enumTypeMap := make(map[string]EnumType)
	for _, col := range table.Columns {
		if col.EnumType != "" {
			// Only add if not already present
			if _, exists := enumTypeMap[col.EnumType]; !exists {
				enumTypeMap[col.EnumType] = EnumType{
					Name:   col.EnumType,
					Values: col.EnumValues,
				}
			}
		}
	}

	// Convert map to slice
	table.EnumTypes = make([]EnumType, 0, len(enumTypeMap))
	for _, enumType := range enumTypeMap {
		table.EnumTypes = append(table.EnumTypes, enumType)
	}

	// Parse foreign keys from tags
	if err := p.parseForeignKeys(modelType, table); err != nil {
		return nil, fmt.Errorf("failed to parse foreign keys: %w", err)
	}

	// Parse relationships
	if err := p.ParseRelationships(modelType, table); err != nil {
		return nil, fmt.Errorf("failed to parse relationships: %w", err)
	}

	// Parse table-level indexes from source comments
	if err := p.parseTableIndexes(modelType, table); err != nil {
		return nil, fmt.Errorf("failed to parse table indexes: %w", err)
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

	// Parse enum column
	if enumValues := opts.Get("enum"); enumValues != "" {
		// Parse enum values from tag (e.g., "pending,active,completed")
		values := strings.Split(enumValues, ",")
		for i := range values {
			values[i] = strings.TrimSpace(values[i])
		}
		column.EnumValues = values

		// Derive enum type name from Go type (e.g., OrderStatus -> order_status)
		column.EnumType = toSnakeCase(field.Type.Name())

		// Override SQL type to use the enum type
		column.SQLType = column.EnumType
	}

	// Detect JSONB columns for automatic marshaling
	// JSONB columns are detected by:
	// 1. Explicit jsonb tag option
	// 2. SQLType containing "json" or "jsonb"
	sqlTypeLower := strings.ToLower(column.SQLType)
	column.IsJSONB = opts.Has("jsonb") || opts.Has("json") ||
		sqlTypeLower == "jsonb" || sqlTypeLower == "json"

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
		} else if before, after, ok := strings.Cut(opt, ":"); ok {
			// Support colon format: key:value
			key := before
			value := after
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

// ParseIndexFromComment extracts index definition from a comment.
// Format: // index: idx_name ON (columns) [USING type] [INCLUDE (cols)] [WHERE condition]
// Examples:
//   - // index: idx_email ON (email)
//   - // index: idx_email_lower ON (lower(email))
//   - // index: idx_active ON (email) WHERE deleted_at IS NULL
//   - // index: idx_covering ON (email) INCLUDE (name, created_at)
//   - // index: idx_multi ON (tenant_id, status, created_at DESC)
func ParseIndexFromComment(comment string) *IndexMetadata {
	// Match the index directive and name
	prefixPattern := regexp.MustCompile(`index:\s*(\w+)\s+ON\s+\(`)
	prefixMatches := prefixPattern.FindStringIndex(comment)
	if prefixMatches == nil {
		return nil
	}

	// Extract the index name
	namePattern := regexp.MustCompile(`index:\s*(\w+)\s+ON`)
	nameMatches := namePattern.FindStringSubmatch(comment)
	if len(nameMatches) < 2 {
		return nil
	}

	index := &IndexMetadata{
		Name: strings.TrimSpace(nameMatches[1]),
	}

	// Find the content within parentheses (handling nested parens)
	startIdx := prefixMatches[1] // Position after "ON ("
	columnsOrExpr, remaining := extractBalancedParens(comment[startIdx:])
	if columnsOrExpr == "" {
		return nil
	}

	// Check if it's an expression (contains function calls or operators)
	if strings.Contains(columnsOrExpr, "(") || strings.Contains(columnsOrExpr, "||") {
		// It's an expression
		index.Expression = columnsOrExpr
	} else {
		// It's column list - parse columns and ordering
		index.Columns, index.ColumnOrdering = parseIndexColumns(columnsOrExpr)
	}

	// Parse USING clause
	usingPattern := regexp.MustCompile(`USING\s+(\w+)`)
	if usingMatches := usingPattern.FindStringSubmatch(remaining); len(usingMatches) > 1 {
		index.Type = strings.ToLower(usingMatches[1])
	} else {
		index.Type = "btree" // default
	}

	// Parse INCLUDE clause
	includePattern := regexp.MustCompile(`INCLUDE\s+\(([^)]+)\)`)
	if includeMatches := includePattern.FindStringSubmatch(remaining); len(includeMatches) > 1 {
		includeCols := strings.SplitSeq(includeMatches[1], ",")
		for col := range includeCols {
			index.Include = append(index.Include, strings.TrimSpace(col))
		}
	}

	// Parse WHERE clause (stop before CONCURRENTLY if present)
	wherePattern := regexp.MustCompile(`WHERE\s+(.+?)(?:\s+CONCURRENTLY|\s*$)`)
	if whereMatches := wherePattern.FindStringSubmatch(remaining); len(whereMatches) > 1 {
		index.Where = strings.TrimSpace(whereMatches[1])
	}

	// Parse CONCURRENTLY flag
	if strings.Contains(strings.ToUpper(remaining), "CONCURRENTLY") {
		index.Concurrent = true
	}

	return index
}

// parseIndexColumns parses a column list with optional modifiers.
// Supports PostgreSQL syntax: column_name [opclass] [COLLATE "collation"] [ASC|DESC] [NULLS FIRST|LAST]
// Examples:
//   - "email" -> ["email"], []
//   - "email varchar_pattern_ops" -> ["email"], [ColumnOrder{OpClass: "varchar_pattern_ops"}]
//   - "name COLLATE \"en_US\"" -> ["name"], [ColumnOrder{Collation: "en_US"}]
//   - "created_at DESC NULLS LAST" -> ["created_at"], [ColumnOrder{Direction: DESC, Nulls: NULLS LAST}]
//   - "email varchar_pattern_ops COLLATE \"C\" DESC" -> complete example
func parseIndexColumns(columnsStr string) ([]string, []ColumnOrder) {
	parts := strings.Split(columnsStr, ",")
	var columns []string
	var ordering []ColumnOrder

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Parse tokens with special handling for quoted strings
		tokens := tokenizeIndexColumn(part)
		if len(tokens) == 0 {
			continue
		}

		columnName := tokens[0]
		columns = append(columns, columnName)

		// Parse modifiers if present
		if len(tokens) > 1 {
			order := ColumnOrder{
				Column:    columnName,
				Direction: Ascending, // default
			}

			i := 1
			for i < len(tokens) {
				token := tokens[i]
				tokenUpper := strings.ToUpper(token)

				switch tokenUpper {
				case "ASC":
					order.Direction = Ascending
					i++
				case "DESC":
					order.Direction = Descending
					i++
				case "NULLS":
					if i+1 < len(tokens) {
						next := strings.ToUpper(tokens[i+1])
						if next == "FIRST" {
							order.Nulls = NullsFirst
							i += 2
						} else if next == "LAST" {
							order.Nulls = NullsLast
							i += 2
						} else {
							i++
						}
					} else {
						i++
					}
				case "COLLATE":
					// Next token should be the collation (possibly quoted)
					if i+1 < len(tokens) {
						collation := tokens[i+1]
						// Remove quotes if present
						collation = strings.Trim(collation, "\"'")
						order.Collation = collation
						i += 2
					} else {
						i++
					}
				default:
					// If it's not a reserved word, it's an operator class
					if !isReservedIndexKeyword(tokenUpper) {
						order.OpClass = token
					}
					i++
				}
			}

			ordering = append(ordering, order)
		}
	}

	return columns, ordering
}

// tokenizeIndexColumn splits a column specification into tokens, handling quoted strings.
// Example: 'name COLLATE "en_US" DESC' -> ["name", "COLLATE", "en_US", "DESC"]
func tokenizeIndexColumn(s string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for i, ch := range s {
		switch {
		case (ch == '"' || ch == '\'') && !inQuote:
			// Start of quoted string
			inQuote = true
			quoteChar = ch
		case ch == quoteChar && inQuote:
			// End of quoted string
			tokens = append(tokens, current.String())
			current.Reset()
			inQuote = false
			quoteChar = 0
		case ch == ' ' && !inQuote:
			// Whitespace outside quotes
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		case inQuote || (ch != ' ' && ch != '"' && ch != '\''):
			// Regular character or quoted content
			current.WriteRune(ch)
		}

		// Handle end of string
		if i == len(s)-1 && current.Len() > 0 {
			tokens = append(tokens, current.String())
		}
	}

	return tokens
}

// isReservedIndexKeyword checks if a token is a reserved index keyword.
func isReservedIndexKeyword(token string) bool {
	reserved := map[string]bool{
		"ASC":     true,
		"DESC":    true,
		"NULLS":   true,
		"FIRST":   true,
		"LAST":    true,
		"COLLATE": true,
	}
	return reserved[token]
}

// extractBalancedParens extracts content within balanced parentheses.
// Returns the content and the remaining string after the closing paren.
// Example: "lower(email)) WHERE ..." -> "lower(email)", " WHERE ..."
func extractBalancedParens(s string) (content, remaining string) {
	depth := 0
	for i, ch := range s {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == -1 {
				// Found the matching closing paren
				return s[:i], s[i+1:]
			}
		}
	}
	return "", ""
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
	for field := range modelType.Fields() {
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

// parseColumnIndexes extracts index definitions from column tags.
// Supports formats:
//   - `po:"column,type,index"` - simple index with auto-generated name
//   - `po:"column,type,index(name)"` - named index
//   - `po:"column,type,index(name,gin)"` - named index with type
//   - `po:"column,type,index(name,btree,desc)"` - with ordering
func (p *Parser) parseColumnIndexes(modelType reflect.Type, table *TableMetadata) error {
	for field := range modelType.Fields() {
		if !field.IsExported() {
			continue
		}

		tagValue := field.Tag.Get(StructTagKey)
		if tagValue == "" {
			continue
		}

		opts, err := p.parseTag(tagValue)
		if err != nil {
			continue
		}

		// Check for index option
		indexValue := opts.Get("index")
		if indexValue == "" && !opts.Has("index") {
			continue
		}

		// Find the column name for this field
		columnName := opts.Name
		if columnName == "" || columnName == "-" {
			continue
		}

		// Parse index parameters
		indexName, indexType, direction := p.parseIndexParameters(indexValue, table.Name, columnName)

		// Create index metadata
		index := IndexMetadata{
			Name:    indexName,
			Columns: []string{columnName},
			Type:    indexType,
		}

		// Add column ordering if specified
		if direction == "desc" {
			index.ColumnOrdering = []ColumnOrder{
				{
					Column:    columnName,
					Direction: Descending,
				},
			}
		}

		table.Indexes = append(table.Indexes, index)
	}

	return nil
}

// parseIndexParameters parses the index tag value and returns name, type, and direction.
// Supports:
//   - "" or no value: auto-generated name, btree, asc
//   - "name": custom name, btree, asc
//   - "name,gin": custom name, gin, asc
//   - "name,btree,desc": custom name, btree, desc
func (p *Parser) parseIndexParameters(value, tableName, columnName string) (name, indexType, direction string) {
	// Default values
	name = fmt.Sprintf("idx_%s_%s", tableName, columnName)
	indexType = "btree"
	direction = "asc"

	if value == "" {
		return
	}

	// Split by comma
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return
	}

	// First part is the index name
	if parts[0] != "" {
		name = strings.TrimSpace(parts[0])
	}

	// Second part is the index type (if present)
	if len(parts) > 1 && parts[1] != "" {
		indexType = strings.ToLower(strings.TrimSpace(parts[1]))
	}

	// Third part is the direction (if present)
	if len(parts) > 2 && parts[2] != "" {
		direction = strings.ToLower(strings.TrimSpace(parts[2]))
	}

	return
}

// parseTableIndexes extracts index definitions from struct-level comments.
// It looks for comments like: // index: idx_name ON (columns) ...
func (p *Parser) parseTableIndexes(modelType reflect.Type, table *TableMetadata) error {
	// Get the package path and struct name
	pkgPath := modelType.PkgPath()
	structName := modelType.Name()
	if pkgPath == "" || structName == "" {
		return nil // Not an error, just no source file available
	}

	// Find the source file
	sourceFile, err := findSourceFile(pkgPath, structName)
	if err != nil {
		return nil // Silently fail - not critical
	}

	// Parse the source file and extract indexes
	indexes, err := extractIndexesFromFile(sourceFile, structName)
	if err != nil {
		return nil // Silently fail - not critical
	}

	// Add indexes to table
	table.Indexes = append(table.Indexes, indexes...)

	return nil
}

// extractIndexesFromFile parses a Go source file and extracts index definitions from comments.
func extractIndexesFromFile(filename, structName string) ([]IndexMetadata, error) {
	fset := token.NewFileSet()
	// Parse the file
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	var indexes []IndexMetadata

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

			// Found the struct! Now check for index comments
			if genDecl.Doc != nil {
				for _, comment := range genDecl.Doc.List {
					index := ParseIndexFromComment(comment.Text)
					if index != nil {
						indexes = append(indexes, *index)
					}
				}
			}

			// Also check line comments
			if typeSpec.Comment != nil {
				for _, comment := range typeSpec.Comment.List {
					index := ParseIndexFromComment(comment.Text)
					if index != nil {
						indexes = append(indexes, *index)
					}
				}
			}
		}
	}

	return indexes, nil
}
