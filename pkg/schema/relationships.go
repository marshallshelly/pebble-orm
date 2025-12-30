package schema

import (
	"fmt"
	"reflect"
)

// ParseRelationships extracts relationship metadata from struct fields.
func (p *Parser) ParseRelationships(modelType reflect.Type, table *TableMetadata) error {
	for modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	if modelType.Kind() != reflect.Struct {
		return fmt.Errorf("model must be a struct")
	}

	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)

		if !field.IsExported() {
			continue
		}

		tagValue := field.Tag.Get(StructTagKey)
		if tagValue == "" {
			continue
		}

		tagOpts, err := p.parseTag(tagValue)
		if err != nil {
			continue
		}

		// Check if this is a relationship field
		if !p.isRelationshipTag(tagOpts) {
			continue
		}

		rel, err := p.parseRelationship(field, tagOpts, table)
		if err != nil {
			return fmt.Errorf("failed to parse relationship for field %s: %w", field.Name, err)
		}

		if rel != nil {
			// Store relationship in table metadata
			if table.Relationships == nil {
				table.Relationships = make([]RelationshipMetadata, 0)
			}
			table.Relationships = append(table.Relationships, *rel)
		}
	}

	return nil
}

// parseRelationship parses a relationship from a struct field.
func (p *Parser) parseRelationship(field reflect.StructField, opts *TagOptions, sourceTable *TableMetadata) (*RelationshipMetadata, error) {
	rel := &RelationshipMetadata{
		SourceTable: sourceTable.Name,
		SourceField: field.Name,
	}

	// Determine relationship type
	if opts.Has("belongsTo") {
		rel.Type = BelongsTo
	} else if opts.Has("hasOne") {
		rel.Type = HasOne
	} else if opts.Has("hasMany") {
		rel.Type = HasMany
	} else if opts.Has("manyToMany") {
		rel.Type = ManyToMany
	} else {
		return nil, fmt.Errorf("unknown relationship type")
	}

	// Get foreign key
	if foreignKey := opts.Get("foreignKey"); foreignKey != "" {
		rel.ForeignKey = foreignKey
	}

	// Get references
	if references := opts.Get("references"); references != "" {
		rel.References = references
	}

	// Get target table from field type
	fieldType := field.Type

	// Handle slice types (hasMany, manyToMany)
	if fieldType.Kind() == reflect.Slice {
		fieldType = fieldType.Elem()
	}

	// Handle pointer types
	for fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	if fieldType.Kind() == reflect.Struct {
		rel.TargetType = fieldType                      // Store the actual Go type for accurate table name lookup
		rel.TargetTable = toSnakeCase(fieldType.Name()) // Fallback (may be incorrect with custom table names)
		rel.TargetField = fieldType.Name()
	}

	// For manyToMany, get junction table
	if rel.Type == ManyToMany {
		if joinTable := opts.Get("joinTable"); joinTable != "" {
			rel.JoinTable = &joinTable
		} else {
			// Generate junction table name
			// e.g., users_posts
			junction := generateJunctionTableName(sourceTable.Name, rel.TargetTable)
			rel.JoinTable = &junction
		}
	}

	// Set default foreign key if not specified
	if rel.ForeignKey == "" {
		switch rel.Type {
		case BelongsTo:
			// For belongsTo, foreign key is on the source table
			// e.g., user_id
			rel.ForeignKey = toSnakeCase(rel.TargetField) + "_id"
		case HasOne, HasMany:
			// For hasOne/hasMany, foreign key is on the target table
			// e.g., user_id
			rel.ForeignKey = toSnakeCase(sourceTable.GoType.Name()) + "_id"
		}
	}

	// Set default references if not specified
	if rel.References == "" {
		rel.References = "id" // Default to id column
	}

	// Get inverse field name
	if inverse := opts.Get("inverse"); inverse != "" {
		rel.InverseField = &inverse
	}

	return rel, nil
}

// generateJunctionTableName generates a junction table name from two table names.
func generateJunctionTableName(table1, table2 string) string {
	// Sort alphabetically for consistency
	if table1 > table2 {
		table1, table2 = table2, table1
	}
	return table1 + "_" + table2
}

// GetRelationship returns a relationship by source field name.
func (t *TableMetadata) GetRelationship(fieldName string) *RelationshipMetadata {
	if t.Relationships == nil {
		return nil
	}

	for i := range t.Relationships {
		if t.Relationships[i].SourceField == fieldName {
			return &t.Relationships[i]
		}
	}

	return nil
}

// GetRelationshipsByType returns all relationships of a specific type.
func (t *TableMetadata) GetRelationshipsByType(relType RelationType) []RelationshipMetadata {
	if t.Relationships == nil {
		return nil
	}

	var result []RelationshipMetadata
	for _, rel := range t.Relationships {
		if rel.Type == relType {
			result = append(result, rel)
		}
	}

	return result
}

// HasRelationships checks if the table has any relationships.
func (t *TableMetadata) HasRelationships() bool {
	return len(t.Relationships) > 0
}
