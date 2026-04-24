package gemini_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/gemini"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
)

func TestConvertRequest_JsonObject(t *testing.T) {
	// Setup metadata
	channel := &model.Channel{
		Type: model.ChannelTypeGoogleGemini,
	}
	meta := meta.NewMeta(
		channel,
		mode.Gemini,
		"gemini-1.5-pro",
		model.ModelConfig{},
	)

	// Create OpenAI request with response_format: {"type": "json_object"}
	openAIReq := relaymodel.GeneralOpenAIRequest{
		Model: "gemini-1.5-pro",
		Messages: []relaymodel.Message{
			{
				Role:    "user",
				Content: "Hello, give me JSON",
			},
		},
		ResponseFormat: &relaymodel.ResponseFormat{
			Type: "json_object",
		},
	}

	jsonData, _ := sonic.Marshal(openAIReq)
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)

	// Convert request
	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	// Parse body to check GenerationConfig
	bodyBytes, _ := io.ReadAll(result.Body)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)

	// Verify GenerationConfig.ResponseMimeType is "application/json"
	assert.NotNil(t, geminiReq.GenerationConfig)
	assert.Equal(t, "application/json", geminiReq.GenerationConfig.ResponseMimeType)
}

func TestConvertRequest_JsonSchema(t *testing.T) {
	// Setup metadata
	channel := &model.Channel{
		Type: model.ChannelTypeGoogleGemini,
	}
	meta := meta.NewMeta(
		channel,
		mode.Gemini,
		"gemini-1.5-pro",
		model.ModelConfig{},
	)

	// Create OpenAI request with response_format: {"type": "json_schema", "json_schema": {"schema": {"type": "object", "properties": {"foo": {"type": "string"}}, "additionalProperties": false, "$schema": "http://json-schema.org/draft-07/schema#"}}}
	openAIReq := relaymodel.GeneralOpenAIRequest{
		Model: "gemini-1.5-pro",
		Messages: []relaymodel.Message{
			{
				Role:    "user",
				Content: "Hello, give me JSON",
			},
		},
		ResponseFormat: &relaymodel.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &relaymodel.JSONSchema{
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"foo": map[string]any{
							"type": "string",
						},
					},
					"additionalProperties": false,
					"$schema":              "http://json-schema.org/draft-07/schema#",
				},
				Name: "test_schema",
			},
		},
	}

	jsonData, _ := sonic.Marshal(openAIReq)
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)

	// Convert request
	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	// Parse body to check GenerationConfig
	bodyBytes, _ := io.ReadAll(result.Body)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)

	// Verify GenerationConfig
	assert.NotNil(t, geminiReq.GenerationConfig)
	assert.Equal(t, "application/json", geminiReq.GenerationConfig.ResponseMimeType)
	assert.NotNil(t, geminiReq.GenerationConfig.ResponseSchema)

	schema := geminiReq.GenerationConfig.ResponseSchema

	// Check if unsupported fields are removed
	_, hasSchema := schema["$schema"]
	assert.False(t, hasSchema, "Expected $schema to be removed")

	_, hasAdditionalProperties := schema["additionalProperties"]
	assert.False(t, hasAdditionalProperties, "Expected additionalProperties to be removed")
}

func TestConvertRequest_Gemini25FlashLiteDoesNotAutoInjectThinkingConfig(t *testing.T) {
	channel := &model.Channel{
		Type: model.ChannelTypeVertexAI,
	}
	meta := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"gemini-2.5-flash-lite",
		model.ModelConfig{},
	)

	openAIReq := relaymodel.GeneralOpenAIRequest{
		Model: "gemini-2.5-flash-lite",
		Messages: []relaymodel.Message{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	jsonData, _ := sonic.Marshal(openAIReq)
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)

	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, _ := io.ReadAll(result.Body)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.NotNil(t, geminiReq.GenerationConfig)
	assert.Nil(t, geminiReq.GenerationConfig.ThinkingConfig)
}

