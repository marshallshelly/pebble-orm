package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

var (
	reCreateTableName = regexp.MustCompile(`(?i)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?"?(\w+)"?`)
	reDropTableName   = regexp.MustCompile(`(?i)^\s*DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?"?(\w+)"?`)
	reAlterTableParts = regexp.MustCompile(`(?i)^\s*ALTER\s+TABLE\s+(\w+)\s+(.+)`)
	reAlterColType    = regexp.MustCompile(`(?i)^ALTER\s+COLUMN\s+(\w+)\s+TYPE\s+(.+)$`)
	reAddConstraint   = regexp.MustCompile(`(?i)^ADD\s+CONSTRAINT\s+\w+\s+UNIQUE\s*\((\w+)\)$`)
)

// HasMigrationFiles reports whether any *.up.sql files exist in dir.
func HasMigrationFiles(dir string) (bool, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	return len(matches) > 0, err
}

// ReconstructSchemaFromMigrations replays all *.up.sql migration files in
// version order to reconstruct the current schema state. It is used as the
// offline baseline when no database connection is provided.
func ReconstructSchemaFromMigrations(dir string) (map[string]*schema.TableMetadata, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}
	sort.Strings(files) // timestamps make lexicographic order == chronological order

	tables := make(map[string]*schema.TableMetadata)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", filepath.Base(f), err)
		}
		applySQLToSchema(tables, string(data))
	}
	return tables, nil
}

// applySQLToSchema applies DDL statements from sql to the in-memory schema map.
func applySQLToSchema(tables map[string]*schema.TableMetadata, sql string) {
	for _, stmt := range splitStatements(sql) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		upper := strings.ToUpper(stmt)
		switch {
		case reCreateTableName.MatchString(stmt):
			applyCreateTable(tables, stmt)
		case strings.HasPrefix(upper, "DROP TABLE"):
			applyDropTable(tables, stmt)
		case strings.HasPrefix(upper, "ALTER TABLE"):
			applyAlterTable(tables, stmt)
		}
	}
}

