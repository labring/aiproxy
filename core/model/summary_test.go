package model_test

import (
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/labring/aiproxy/core/model"
)

func TestParseSummaryFields_EmptyInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  model.SummarySelectFields
	}{
		{
			name:  "Empty string returns nil",
			input: "",
			want:  nil,
		},
		{
			name:  "Whitespace only returns nil",
			input: "   ",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.ParseSummaryFields(tt.input)
			if tt.want == nil && got != nil {
				t.Errorf("ParseSummaryFields(%q) = %v, want nil", tt.input, got)
			}
		})
	}
}

func TestParseSummaryFields_SingleFields(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantField     string
		wantFieldOnly bool
	}{
		{
			name:          "Single field request_count",
			input:         "request_count",
			wantField:     "request_count",
			wantFieldOnly: true,
		},
		{
			name:          "Single field exception_count",
			input:         "exception_count",
			wantField:     "exception_count",
			wantFieldOnly: true,
		},
		{
			name:          "Single field cache_hit_count",
			input:         "cache_hit_count",
			wantField:     "cache_hit_count",
			wantFieldOnly: true,
		},
		{
			name:          "Single field total_tokens",
			input:         "total_tokens",
			wantField:     "total_tokens",
			wantFieldOnly: true,
		},
		{
			name:          "Single field used_amount",
			input:         "used_amount",
			wantField:     "used_amount",
			wantFieldOnly: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.ParseSummaryFields(tt.input)
			if got == nil {
				t.Fatalf("ParseSummaryFields(%q) returned nil", tt.input)
			}

			if tt.wantFieldOnly && len(got) != 1 {
				t.Errorf(
					"ParseSummaryFields(%q) returned %d fields, want 1",
					tt.input, len(got),
				)
			}

			if !slices.Contains(got, tt.wantField) {
				t.Errorf(
					"ParseSummaryFields(%q) = %v, want to contain %q",
					tt.input, got, tt.wantField,
				)
			}
		})
	}
}

func TestParseSummaryFields_MultipleFields(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantFields []string
		wantLen    int
	}{
		{
			name:       "Two fields",
			input:      "request_count,exception_count",
			wantFields: []string{"request_count", "exception_count"},
			wantLen:    2,
		},
		{
			name:       "Three fields",
			input:      "request_count,exception_count,cache_hit_count",
			wantFields: []string{"request_count", "exception_count", "cache_hit_count"},
			wantLen:    3,
		},
		{
			name:       "Fields with spaces",
			input:      "request_count, exception_count , cache_hit_count",
			wantFields: []string{"request_count", "exception_count", "cache_hit_count"},
			wantLen:    3,
		},
		{
			name:       "Duplicate fields should be deduplicated",
			input:      "request_count,request_count,exception_count",
			wantFields: []string{"request_count", "exception_count"},
			wantLen:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.ParseSummaryFields(tt.input)
			if got == nil {
				t.Fatalf("ParseSummaryFields(%q) returned nil", tt.input)
			}

			if len(got) != tt.wantLen {
				t.Errorf(
					"ParseSummaryFields(%q) returned %d fields, want %d",
					tt.input, len(got), tt.wantLen,
				)
			}

			for _, wantField := range tt.wantFields {
				if !slices.Contains(got, wantField) {
					t.Errorf(
						"ParseSummaryFields(%q) = %v, want to contain %q",
						tt.input, got, wantField,
					)
				}
			}
		})
	}
}

func TestParseSummaryFields_Groups(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantNil         bool
		wantMinFields   int
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:          "Group 'all' returns nil",
			input:         "all",
			wantNil:       true,
			wantMinFields: 0,
		},
		{
			name:          "Group 'count' returns count fields",
			input:         "count",
			wantNil:       false,
			wantMinFields: 5,
			wantContains:  []string{"request_count", "exception_count", "cache_hit_count"},
			wantNotContains: []string{
				"input_tokens", "output_tokens", "used_amount",
			},
		},
		{
			name:          "Group 'usage' returns usage fields",
			input:         "usage",
			wantNil:       false,
			wantMinFields: 5,
			wantContains:  []string{"input_tokens", "output_tokens", "total_tokens"},
			wantNotContains: []string{
				"request_count", "exception_count",
			},
		},
		{
			name:          "Group 'time' returns time fields",
			input:         "time",
			wantNil:       false,
			wantMinFields: 2,
			wantContains: []string{
				"total_time_milliseconds", "total_ttfb_milliseconds",
			},
			wantNotContains: []string{"request_count", "input_tokens"},
		},
		{
			name:          "Combined group and field",
			input:         "count,used_amount",
			wantNil:       false,
			wantMinFields: 6,
			wantContains: []string{
				"request_count", "exception_count", "used_amount",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.ParseSummaryFields(tt.input)

			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseSummaryFields(%q) = %v, want nil", tt.input, got)
				}

				return
			}

			if got == nil {
				t.Fatalf("ParseSummaryFields(%q) returned nil", tt.input)
			}

			if len(got) < tt.wantMinFields {
				t.Errorf(
					"ParseSummaryFields(%q) returned %d fields, want at least %d",
					tt.input, len(got), tt.wantMinFields,
				)
			}

			for _, wantField := range tt.wantContains {
				if !slices.Contains(got, wantField) {
					t.Errorf(
						"ParseSummaryFields(%q) = %v, want to contain %q",
						tt.input, got, wantField,
					)
				}
			}

			for _, notWantField := range tt.wantNotContains {
				if slices.Contains(got, notWantField) {
					t.Errorf(
						"ParseSummaryFields(%q) = %v, should not contain %q",
						tt.input, got, notWantField,
					)
				}
			}
		})
	}
}

