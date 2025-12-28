package schema

import (
	"strings"
	"testing"
)

func TestValidateDefaultValue(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid CURRENT_TIMESTAMP",
			value:     "CURRENT_TIMESTAMP",
			wantError: false,
		},
		{
			name:      "valid NOW()",
			value:     "NOW()",
			wantError: false,
		},
		{
			name:      "valid gen_random_uuid()",
			value:     "gen_random_uuid()",
			wantError: false,
		},
		{
			name:      "valid number",
			value:     "0",
			wantError: false,
		},
		{
			name:      "valid boolean",
			value:     "true",
			wantError: false,
		},
		{
			name:      "valid string literal",
			value:     "'default value'",
			wantError: false,
		},
		{
			name:      "INVALID CURRENT TIMESTAMP with space",
			value:     "CURRENT TIMESTAMP",
			wantError: true,
			errorMsg:  "CURRENT_TIMESTAMP",
		},
		{
			name:      "INVALID CURRENT TIME with space",
			value:     "CURRENT TIME",
			wantError: true,
			errorMsg:  "CURRENT_TIME",
		},
		{
			name:      "INVALID NOW with space",
			value:     "NOW ()",
			wantError: true,
			errorMsg:  "NOW()",
		},
		{
			name:      "case insensitive detection",
			value:     "current timestamp",
			wantError: true,
			errorMsg:  "CURRENT_TIMESTAMP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDefaultValue(tt.value)

			if tt.wantError && err == nil {
				t.Errorf("Expected error for value '%s', got nil", tt.value)
			}

			if !tt.wantError && err != nil {
				t.Errorf("Expected no error for value '%s', got: %v", tt.value, err)
			}

			if tt.wantError && err != nil && tt.errorMsg != "" {
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to mention '%s', got: %v", tt.errorMsg, err)
				}
			}
		})
	}
}
