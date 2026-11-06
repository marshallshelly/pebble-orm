package builder

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/marshallshelly/pebble-orm/pkg/registry"
	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// relationshipLoader loads preloaded relationships for a result set.
type relationshipLoader struct {
	exec     queryExecutor
	table    *schema.TableMetadata
	preloads []string
}

// preloadTree maps a relationship field to the preloads nested under it, so a
// field named both on its own and as a parent ("Posts", "Posts.Comments")
// loads once.
type preloadTree map[string]preloadTree

func buildPreloadTree(paths []string) preloadTree {
	root := preloadTree{}
	for _, path := range paths {
		node := root
		for _, field := range strings.Split(path, ".") {
			if node[field] == nil {
				node[field] = preloadTree{}
			}
			node = node[field]
		}
	}
	return root
}

// preloadLevel is a set of objects of one table and the preloads to load on
// them. objects is a slice of structs or of struct pointers.
type preloadLevel struct {
	objects reflect.Value
	table   *schema.TableMetadata
	tree    preloadTree
}

// loadRelationships loads the preloads one depth at a time. Every query for a
// depth is sent in one pgx batch, so sibling preloads share a round trip; the
// next depth then loads on the objects this one produced.
func (q *relationshipLoader) loadRelationships(ctx context.Context, results interface{}) error {
	if len(q.preloads) == 0 {
		return nil
	}

	resultsVal := reflect.ValueOf(results)
	if resultsVal.Kind() != reflect.Pointer || resultsVal.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("results must be a pointer to slice")
	}
	resultsVal = resultsVal.Elem()
	if resultsVal.Len() == 0 {
		return nil
	}

	type loaded struct {
		objects reflect.Value
		rel     *schema.RelationshipMetadata
		target  *schema.TableMetadata
		tree    preloadTree
	}

	levels := []preloadLevel{{resultsVal, q.table, buildPreloadTree(q.preloads)}}
	for len(levels) > 0 {
		batch := &pgx.Batch{}
		var finish []func()
		var children []loaded
		for _, level := range levels {
			for _, field := range slices.Sorted(maps.Keys(level.tree)) {
				rel := level.table.GetRelationship(field)
				if rel == nil {
					return fmt.Errorf("relationship %s not found on %s", field, level.table.Name)
				}
				target, err := relationshipTarget(rel)
				if err != nil {
					return err
				}
				done, err := queueRelationship(batch, level.objects, rel, level.table, target)
				if err != nil {
					return fmt.Errorf("failed to load relationship %s: %w", field, err)
				}
				if done != nil {
					finish = append(finish, done)
				}
				if len(level.tree[field]) > 0 {
					children = append(children, loaded{level.objects, rel, target, level.tree[field]})
				}
			}
		}

		if batch.Len() > 0 {
			if err := q.exec.SendBatch(ctx, batch).Close(); err != nil {
				return err
			}
		}
		for _, done := range finish {
			done()
		}

		levels = levels[:0:0]
		for _, c := range children {
			if objects := relatedObjects(c.objects, c.rel, c.target); objects.Len() > 0 {
				levels = append(levels, preloadLevel{objects, c.target, c.tree})
			}
		}
	}
	return nil
}

// relationshipTarget resolves the table a relationship points at, by Go type
// when known and by table name otherwise.
func relationshipTarget(rel *schema.RelationshipMetadata) (*schema.TableMetadata, error) {
	var target *schema.TableMetadata
	var err error
	if rel.TargetType != nil {
		target, err = registry.Get(rel.TargetType)
	} else {
		target, err = registry.GetByName(rel.TargetTable)
	}
	if err != nil {
		return nil, fmt.Errorf("target table %s not registered: %w", rel.TargetTable, err)
	}
	return target, nil
}

// queueRelationship adds the queries that load rel onto objects to batch. The
// returned func, when not nil, must run after the batch has been read.
func queueRelationship(batch *pgx.Batch, objects reflect.Value, rel *schema.RelationshipMetadata, source, target *schema.TableMetadata) (func(), error) {
	switch rel.Type {
	case schema.BelongsTo:
		queueBelongsTo(batch, objects, rel, target)
	case schema.HasOne:
		queueHasOne(batch, objects, rel, target)
	case schema.HasMany:
		queueHasMany(batch, objects, rel, target)
	case schema.ManyToMany:
		return queueManyToMany(batch, objects, rel, source, target)
	default:
		return nil, fmt.Errorf("unsupported relationship type: %s", rel.Type)
	}
	return nil, nil
}

