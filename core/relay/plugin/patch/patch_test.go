package patch_test

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/plugin/patch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	plugin := patch.NewPatchPlugin()
	assert.NotNil(t, plugin)
	assert.True(t, len(patch.DefaultPredefinedPatches) > 0)
}

func TestApplyPatches_DeepSeekMaxTokensLimit(t *testing.T) {
	plugin := patch.NewPatchPlugin()
	config := &patch.Config{}

	testCases := []struct {
		name              string
		input             map[string]any
		actualModel       string
		expectedMaxTokens int
		shouldModify      bool
	}{
		{
			name: "deepseek model with high max_tokens",
			input: map[string]any{
				"model":      "deepseek-chat",
				"max_tokens": 20000,
			},
			actualModel:       "deepseek-chat",
			expectedMaxTokens: 8192,
			shouldModify:      true,
		},
		{
			name: "deepseek model with high max_tokens",
			input: map[string]any{
				"model":      "deepseek-v3",
				"max_tokens": 20000,
			},
			actualModel:       "deepseek-v3",
			expectedMaxTokens: 16384,
			shouldModify:      true,
		},
		{
			name: "deepseek model with low max_tokens",
			input: map[string]any{
				"model":      "deepseek-chat",
				"max_tokens": 8000,
			},
			actualModel:       "deepseek-chat",
			expectedMaxTokens: 8000,
			shouldModify:      false,
		},
		{
			name: "non-deepseek model",
			input: map[string]any{
				"model":      "gpt-4",
				"max_tokens": 20000,
			},
			actualModel:       "gpt-4",
			expectedMaxTokens: 20000,
			shouldModify:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputBytes, err := sonic.Marshal(tc.input)
			require.NoError(t, err)

			meta := &meta.Meta{ActualModel: tc.actualModel}
			outputBytes, modified, err := plugin.ApplyPatches(inputBytes, meta, config)
			require.NoError(t, err)
			assert.Equal(t, tc.shouldModify, modified)

			var output map[string]any

			err = sonic.Unmarshal(outputBytes, &output)
			require.NoError(t, err)

			if maxTokens, exists := output["max_tokens"]; exists {
				maxTokensFloat, ok := maxTokens.(float64)
				require.True(t, ok, "max_tokens should be float64")
				assert.Equal(t, tc.expectedMaxTokens, int(maxTokensFloat))
			}
		})
	}
}

func TestApplyPatches_GPT5MaxTokensConversion(t *testing.T) {
	plugin := patch.NewPatchPlugin()
	config := &patch.Config{}

	testCases := []struct {
		name                          string
		input                         map[string]any
		actualModel                   string
		expectedMaxCompletionTokens   int
		shouldHaveMaxTokens           bool
		shouldModify                  bool
		shouldHaveMaxCompletionTokens bool
	}{
		{
			name: "gpt-5 model with max_tokens",
			input: map[string]any{
				"model":       "gpt-5",
				"max_tokens":  4000,
				"temperature": 0.7,
			},
			actualModel:                   "gpt-5",
			expectedMaxCompletionTokens:   4000,
			shouldHaveMaxTokens:           false,
			shouldModify:                  true,
			shouldHaveMaxCompletionTokens: true,
		},
		{
			name: "gpt-5 model without max_tokens",
			input: map[string]any{
				"model":       "gpt-5",
				"temperature": 0.7,
			},
			actualModel:                   "gpt-5",
			shouldHaveMaxTokens:           false,
			shouldModify:                  true,
			shouldHaveMaxCompletionTokens: false,
		},
		{
			name: "gpt-4 model with max_tokens",
			input: map[string]any{
				"model":      "gpt-4",
				"max_tokens": 4000,
			},
			actualModel:                   "gpt-4",
			shouldHaveMaxTokens:           true,
			shouldModify:                  false,
			shouldHaveMaxCompletionTokens: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputBytes, err := sonic.Marshal(tc.input)
			require.NoError(t, err)

			meta := &meta.Meta{ActualModel: tc.actualModel}
			outputBytes, modified, err := plugin.ApplyPatches(inputBytes, meta, config)
			require.NoError(t, err)
			assert.Equal(t, tc.shouldModify, modified)

			var output map[string]any

			err = sonic.Unmarshal(outputBytes, &output)
			require.NoError(t, err)

			if tc.shouldHaveMaxCompletionTokens {
				maxCompletionTokens, ok := output["max_completion_tokens"].(float64)
				require.True(t, ok, "max_completion_tokens should be float64")
				assert.Equal(
					t,
					tc.expectedMaxCompletionTokens,
					int(maxCompletionTokens),
				)
			} else {
				_, hasMaxCompletionTokens := output["max_completion_tokens"]
				assert.False(t, hasMaxCompletionTokens, "max_completion_tokens should not exist")
			}

			_, hasMaxTokens := output["max_tokens"]
			assert.Equal(t, tc.shouldHaveMaxTokens, hasMaxTokens)
		})
	}
}

