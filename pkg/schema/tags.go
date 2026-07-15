package schema

import (
	"fmt"
	"strings"
)

// This file holds the single interpretation of a `po:` struct tag, shared by
// the reflection parser (pkg/schema/parser.go) and the AST loader
// (pkg/loader/loader.go). Both extract Go-type facts differently — one via
// reflect, one from the AST — but funnel them through FieldMeta and the
// functions here so a tag means exactly one thing regardless of entry point.

// FieldMeta carries the Go-type-derived facts the tag interpreter needs.
type FieldMeta struct {
	GoField      string // Go struct field name
	TypeName     string // base Go type name (pointer/slice deref'd), for enum type naming
	Nullable     bool   // Go type is a pointer or a sql.Null* type
	InferredType string // PostgreSQL type inferred from the Go type ("" if unknown)
	IsJSONBHint  bool   // Go type implies JSONB (e.g. map[string]any)
	Position     int
}

// ParseTag splits a po tag value into TagOptions. Supports both the
// option(value) and option:value forms.
func ParseTag(tag string) (*TagOptions, error) {
	parts := splitTag(tag)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty tag value")
	}
	opts := &TagOptions{
		Name:    parts[0],
		Options: make(map[string]string),
	}
	for i := 1; i < len(parts); i++ {
		opt := parts[i]
		parenIdx := strings.Index(opt, "(")
		colonIdx := strings.Index(opt, ":")
		switch {
		case colonIdx != -1 && (parenIdx == -1 || colonIdx < parenIdx):
			// Colon format key:value. The value may itself contain parens,
			// e.g. fk:users(id) — so a colon before the paren wins.
			opts.Options[opt[:colonIdx]] = opt[colonIdx+1:]
		case parenIdx != -1:
			// Parenthesised format key(value), e.g. varchar(255), default(NOW()).
			if !strings.HasSuffix(opt, ")") {
				return nil, fmt.Errorf("invalid option format: %s", opt)
			}
			opts.Options[opt[:parenIdx]] = opt[parenIdx+1 : len(opt)-1]
		default:
			// Boolean option.
			opts.Options[opt] = ""
		}
	}
	return opts, nil
}

// IsRelationshipTag reports whether the tag denotes a relationship field
// (belongsTo/hasOne/hasMany/manyToMany) rather than a column.
func IsRelationshipTag(opts *TagOptions) bool {
	return opts.Has("belongsTo") || opts.Has("hasOne") ||
		opts.Has("hasMany") || opts.Has("manyToMany")
}

// BuildColumn interprets a parsed po tag against Go-type facts to produce a
// column definition. This is the single source of truth for how a tag becomes
// a ColumnMetadata, covering types, constraints, identity, generated, enum and
// JSONB handling.
func BuildColumn(opts *TagOptions, fm FieldMeta) ColumnMetadata {
	column := ColumnMetadata{
		Name:     opts.Name,
		GoField:  fm.GoField,
		Position: fm.Position,
	}

	// SQL type: explicit tag type wins, then Go-type inference, then text.
	if sqlType := opts.GetSQLType(); sqlType != "" {
		column.SQLType = sqlType
	} else if fm.InferredType != "" {
		column.SQLType = fm.InferredType
	} else {
		column.SQLType = "text"
	}

	// Nullability: nullable unless notNull/primaryKey, or forced by a nullable
	// Go type (pointer, sql.Null*).
	column.Nullable = !opts.Has("notNull") && !opts.Has("primaryKey")
	if fm.Nullable {
		column.Nullable = true
	}

	if defaultVal := opts.Get("default"); defaultVal != "" {
		column.Default = &defaultVal
	}

	column.Unique = opts.Has("unique")
	column.AutoIncrement = opts.Has("autoIncrement") || opts.Has("serial") ||
		opts.Has("bigserial") || opts.Has("smallserial")

	// Identity columns (PostgreSQL 10+). Implicitly NOT NULL.
	if opts.Has("identity") || opts.Has("identityAlways") {
		column.Identity = &IdentityColumn{Generation: IdentityAlways}
	} else if opts.Has("identityByDefault") {
		column.Identity = &IdentityColumn{Generation: IdentityByDefault}
	}
	if column.Identity != nil {
		column.Nullable = false
	}

	// Generated columns.
	if generatedExpr := opts.Get("generated"); generatedExpr != "" {
		genType := GeneratedStored
		if opts.Has("virtual") {
			genType = GeneratedVirtual
		}
		column.Generated = &GeneratedColumn{Expression: generatedExpr, Type: genType}
	}

	// Enum columns: derive the enum type name from the Go type.
	if enumValues := opts.Get("enum"); enumValues != "" {
		values := strings.Split(enumValues, ",")
		for i := range values {
			values[i] = strings.TrimSpace(values[i])
		}
		column.EnumValues = values
		column.EnumType = toSnakeCase(fm.TypeName)
		column.SQLType = column.EnumType
	}

	// JSONB detection.
	sqlTypeLower := strings.ToLower(column.SQLType)
	column.IsJSONB = opts.Has("jsonb") || opts.Has("json") ||
		sqlTypeLower == "jsonb" || sqlTypeLower == "json" || fm.IsJSONBHint

	return column
}