// relatedObjects collects pointers to the objects a loaded relationship put on
// objects, so the next depth's assignments reach the real values. A value
// element of a slice ([]Post) is addressed rather than copied, and an object
// several parents point at is collected once.
func relatedObjects(objects reflect.Value, rel *schema.RelationshipMetadata, target *schema.TableMetadata) reflect.Value {
	collected := reflect.MakeSlice(reflect.SliceOf(reflect.PointerTo(target.GoType)), 0, objects.Len())
	seen := make(map[uintptr]bool)
	add := func(ptr reflect.Value) {
		if !seen[ptr.Pointer()] {
			seen[ptr.Pointer()] = true
			collected = reflect.Append(collected, ptr)
		}
	}
	for i := 0; i < objects.Len(); i++ {
		field := structAt(objects, i).FieldByName(rel.SourceField)
		if !field.IsValid() {
			continue
		}
		switch {
		case field.Kind() == reflect.Slice:
			for j := 0; j < field.Len(); j++ {
				elem := field.Index(j)
				if elem.Kind() != reflect.Pointer {
					elem = elem.Addr()
				}
				add(elem)
			}
		case field.Kind() == reflect.Pointer:
			if !field.IsNil() {
				add(field)
			}
		case !field.IsZero():
			add(field.Addr())
		}
	}
	return collected
}

// structAt returns the struct at index i, dereferencing a pointer element.
func structAt(objects reflect.Value, i int) reflect.Value {
	item := objects.Index(i)
	if item.Kind() == reflect.Pointer {
		item = item.Elem()
	}
	return item
}

// keyValue returns a field's value as a map key and query argument, with a
// non-nil pointer dereferenced.
func keyValue(field reflect.Value) interface{} {
	if field.Kind() == reflect.Pointer && !field.IsNil() {
		return field.Elem().Interface()
	}
	return field.Interface()
}

// setRelated stores related (a pointer to a target struct) in field, as the
// pointer or the value depending on the field's type.
func setRelated(field reflect.Value, related reflect.Value) {
	if field.Kind() == reflect.Pointer {
		field.Set(related)
	} else {
		field.Set(related.Elem())
	}
}

// appendRelated appends related to a slice field of values or pointers.
func appendRelated(field reflect.Value, related reflect.Value) {
	if field.Type().Elem().Kind() == reflect.Pointer {
		field.Set(reflect.Append(field, related))
	} else {
		field.Set(reflect.Append(field, related.Elem()))
	}
}

// queueRows queues sql and scans each result row into a new target struct,
// handing it to assign.
func queueRows(batch *pgx.Batch, rel *schema.RelationshipMetadata, target *schema.TableMetadata, sql string, keys []interface{}, assign func(related reflect.Value)) {
	scanner := newRowScanner(target)
	batch.Queue(sql, convertToTypedSlice(keys)).Query(func(rows pgx.Rows) error {
		for rows.Next() {
			related := reflect.New(target.GoType)
			if err := scanner.scan(rows, related.Interface()); err != nil {
				return fmt.Errorf("failed to load relationship %s: failed to scan related record: %w", rel.SourceField, err)
			}
			assign(related)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed to load relationship %s: %w", rel.SourceField, err)
		}
		return nil
	})
}

// queueBelongsTo loads a belongsTo relationship.
// Example: Post belongsTo User (post.user_id -> users.id)
func queueBelongsTo(batch *pgx.Batch, objects reflect.Value, rel *schema.RelationshipMetadata, target *schema.TableMetadata) {
	var keys []interface{}
	byKey := make(map[interface{}][]int)
	for i := 0; i < objects.Len(); i++ {
		fk := structAt(objects, i).FieldByName(toPascalCase(rel.ForeignKey))
		if !fk.IsValid() || isZeroValue(fk.Interface()) {
			continue
		}
		key := keyValue(fk)
		if _, ok := byKey[key]; !ok {
			keys = append(keys, key)
		}
		byKey[key] = append(byKey[key], i)
	}
	if len(keys) == 0 {
		return
	}

	sql := fmt.Sprintf("SELECT * FROM %s WHERE %s = ANY($1)", schema.QuoteReservedIdent(target.Name), schema.QuoteReservedIdent(rel.References))
	queueRows(batch, rel, target, sql, keys, func(related reflect.Value) {
		id := related.Elem().FieldByName(toPascalCase(rel.References))
		if !id.IsValid() {
			return
		}
		for _, i := range byKey[id.Interface()] {
			if field := structAt(objects, i).FieldByName(rel.SourceField); field.IsValid() && field.CanSet() {
				setRelated(field, related)
			}
		}
	})
}