func TestConvertRequest_Gemini25ProAutoInjectsThinkingConfig(t *testing.T) {
	channel := &model.Channel{
		Type: model.ChannelTypeVertexAI,
	}
	meta := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"gemini-2.5-pro",
		model.ModelConfig{},
	)

	openAIReq := relaymodel.GeneralOpenAIRequest{
		Model: "gemini-2.5-pro",
		Messages: []relaymodel.Message{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	jsonData, _ := sonic.Marshal(openAIReq)
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)

	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, _ := io.ReadAll(result.Body)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.NotNil(t, geminiReq.GenerationConfig)
	assert.NotNil(t, geminiReq.GenerationConfig.ThinkingConfig)
	assert.True(t, geminiReq.GenerationConfig.ThinkingConfig.IncludeThoughts)
}

func TestConvertRequest_ReasoningEffortToThinkingConfig(t *testing.T) {
	tests := []struct {
		name             string
		modelName        string
		reasoningEffort  string
		expectedBudget   *int
		expectedLevel    string
		expectedThoughts bool
	}{
		{
			name:             "gemini 2.5 uses thinking budget",
			modelName:        "gemini-2.5-pro",
			reasoningEffort:  "high",
			expectedBudget:   new(16384),
			expectedThoughts: true,
		},
		{
			name:             "gemini 3 pro uses thinking level",
			modelName:        "gemini-3-pro",
			reasoningEffort:  "high",
			expectedLevel:    "high",
			expectedThoughts: false,
		},
		{
			name:             "gemini 2.5 flash disables thinking with none",
			modelName:        "gemini-2.5-flash",
			reasoningEffort:  "none",
			expectedBudget:   new(0),
			expectedThoughts: false,
		},
		{
			name:             "gemini 2.5 pro none uses minimum budget",
			modelName:        "gemini-2.5-pro",
			reasoningEffort:  "none",
			expectedBudget:   new(128),
			expectedThoughts: false,
		},
		{
			name:             "gemini 3 pro cannot disable thinking",
			modelName:        "gemini-3-pro",
			reasoningEffort:  "none",
			expectedLevel:    "low",
			expectedThoughts: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &model.Channel{
				Type: model.ChannelTypeGoogleGemini,
			}
			meta := meta.NewMeta(
				channel,
				mode.ChatCompletions,
				tt.modelName,
				model.ModelConfig{},
			)

			openAIReq := relaymodel.GeneralOpenAIRequest{
				Model:           tt.modelName,
				ReasoningEffort: &tt.reasoningEffort,
				Messages: []relaymodel.Message{
					{
						Role:    "user",
						Content: "hello",
					},
				},
			}

			jsonData, _ := sonic.Marshal(openAIReq)
			req, _ := http.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"http://localhost/v1/chat/completions",
				bytes.NewBuffer(jsonData),
			)

			result, err := gemini.ConvertRequest(meta, req)
			assert.NoError(t, err)

			bodyBytes, _ := io.ReadAll(result.Body)

			var geminiReq relaymodel.GeminiChatRequest

			err = json.Unmarshal(bodyBytes, &geminiReq)
			assert.NoError(t, err)
			assert.NotNil(t, geminiReq.GenerationConfig)
			assert.NotNil(t, geminiReq.GenerationConfig.ThinkingConfig)

			if tt.expectedBudget != nil {
				assert.NotNil(t, geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
				assert.Equal(
					t,
					*tt.expectedBudget,
					*geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget,
				)
			} else {
				assert.Nil(t, geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
			}

			assert.Equal(
				t,
				tt.expectedLevel,
				geminiReq.GenerationConfig.ThinkingConfig.ThinkingLevel,
			)
			assert.Equal(
				t,
				tt.expectedThoughts,
				geminiReq.GenerationConfig.ThinkingConfig.IncludeThoughts,
			)
		})
	}
}

