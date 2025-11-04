package inducer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDSLConverter_Convert(t *testing.T) {
	converter := NewDSLConverter()

	tests := []struct {
		name         string
		regexPattern string
		wantTemplate string
		wantPayloads map[string][]string
		wantErr      bool
	}{
		{
			name:         "simple alternation",
			regexPattern: "(api|web|cdn)",
			wantTemplate: "{{p0}}.{{suffix}}",
			wantPayloads: map[string][]string{
				"p0": {"api", "cdn", "web"}, // sorted
			},
			wantErr: false,
		},
		{
			name:         "two alternations with dash",
			regexPattern: "(api|web|cdn)-(dev|prod|staging)",
			wantTemplate: "{{p0}}-{{p1}}.{{suffix}}",
			wantPayloads: map[string][]string{
				"p0": {"api", "cdn", "web"},
				"p1": {"dev", "prod", "staging"},
			},
			wantErr: false,
		},
		{
			name:         "escaped dot separator",
			regexPattern: "(api|web)\\.staging",
			wantTemplate: "{{p0}}.staging.{{suffix}}",
			wantPayloads: map[string][]string{
				"p0": {"api", "web"},
			},
			wantErr: false,
		},
		{
			name:         "multiple levels with dots",
			regexPattern: "(api|web)\\.(dev|prod)\\.(internal|external)",
			wantTemplate: "{{p0}}.{{p1}}.{{p2}}.{{suffix}}",
			wantPayloads: map[string][]string{
				"p0": {"api", "web"},
				"p1": {"dev", "prod"},
				"p2": {"external", "internal"},
			},
			wantErr: false,
		},
		{
			name:         "numeric alternation",
			regexPattern: "api(01|02|03)",
			wantTemplate: "api{{p0}}.{{suffix}}",
			wantPayloads: map[string][]string{
				"p0": {"01", "02", "03"},
			},
			wantErr: false,
		},
		{
			name:         "mixed literals and alternations",
			regexPattern: "prefix-(api|web)-suffix",
			wantTemplate: "prefix-{{p0}}-suffix.{{suffix}}",
			wantPayloads: map[string][]string{
				"p0": {"api", "web"},
			},
			wantErr: false,
		},
		{
			name:         "escaped dash in alternation",
			regexPattern: "(api\\-v1|web\\-v2)",
			wantTemplate: "{{p0}}.{{suffix}}",
			wantPayloads: map[string][]string{
				"p0": {"api-v1", "web-v2"},
			},
			wantErr: false,
		},
		{
			name:         "single value (no alternation)",
			regexPattern: "api-dev",
			wantTemplate: "api-dev.{{suffix}}",
			wantPayloads: map[string][]string{},
			wantErr:      false,
		},
		{
			name:         "optional group",
			regexPattern: "api(\\.staging)?",
			wantTemplate: "api{{p0}}.{{suffix}}", // The ? is removed - optional is implicit in DSL
			wantPayloads: map[string][]string{
				"p0": {".staging"},
			},
			wantErr: false,
		},
		{
			name:         "complex pattern with multiple types",
			regexPattern: "(api|web|cdn)-(dev|prod)-(us|eu)(01|02|03)",
			wantTemplate: "{{p0}}-{{p1}}-{{p2}}{{p3}}.{{suffix}}",
			wantPayloads: map[string][]string{
				"p0": {"api", "cdn", "web"},
				"p1": {"dev", "prod"},
				"p2": {"eu", "us"},
				"p3": {"01", "02", "03"},
			},
			wantErr: false,
		},
		{
			name:         "empty pattern",
			regexPattern: "",
			wantTemplate: "",
			wantPayloads: map[string][]string{},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.Convert(tt.regexPattern)

			if tt.wantErr {
				assert.Error(t, result.Error)
				return
			}

			require.NoError(t, result.Error)
			assert.Equal(t, tt.wantTemplate, result.Template)
			assert.Equal(t, tt.wantPayloads, result.Payloads)
		})
	}
}