// parentKeys collects the distinct referenced keys of objects, for hasOne,
// hasMany and manyToMany, and maps each key to every object that has it: rows
// below a shared belongsTo hold separate copies of the same parent.
func parentKeys(objects reflect.Value, rel *schema.RelationshipMetadata) ([]interface{}, map[interface{}][]int) {
	keys := make([]interface{}, 0, objects.Len())
	byKey := make(map[interface{}][]int)
	for i := 0; i < objects.Len(); i++ {
		pk := structAt(objects, i).FieldByName(toPascalCase(rel.References))
		if !pk.IsValid() {
			continue
		}
		key := keyValue(pk)
		if _, ok := byKey[key]; !ok {
			keys = append(keys, key)
		}
		byKey[key] = append(byKey[key], i)
	}
	return keys, byKey
}

// queueHasOne loads a hasOne relationship.
// Example: User hasOne Profile (profiles.user_id -> users.id)
func queueHasOne(batch *pgx.Batch, objects reflect.Value, rel *schema.RelationshipMetadata, target *schema.TableMetadata) {
	keys, byKey := parentKeys(objects, rel)
	if len(keys) == 0 {
		return
	}

	sql := fmt.Sprintf("SELECT * FROM %s WHERE %s = ANY($1)", schema.QuoteReservedIdent(target.Name), schema.QuoteReservedIdent(rel.ForeignKey))
	queueRows(batch, rel, target, sql, keys, func(related reflect.Value) {
		fk := related.Elem().FieldByName(toPascalCase(rel.ForeignKey))
		if !fk.IsValid() {
			return
		}
		for _, i := range byKey[fk.Interface()] {
			if field := structAt(objects, i).FieldByName(rel.SourceField); field.IsValid() && field.CanSet() {
				setRelated(field, related)
			}
		}
	})
}

// initSlices gives every object an empty slice for rel, so a parent with no
// related rows reads as loaded rather than nil.
func initSlices(objects reflect.Value, rel *schema.RelationshipMetadata) {
	for i := 0; i < objects.Len(); i++ {
		if field := structAt(objects, i).FieldByName(rel.SourceField); field.IsValid() && field.CanSet() && field.IsNil() {
			field.Set(reflect.MakeSlice(field.Type(), 0, 0))
		}
	}
}

// queueHasMany loads a hasMany relationship.
// Example: User hasMany Posts (posts.user_id -> users.id)
func queueHasMany(batch *pgx.Batch, objects reflect.Value, rel *schema.RelationshipMetadata, target *schema.TableMetadata) {
	keys, byKey := parentKeys(objects, rel)
	initSlices(objects, rel)
	if len(keys) == 0 {
		return
	}

	sql := fmt.Sprintf("SELECT * FROM %s WHERE %s = ANY($1)", schema.QuoteReservedIdent(target.Name), schema.QuoteReservedIdent(rel.ForeignKey))
	queueRows(batch, rel, target, sql, keys, func(related reflect.Value) {
		fk := related.Elem().FieldByName(toPascalCase(rel.ForeignKey))
		if !fk.IsValid() {
			return
		}
		for _, i := range byKey[fk.Interface()] {
			if field := structAt(objects, i).FieldByName(rel.SourceField); field.IsValid() && field.CanSet() && field.Kind() == reflect.Slice {
				appendRelated(field, related)
			}
		}
	})
}