func TestConvertRequest_ReasoningBudgetNotClampedByMaxOutputTokens(t *testing.T) {
	channel := &model.Channel{
		Type: model.ChannelTypeGoogleGemini,
	}
	meta := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"gemini-2.5-pro",
		model.ModelConfig{},
	)

	reasoningEffort := "high"
	maxTokens := 1000
	openAIReq := relaymodel.GeneralOpenAIRequest{
		Model:           "gemini-2.5-pro",
		ReasoningEffort: &reasoningEffort,
		MaxTokens:       maxTokens,
		Messages: []relaymodel.Message{
			{
				Role:    "user",
				Content: "hello",
			},
		},
	}

	jsonData, _ := sonic.Marshal(openAIReq)
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)

	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, _ := io.ReadAll(result.Body)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.NotNil(t, geminiReq.GenerationConfig)
	assert.NotNil(t, geminiReq.GenerationConfig.ThinkingConfig)
	assert.NotNil(t, geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
	assert.Equal(t, 16384, *geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
}

func TestConvertRequest_ReasoningUsesOriginModelNameFirst(t *testing.T) {
	channel := &model.Channel{
		Type: model.ChannelTypeGoogleGemini,
	}
	meta := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"gemini-2.5-pro",
		model.ModelConfig{},
	)
	meta.ActualModel = "mapped-upstream-model"

	reasoningEffort := "none"
	openAIReq := relaymodel.GeneralOpenAIRequest{
		Model:           "gemini-2.5-pro",
		ReasoningEffort: &reasoningEffort,
		Messages: []relaymodel.Message{
			{
				Role:    "user",
				Content: "hello",
			},
		},
	}

	jsonData, _ := sonic.Marshal(openAIReq)
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)

	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, _ := io.ReadAll(result.Body)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.NotNil(t, geminiReq.GenerationConfig)
	assert.NotNil(t, geminiReq.GenerationConfig.ThinkingConfig)
	assert.NotNil(t, geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
	assert.Equal(t, 128, *geminiReq.GenerationConfig.ThinkingConfig.ThinkingBudget)
}

func TestConvertRequest_DisableAutoImageURLToBase64(t *testing.T) {
	channel := &model.Channel{
		Type: model.ChannelTypeGoogleGemini,
		Configs: model.ChannelConfigs{
			"disable_auto_image_url_to_base64": true,
		},
	}
	meta := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"gemini-1.5-pro",
		model.ModelConfig{},
	)

	openAIReq := relaymodel.GeneralOpenAIRequest{
		Model: "gemini-1.5-pro",
		Messages: []relaymodel.Message{
			{
				Role: "user",
				Content: []relaymodel.MessageContent{
					{
						Type: relaymodel.ContentTypeImageURL,
						ImageURL: &relaymodel.ImageURL{
							URL: "https://example.com/test.png",
						},
					},
				},
			},
		},
	}

	jsonData, _ := sonic.Marshal(openAIReq)
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)

	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, _ := io.ReadAll(result.Body)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.Len(t, geminiReq.Contents, 1)
	assert.Len(t, geminiReq.Contents[0].Parts, 1)
	assert.Nil(t, geminiReq.Contents[0].Parts[0].InlineData)
	assert.NotNil(t, geminiReq.Contents[0].Parts[0].FileData)
	assert.Equal(t, "", geminiReq.Contents[0].Parts[0].FileData.MimeType)
	assert.Equal(t, "https://example.com/test.png", geminiReq.Contents[0].Parts[0].FileData.FileURI)
}

func TestBuildMessagePartsUsesInlineDataForDataURL(t *testing.T) {
	t.Parallel()

	part := gemini.BuildMessagePartForTest(
		relaymodel.MessageContent{
			Type: relaymodel.ContentTypeImageURL,
			ImageURL: &relaymodel.ImageURL{
				URL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
			},
		},
	)

	assert.NotNil(t, part.InlineData)
	assert.Nil(t, part.FileData)
	assert.Equal(t, "image/png", part.InlineData.MimeType)
	assert.NotEmpty(t, part.InlineData.Data)
}