func TestCustomUserPatches(t *testing.T) {
	plugin := patch.NewPatchPlugin()
	config := &patch.Config{
		UserPatches: []patch.PatchRule{
			{
				Name: "test_temperature_limit",
				Conditions: []patch.PatchCondition{
					{
						Key:      "model",
						Operator: patch.OperatorContains,
						Value:    "test",
					},
				},
				Operations: []patch.PatchOperation{
					{
						Op:    patch.OpLimit,
						Key:   "temperature",
						Value: 1.0,
					},
				},
			},
			{
				Name: "add_default_top_p",
				Conditions: []patch.PatchCondition{
					{
						Key:      "top_p",
						Operator: patch.OperatorNotExists,
						Value:    "",
					},
				},
				Operations: []patch.PatchOperation{
					{
						Op:    patch.OpAdd,
						Key:   "top_p",
						Value: 0.9,
					},
				},
			},
		},
	}

	// Test temperature limit
	input := map[string]any{
		"model":       "test-model",
		"temperature": 1.5,
	}
	inputBytes, err := sonic.Marshal(input)
	require.NoError(t, err)

	meta := &meta.Meta{ActualModel: "test-model"}
	outputBytes, modified, err := plugin.ApplyPatches(inputBytes, meta, config)
	require.NoError(t, err)
	assert.True(t, modified)

	var output map[string]any

	err = sonic.Unmarshal(outputBytes, &output)
	require.NoError(t, err)
	assert.Equal(t, 1.0, output["temperature"])
	assert.Equal(t, 0.9, output["top_p"]) // Should be added
}

func TestNestedFieldOperations(t *testing.T) {
	plugin := patch.NewPatchPlugin()
	config := &patch.Config{
		UserPatches: []patch.PatchRule{
			{
				Name: "nested_operations",
				Operations: []patch.PatchOperation{
					{
						Op:    patch.OpSet,
						Key:   "parameters.max_tokens",
						Value: 2000,
					},
					{
						Op:    patch.OpSet,
						Key:   "metadata.version",
						Value: "1.0",
					},
				},
			},
		},
	}

	input := map[string]any{
		"model": "test",
		"parameters": map[string]any{
			"temperature": 0.7,
		},
	}
	inputBytes, err := sonic.Marshal(input)
	require.NoError(t, err)

	meta := &meta.Meta{ActualModel: "test"}
	outputBytes, modified, err := plugin.ApplyPatches(inputBytes, meta, config)
	require.NoError(t, err)
	assert.True(t, modified)

	var output map[string]any

	err = sonic.Unmarshal(outputBytes, &output)
	require.NoError(t, err)

	// Check nested field access
	params, ok := output["parameters"].(map[string]any)
	require.True(t, ok)
	maxTokens, ok := params["max_tokens"].(float64)
	require.True(t, ok, "max_tokens should be float64")
	assert.Equal(t, 2000, int(maxTokens))
	assert.Equal(t, 0.7, params["temperature"])

	metadata, ok := output["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1.0", metadata["version"])
}