// queueManyToMany loads a manyToMany relationship through its junction table.
// Example: User manyToMany Roles (users_roles junction table)
// The junction rows and the target rows are separate queries in the same
// batch; the returned func pairs them up once both have been read.
func queueManyToMany(batch *pgx.Batch, objects reflect.Value, rel *schema.RelationshipMetadata, source, target *schema.TableMetadata) (func(), error) {
	if rel.JoinTable == nil {
		return nil, fmt.Errorf("manyToMany relationship requires a junction table")
	}

	keys, byKey := parentKeys(objects, rel)
	initSlices(objects, rel)
	if len(keys) == 0 {
		return nil, nil
	}

	// Convention: source_table_id and target_table_id
	sourceFKCol := toSnakeCase(source.GoType.Name()) + "_id"
	targetFKCol := toSnakeCase(target.GoType.Name()) + "_id"

	// Junction keys scan into the struct fields' own types: pgx decodes into
	// int32, [16]byte and so on otherwise, which never equal the map keys.
	refField := toPascalCase(rel.References)
	sourceKeyType, err := pkFieldType(source.GoType, refField)
	if err != nil {
		return nil, fmt.Errorf("source %w", err)
	}
	targetKeyType, err := pkFieldType(target.GoType, refField)
	if err != nil {
		return nil, fmt.Errorf("target %w", err)
	}

	junctionSQL := fmt.Sprintf(
		"SELECT %s, %s FROM %s WHERE %s = ANY($1)",
		schema.QuoteReservedIdent(sourceFKCol),
		schema.QuoteReservedIdent(targetFKCol),
		schema.QuoteReservedIdent(*rel.JoinTable),
		schema.QuoteReservedIdent(sourceFKCol),
	)
	var junction [][2]interface{}
	batch.Queue(junctionSQL, convertToTypedSlice(keys)).Query(func(rows pgx.Rows) error {
		for rows.Next() {
			sourcePtr := reflect.New(sourceKeyType)
			targetPtr := reflect.New(targetKeyType)
			if err := rows.Scan(sourcePtr.Interface(), targetPtr.Interface()); err != nil {
				return fmt.Errorf("failed to load relationship %s: failed to scan junction row: %w", rel.SourceField, err)
			}
			junction = append(junction, [2]interface{}{sourcePtr.Elem().Interface(), targetPtr.Elem().Interface()})
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed to load relationship %s: %w", rel.SourceField, err)
		}
		return nil
	})

	sql := fmt.Sprintf(
		"SELECT t.* FROM %s t INNER JOIN %s j ON t.%s = j.%s WHERE j.%s = ANY($1)",
		schema.QuoteReservedIdent(target.Name),
		schema.QuoteReservedIdent(*rel.JoinTable),
		schema.QuoteReservedIdent(rel.References),
		schema.QuoteReservedIdent(targetFKCol),
		schema.QuoteReservedIdent(sourceFKCol),
	)
	targets := make(map[interface{}]reflect.Value)
	queueRows(batch, rel, target, sql, keys, func(related reflect.Value) {
		if id := related.Elem().FieldByName(refField); id.IsValid() {
			targets[id.Interface()] = related
		}
	})

	return func() {
		for _, pair := range junction {
			related, found := targets[pair[1]]
			if !found {
				continue
			}
			for _, i := range byKey[pair[0]] {
				if field := structAt(objects, i).FieldByName(rel.SourceField); field.IsValid() && field.CanSet() && field.Kind() == reflect.Slice {
					appendRelated(field, related)
				}
			}
		}
	}, nil
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
		var upperPart strings.Builder
		for _, ch := range part {
			upperPart.WriteString(string(toUpper(ch)))
		}

		if commonInitialisms[upperPart.String()] {
			result = append(result, upperPart.String())
		} else {
			// Capitalize first letter, keep rest as-is
			capitalized := string(toUpper(rune(part[0]))) + part[1:]
			result = append(result, capitalized)
		}
	}

	// Join all parts
	var final strings.Builder
	for _, part := range result {
		final.WriteString(part)
	}

	return final.String()
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
	case reflect.Pointer, reflect.Interface:
		return val.IsNil()
	case reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return val.IsNil()
	default:
		return val.IsZero()
	}
}

// pkFieldType returns the type of the named struct field, dereferenced if it is
// a pointer, so junction-table values can be scanned into the same type the
// struct uses for that primary key.
func pkFieldType(structType reflect.Type, fieldName string) (reflect.Type, error) {
	if structType == nil {
		return nil, fmt.Errorf("primary key field %s: struct type not available", fieldName)
	}
	f, ok := structType.FieldByName(fieldName)
	if !ok {
		return nil, fmt.Errorf("primary key field %s not found on %s", fieldName, structType.Name())
	}
	t := f.Type
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t, nil
}

// convertToTypedSlice converts []interface{} to a properly typed slice for pgx encoding.
// As of pgx v5.10 a homogeneous []interface{} does encode for ANY($1); a MIXED
// one does not ("unable to encode []interface {}{...} into binary format").
// Deriving one element type up front keeps the slice homogeneous by
// construction, so this stays rather than relying on the caller's data.
func convertToTypedSlice(values []interface{}) interface{} {
	if len(values) == 0 {
		return values
	}

	// Determine type from first element
	firstVal := reflect.ValueOf(values[0])
	elemType := firstVal.Type()

	// Create a slice of the proper type
	result := reflect.MakeSlice(reflect.SliceOf(elemType), len(values), len(values))

	// Copy all values
	for i, v := range values {
		result.Index(i).Set(reflect.ValueOf(v))
	}

	return result.Interface()
}
