package schema

import (
	"reflect"
	"testing"
)

func TestStringArray_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected StringArray
		wantErr  bool
	}{
		{
			name:     "simple array",
			input:    "{Monday,Tuesday,Wednesday}",
			expected: StringArray{"Monday", "Tuesday", "Wednesday"},
		},
		{
			name:     "simple array as bytes",
			input:    []byte("{Monday,Tuesday,Wednesday}"),
			expected: StringArray{"Monday", "Tuesday", "Wednesday"},
		},
		{
			name:     "empty array",
			input:    "{}",
			expected: StringArray{},
		},
		{
			name:     "single element",
			input:    "{hello}",
			expected: StringArray{"hello"},
		},
		{
			name:     "quoted elements with spaces",
			input:    `{"hello world","foo bar"}`,
			expected: StringArray{"hello world", "foo bar"},
		},
		{
			name:     "quoted elements with commas",
			input:    `{"a,b","c,d"}`,
			expected: StringArray{"a,b", "c,d"},
		},
		{
			name:     "escaped quotes",
			input:    `{"he said \"hi\"",another}`,
			expected: StringArray{`he said "hi"`, "another"},
		},
		{
			name:     "escaped backslash",
			input:    `{"path\\to\\file",normal}`,
			expected: StringArray{`path\to\file`, "normal"},
		},
		{
			name:     "NULL values",
			input:    "{a,NULL,b}",
			expected: StringArray{"a", "", "b"},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "native slice from pgx",
			input:    []string{"a", "b", "c"},
			expected: StringArray{"a", "b", "c"},
		},
		{
			name:     "mixed quoted and unquoted",
			input:    `{simple,"with spaces",another}`,
			expected: StringArray{"simple", "with spaces", "another"},
		},
		{
			name:    "invalid format - no braces",
			input:   "a,b,c",
			wantErr: true,
		},
		{
			name:    "invalid type",
			input:   123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var arr StringArray
			err := arr.Scan(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(arr, tt.expected) {
				t.Errorf("got %v, expected %v", arr, tt.expected)
			}
		})
	}
}