func TestPlaceholderResolution(t *testing.T) {
	plugin := patch.NewPatchPlugin()
	config := &patch.Config{
		UserPatches: []patch.PatchRule{
			{
				Name: "placeholder_test",
				Conditions: []patch.PatchCondition{
					{
						Key:      "max_tokens",
						Operator: patch.OperatorExists,
					},
				},
				Operations: []patch.PatchOperation{
					{
						Op:    patch.OpSet,
						Key:   "max_completion_tokens",
						Value: "{{max_tokens}}",
					},
					{
						Op:  patch.OpDelete,
						Key: "max_tokens",
					},
				},
			},
		},
	}

	input := map[string]any{
		"model":      "test",
		"max_tokens": 3000,
	}
	inputBytes, err := sonic.Marshal(input)
	require.NoError(t, err)

	meta := &meta.Meta{ActualModel: "test"}
	outputBytes, modified, err := plugin.ApplyPatches(inputBytes, meta, config)
	require.NoError(t, err)
	assert.True(t, modified)

	var output map[string]any

	err = sonic.Unmarshal(outputBytes, &output)
	require.NoError(t, err)

	maxCompletionTokens, ok := output["max_completion_tokens"].(float64)
	require.True(t, ok, "max_completion_tokens should be float64")
	assert.Equal(t, 3000, int(maxCompletionTokens))

	_, hasMaxTokens := output["max_tokens"]
	assert.False(t, hasMaxTokens)
}

func TestOperators(t *testing.T) {
	plugin := patch.NewPatchPlugin()
	config := &patch.Config{
		UserPatches: []patch.PatchRule{
			{
				Name: "operator_tests",
				Conditions: []patch.PatchCondition{
					{
						Key:      "model",
						Operator: patch.OperatorRegex,
						Value:    "^gpt-[0-9]$",
					},
				},
				Operations: []patch.PatchOperation{
					{
						Op:    patch.OpSet,
						Key:   "matched",
						Value: true,
					},
				},
			},
		},
	}

	testCases := []struct {
		model       string
		shouldMatch bool
	}{
		{"gpt-4", true},
		{"gpt-3", true},
		{"gpt-4o", false},
		{"claude-3", false},
	}

	for _, tc := range testCases {
		t.Run(tc.model, func(t *testing.T) {
			input := map[string]any{"model": tc.model}
			inputBytes, err := sonic.Marshal(input)
			require.NoError(t, err)

			meta := &meta.Meta{ActualModel: tc.model}
			outputBytes, modified, err := plugin.ApplyPatches(inputBytes, meta, config)
			require.NoError(t, err)
			assert.Equal(t, tc.shouldMatch, modified)

			if tc.shouldMatch {
				var output map[string]any

				err = sonic.Unmarshal(outputBytes, &output)
				require.NoError(t, err)

				matched, ok := output["matched"].(bool)
				require.True(t, ok, "matched should be bool")
				assert.True(t, matched)
			}
		})
	}
}

func TestInvalidJSON(t *testing.T) {
	plugin := patch.NewPatchPlugin()
	config := &patch.Config{}

	invalidJSON := []byte(`{"invalid": json}`)
	meta := &meta.Meta{ActualModel: "test"}

	outputBytes, modified, err := plugin.ApplyPatches(invalidJSON, meta, config)
	require.NoError(t, err)
	assert.False(t, modified)
	assert.Equal(t, invalidJSON, outputBytes)
}

func TestConvertRequest(t *testing.T) {
	// Skip this test since it requires database initialization
	// The functionality is already tested in other unit tests
	t.Skip("Skipping integration test - requires database setup")
}

func TestToFloat64(t *testing.T) {
	testCases := []struct {
		input    any
		expected float64
		hasError bool
	}{
		{float64(3.14), 3.14, false},
		{float32(2.5), 2.5, false},
		{int(42), 42.0, false},
		{int32(100), 100.0, false},
		{int64(200), 200.0, false},
		{"123.45", 123.45, false},
		{"invalid", 0, true},
		{true, 0, true},
	}

	for _, tc := range testCases {
		result, err := patch.ToFloat64(tc.input)
		if tc.hasError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		}
	}
}

