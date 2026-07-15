package migration

import "strings"

// splitSQLStatements splits a SQL script into individual statements on
// semicolons, while respecting single-quoted string literals and dollar-quoted
// bodies ($$ ... $$ or $tag$ ... $tag$) so a semicolon inside a string or a
// PL/pgSQL function body does not split a statement mid-way. Line comments
// (-- ...) outside of strings are stripped. Empty statements are omitted.
func splitSQLStatements(sql string) []string {
	var stmts []string
	var cur strings.Builder

	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			stmts = append(stmts, s)
		}
		cur.Reset()
	}

	inSingle := false // inside a '...' string literal
	dollarTag := ""   // non-empty while inside a $tag$ ... $tag$ body

	for i := 0; i < len(sql); {
		if dollarTag != "" {
			if strings.HasPrefix(sql[i:], dollarTag) {
				cur.WriteString(dollarTag)
				i += len(dollarTag)
				dollarTag = ""
				continue
			}
			cur.WriteByte(sql[i])
			i++
			continue
		}

		if inSingle {
			cur.WriteByte(sql[i])
			if sql[i] == '\'' {
				inSingle = false
			}
			i++
			continue
		}

		ch := sql[i]
		switch {
		case ch == '-' && i+1 < len(sql) && sql[i+1] == '-':
			// Line comment: skip to end of line.
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
		case ch == '$':
			if tag := matchDollarTag(sql[i:]); tag != "" {
				dollarTag = tag
				cur.WriteString(tag)
				i += len(tag)
			} else {
				cur.WriteByte(ch)
				i++
			}
		case ch == '\'':
			inSingle = true
			cur.WriteByte(ch)
			i++
		case ch == ';':
			flush()
			i++
		default:
			cur.WriteByte(ch)
			i++
		}
	}
	flush()
	return stmts
}

// matchDollarTag returns the dollar-quote opening tag at the start of s
// ("$$" or "$name$"), or "" if s does not begin with one. Tag names follow
// PostgreSQL identifier rules (letter or underscore first), which keeps
// positional parameters like $1 from being mistaken for a tag.
func matchDollarTag(s string) string {
	if len(s) < 2 || s[0] != '$' {
		return ""
	}
	if s[1] == '$' {
		return "$$"
	}
	if !isTagStart(s[1]) {
		return ""
	}
	for j := 2; j < len(s); j++ {
		if s[j] == '$' {
			return s[:j+1]
		}
		if !isTagChar(s[j]) {
			return ""
		}
	}
	return ""
}

func isTagStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isTagChar(b byte) bool {
	return isTagStart(b) || (b >= '0' && b <= '9')
}