func TestStringArray_Value(t *testing.T) {
	tests := []struct {
		name     string
		input    StringArray
		expected string
	}{
		{
			name:     "simple array",
			input:    StringArray{"a", "b", "c"},
			expected: "{a,b,c}",
		},
		{
			name:     "empty array",
			input:    StringArray{},
			expected: "{}",
		},
		{
			name:     "elements with spaces",
			input:    StringArray{"hello world", "foo"},
			expected: `{"hello world",foo}`,
		},
		{
			name:     "elements with commas",
			input:    StringArray{"a,b", "c"},
			expected: `{"a,b",c}`,
		},
		{
			name:     "elements with quotes",
			input:    StringArray{`say "hi"`, "normal"},
			expected: `{"say \"hi\"",normal}`,
		},
		{
			name:     "elements with backslash",
			input:    StringArray{`path\to\file`, "normal"},
			expected: `{"path\\to\\file",normal}`,
		},
		{
			name:     "empty string element",
			input:    StringArray{"", "nonempty"},
			expected: `{"",nonempty}`,
		},
		{
			name:     "null-like string",
			input:    StringArray{"NULL", "normal"},
			expected: `{"NULL",normal}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.input.Value()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if val != tt.expected {
				t.Errorf("got %v, expected %v", val, tt.expected)
			}
		})
	}
}

func TestStringArray_NilValue(t *testing.T) {
	var arr StringArray = nil
	val, err := arr.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestStringArray_RoundTrip(t *testing.T) {
	tests := []StringArray{
		{"simple", "values"},
		{"with spaces", "and more"},
		{`with "quotes"`, "and", `back\slash`},
		{"a,b", "c,d,e"},
		{},
	}

	for _, original := range tests {
		val, err := original.Value()
		if err != nil {
			t.Errorf("Value() error: %v", err)
			continue
		}

		var scanned StringArray
		err = scanned.Scan(val)
		if err != nil {
			t.Errorf("Scan() error: %v", err)
			continue
		}

		if !reflect.DeepEqual(scanned, original) {
			t.Errorf("round trip failed: got %v, expected %v", scanned, original)
		}
	}
}

func TestInt32Array_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected Int32Array
		wantErr  bool
	}{
		{
			name:     "simple array",
			input:    "{1,2,3}",
			expected: Int32Array{1, 2, 3},
		},
		{
			name:     "negative numbers",
			input:    "{-1,0,100}",
			expected: Int32Array{-1, 0, 100},
		},
		{
			name:     "empty array",
			input:    "{}",
			expected: Int32Array{},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "native slice from pgx",
			input:    []int32{1, 2, 3},
			expected: Int32Array{1, 2, 3},
		},
		{
			name:    "invalid number",
			input:   "{1,abc,3}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var arr Int32Array
			err := arr.Scan(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(arr, tt.expected) {
				t.Errorf("got %v, expected %v", arr, tt.expected)
			}
		})
	}
}

func TestInt32Array_Value(t *testing.T) {
	arr := Int32Array{1, -2, 300}
	val, err := arr.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	expected := "{1,-2,300}"
	if val != expected {
		t.Errorf("got %v, expected %v", val, expected)
	}
}

func TestInt64Array_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected Int64Array
		wantErr  bool
	}{
		{
			name:     "simple array",
			input:    "{1,2,9223372036854775807}",
			expected: Int64Array{1, 2, 9223372036854775807},
		},
		{
			name:     "empty array",
			input:    "{}",
			expected: Int64Array{},
		},
		{
			name:     "native slice from pgx",
			input:    []int64{1, 2, 3},
			expected: Int64Array{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var arr Int64Array
			err := arr.Scan(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(arr, tt.expected) {
				t.Errorf("got %v, expected %v", arr, tt.expected)
			}
		})
	}
}

func TestFloat64Array_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected Float64Array
		wantErr  bool
	}{
		{
			name:     "simple array",
			input:    "{1.5,2.7,3.14159}",
			expected: Float64Array{1.5, 2.7, 3.14159},
		},
		{
			name:     "integers as floats",
			input:    "{1,2,3}",
			expected: Float64Array{1.0, 2.0, 3.0},
		},
		{
			name:     "negative and zero",
			input:    "{-1.5,0,100.001}",
			expected: Float64Array{-1.5, 0, 100.001},
		},
		{
			name:     "native slice from pgx",
			input:    []float64{1.1, 2.2},
			expected: Float64Array{1.1, 2.2},
		},
		{
			name:    "invalid float",
			input:   "{1.5,abc}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var arr Float64Array
			err := arr.Scan(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(arr, tt.expected) {
				t.Errorf("got %v, expected %v", arr, tt.expected)
			}
		})
	}
}

func TestBoolArray_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected BoolArray
		wantErr  bool
	}{
		{
			name:     "t/f format",
			input:    "{t,f,t}",
			expected: BoolArray{true, false, true},
		},
		{
			name:     "true/false format",
			input:    "{true,false,TRUE,FALSE}",
			expected: BoolArray{true, false, true, false},
		},
		{
			name:     "1/0 format",
			input:    "{1,0,1}",
			expected: BoolArray{true, false, true},
		},
		{
			name:     "yes/no format",
			input:    "{yes,no,YES,NO}",
			expected: BoolArray{true, false, true, false},
		},
		{
			name:     "on/off format",
			input:    "{on,off,ON,OFF}",
			expected: BoolArray{true, false, true, false},
		},
		{
			name:     "native slice from pgx",
			input:    []bool{true, false},
			expected: BoolArray{true, false},
		},
		{
			name:    "invalid boolean",
			input:   "{t,maybe,f}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var arr BoolArray
			err := arr.Scan(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(arr, tt.expected) {
				t.Errorf("got %v, expected %v", arr, tt.expected)
			}
		})
	}
}

func TestBoolArray_Value(t *testing.T) {
	arr := BoolArray{true, false, true}
	val, err := arr.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	expected := "{t,f,t}"
	if val != expected {
		t.Errorf("got %v, expected %v", val, expected)
	}
}

func TestParsePostgresArray(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
		wantErr  bool
	}{
		{
			name:     "simple",
			input:    "{a,b,c}",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty",
			input:    "{}",
			expected: []string{},
		},
		{
			name:     "empty string input",
			input:    "",
			expected: nil,
		},
		{
			name:     "whitespace around",
			input:    "  {a,b}  ",
			expected: []string{"a", "b"},
		},
		{
			name:     "quoted with spaces",
			input:    `{"hello world",foo}`,
			expected: []string{"hello world", "foo"},
		},
		{
			name:     "nested quotes",
			input:    `{"say \"hello\"",bar}`,
			expected: []string{`say "hello"`, "bar"},
		},
		{
			name:     "backslash escape",
			input:    `{"path\\file",normal}`,
			expected: []string{`path\file`, "normal"},
		},
		{
			name:    "no opening brace",
			input:   "a,b,c}",
			wantErr: true,
		},
		{
			name:    "no closing brace",
			input:   "{a,b,c",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parsePostgresArray(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("got %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestFormatPostgresArray(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "simple",
			input:    []string{"a", "b", "c"},
			expected: "{a,b,c}",
		},
		{
			name:     "empty",
			input:    []string{},
			expected: "{}",
		},
		{
			name:     "with spaces needs quotes",
			input:    []string{"hello world"},
			expected: `{"hello world"}`,
		},
		{
			name:     "with comma needs quotes",
			input:    []string{"a,b"},
			expected: `{"a,b"}`,
		},
		{
			name:     "empty string needs quotes",
			input:    []string{""},
			expected: `{""}`,
		},
		{
			name:     "null-like needs quotes",
			input:    []string{"null", "NULL"},
			expected: `{"null","NULL"}`,
		},
		{
			name:     "with quotes needs escaping",
			input:    []string{`say "hi"`},
			expected: `{"say \"hi\""}`,
		},
		{
			name:     "with backslash needs escaping",
			input:    []string{`a\b`},
			expected: `{"a\\b"}`,
		},
		{
			name:     "with braces needs quotes",
			input:    []string{"{nested}"},
			expected: `{"{nested}"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPostgresArray(tt.input)
			if result != tt.expected {
				t.Errorf("got %v, expected %v", result, tt.expected)
			}
		})
	}
}