func TestParseSummaryFields_Aliases(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantField    string
		notWantField string
	}{
		{
			name:         "Alias total_time maps to total_time_milliseconds",
			input:        "total_time",
			wantField:    "total_time_milliseconds",
			notWantField: "total_time",
		},
		{
			name:         "Alias total_ttfb maps to total_ttfb_milliseconds",
			input:        "total_ttfb",
			wantField:    "total_ttfb_milliseconds",
			notWantField: "total_ttfb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.ParseSummaryFields(tt.input)
			if got == nil {
				t.Fatalf("ParseSummaryFields(%q) returned nil", tt.input)
			}

			found := slices.Contains(got, tt.wantField)

			if slices.Contains(got, tt.notWantField) {
				t.Errorf(
					"ParseSummaryFields(%q) contains alias %q instead of canonical name",
					tt.input, tt.notWantField,
				)
			}

			if !found {
				t.Errorf(
					"ParseSummaryFields(%q) = %v, want to contain %q",
					tt.input, got, tt.wantField,
				)
			}
		})
	}
}

func TestParseSummaryFields_InvalidFields(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
	}{
		{
			name:    "Single invalid field returns nil",
			input:   "invalid_field",
			wantNil: true,
		},
		{
			name:    "Multiple invalid fields return nil",
			input:   "invalid1,invalid2",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.ParseSummaryFields(tt.input)
			if tt.wantNil && got != nil {
				t.Errorf("ParseSummaryFields(%q) = %v, want nil", tt.input, got)
			}
		})
	}
}

func TestParseSummaryFields_MixedValidInvalid(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantFields []string
		wantLen    int
	}{
		{
			name:       "Mixed valid and invalid fields",
			input:      "request_count,invalid_field,exception_count",
			wantFields: []string{"request_count", "exception_count"},
			wantLen:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.ParseSummaryFields(tt.input)
			if got == nil {
				t.Fatalf("ParseSummaryFields(%q) returned nil", tt.input)
			}

			if len(got) != tt.wantLen {
				t.Errorf(
					"ParseSummaryFields(%q) returned %d fields, want %d",
					tt.input, len(got), tt.wantLen,
				)
			}

			for _, wantField := range tt.wantFields {
				if !slices.Contains(got, wantField) {
					t.Errorf(
						"ParseSummaryFields(%q) = %v, want to contain %q",
						tt.input, got, wantField,
					)
				}
			}
		})
	}
}

