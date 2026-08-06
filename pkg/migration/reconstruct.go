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
	reCreateTableName  = regexp.MustCompile(`(?i)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?"?(\w+)"?`)
	reDropTableName    = regexp.MustCompile(`(?i)^\s*DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?"?(\w+)"?`)
	reAlterTableParts  = regexp.MustCompile(`(?i)^\s*ALTER\s+TABLE\s+"?(\w+)"?\s+(.+)`)
	reAlterColType     = regexp.MustCompile(`(?i)^ALTER\s+COLUMN\s+"?(\w+)"?\s+TYPE\s+(.+)$`)
	reUniqueConstraint = regexp.MustCompile(`(?i)^(?:ADD\s+)?CONSTRAINT\s+(\w+)\s+UNIQUE\s*\(([^)]+)\)$`)
	reFKConstraint     = regexp.MustCompile(`(?i)CONSTRAINT\s+(\w+)\s+FOREIGN\s+KEY\s*\(([^)]+)\)\s+REFERENCES\s+"?(\w+)"?\s*\(([^)]+)\)(?:\s+ON\s+DELETE\s+([\w\s]+?))?$`)
	reAddFKConstraint  = regexp.MustCompile(`(?i)^ADD\s+CONSTRAINT\s+(\w+)\s+FOREIGN\s+KEY\s*\(([^)]+)\)\s+REFERENCES\s+"?(\w+)"?\s*\(([^)]+)\)(?:\s+ON\s+DELETE\s+([\w\s]+?))?$`)
	reExclusion        = regexp.MustCompile(`(?i)^(?:ADD\s+)?CONSTRAINT\s+(\w+)\s+EXCLUDE\s+(.+)$`)
	reCheck            = regexp.MustCompile(`(?i)^(?:ADD\s+)?CONSTRAINT\s+(\w+)\s+CHECK\s+(.+)$`)
	reCreateExtension  = regexp.MustCompile(`(?i)^\s*CREATE\s+EXTENSION\s+(?:IF\s+NOT\s+EXISTS\s+)?"?([\w-]+)"?`)
	reDropExtension    = regexp.MustCompile(`(?i)^\s*DROP\s+EXTENSION\s+(?:IF\s+EXISTS\s+)?"?([\w-]+)"?`)
	reCreateDomain     = regexp.MustCompile(`(?i)^\s*CREATE\s+DOMAIN\s+"?(\w+)"?\s+AS\s+(.+?)\s*;?\s*$`)
	reDropDomain       = regexp.MustCompile(`(?i)^\s*DROP\s+DOMAIN\s+(?:IF\s+EXISTS\s+)?"?(\w+)"?`)

	reAlterColSetDefault  = regexp.MustCompile(`(?i)^ALTER\s+COLUMN\s+"?(\w+)"?\s+SET\s+DEFAULT\s+(.+)$`)
	reAlterColDropDefault = regexp.MustCompile(`(?i)^ALTER\s+COLUMN\s+"?(\w+)"?\s+DROP\s+DEFAULT`)
	reCreateIndex         = regexp.MustCompile(`(?i)^\s*CREATE\s+(UNIQUE\s+)?INDEX\s+(?:CONCURRENTLY\s+)?(?:IF\s+NOT\s+EXISTS\s+)?"?(\w+)"?\s+ON\s+"?(\w+)"?\s+(.+?)\s*;?\s*$`)
	reDropIndex           = regexp.MustCompile(`(?i)^\s*DROP\s+INDEX\s+(?:CONCURRENTLY\s+)?(?:IF\s+EXISTS\s+)?"?(\w+)"?`)
	reIndexUsing          = regexp.MustCompile(`(?i)^USING\s+(\w+)\s+`)
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
	extensions := make(map[string]bool)
	domains := make(map[string]schema.DomainType)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", filepath.Base(f), err)
		}
		applySQLToSchema(tables, extensions, domains, string(data))
	}

	// Extensions and domains are schema-global facts; attach them to any one
	// table so the differ's union sees them (mirrors the live introspector).
	if len(extensions) > 0 || len(domains) > 0 {
		names := make([]string, 0, len(extensions))
		for name := range extensions {
			names = append(names, name)
		}
		sort.Strings(names)
		doms := make([]schema.DomainType, 0, len(domains))
		for _, d := range domains {
			doms = append(doms, d)
		}
		sort.Slice(doms, func(a, b int) bool { return doms[a].Name < doms[b].Name })
		for _, table := range tables {
			table.Extensions = names
			table.Domains = doms
			break
		}
	}

	return tables, nil
}