func TestConditionLogicOperators(t *testing.T) {
	plugin := patch.NewPatchPlugin()

	testCases := []struct {
		name         string
		config       *patch.Config
		input        map[string]any
		actualModel  string
		shouldModify bool
	}{
		{
			name: "OR logic - one condition matches",
			config: &patch.Config{
				UserPatches: []patch.PatchRule{
					{
						Name:           "or_logic_test",
						ConditionLogic: patch.LogicOr,
						Conditions: []patch.PatchCondition{
							{
								Key:      "model",
								Operator: patch.OperatorEquals,
								Value:    "gpt-4",
							},
							{
								Key:      "temperature",
								Operator: patch.OperatorGreaterThan,
								Value:    "1.5",
							},
						},
						Operations: []patch.PatchOperation{
							{
								Op:    patch.OpSet,
								Key:   "modified",
								Value: true,
							},
						},
					},
				},
			},
			input: map[string]any{
				"model":       "claude-3",
				"temperature": 2.0,
			},
			actualModel:  "claude-3",
			shouldModify: true,
		},
		{
			name: "OR logic - no condition matches",
			config: &patch.Config{
				UserPatches: []patch.PatchRule{
					{
						Name:           "or_logic_test_no_match",
						ConditionLogic: patch.LogicOr,
						Conditions: []patch.PatchCondition{
							{
								Key:      "model",
								Operator: patch.OperatorEquals,
								Value:    "gpt-4",
							},
							{
								Key:      "temperature",
								Operator: patch.OperatorGreaterThan,
								Value:    "1.5",
							},
						},
						Operations: []patch.PatchOperation{
							{
								Op:    patch.OpSet,
								Key:   "modified",
								Value: true,
							},
						},
					},
				},
			},
			input: map[string]any{
				"model":       "claude-3",
				"temperature": 1.0,
			},
			actualModel:  "claude-3",
			shouldModify: false,
		},
		{
			name: "AND logic (default) - all conditions match",
			config: &patch.Config{
				UserPatches: []patch.PatchRule{
					{
						Name: "and_logic_test",
						Conditions: []patch.PatchCondition{
							{
								Key:      "model",
								Operator: patch.OperatorContains,
								Value:    "gpt",
							},
							{
								Key:      "temperature",
								Operator: patch.OperatorLessThan,
								Value:    "1.5",
							},
						},
						Operations: []patch.PatchOperation{
							{
								Op:    patch.OpSet,
								Key:   "modified",
								Value: true,
							},
						},
					},
				},
			},
			input: map[string]any{
				"model":       "gpt-4",
				"temperature": 1.0,
			},
			actualModel:  "gpt-4",
			shouldModify: true,
		},
		{
			name: "AND logic (default) - one condition fails",
			config: &patch.Config{
				UserPatches: []patch.PatchRule{
					{
						Name: "and_logic_test_fail",
						Conditions: []patch.PatchCondition{
							{
								Key:      "model",
								Operator: patch.OperatorContains,
								Value:    "gpt",
							},
							{
								Key:      "temperature",
								Operator: patch.OperatorLessThan,
								Value:    "1.5",
							},
						},
						Operations: []patch.PatchOperation{
							{
								Op:    patch.OpSet,
								Key:   "modified",
								Value: true,
							},
						},
					},
				},
			},
			input: map[string]any{
				"model":       "gpt-4",
				"temperature": 2.0,
			},
			actualModel:  "gpt-4",
			shouldModify: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputBytes, err := sonic.Marshal(tc.input)
			require.NoError(t, err)

			meta := &meta.Meta{ActualModel: tc.actualModel}
			outputBytes, modified, err := plugin.ApplyPatches(inputBytes, meta, tc.config)
			require.NoError(t, err)
			assert.Equal(t, tc.shouldModify, modified)

			if tc.shouldModify {
				var output map[string]any

				err = sonic.Unmarshal(outputBytes, &output)
				require.NoError(t, err)
				assert.Equal(t, output["modified"], true)
			}
		})
	}
}

