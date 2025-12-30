package builder

import (
	"context"
	"fmt"
	"reflect"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// loadRelationships loads all preloaded relationships for a set of results.
func (q *SelectQuery[T]) loadRelationships(ctx context.Context, results interface{}) error {
	if len(q.preloads) == 0 {
		return nil
	}

	resultsVal := reflect.ValueOf(results)
	if resultsVal.Kind() != reflect.Ptr {
		return fmt.Errorf("results must be a pointer to slice")
	}

	resultsVal = resultsVal.Elem()
	if resultsVal.Kind() != reflect.Slice {
		return fmt.Errorf("results must be a pointer to slice")
	}

	if resultsVal.Len() == 0 {
		return nil // No results to load relationships for
	}

	// Load each preloaded relationship
	for _, fieldName := range q.preloads {
		rel := q.table.GetRelationship(fieldName)
		if rel == nil {
			return fmt.Errorf("relationship %s not found on %s", fieldName, q.table.Name)
		}

		if err := q.loadRelationship(ctx, resultsVal, rel); err != nil {
			return fmt.Errorf("failed to load relationship %s: %w", fieldName, err)
		}
	}

	return nil
}

// loadRelationship loads a specific relationship for all results.
func (q *SelectQuery[T]) loadRelationship(ctx context.Context, results reflect.Value, rel *schema.RelationshipMetadata) error {
	switch rel.Type {
	case schema.BelongsTo:
		return q.loadBelongsTo(ctx, results, rel)
	case schema.HasOne:
		return q.loadHasOne(ctx, results, rel)
	case schema.HasMany:
		return q.loadHasMany(ctx, results, rel)
	case schema.ManyToMany:
		return q.loadManyToMany(ctx, results, rel)
	default:
		return fmt.Errorf("unsupported relationship type: %s", rel.Type)
	}
}

// loadBelongsTo loads belongsTo relationships.
// Example: Post belongsTo User (post.user_id -> users.id)
func (q *SelectQuery[T]) loadBelongsTo(ctx context.Context, results reflect.Value, rel *schema.RelationshipMetadata) error {
	// Get target table metadata using TargetType (accurate) or fallback to TargetTable (legacy)
	var targetTable *schema.TableMetadata
	var err error

	if rel.TargetType != nil {
		targetTable, err = registry.Get(rel.TargetType)
	} else {
		targetTable, err = registry.GetByName(rel.TargetTable)
	}

	if err != nil {
		return fmt.Errorf("target table %s not registered: %w", rel.TargetTable, err)
	}

	// Collect foreign key values from all results
	foreignKeys := make([]interface{}, 0, results.Len())
	foreignKeyMap := make(map[interface{}][]int) // Map FK value to result indices

	for i := 0; i < results.Len(); i++ {
		item := results.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		// Get the foreign key field value
		fkField := item.FieldByName(toPascalCase(rel.ForeignKey))
		if !fkField.IsValid() {
			continue
		}

		fkValue := fkField.Interface()

		// Skip nil/zero values
		if isZeroValue(fkValue) {
			continue
		}

		// Dereference pointer if needed (for nullable foreign keys like *string)
		if fkField.Kind() == reflect.Ptr && !fkField.IsNil() {
			fkValue = fkField.Elem().Interface()
		}

		// Track which results have this FK value
		if _, exists := foreignKeyMap[fkValue]; !exists {
			foreignKeys = append(foreignKeys, fkValue)
			foreignKeyMap[fkValue] = make([]int, 0)
		}
		foreignKeyMap[fkValue] = append(foreignKeyMap[fkValue], i)
	}

	if len(foreignKeys) == 0 {
		return nil // No foreign keys to load
	}

	// Query related records using IN clause
	sql := fmt.Sprintf("SELECT * FROM %s WHERE %s = ANY($1)", targetTable.Name, rel.References)
	rows, err := q.db.db.Query(ctx, sql, foreignKeys)
	if err != nil {
		return fmt.Errorf("failed to query related records: %w", err)
	}
	defer rows.Close()

	// Scan related records and assign to results
	for rows.Next() {
		related := reflect.New(targetTable.GoType)
		if err := scanIntoStruct(rows, related.Interface(), targetTable); err != nil {
			return fmt.Errorf("failed to scan related record: %w", err)
		}

		// Get the ID value from the related record
		relatedElem := related.Elem()
		idField := relatedElem.FieldByName(toPascalCase(rel.References))
		if !idField.IsValid() {
			continue
		}
		idValue := idField.Interface()

		// Assign to all results that reference this related record
		for _, idx := range foreignKeyMap[idValue] {
			item := results.Index(idx)
			if item.Kind() == reflect.Ptr {
				item = item.Elem()
			}

			relationField := item.FieldByName(rel.SourceField)
			if !relationField.IsValid() || !relationField.CanSet() {
				continue
			}

			// Set the relationship field
			if relationField.Kind() == reflect.Ptr {
				relationField.Set(related)
			} else {
				relationField.Set(related.Elem())
			}
		}
	}

	return rows.Err()
}

// loadHasOne loads hasOne relationships.
// Example: User hasOne Profile (profiles.user_id -> users.id)
func (q *SelectQuery[T]) loadHasOne(ctx context.Context, results reflect.Value, rel *schema.RelationshipMetadata) error {
	// Get target table metadata using TargetType (accurate) or fallback to TargetTable (legacy)
	var targetTable *schema.TableMetadata
	var err error

	if rel.TargetType != nil {
		targetTable, err = registry.Get(rel.TargetType)
	} else {
		targetTable, err = registry.GetByName(rel.TargetTable)
	}

	if err != nil {
		return fmt.Errorf("target table %s not registered: %w", rel.TargetTable, err)
	}

	// Collect primary key values from all results
	primaryKeys := make([]interface{}, 0, results.Len())
	pkMap := make(map[interface{}]int) // Map PK value to result index

	for i := 0; i < results.Len(); i++ {
		item := results.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		// Get the primary key field value
		pkField := item.FieldByName(toPascalCase(rel.References))
		if !pkField.IsValid() {
			continue
		}

		pkValue := pkField.Interface()

		// Dereference pointer if needed
		if pkField.Kind() == reflect.Ptr && !pkField.IsNil() {
			pkValue = pkField.Elem().Interface()
		}

		primaryKeys = append(primaryKeys, pkValue)
		pkMap[pkValue] = i
	}

	if len(primaryKeys) == 0 {
		return nil
	}

	// Query related records using IN clause
	sql := fmt.Sprintf("SELECT * FROM %s WHERE %s = ANY($1)", targetTable.Name, rel.ForeignKey)
	rows, err := q.db.db.Query(ctx, sql, primaryKeys)
	if err != nil {
		return fmt.Errorf("failed to query related records: %w", err)
	}
	defer rows.Close()

	// Scan related records and assign to results
	for rows.Next() {
		related := reflect.New(targetTable.GoType)
		if err := scanIntoStruct(rows, related.Interface(), targetTable); err != nil {
			return fmt.Errorf("failed to scan related record: %w", err)
		}

		// Get the foreign key value from the related record
		relatedElem := related.Elem()
		fkField := relatedElem.FieldByName(toPascalCase(rel.ForeignKey))
		if !fkField.IsValid() {
			continue
		}
		fkValue := fkField.Interface()

		// Find the parent result
		idx, exists := pkMap[fkValue]
		if !exists {
			continue
		}

		item := results.Index(idx)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		relationField := item.FieldByName(rel.SourceField)
		if !relationField.IsValid() || !relationField.CanSet() {
			continue
		}

		// Set the relationship field
		if relationField.Kind() == reflect.Ptr {
			relationField.Set(related)
		} else {
			relationField.Set(related.Elem())
		}
	}

	return rows.Err()
}

// loadHasMany loads hasMany relationships.
// Example: User hasMany Posts (posts.user_id -> users.id)
func (q *SelectQuery[T]) loadHasMany(ctx context.Context, results reflect.Value, rel *schema.RelationshipMetadata) error {
	// Get target table metadata using TargetType (accurate) or fallback to TargetTable (legacy)
	var targetTable *schema.TableMetadata
	var err error

	if rel.TargetType != nil {
		targetTable, err = registry.Get(rel.TargetType)
	} else {
		targetTable, err = registry.GetByName(rel.TargetTable)
	}

	if err != nil {
		return fmt.Errorf("target table %s not registered: %w", rel.TargetTable, err)
	}

	// Collect primary key values from all results
	primaryKeys := make([]interface{}, 0, results.Len())
	pkMap := make(map[interface{}]int) // Map PK value to result index

	for i := 0; i < results.Len(); i++ {
		item := results.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		// Get the primary key field value
		pkField := item.FieldByName(toPascalCase(rel.References))
		if !pkField.IsValid() {
			continue
		}

		pkValue := pkField.Interface()

		// Dereference pointer if needed
		if pkField.Kind() == reflect.Ptr && !pkField.IsNil() {
			pkValue = pkField.Elem().Interface()
		}

		primaryKeys = append(primaryKeys, pkValue)
		pkMap[pkValue] = i

		// Initialize the slice field
		relationField := item.FieldByName(rel.SourceField)
		if relationField.IsValid() && relationField.CanSet() {
			if relationField.IsNil() {
				relationField.Set(reflect.MakeSlice(relationField.Type(), 0, 0))
			}
		}
	}

	if len(primaryKeys) == 0 {
		return nil
	}

	// Query related records using IN clause
	sql := fmt.Sprintf("SELECT * FROM %s WHERE %s = ANY($1)", targetTable.Name, rel.ForeignKey)
	rows, err := q.db.db.Query(ctx, sql, primaryKeys)
	if err != nil {
		return fmt.Errorf("failed to query related records: %w", err)
	}
	defer rows.Close()

	// Scan related records and append to results
	for rows.Next() {
		related := reflect.New(targetTable.GoType)
		if err := scanIntoStruct(rows, related.Interface(), targetTable); err != nil {
			return fmt.Errorf("failed to scan related record: %w", err)
		}

		// Get the foreign key value from the related record
		relatedElem := related.Elem()
		fkField := relatedElem.FieldByName(toPascalCase(rel.ForeignKey))
		if !fkField.IsValid() {
			continue
		}
		fkValue := fkField.Interface()

		// Find the parent result
		idx, exists := pkMap[fkValue]
		if !exists {
			continue
		}

		item := results.Index(idx)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		relationField := item.FieldByName(rel.SourceField)
		if !relationField.IsValid() || !relationField.CanSet() {
			continue
		}

		// Append to the slice
		if relationField.Kind() == reflect.Slice {
			var elemToAppend reflect.Value
			if relationField.Type().Elem().Kind() == reflect.Ptr {
				elemToAppend = related
			} else {
				elemToAppend = related.Elem()
			}
			relationField.Set(reflect.Append(relationField, elemToAppend))
		}
	}

	return rows.Err()
}

// loadManyToMany loads manyToMany relationships through a junction table.
// Example: User manyToMany Roles (users_roles junction table)
func (q *SelectQuery[T]) loadManyToMany(ctx context.Context, results reflect.Value, rel *schema.RelationshipMetadata) error {
	if rel.JoinTable == nil {
		return fmt.Errorf("manyToMany relationship requires a junction table")
	}

	// Get target table metadata using TargetType (accurate) or fallback to TargetTable (legacy)
	var targetTable *schema.TableMetadata
	var err error

	if rel.TargetType != nil {
		targetTable, err = registry.Get(rel.TargetType)
	} else {
		targetTable, err = registry.GetByName(rel.TargetTable)
	}

	if err != nil {
		return fmt.Errorf("target table %s not registered: %w", rel.TargetTable, err)
	}

	// Collect primary key values from all results
	primaryKeys := make([]interface{}, 0, results.Len())
	pkMap := make(map[interface{}]int) // Map PK value to result index

	for i := 0; i < results.Len(); i++ {
		item := results.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		// Get the primary key field value
		pkField := item.FieldByName(toPascalCase(rel.References))
		if !pkField.IsValid() {
			continue
		}

		pkValue := pkField.Interface()

		// Dereference pointer if needed
		if pkField.Kind() == reflect.Ptr && !pkField.IsNil() {
			pkValue = pkField.Elem().Interface()
		}

		primaryKeys = append(primaryKeys, pkValue)
		pkMap[pkValue] = i

		// Initialize the slice field
		relationField := item.FieldByName(rel.SourceField)
		if relationField.IsValid() && relationField.CanSet() {
			if relationField.IsNil() {
				relationField.Set(reflect.MakeSlice(relationField.Type(), 0, 0))
			}
		}
	}

	if len(primaryKeys) == 0 {
		return nil
	}

	// Generate foreign key column names for junction table
	// Convention: source_table_id and target_table_id
	sourceFKCol := toSnakeCase(q.table.GoType.Name()) + "_id"
	targetFKCol := toSnakeCase(targetTable.GoType.Name()) + "_id"

	// Query through junction table with JOIN
	sql := fmt.Sprintf(
		"SELECT t.* FROM %s t INNER JOIN %s j ON t.%s = j.%s WHERE j.%s = ANY($1)",
		targetTable.Name,
		*rel.JoinTable,
		rel.References,
		targetFKCol,
		sourceFKCol,
	)

	rows, err := q.db.db.Query(ctx, sql, primaryKeys)
	if err != nil {
		return fmt.Errorf("failed to query related records: %w", err)
	}
	defer rows.Close()

	// We need to query the junction table to get the associations
	junctionSQL := fmt.Sprintf(
		"SELECT %s, %s FROM %s WHERE %s = ANY($1)",
		sourceFKCol,
		targetFKCol,
		*rel.JoinTable,
		sourceFKCol,
	)

	junctionRows, err := q.db.db.Query(ctx, junctionSQL, primaryKeys)
	if err != nil {
		return fmt.Errorf("failed to query junction table: %w", err)
	}
	defer junctionRows.Close()

	// Build a map of source PK -> target PKs
	junctionMap := make(map[interface{}][]interface{})
	for junctionRows.Next() {
		var sourcePK, targetPK interface{}
		if err := junctionRows.Scan(&sourcePK, &targetPK); err != nil {
			return fmt.Errorf("failed to scan junction row: %w", err)
		}
		junctionMap[sourcePK] = append(junctionMap[sourcePK], targetPK)
	}

	if err := junctionRows.Err(); err != nil {
		return err
	}

	// Build a map of target records by their PK
	targetMap := make(map[interface{}]reflect.Value)
	for rows.Next() {
		related := reflect.New(targetTable.GoType)
		if err := scanIntoStruct(rows, related.Interface(), targetTable); err != nil {
			return fmt.Errorf("failed to scan related record: %w", err)
		}

		// Get the ID value from the related record
		relatedElem := related.Elem()
		idField := relatedElem.FieldByName(toPascalCase(rel.References))
		if !idField.IsValid() {
			continue
		}
		targetMap[idField.Interface()] = related
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Assign related records to results based on junction table
	for sourcePK, targetPKs := range junctionMap {
		idx, exists := pkMap[sourcePK]
		if !exists {
			continue
		}

		item := results.Index(idx)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		relationField := item.FieldByName(rel.SourceField)
		if !relationField.IsValid() || !relationField.CanSet() {
			continue
		}

		// Append all related records
		for _, targetPK := range targetPKs {
			related, exists := targetMap[targetPK]
			if !exists {
				continue
			}

			if relationField.Kind() == reflect.Slice {
				var elemToAppend reflect.Value
				if relationField.Type().Elem().Kind() == reflect.Ptr {
					elemToAppend = related
				} else {
					elemToAppend = related.Elem()
				}
				relationField.Set(reflect.Append(relationField, elemToAppend))
			}
		}
	}

	return nil
}

// Helper functions

// commonInitialisms contains Go initialisms that should be all uppercase.
// See: https://github.com/golang/lint/blob/master/lint.go
var commonInitialisms = map[string]bool{
	"ACL":   true,
	"API":   true,
	"ASCII": true,
	"CPU":   true,
	"CSS":   true,
	"DNS":   true,
	"EOF":   true,
	"GUID":  true,
	"HTML":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"JSON":  true,
	"LHS":   true,
	"QPS":   true,
	"RAM":   true,
	"RHS":   true,
	"RPC":   true,
	"SLA":   true,
	"SMTP":  true,
	"SQL":   true,
	"SSH":   true,
	"TCP":   true,
	"TLS":   true,
	"TTL":   true,
	"UDP":   true,
	"UI":    true,
	"UID":   true,
	"UUID":  true,
	"URI":   true,
	"URL":   true,
	"UTF8":  true,
	"VM":    true,
	"XML":   true,
	"XMPP":  true,
	"XSRF":  true,
	"XSS":   true,
}

// toPascalCase converts snake_case to PascalCase for field names.
// Handles Go initialisms properly (e.g., "user_id" -> "UserID", not "UserId").
func toPascalCase(s string) string {
	if s == "" {
		return ""
	}

	// Split by underscore
	parts := make([]string, 0)
	currentPart := make([]rune, 0)

	for _, ch := range s {
		if ch == '_' {
			if len(currentPart) > 0 {
				parts = append(parts, string(currentPart))
				currentPart = make([]rune, 0)
			}
		} else {
			currentPart = append(currentPart, ch)
		}
	}
	if len(currentPart) > 0 {
		parts = append(parts, string(currentPart))
	}

	// Capitalize each part, checking for initialisms
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}

		// Check if this part is a common initialism (case-insensitive)
		upperPart := ""
		for _, ch := range part {
			upperPart += string(toUpper(ch))
		}

		if commonInitialisms[upperPart] {
			result = append(result, upperPart)
		} else {
			// Capitalize first letter, keep rest as-is
			capitalized := string(toUpper(rune(part[0]))) + part[1:]
			result = append(result, capitalized)
		}
	}

	// Join all parts
	final := ""
	for _, part := range result {
		final += part
	}

	return final
}

// toUpper converts a character to uppercase.
func toUpper(ch rune) rune {
	if ch >= 'a' && ch <= 'z' {
		return ch - 32
	}
	return ch
}

// toSnakeCase converts PascalCase to snake_case.
func toSnakeCase(s string) string {
	var result []rune
	for i, ch := range s {
		if i > 0 && ch >= 'A' && ch <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, toLower(ch))
	}
	return string(result)
}

// toLower converts a character to lowercase.
func toLower(ch rune) rune {
	if ch >= 'A' && ch <= 'Z' {
		return ch + 32
	}
	return ch
}

// isZeroValue checks if a value is the zero value for its type.
func isZeroValue(v interface{}) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface:
		return val.IsNil()
	case reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return val.IsNil()
	default:
		return val.IsZero()
	}
}