func TestSummarySelectFields_IsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		fields model.SummarySelectFields
		want   bool
	}{
		{
			name:   "Nil is empty",
			fields: nil,
			want:   true,
		},
		{
			name:   "Empty slice is empty",
			fields: model.SummarySelectFields{},
			want:   true,
		},
		{
			name:   "Non-empty slice is not empty",
			fields: model.SummarySelectFields{"request_count"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fields.IsEmpty()
			if got != tt.want {
				t.Errorf("SummarySelectFields.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSummarySelectFields_BuildSelectFields(t *testing.T) {
	tests := []struct {
		name           string
		fields         model.SummarySelectFields
		timestampField string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:           "Empty fields selects all",
			fields:         nil,
			timestampField: "hour_timestamp",
			wantContains: []string{
				"hour_timestamp as timestamp",
				"sum(request_count) as request_count",
				"sum(exception_count) as exception_count",
				"sum(input_tokens) as input_tokens",
				"sum(used_amount) as used_amount",
			},
		},
		{
			name:           "Single field",
			fields:         model.SummarySelectFields{"request_count"},
			timestampField: "minute_timestamp",
			wantContains: []string{
				"minute_timestamp as timestamp",
				"sum(request_count) as request_count",
			},
			wantNotContain: []string{
				"sum(exception_count)",
				"sum(input_tokens)",
			},
		},
		{
			name: "Multiple fields",
			fields: model.SummarySelectFields{
				"request_count", "exception_count", "cache_hit_count",
			},
			timestampField: "hour_timestamp",
			wantContains: []string{
				"hour_timestamp as timestamp",
				"sum(request_count) as request_count",
				"sum(exception_count) as exception_count",
				"sum(cache_hit_count) as cache_hit_count",
			},
			wantNotContain: []string{
				"sum(input_tokens)",
				"sum(used_amount)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fields.BuildSelectFields(tt.timestampField)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("BuildSelectFields() = %q, want to contain %q", got, want)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(got, notWant) {
					t.Errorf("BuildSelectFields() = %q, should not contain %q", got, notWant)
				}
			}
		})
	}
}

func TestSummarySelectFields_BuildSelectFieldsV2(t *testing.T) {
	tests := []struct {
		name           string
		fields         model.SummarySelectFields
		timestampField string
		groupFields    string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:           "Empty fields with channel grouping",
			fields:         nil,
			timestampField: "hour_timestamp",
			groupFields:    "channel_id, model",
			wantContains: []string{
				"hour_timestamp as timestamp",
				"channel_id, model",
				"sum(request_count) as request_count",
				"sum(input_tokens) as input_tokens",
			},
		},
		{
			name:           "Empty fields with group grouping",
			fields:         nil,
			timestampField: "minute_timestamp",
			groupFields:    "group_id, token_name, model",
			wantContains: []string{
				"minute_timestamp as timestamp",
				"group_id, token_name, model",
				"sum(request_count) as request_count",
			},
		},
		{
			name:           "Specific fields with grouping",
			fields:         model.SummarySelectFields{"request_count", "total_tokens"},
			timestampField: "hour_timestamp",
			groupFields:    "channel_id, model",
			wantContains: []string{
				"hour_timestamp as timestamp",
				"channel_id, model",
				"sum(request_count) as request_count",
				"sum(total_tokens) as total_tokens",
			},
			wantNotContain: []string{
				"sum(exception_count)",
				"sum(input_tokens)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fields.BuildSelectFieldsV2(tt.timestampField, tt.groupFields)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("BuildSelectFieldsV2() = %q, want to contain %q", got, want)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(got, notWant) {
					t.Errorf("BuildSelectFieldsV2() = %q, should not contain %q", got, notWant)
				}
			}
		})
	}
}

func TestParseSummaryFields_AllValidFields(t *testing.T) {
	// Test all individual valid fields
	validFields := []string{
		"request_count", "retry_count", "exception_count",
		"status4xx_count", "status5xx_count", "status400_count",
		"status429_count", "status500_count", "cache_hit_count",
		"input_tokens", "image_input_tokens", "audio_input_tokens",
		"output_tokens", "image_output_tokens", "cached_tokens",
		"cache_creation_tokens", "total_tokens", "web_search_count",
		"used_amount", "total_time_milliseconds", "total_ttfb_milliseconds",
	}

	for _, field := range validFields {
		t.Run("Valid field: "+field, func(t *testing.T) {
			got := model.ParseSummaryFields(field)
			if got == nil {
				t.Errorf("ParseSummaryFields(%q) returned nil for valid field", field)

				return
			}

			if len(got) != 1 {
				t.Errorf("ParseSummaryFields(%q) returned %d fields, want 1", field, len(got))

				return
			}

			if got[0] != field {
				t.Errorf("ParseSummaryFields(%q) = %v, want [%q]", field, got, field)
			}
		})
	}
}

func TestBuildSelectFields_SQLInjectionPrevention(t *testing.T) {
	// Ensure that field names don't allow SQL injection
	// The ParseSummaryFields function should filter out any invalid field names
	maliciousInputs := []string{
		"request_count; DROP TABLE summaries;--",
		"request_count' OR '1'='1",
		"request_count UNION SELECT * FROM users",
		"1; DELETE FROM summaries WHERE 1=1;",
	}

	for _, input := range maliciousInputs {
		t.Run("SQL injection attempt: "+input[:20], func(t *testing.T) {
			got := model.ParseSummaryFields(input)
			// Should either return nil or only valid field names
			for _, field := range got {
				// Field should be a simple alphanumeric name with underscores only
				for _, c := range field {
					isValid := unicode.IsLower(c) ||
						unicode.IsDigit(c) ||
						c == '_'
					if !isValid {
						t.Errorf("Field contains invalid characters: %q", field)
					}
				}
			}
		})
	}
}