// splitStatements splits SQL text into individual statements on semicolons,
// skipping DO $$ ... END $$ dollar-quoted blocks.
func splitStatements(sql string) []string {
	var stmts []string
	var cur strings.Builder
	inDollar := false

	for _, line := range strings.Split(sql, "\n") {
		trimmed := strings.TrimSpace(line)
		// Toggle dollar-quote state on odd number of $$ in a line.
		if cnt := strings.Count(trimmed, "$$"); cnt%2 != 0 {
			inDollar = !inDollar
		}
		cur.WriteString(line)
		cur.WriteByte('\n')
		if !inDollar && strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSuffix(strings.TrimSpace(cur.String()), ";")
			stmts = append(stmts, stmt)
			cur.Reset()
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}

func applyCreateTable(tables map[string]*schema.TableMetadata, stmt string) {
	m := reCreateTableName.FindStringSubmatch(stmt)
	if m == nil {
		return
	}
	tableName := strings.ToLower(m[1])

	open := strings.Index(stmt, "(")
	if open < 0 {
		return
	}
	close := findMatchingCloseParen(stmt, open)
	if close < 0 {
		return
	}

	cols, pkCols := parseColumnList(tableName, stmt[open+1:close])

	table := &schema.TableMetadata{
		Name:        tableName,
		Columns:     cols,
		Constraints: make([]schema.ConstraintMetadata, 0),
	}
	if len(pkCols) > 0 {
		table.PrimaryKey = &schema.PrimaryKeyMetadata{
			Columns: pkCols,
			Name:    tableName + "_pkey",
		}
	}
	// Unique constraints from column-level UNIQUE attribute.
	for _, col := range cols {
		if col.Unique {
			table.Constraints = append(table.Constraints, schema.ConstraintMetadata{
				Type:    schema.UniqueConstraint,
				Columns: []string{col.Name},
				Name:    tableName + "_" + col.Name + "_key",
			})
		}
	}

	tables[tableName] = table
}

func applyDropTable(tables map[string]*schema.TableMetadata, stmt string) {
	m := reDropTableName.FindStringSubmatch(stmt)
	if m != nil {
		delete(tables, strings.ToLower(m[1]))
	}
}

func applyAlterTable(tables map[string]*schema.TableMetadata, stmt string) {
	m := reAlterTableParts.FindStringSubmatch(stmt)
	if m == nil {
		return
	}
	tableName := strings.ToLower(m[1])
	rest := strings.TrimSpace(m[2])
	upper := strings.ToUpper(rest)

	table, ok := tables[tableName]
	if !ok {
		return
	}

	switch {
	case strings.HasPrefix(upper, "ADD COLUMN"):
		colDef := strings.TrimSpace(rest[len("ADD COLUMN"):])
		col := parseColDef(colDef, len(table.Columns))
		if col.Name != "" {
			table.Columns = append(table.Columns, col)
		}

	case strings.HasPrefix(upper, "DROP COLUMN"):
		// "DROP COLUMN [IF EXISTS] colname"
		parts := strings.Fields(rest) // ["DROP","COLUMN",...,"colname"]
		if len(parts) < 3 {
			return
		}
		idx := 2
		if strings.ToUpper(parts[idx]) == "IF" {
			idx = 4 // skip "IF EXISTS"
		}
		if idx < len(parts) {
			table.Columns = removeColumn(table.Columns, strings.ToLower(parts[idx]))
		}

	case strings.HasPrefix(upper, "ALTER COLUMN"):
		am := reAlterColType.FindStringSubmatch(rest)
		if am != nil {
			colName := strings.ToLower(am[1])
			newType := strings.TrimSpace(am[2])
			for i, col := range table.Columns {
				if col.Name == colName {
					table.Columns[i].SQLType = newType
					break
				}
			}
		}

	case strings.HasPrefix(upper, "ADD CONSTRAINT"):
		cm := reAddConstraint.FindStringSubmatch(rest)
		if cm != nil {
			colName := strings.ToLower(cm[1])
			for i, col := range table.Columns {
				if col.Name == colName {
					table.Columns[i].Unique = true
					break
				}
			}
			// Also add to the Constraints slice so the differ sees it.
			constraintName := tableName + "_" + colName + "_key"
			table.Constraints = append(table.Constraints, schema.ConstraintMetadata{
				Type:    schema.UniqueConstraint,
				Columns: []string{colName},
				Name:    constraintName,
			})
		}
	}
}

// removeColumn returns cols without the named column.
func removeColumn(cols []schema.ColumnMetadata, name string) []schema.ColumnMetadata {
	out := make([]schema.ColumnMetadata, 0, len(cols))
	for _, c := range cols {
		if c.Name != name {
			out = append(out, c)
		}
	}
	return out
}

// findMatchingCloseParen returns the index of the ')' that matches the '(' at pos.
func findMatchingCloseParen(s string, pos int) int {
	depth := 0
	for i := pos; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseColumnList splits a CREATE TABLE column list into ColumnMetadata and
// returns the primary key column names detected from column-level PRIMARY KEY
// attributes or table-level PRIMARY KEY constraints.
func parseColumnList(tableName, colList string) ([]schema.ColumnMetadata, []string) {
	var cols []schema.ColumnMetadata
	var pkCols []string
	pos := 0

	for _, part := range splitTopLevelCommas(colList) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		upper := strings.ToUpper(part)

		// Table-level PRIMARY KEY constraint: PRIMARY KEY (col, ...)
		if strings.HasPrefix(upper, "PRIMARY KEY") {
			pkCols = extractParenList(part)
			continue
		}
		// Skip other table-level constraints.
		if strings.HasPrefix(upper, "FOREIGN KEY") ||
			strings.HasPrefix(upper, "UNIQUE") ||
			strings.HasPrefix(upper, "CHECK") ||
			strings.HasPrefix(upper, "CONSTRAINT") {
			continue
		}

		col := parseColDef(part, pos)
		if col.Name == "" {
			continue
		}
		// Column-level PRIMARY KEY attribute.
		if strings.Contains(upper, "PRIMARY KEY") {
			pkCols = []string{col.Name}
			col.Unique = false // PK does not need a separate UNIQUE constraint
		}
		cols = append(cols, col)
		pos++
	}
	return cols, pkCols
}

// extractParenList extracts a comma-separated list from the first parenthesised
// group in s, e.g. "PRIMARY KEY (id, name)" → ["id", "name"].
func extractParenList(s string) []string {
	open := strings.Index(s, "(")
	if open < 0 {
		return nil
	}
	close := findMatchingCloseParen(s, open)
	if close < 0 {
		return nil
	}
	inner := s[open+1 : close]
	var items []string
	for _, item := range strings.Split(inner, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, strings.ToLower(item))
		}
	}
	return items
}