func TestConditionNegation(t *testing.T) {
	plugin := patch.NewPatchPlugin()

	testCases := []struct {
		name         string
		config       *patch.Config
		input        map[string]any
		actualModel  string
		shouldModify bool
	}{
		{
			name: "negate condition - should match when negated",
			config: &patch.Config{
				UserPatches: []patch.PatchRule{
					{
						Name: "negate_test",
						Conditions: []patch.PatchCondition{
							{
								Key:      "model",
								Operator: patch.OperatorEquals,
								Value:    "gpt-4",
								Negate:   true,
							},
						},
						Operations: []patch.PatchOperation{
							{
								Op:    patch.OpSet,
								Key:   "modified",
								Value: true,
							},
						},
					},
				},
			},
			input: map[string]any{
				"model": "claude-3",
			},
			actualModel:  "claude-3",
			shouldModify: true,
		},
		{
			name: "negate condition - should not match when negated",
			config: &patch.Config{
				UserPatches: []patch.PatchRule{
					{
						Name: "negate_test_no_match",
						Conditions: []patch.PatchCondition{
							{
								Key:      "model",
								Operator: patch.OperatorEquals,
								Value:    "gpt-4",
								Negate:   true,
							},
						},
						Operations: []patch.PatchOperation{
							{
								Op:    patch.OpSet,
								Key:   "modified",
								Value: true,
							},
						},
					},
				},
			},
			input: map[string]any{
				"model": "gpt-4",
			},
			actualModel:  "gpt-4",
			shouldModify: false,
		},
		{
			name: "OR with negation - complex logic",
			config: &patch.Config{
				UserPatches: []patch.PatchRule{
					{
						Name:           "or_with_negate",
						ConditionLogic: patch.LogicOr,
						Conditions: []patch.PatchCondition{
							{
								Key:      "model",
								Operator: patch.OperatorEquals,
								Value:    "gpt-4",
							},
							{
								Key:      "temperature",
								Operator: patch.OperatorExists,
								Negate:   true, // NOT exists
							},
						},
						Operations: []patch.PatchOperation{
							{
								Op:    patch.OpSet,
								Key:   "modified",
								Value: true,
							},
						},
					},
				},
			},
			input: map[string]any{
				"model": "claude-3",
				// no temperature field
			},
			actualModel:  "claude-3",
			shouldModify: true, // Should match because temperature doesn't exist (negated exists)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputBytes, err := sonic.Marshal(tc.input)
			require.NoError(t, err)

			meta := &meta.Meta{ActualModel: tc.actualModel}
			outputBytes, modified, err := plugin.ApplyPatches(inputBytes, meta, tc.config)
			require.NoError(t, err)
			assert.Equal(t, tc.shouldModify, modified)

			if tc.shouldModify {
				var output map[string]any

				err = sonic.Unmarshal(outputBytes, &output)
				require.NoError(t, err)
				assert.Equal(t, output["modified"], true)
			}
		})
	}
}

func TestApplyPatches_GeminiGPT51RemoveTopP(t *testing.T) {
	plugin := patch.NewPatchPlugin()
	config := &patch.Config{}

	testCases := []struct {
		name           string
		input          map[string]any
		actualModel    string
		shouldModify   bool
		shouldHaveTopP bool
	}{
		{
			name: "gpt-5.1 model with top_p in generationConfig",
			input: map[string]any{
				"generationConfig": map[string]any{
					"topP":        0.95,
					"temperature": 0.7,
				},
			},
			actualModel:    "gpt-5.1-chat-latest",
			shouldModify:   true,
			shouldHaveTopP: false,
		},
		{
			name: "gpt-5.1 model without top_p",
			input: map[string]any{
				"generationConfig": map[string]any{
					"temperature": 0.7,
				},
			},
			actualModel:    "gpt-5.1-chat-latest",
			shouldModify:   false,
			shouldHaveTopP: false,
		},
		{
			name: "non-gpt-5.1 model",
			input: map[string]any{
				"generationConfig": map[string]any{
					"topP":        0.95,
					"temperature": 0.7,
				},
			},
			actualModel:    "gpt-4o",
			shouldModify:   false,
			shouldHaveTopP: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputBytes, err := sonic.Marshal(tc.input)
			require.NoError(t, err)

			meta := &meta.Meta{ActualModel: tc.actualModel}
			outputBytes, modified, err := plugin.ApplyPatches(inputBytes, meta, config)
			require.NoError(t, err)
			assert.Equal(t, tc.shouldModify, modified)

			var output map[string]any

			err = sonic.Unmarshal(outputBytes, &output)
			require.NoError(t, err)

			if genConfig, ok := output["generationConfig"].(map[string]any); ok {
				_, hasTopP := genConfig["topP"]
				assert.Equal(t, tc.shouldHaveTopP, hasTopP)
			} else if tc.shouldHaveTopP {
				t.Error("generationConfig missing when topP expected")
			}
		})
	}
}