func TestDSLConverter_UnescapeRegex(t *testing.T) {
	converter := NewDSLConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "escaped dot",
			input:    `api\.staging`,
			expected: `api.staging`,
		},
		{
			name:     "escaped dash",
			input:    `api\-dev`,
			expected: `api-dev`,
		},
		{
			name:     "multiple escapes",
			input:    `api\.staging\-dev`,
			expected: `api.staging-dev`,
		},
		{
			name:     "escaped parentheses",
			input:    `\(api\)`,
			expected: `(api)`,
		},
		{
			name:     "escaped brackets",
			input:    `\[0\-9\]`,
			expected: `[0-9]`,
		},
		{
			name:     "no escapes",
			input:    `api-dev`,
			expected: `api-dev`,
		},
		{
			name:     "backslash",
			input:    `api\\dev`,
			expected: `api\dev`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.unescapeRegex(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDSLConverter_EnsureSuffix(t *testing.T) {
	converter := NewDSLConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no suffix",
			input:    "api-dev",
			expected: "api-dev.{{suffix}}",
		},
		{
			name:     "already has suffix",
			input:    "api-dev.{{suffix}}",
			expected: "api-dev.{{suffix}}",
		},
		{
			name:     "ends with dot",
			input:    "api-dev.",
			expected: "api-dev.{{suffix}}",
		},
		{
			name:     "empty string",
			input:    "",
			expected: ".{{suffix}}",
		},
		{
			name:     "just variable",
			input:    "{{p0}}",
			expected: "{{p0}}.{{suffix}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.ensureSuffix(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDSLConverter_ValidateTemplate(t *testing.T) {
	converter := NewDSLConverter()

	tests := []struct {
		name     string
		template string
		payloads map[string][]string
		wantErr  bool
	}{
		{
			name:     "valid template with payloads",
			template: "{{p0}}-{{p1}}.{{suffix}}",
			payloads: map[string][]string{
				"p0": {"api", "web"},
				"p1": {"dev", "prod"},
			},
			wantErr: false,
		},
		{
			name:     "missing payload",
			template: "{{p0}}-{{p1}}.{{suffix}}",
			payloads: map[string][]string{
				"p0": {"api", "web"},
				// p1 is missing
			},
			wantErr: true,
		},
		{
			name:     "empty payload",
			template: "{{p0}}.{{suffix}}",
			payloads: map[string][]string{
				"p0": {}, // empty
			},
			wantErr: true,
		},
		{
			name:     "builtin variables are valid",
			template: "{{sub}}-{{p0}}.{{suffix}}",
			payloads: map[string][]string{
				"p0": {"api", "web"},
			},
			wantErr: false,
		},
		{
			name:     "all builtin variables",
			template: "{{sub}}.{{suffix}}",
			payloads: map[string][]string{},
			wantErr:  false,
		},
		{
			name:     "subN variables are valid",
			template: "{{sub1}}-{{sub2}}.{{suffix}}",
			payloads: map[string][]string{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := converter.ValidateTemplate(tt.template, tt.payloads)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDSLConverter_ConvertPattern(t *testing.T) {
	converter := NewDSLConverter()

	pattern := &Pattern{
		Regex:      "(api|web)-(dev|prod)",
		Coverage:   100,
		Ratio:      1.5,
		Confidence: 0.75,
		Domains:    []string{"api-dev.example.com", "web-prod.example.com"},
	}

	converted, err := converter.ConvertPattern(pattern)
	require.NoError(t, err)
	require.NotNil(t, converted)

	// Verify original fields are preserved
	assert.Equal(t, pattern.Regex, converted.Regex)
	assert.Equal(t, pattern.Coverage, converted.Coverage)
	assert.Equal(t, pattern.Ratio, converted.Ratio)
	assert.Equal(t, pattern.Confidence, converted.Confidence)
	assert.Equal(t, pattern.Domains, converted.Domains)
}

func TestDSLConverter_ExtractPayloadsFromRegex(t *testing.T) {
	converter := NewDSLConverter()

	tests := []struct {
		name         string
		regexPattern string
		want         map[string][]string
		wantErr      bool
	}{
		{
			name:         "single alternation",
			regexPattern: "(api|web|cdn)",
			want: map[string][]string{
				"p0": {"api", "cdn", "web"},
			},
			wantErr: false,
		},
		{
			name:         "multiple alternations",
			regexPattern: "(api|web)-(dev|prod)",
			want: map[string][]string{
				"p0": {"api", "web"},
				"p1": {"dev", "prod"},
			},
			wantErr: false,
		},
		{
			name:         "no alternations",
			regexPattern: "api-dev",
			want:         map[string][]string{},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloads, err := converter.ExtractPayloadsFromRegex(tt.regexPattern)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, payloads)
		})
	}
}

func TestDSLConverter_FormatPayloadsForYAML(t *testing.T) {
	converter := NewDSLConverter()

	payloads := map[string][]string{
		"p0": {"api", "web", "cdn"},
		"p1": {"dev", "prod"},
	}

	result := converter.FormatPayloadsForYAML(payloads)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "p0")
	assert.Contains(t, result, "p1")

	// Check structure
	p0Map, ok := result["p0"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, []string{"api", "web", "cdn"}, p0Map["values"])

	p1Map, ok := result["p1"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, []string{"dev", "prod"}, p1Map["values"])
}

func TestDSLConverter_RealWorldPatterns(t *testing.T) {
	converter := NewDSLConverter()

	tests := []struct {
		name         string
		regexPattern string
		wantTemplate string
	}{
		{
			name:         "AWS environment pattern",
			regexPattern: "(api|web|cdn)-(dev|staging|prod)-(us\\-east\\-1|us\\-west\\-2)",
			wantTemplate: "{{p0}}-{{p1}}-{{p2}}.{{suffix}}",
		},
		{
			name:         "versioned API pattern",
			regexPattern: "(api|gateway)\\-(v1|v2|v3)",
			wantTemplate: "{{p0}}-{{p1}}.{{suffix}}",
		},
		{
			name:         "numbered instances",
			regexPattern: "(web|app)(01|02|03|04|05)",
			wantTemplate: "{{p0}}{{p1}}.{{suffix}}",
		},
		{
			name:         "multi-level subdomain",
			regexPattern: "(api|web)\\.(internal|external)\\.(dev|prod)",
			wantTemplate: "{{p0}}.{{p1}}.{{p2}}.{{suffix}}",
		},
		{
			name:         "kubernetes style",
			regexPattern: "(frontend|backend)\\-(deployment|service)\\-(blue|green)",
			wantTemplate: "{{p0}}-{{p1}}-{{p2}}.{{suffix}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.Convert(tt.regexPattern)
			require.NoError(t, result.Error)
			assert.Equal(t, tt.wantTemplate, result.Template)
			assert.NotEmpty(t, result.Payloads)
		})
	}
}