// splitTopLevelCommas splits s by commas that are not inside parentheses.
func splitTopLevelCommas(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i, ch := range s {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// parseColDef parses a single column definition, e.g.:
//
//	"id serial NOT NULL PRIMARY KEY"
//	"email varchar(320) NOT NULL UNIQUE"
//	"created_at timestamptz NOT NULL DEFAULT NOW()"
//	"phone varchar(20)"
func parseColDef(def string, position int) schema.ColumnMetadata {
	def = strings.TrimSpace(def)
	col := schema.ColumnMetadata{Position: position}

	spaceIdx := strings.IndexByte(def, ' ')
	if spaceIdx < 0 {
		col.Name = strings.ToLower(def)
		return col
	}
	col.Name = strings.ToLower(def[:spaceIdx])
	rest := strings.TrimSpace(def[spaceIdx+1:])
	restUpper := strings.ToUpper(rest)

	// SQL type: everything up to the first space that is outside parentheses.
	typeEnd := typeTokenEnd(rest)
	col.SQLType = rest[:typeEnd]
	remainder := strings.TrimSpace(rest[typeEnd:])
	remainderUpper := strings.ToUpper(remainder)

	col.Nullable = !strings.Contains(restUpper, "NOT NULL")
	col.Unique = strings.Contains(restUpper, " UNIQUE") || strings.HasSuffix(restUpper, "UNIQUE")

	typeLower := strings.ToLower(col.SQLType)
	if typeLower == "serial" || typeLower == "bigserial" || typeLower == "smallserial" {
		col.AutoIncrement = true
		col.Nullable = false
	}

	if idx := strings.Index(remainderUpper, "DEFAULT"); idx >= 0 {
		afterDefault := strings.TrimSpace(remainder[idx+7:])
		defVal := extractDefaultValue(afterDefault)
		col.Default = &defVal
	}

	return col
}

// typeTokenEnd returns the index in s where the type token ends.
// Handles types with parentheses like varchar(255) or numeric(10,2).
func typeTokenEnd(s string) int {
	i := 0
	for i < len(s) {
		switch s[i] {
		case '(':
			depth := 0
			for i < len(s) {
				if s[i] == '(' {
					depth++
				} else if s[i] == ')' {
					depth--
					if depth == 0 {
						i++
						goto nextChar
					}
				}
				i++
			}
		case ' ', '\t', '\n':
			return i
		default:
			i++
		}
	nextChar:
	}
	return i
}

// extractDefaultValue extracts the default expression, stopping at trailing
// SQL keywords like NOT, NULL, UNIQUE, PRIMARY, CHECK, REFERENCES.
func extractDefaultValue(s string) string {
	stops := []string{" NOT ", " NOT\t", " NULL", " UNIQUE", " PRIMARY", " CHECK", " REFERENCES"}
	sUpper := strings.ToUpper(s)
	end := len(s)
	for _, kw := range stops {
		if idx := strings.Index(sUpper, kw); idx >= 0 && idx < end {
			end = idx
		}
	}
	return strings.TrimSpace(s[:end])
}