// ColumnIndex builds a column-level index from an index tag option, or returns
// ok=false if the tag has none. Supports index, index(name), index(name,type)
// and index(name,type,desc).
func ColumnIndex(opts *TagOptions, tableName string) (IndexMetadata, bool) {
	indexValue := opts.Get("index")
	if indexValue == "" && !opts.Has("index") {
		return IndexMetadata{}, false
	}
	columnName := opts.Name
	if columnName == "" || columnName == "-" {
		return IndexMetadata{}, false
	}

	name, indexType, direction := parseIndexParameters(indexValue, tableName, columnName)
	index := IndexMetadata{
		Name:    name,
		Columns: []string{columnName},
		Type:    indexType,
	}
	if direction == "desc" {
		index.ColumnOrdering = []ColumnOrder{{Column: columnName, Direction: Descending}}
	}
	return index, true
}

// ColumnForeignKey builds a foreign key from an fk tag option, or returns
// ok=false if there is none. Supports fk:table(column) and fk:table.column
// (and the parenthesised option form), with optional onDelete/onUpdate.
func ColumnForeignKey(opts *TagOptions, tableName string) (ForeignKeyMetadata, bool) {
	fkStr := opts.Get("fk")
	if fkStr == "" {
		return ForeignKeyMetadata{}, false
	}

	var refTable, refColumn string
	if strings.Contains(fkStr, ".") {
		if parts := strings.SplitN(fkStr, ".", 2); len(parts) == 2 {
			refTable, refColumn = parts[0], parts[1]
		}
	} else if idx := strings.Index(fkStr, "("); idx > 0 && strings.HasSuffix(fkStr, ")") {
		refTable = fkStr[:idx]
		refColumn = fkStr[idx+1 : len(fkStr)-1]
	}
	if refTable == "" || refColumn == "" {
		return ForeignKeyMetadata{}, false
	}

	columnName := opts.Name
	return ForeignKeyMetadata{
		Name:              fmt.Sprintf("fk_%s_%s_%s", tableName, columnName, refTable),
		Columns:           []string{columnName},
		ReferencedTable:   refTable,
		ReferencedColumns: []string{refColumn},
		OnDelete:          ParseReferenceAction(opts.Get("onDelete")),
		OnUpdate:          ParseReferenceAction(opts.Get("onUpdate")),
	}, true
}

// UniqueConstraintsFor returns the UNIQUE constraints implied by columns marked
// unique, so the migration system can detect and manage them.
func UniqueConstraintsFor(tableName string, columns []ColumnMetadata) []ConstraintMetadata {
	var constraints []ConstraintMetadata
	for _, col := range columns {
		if col.Unique {
			constraints = append(constraints, ConstraintMetadata{
				Name:    fmt.Sprintf("%s_%s_key", tableName, col.Name),
				Type:    UniqueConstraint,
				Columns: []string{col.Name},
			})
		}
	}
	return constraints
}

// CollectEnumTypes gathers the distinct enum types used by the given columns.
func CollectEnumTypes(columns []ColumnMetadata) []EnumType {
	seen := make(map[string]EnumType)
	for _, col := range columns {
		if col.EnumType != "" {
			if _, exists := seen[col.EnumType]; !exists {
				seen[col.EnumType] = EnumType{Name: col.EnumType, Values: col.EnumValues}
			}
		}
	}
	enums := make([]EnumType, 0, len(seen))
	for _, e := range seen {
		enums = append(enums, e)
	}
	return enums
}

// ParseReferenceAction converts an ON DELETE / ON UPDATE action string to a
// ReferenceAction, accepting both spaced ("SET NULL") and tag ("SETNULL") forms.
func ParseReferenceAction(action string) ReferenceAction {
	switch strings.ToUpper(strings.TrimSpace(action)) {
	case "CASCADE":
		return Cascade
	case "RESTRICT":
		return Restrict
	case "SETNULL", "SET NULL":
		return SetNull
	case "SETDEFAULT", "SET DEFAULT":
		return SetDefault
	case "NOACTION", "NO ACTION", "":
		return NoAction
	default:
		return NoAction
	}
}