func TestBuildSelectFields_OutputFormat(t *testing.T) {
	tests := []struct {
		name   string
		fields model.SummarySelectFields
		want   string
	}{
		{
			name:   "Single field format",
			fields: model.SummarySelectFields{"request_count"},
			want:   "hour_timestamp as timestamp, sum(request_count) as request_count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fields.BuildSelectFields("hour_timestamp")
			if got != tt.want {
				t.Errorf("BuildSelectFields() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildSelectFields_Deduplication(t *testing.T) {
	tests := []struct {
		name   string
		fields model.SummarySelectFields
		want   string
	}{
		{
			name: "Duplicate fields should be deduplicated",
			fields: model.SummarySelectFields{
				"request_count", "request_count", "exception_count",
			},
			want: "hour_timestamp as timestamp, " +
				"sum(request_count) as request_count, " +
				"sum(exception_count) as exception_count",
		},
		{
			name: "Multiple duplicates",
			fields: model.SummarySelectFields{
				"request_count", "exception_count", "request_count",
				"exception_count", "total_tokens",
			},
			want: "hour_timestamp as timestamp, " +
				"sum(request_count) as request_count, " +
				"sum(exception_count) as exception_count, " +
				"sum(total_tokens) as total_tokens",
		},
		{
			name: "All same field",
			fields: model.SummarySelectFields{
				"request_count", "request_count", "request_count",
			},
			want: "hour_timestamp as timestamp, sum(request_count) as request_count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fields.BuildSelectFields("hour_timestamp")
			if got != tt.want {
				t.Errorf("BuildSelectFields() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildSelectFieldsV2_Deduplication(t *testing.T) {
	tests := []struct {
		name        string
		fields      model.SummarySelectFields
		groupFields string
		want        string
	}{
		{
			name: "Duplicate fields should be deduplicated",
			fields: model.SummarySelectFields{
				"request_count", "request_count", "total_tokens",
			},
			groupFields: "channel_id, model",
			want: "hour_timestamp as timestamp, channel_id, model, " +
				"sum(request_count) as request_count, " +
				"sum(total_tokens) as total_tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fields.BuildSelectFieldsV2("hour_timestamp", tt.groupFields)
			if got != tt.want {
				t.Errorf("BuildSelectFieldsV2() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildSelectFields_WhitelistValidation(t *testing.T) {
	// Test that invalid fields passed directly to BuildSelectFields are filtered out
	// This tests the defense-in-depth whitelist validation
	tests := []struct {
		name   string
		fields model.SummarySelectFields
		want   string
	}{
		{
			name: "Invalid fields are filtered out",
			fields: model.SummarySelectFields{
				"request_count", "invalid_field", "exception_count",
			},
			want: "hour_timestamp as timestamp, " +
				"sum(request_count) as request_count, " +
				"sum(exception_count) as exception_count",
		},
		{
			name: "SQL injection attempt is filtered",
			fields: model.SummarySelectFields{
				"request_count",
				"request_count; DROP TABLE summaries;--",
				"exception_count",
			},
			want: "hour_timestamp as timestamp, " +
				"sum(request_count) as request_count, " +
				"sum(exception_count) as exception_count",
		},
		{
			name: "All invalid fields result in only timestamp",
			fields: model.SummarySelectFields{
				"invalid1", "invalid2", "DROP TABLE",
			},
			want: "hour_timestamp as timestamp",
		},
		{
			name: "Mix of valid, invalid, and duplicate",
			fields: model.SummarySelectFields{
				"request_count",
				"invalid",
				"request_count",
				"total_tokens",
			},
			want: "hour_timestamp as timestamp, " +
				"sum(request_count) as request_count, " +
				"sum(total_tokens) as total_tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fields.BuildSelectFields("hour_timestamp")
			if got != tt.want {
				t.Errorf("BuildSelectFields() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildSelectFieldsV2_WhitelistValidation(t *testing.T) {
	// Test that invalid fields passed directly to BuildSelectFieldsV2 are filtered out
	tests := []struct {
		name        string
		fields      model.SummarySelectFields
		groupFields string
		want        string
	}{
		{
			name: "Invalid fields are filtered out",
			fields: model.SummarySelectFields{
				"request_count", "DROP TABLE users", "total_tokens",
			},
			groupFields: "channel_id, model",
			want: "hour_timestamp as timestamp, channel_id, model, " +
				"sum(request_count) as request_count, " +
				"sum(total_tokens) as total_tokens",
		},
		{
			name: "All invalid fields result in only timestamp and group fields",
			fields: model.SummarySelectFields{
				"invalid1", "invalid2",
			},
			groupFields: "group_id, token_name",
			want:        "hour_timestamp as timestamp, group_id, token_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fields.BuildSelectFieldsV2("hour_timestamp", tt.groupFields)
			if got != tt.want {
				t.Errorf("BuildSelectFieldsV2() = %q, want %q", got, tt.want)
			}
		})
	}
}