// applySQLToSchema applies DDL statements from sql to the in-memory schema map.
func applySQLToSchema(tables map[string]*schema.TableMetadata, extensions map[string]bool, domains map[string]schema.DomainType, sql string) {
	for _, stmt := range splitSQLStatements(sql) {
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
		case reCreateDomain.MatchString(stmt):
			m := reCreateDomain.FindStringSubmatch(stmt)
			rest := strings.TrimSpace(m[2])
			base, check := rest, ""
			if loc := regexp.MustCompile(`(?i)\bCHECK\b`).FindStringIndex(rest); loc != nil {
				base = strings.TrimSpace(rest[:loc[0]])
				check = strings.TrimSpace(rest[loc[0]:])
			}
			domains[m[1]] = schema.DomainType{Name: m[1], BaseType: base, Check: check}
		case reDropDomain.MatchString(stmt):
			delete(domains, reDropDomain.FindStringSubmatch(stmt)[1])
		case reCreateExtension.MatchString(stmt):
			extensions[reCreateExtension.FindStringSubmatch(stmt)[1]] = true
		case reDropExtension.MatchString(stmt):
			delete(extensions, reDropExtension.FindStringSubmatch(stmt)[1])
		case reCreateIndex.MatchString(stmt):
			applyCreateIndex(tables, stmt)
		case reDropIndex.MatchString(stmt):
			applyDropIndex(tables, reDropIndex.FindStringSubmatch(stmt)[1])
		}
	}
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

	cols, pkCols, fks, exclusions := parseColumnList(stmt[open+1 : close])

	table := &schema.TableMetadata{
		Name:        tableName,
		Columns:     cols,
		ForeignKeys: fks,
		Constraints: exclusions,
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

// applyCreateIndex reconstructs an index from a CREATE INDEX statement and
// attaches it to its table. Column/ordering/WHERE parsing is delegated to the
// canonical schema.ParseIndexFromComment by reformatting the statement into the
// comment grammar it understands (which expects USING after the column list).
func applyCreateIndex(tables map[string]*schema.TableMetadata, stmt string) {
	m := reCreateIndex.FindStringSubmatch(stmt)
	if m == nil {
		return
	}
	unique := strings.TrimSpace(m[1]) != ""
	name := m[2]
	table, ok := tables[strings.ToLower(m[3])]
	if !ok {
		return
	}
	tail := strings.TrimSpace(m[4])

	// Pull a leading "USING <type>" out of the way — the comment grammar wants it
	// after the column list, before any WHERE.
	usingType := ""
	if um := reIndexUsing.FindStringSubmatch(tail); um != nil {
		usingType = um[1]
		tail = strings.TrimSpace(tail[len(um[0]):])
	}
	closeIdx := matchingParen(tail)
	if closeIdx < 0 {
		return
	}
	synthetic := "index: " + name + " ON " + tail[:closeIdx+1]
	if usingType != "" {
		synthetic += " USING " + usingType
	}
	synthetic += " " + strings.TrimSpace(tail[closeIdx+1:])

	idx := schema.ParseIndexFromComment(synthetic)
	if idx == nil {
		return
	}
	idx.Unique = unique
	table.Indexes = append(table.Indexes, *idx)
}

// applyDropIndex removes an index by name from whichever table holds it.
func applyDropIndex(tables map[string]*schema.TableMetadata, name string) {
	name = strings.ToLower(name)
	for _, table := range tables {
		for i, idx := range table.Indexes {
			if strings.ToLower(idx.Name) == name {
				table.Indexes = append(table.Indexes[:i], table.Indexes[i+1:]...)
				return
			}
		}
	}
}

// matchingParen returns the index of the ')' that closes the first '(' in s, or
// -1 if s does not start with '(' or is unbalanced.
func matchingParen(s string) int {
	if len(s) == 0 || s[0] != '(' {
		return -1
	}
	depth := 0
	for i, r := range s {
		switch r {
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
			table.Columns = removeColumn(table.Columns, strings.ToLower(strings.Trim(parts[idx], `"`)))
		}

	case strings.HasPrefix(upper, "ALTER COLUMN"):
		if am := reAlterColType.FindStringSubmatch(rest); am != nil {
			colName := strings.ToLower(am[1])
			newType := strings.TrimSpace(am[2])
			for i, col := range table.Columns {
				if col.Name == colName {
					table.Columns[i].SQLType = newType
					break
				}
			}
		} else if dm := reAlterColSetDefault.FindStringSubmatch(rest); dm != nil {
			colName := strings.ToLower(dm[1])
			defVal := extractDefaultValue(strings.TrimSpace(dm[2]))
			for i, col := range table.Columns {
				if col.Name == colName {
					table.Columns[i].Default = &defVal
					break
				}
			}
		} else if dm := reAlterColDropDefault.FindStringSubmatch(rest); dm != nil {
			colName := strings.ToLower(dm[1])
			for i, col := range table.Columns {
				if col.Name == colName {
					table.Columns[i].Default = nil
					break
				}
			}
		}

	case strings.HasPrefix(upper, "ADD CONSTRAINT"):
		if strings.Contains(upper, "EXCLUDE") {
			if ex := reExclusion.FindStringSubmatch(rest); ex != nil {
				table.Constraints = append(table.Constraints, schema.ConstraintMetadata{
					Name:       ex[1],
					Type:       schema.ExclusionConstraint,
					Expression: strings.TrimSpace(ex[2]),
				})
			}
		} else if strings.Contains(upper, "CHECK") {
			if ck := reCheck.FindStringSubmatch(rest); ck != nil {
				table.Constraints = append(table.Constraints, schema.ConstraintMetadata{
					Name:       ck[1],
					Type:       schema.CheckConstraint,
					Expression: strings.TrimSpace(ck[2]),
				})
			}
		} else if strings.Contains(upper, "FOREIGN KEY") {
			// ADD CONSTRAINT name FOREIGN KEY (cols) REFERENCES table (cols) [ON DELETE action]
			fkm := reAddFKConstraint.FindStringSubmatch(rest)
			if fkm != nil {
				fk := schema.ForeignKeyMetadata{
					Name:              fkm[1],
					Columns:           splitCSV(fkm[2]),
					ReferencedTable:   strings.ToLower(strings.TrimSpace(fkm[3])),
					ReferencedColumns: splitCSV(fkm[4]),
					OnDelete:          reconstructParseReferenceAction(strings.TrimSpace(fkm[5])),
				}
				table.ForeignKeys = append(table.ForeignKeys, fk)
			}
		} else if cm := reUniqueConstraint.FindStringSubmatch(rest); cm != nil {
			cols := splitCSV(cm[2])
			// A single-column UNIQUE also flips the column's Unique flag so the
			// differ's column-level detection stays consistent.
			if len(cols) == 1 {
				for i, col := range table.Columns {
					if col.Name == cols[0] {
						table.Columns[i].Unique = true
						break
					}
				}
			}
			table.Constraints = append(table.Constraints, schema.ConstraintMetadata{
				Name:    cm[1],
				Type:    schema.UniqueConstraint,
				Columns: cols,
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
// returns the primary key column names and foreign keys detected from column-level
// PRIMARY KEY attributes or table-level constraints.
func parseColumnList(colList string) ([]schema.ColumnMetadata, []string, []schema.ForeignKeyMetadata, []schema.ConstraintMetadata) {
	var cols []schema.ColumnMetadata
	var pkCols []string
	var fks []schema.ForeignKeyMetadata
	var exclusions []schema.ConstraintMetadata
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
		// Table-level CONSTRAINT: check for FOREIGN KEY or UNIQUE.
		if strings.HasPrefix(upper, "CONSTRAINT") {
			if strings.Contains(upper, "FOREIGN KEY") {
				if fk := parseReconstructFKConstraint(part); fk != nil {
					fks = append(fks, *fk)
				}
			} else if ex := reExclusion.FindStringSubmatch(part); ex != nil {
				exclusions = append(exclusions, schema.ConstraintMetadata{
					Name:       ex[1],
					Type:       schema.ExclusionConstraint,
					Expression: strings.TrimSpace(ex[2]),
				})
			} else if ck := reCheck.FindStringSubmatch(part); ck != nil {
				exclusions = append(exclusions, schema.ConstraintMetadata{
					Name:       ck[1],
					Type:       schema.CheckConstraint,
					Expression: strings.TrimSpace(ck[2]),
				})
			} else if uq := reUniqueConstraint.FindStringSubmatch(part); uq != nil {
				exclusions = append(exclusions, schema.ConstraintMetadata{
					Name:    uq[1],
					Type:    schema.UniqueConstraint,
					Columns: splitCSV(uq[2]),
				})
			}
			continue
		}
		// Bare FOREIGN KEY (no CONSTRAINT prefix) — skip.
		if strings.HasPrefix(upper, "FOREIGN KEY") ||
			strings.HasPrefix(upper, "UNIQUE") ||
			strings.HasPrefix(upper, "CHECK") {
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
	return cols, pkCols, fks, exclusions
}

// parseReconstructFKConstraint parses a CONSTRAINT ... FOREIGN KEY ... REFERENCES ... line.
func parseReconstructFKConstraint(s string) *schema.ForeignKeyMetadata {
	m := reFKConstraint.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	return &schema.ForeignKeyMetadata{
		Name:              m[1],
		Columns:           splitCSV(m[2]),
		ReferencedTable:   strings.ToLower(strings.TrimSpace(m[3])),
		ReferencedColumns: splitCSV(m[4]),
		OnDelete:          reconstructParseReferenceAction(strings.TrimSpace(m[5])),
	}
}

// splitCSV trims and splits a comma-separated identifier list.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, strings.ToLower(strings.Trim(p, `"`)))
		}
	}
	return out
}

// reconstructParseReferenceAction converts a string to schema.ReferenceAction.
func reconstructParseReferenceAction(action string) schema.ReferenceAction {
	switch strings.ToUpper(action) {
	case "CASCADE":
		return schema.Cascade
	case "RESTRICT":
		return schema.Restrict
	case "SET NULL", "SETNULL":
		return schema.SetNull
	case "SET DEFAULT", "SETDEFAULT":
		return schema.SetDefault
	default:
		return schema.NoAction
	}
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

	name, rest, ok := strings.Cut(def, " ")
	if !ok {
		col.Name = strings.ToLower(strings.Trim(def, `"`))
		return col
	}
	col.Name = strings.ToLower(strings.Trim(name, `"`))
	rest = strings.TrimSpace(rest)
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
				switch s[i] {
				case '(':
					depth++
				case ')':
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