func TestBuildMessagePartsUsesFileDataForHTTPWhenAutoBase64Disabled(t *testing.T) {
	t.Parallel()

	part := gemini.BuildMessagePartForTest(
		relaymodel.MessageContent{
			Type: relaymodel.ContentTypeImageURL,
			ImageURL: &relaymodel.ImageURL{
				URL: "https://example.com/test.png",
			},
		},
	)

	assert.Nil(t, part.InlineData)
	assert.NotNil(t, part.FileData)
	assert.Equal(t, "https://example.com/test.png", part.FileData.FileURI)
}

func TestBuildMessagePartsUsesFileDataForHTTPURL(t *testing.T) {
	t.Parallel()

	part := gemini.BuildMessagePartForTest(
		relaymodel.MessageContent{
			Type: relaymodel.ContentTypeImageURL,
			ImageURL: &relaymodel.ImageURL{
				URL: "https://example.com/test.png",
			},
		},
	)

	assert.Nil(t, part.InlineData)
	assert.NotNil(t, part.FileData)
	assert.Equal(t, "https://example.com/test.png", part.FileData.FileURI)
}

func TestProcessImageTasksRewritesInlineDataHTTPURLToBase64(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{
			0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
			0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
			0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
			0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
			0x89, 0x00, 0x00, 0x00, 0x0d, 'I', 'D', 'A', 'T',
			0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0x50, 0x0f,
			0x00, 0x04, 0x85, 0x01, 0x80, 0x84, 0xa9, 0x8c,
			0x20, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D', 0xae,
			0x42, 0x60, 0x82,
		})
	}))
	defer ts.Close()

	part := gemini.BuildMessagePartForTest(
		relaymodel.MessageContent{
			Type: relaymodel.ContentTypeImageURL,
			ImageURL: &relaymodel.ImageURL{
				URL: ts.URL + "/test.png",
			},
		},
	)

	err := gemini.ProcessImageTasksForTest(context.Background(), []*relaymodel.GeminiPart{part})
	assert.NoError(t, err)
	assert.NotNil(t, part.InlineData)
	assert.Nil(t, part.FileData)
	assert.Equal(t, "image/png", part.InlineData.MimeType)
	assert.NotEmpty(t, part.InlineData.Data)
}

func TestBuildMessagePartsPreservesInvalidDataURLAsFileData(t *testing.T) {
	t.Parallel()

	part := gemini.BuildMessagePartForTest(
		relaymodel.MessageContent{
			Type: relaymodel.ContentTypeImageURL,
			ImageURL: &relaymodel.ImageURL{
				URL: "data:image/png;bad",
			},
		},
	)

	assert.Nil(t, part.InlineData)
	assert.NotNil(t, part.FileData)
	assert.Equal(t, "data:image/png;bad", part.FileData.FileURI)
}

func TestConvertRequestKeepsFileDataWhenImageConversionFails(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{
		Type: model.ChannelTypeGoogleGemini,
	}
	meta := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"gemini-1.5-pro",
		model.ModelConfig{},
	)

	openAIReq := relaymodel.GeneralOpenAIRequest{
		Model: "gemini-1.5-pro",
		Messages: []relaymodel.Message{
			{
				Role: "user",
				Content: []relaymodel.MessageContent{
					{
						Type: relaymodel.ContentTypeImageURL,
						ImageURL: &relaymodel.ImageURL{
							URL: "data:image/png;bad",
						},
					},
				},
			},
		},
	}

	jsonData, _ := sonic.Marshal(openAIReq)
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)

	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, _ := io.ReadAll(result.Body)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.Len(t, geminiReq.Contents, 1)
	assert.Len(t, geminiReq.Contents[0].Parts, 1)
	assert.Nil(t, geminiReq.Contents[0].Parts[0].InlineData)
	assert.NotNil(t, geminiReq.Contents[0].Parts[0].FileData)
	assert.Equal(
		t,
		"data:image/png;bad",
		geminiReq.Contents[0].Parts[0].FileData.FileURI,
	)
}
