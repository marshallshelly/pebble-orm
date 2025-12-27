package schema

import (
	"fmt"
	"strings"
)

// ValidateDefaultValue checks if a default value expression is likely valid SQL.
// Returns an error with helpful suggestions if issues are detected.
func ValidateDefaultValue(defaultVal string) error {
	trimmed := strings.TrimSpace(defaultVal)

	// Convert to uppercase for case-insensitive comparison
	upperVal := strings.ToUpper(trimmed)

	// Check mistakes in order (longer patterns first to avoid false matches)
	mistakes := []struct {
		wrong   string
		correct string
	}{
		{"CURRENT TIMESTAMP", "CURRENT_TIMESTAMP"}, // Must check before "CURRENT TIME"
		{"CURRENT TIME", "CURRENT_TIME"},
		{"CURRENT DATE", "CURRENT_DATE"},
		{"NOW ()", "NOW()"},
		{"GEN RANDOM UUID", "gen_random_uuid()"},
		{"UUID GENERATE V4", "uuid_generate_v4()"},
	}

	for _, m := range mistakes {
		if strings.Contains(upperVal, m.wrong) {
			return fmt.Errorf(
				"invalid DEFAULT value: '%s' contains '%s' which should be '%s'\n"+
					"Fix: Change default(%s) to default(%s)",
				defaultVal, m.wrong, m.correct, defaultVal, strings.Replace(defaultVal, strings.ToLower(m.wrong), strings.ToLower(m.correct), 1),
			)
		}
	}

	// Check for function names without parentheses (except SQL keywords)
	sqlKeywords := map[string]bool{
		"NULL": true, "TRUE": true, "FALSE": true,
		"CURRENT_TIMESTAMP": true, "CURRENT_TIME": true, "CURRENT_DATE": true,
		"LOCALTIMESTAMP": true, "LOCALTIME": true,
	}

	// If it looks like a function call but missing parentheses
	if !sqlKeywords[upperVal] && !strings.Contains(trimmed, "(") && !strings.Contains(trimmed, "'") &&
		!isNumeric(trimmed) && len(trimmed) > 3 {
		// Could be a function missing ()
		if strings.Contains(strings.ToLower(trimmed), "random") ||
			strings.Contains(strings.ToLower(trimmed), "generate") ||
			strings.Contains(strings.ToLower(trimmed), "uuid") {
			return fmt.Errorf(
				"invalid DEFAULT value: '%s' looks like a function but is missing parentheses ()\n"+
					"Possible fix: default(%s())",
				defaultVal, defaultVal,
			)
		}
	}

	return nil
}

// isNumeric checks if a string is a valid number
func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, c := range s {
		if i == 0 && (c == '-' || c == '+') {
			continue
		}
		if c == '.' {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
