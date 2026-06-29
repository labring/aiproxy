package gemini_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
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

func TestConvertRequest_TTSModelSetsAudioModalityAndSpeechConfig(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{
		Type: model.ChannelTypeGoogleGemini,
	}
	meta := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"gemini-2.5-flash-tts",
		model.ModelConfig{},
	)

	openAIReq := relaymodel.GeneralOpenAIRequest{
		Model: "gemini-2.5-flash-tts",
		Messages: []relaymodel.Message{
			{
				Role:    relaymodel.RoleUser,
				Content: "Say hello.",
			},
		},
		Audio: &relaymodel.Audio{
			Voice: "Puck",
		},
	}

	jsonData, err := sonic.Marshal(openAIReq)
	assert.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)
	assert.NoError(t, err)

	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, err := io.ReadAll(result.Body)
	assert.NoError(t, err)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.NotNil(t, geminiReq.GenerationConfig)
	assert.Equal(
		t,
		[]string{relaymodel.GeminiModalityAudio},
		geminiReq.GenerationConfig.ResponseModalities,
	)
	assert.NotNil(t, geminiReq.GenerationConfig.SpeechConfig)
	assert.NotNil(t, geminiReq.GenerationConfig.SpeechConfig.VoiceConfig)
	assert.NotNil(
		t,
		geminiReq.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig,
	)
	assert.Equal(
		t,
		"Puck",
		geminiReq.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName,
	)
	assert.Nil(t, geminiReq.GenerationConfig.ThinkingConfig)
}

func TestConvertTTSRequestMapsOpenAISpeechToGemini(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{Type: model.ChannelTypeGoogleGemini}
	meta := meta.NewMeta(
		channel,
		mode.AudioSpeech,
		"gemini-2.5-flash-tts",
		model.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/audio/speech",
		bytes.NewBufferString(
			`{"model":"gemini-2.5-flash-tts","input":"Say hello.","voice":"Kore"}`,
		),
	)
	assert.NoError(t, err)

	result, err := gemini.ConvertTTSRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, err := io.ReadAll(result.Body)
	assert.NoError(t, err)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.Len(t, geminiReq.Contents, 1)
	assert.Equal(t, "Say hello.", geminiReq.Contents[0].Parts[0].Text)
	assert.Equal(
		t,
		[]string{relaymodel.GeminiModalityAudio},
		geminiReq.GenerationConfig.ResponseModalities,
	)
	assert.Equal(
		t,
		"Kore",
		geminiReq.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName,
	)
}

func TestTTSHandlerReturnsAudioOutputTokensForPricing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &gemini.Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/audio/speech",
		nil,
	)

	m := meta.NewMeta(
		&model.Channel{Type: model.ChannelTypeGoogleGemini},
		mode.AudioSpeech,
		"gemini-2.5-flash-tts",
		model.ModelConfig{},
	)
	m.RequestUsage.InputTokens = 10

	audioData := base64.StdEncoding.EncodeToString([]byte("audio bytes"))
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(bytes.NewBufferString(`{
			"candidates": [{
				"content": {
					"parts": [{
						"inlineData": {
							"mimeType": "audio/wav",
							"data": "` + audioData + `"
						}
					}]
				},
				"finishReason": "STOP"
			}],
			"usageMetadata": {
				"promptTokenCount": 10,
				"candidatesTokenCount": 20,
				"totalTokenCount": 30,
				"promptTokensDetails": [{"modality": "TEXT", "tokenCount": 10}],
				"candidatesTokensDetails": [{"modality": "AUDIO", "tokenCount": 20}]
			}
		}`)),
	}

	result, err := adaptor.DoResponse(m, nil, ctx, resp)
	assert.Nil(t, err)
	assert.Equal(t, model.ZeroNullInt64(20), result.Usage.OutputTokens)
	assert.Equal(t, model.ZeroNullInt64(20), result.Usage.AudioOutputTokens)
	assert.Equal(t, model.ZeroNullInt64(30), result.Usage.TotalTokens)
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

func TestProcessImageTasksKeepsFileDataWhenConversionFails(t *testing.T) {
	t.Parallel()

	part := gemini.BuildMessagePartForTest(
		relaymodel.MessageContent{
			Type: relaymodel.ContentTypeImageURL,
			ImageURL: &relaymodel.ImageURL{
				URL: "data:image/png;bad",
			},
		},
	)

	err := gemini.ProcessImageTasksForTest(context.Background(), []*relaymodel.GeminiPart{part})
	assert.NoError(t, err)
	assert.Nil(t, part.InlineData)
	assert.NotNil(t, part.FileData)
	assert.Equal(t, "data:image/png;bad", part.FileData.FileURI)
}

func TestConvertRequest_OpenAIAudioAndVideoParts(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{
		Type: model.ChannelTypeGoogleGemini,
	}
	meta := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"gemini-2.5-flash",
		model.ModelConfig{},
	)

	audioData := base64.StdEncoding.EncodeToString([]byte("audio bytes"))
	openAIReq := map[string]any{
		"model": "gemini-2.5-flash",
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "text",
						"text": "Describe these files",
					},
					{
						"type": "input_audio",
						"input_audio": map[string]any{
							"data":   audioData,
							"format": "wav",
						},
					},
					{
						"type": "video_url",
						"video_url": map[string]any{
							"url": "gs://bucket/video.mp4",
						},
					},
				},
			},
		},
	}

	jsonData, err := sonic.Marshal(openAIReq)
	assert.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)
	assert.NoError(t, err)

	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, err := io.ReadAll(result.Body)
	assert.NoError(t, err)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.Len(t, geminiReq.Contents, 1)
	assert.Len(t, geminiReq.Contents[0].Parts, 3)
	assert.Equal(t, "Describe these files", geminiReq.Contents[0].Parts[0].Text)
	assert.NotNil(t, geminiReq.Contents[0].Parts[1].InlineData)
	assert.Equal(t, "audio/wav", geminiReq.Contents[0].Parts[1].InlineData.MimeType)
	assert.Equal(t, audioData, geminiReq.Contents[0].Parts[1].InlineData.Data)
	assert.NotNil(t, geminiReq.Contents[0].Parts[2].FileData)
	assert.Equal(t, "video/mp4", geminiReq.Contents[0].Parts[2].FileData.MimeType)
	assert.Equal(t, "gs://bucket/video.mp4", geminiReq.Contents[0].Parts[2].FileData.FileURI)
}

func TestConvertRequestAutoConvertsAudioAndVideoURLs(t *testing.T) {
	t.Parallel()

	audioData := []byte("audio bytes")
	videoData := []byte("video bytes")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio.wav":
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write(audioData)
		case "/video.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write(videoData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	channel := &model.Channel{
		Type: model.ChannelTypeGoogleGemini,
	}
	meta := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"gemini-2.5-flash",
		model.ModelConfig{},
	)

	openAIReq := map[string]any{
		"model": "gemini-2.5-flash",
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_audio",
						"input_audio": map[string]any{
							"url": ts.URL + "/audio.wav",
						},
					},
					{
						"type": "video_url",
						"video_url": map[string]any{
							"url": ts.URL + "/video.mp4",
						},
					},
				},
			},
		},
	}

	jsonData, err := sonic.Marshal(openAIReq)
	assert.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)
	assert.NoError(t, err)

	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, err := io.ReadAll(result.Body)
	assert.NoError(t, err)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.Len(t, geminiReq.Contents, 1)
	assert.Len(t, geminiReq.Contents[0].Parts, 2)
	assert.NotNil(t, geminiReq.Contents[0].Parts[0].InlineData)
	assert.Equal(t, "audio/wav", geminiReq.Contents[0].Parts[0].InlineData.MimeType)
	assert.Equal(
		t,
		base64.StdEncoding.EncodeToString(audioData),
		geminiReq.Contents[0].Parts[0].InlineData.Data,
	)
	assert.Nil(t, geminiReq.Contents[0].Parts[0].FileData)
	assert.NotNil(t, geminiReq.Contents[0].Parts[1].InlineData)
	assert.Equal(t, "video/mp4", geminiReq.Contents[0].Parts[1].InlineData.MimeType)
	assert.Equal(
		t,
		base64.StdEncoding.EncodeToString(videoData),
		geminiReq.Contents[0].Parts[1].InlineData.Data,
	)
	assert.Nil(t, geminiReq.Contents[0].Parts[1].FileData)
}

func TestConvertRequestAutoConvertsAudioHTTPDataURL(t *testing.T) {
	t.Parallel()

	audioData := []byte("audio bytes")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/audio.wav", r.URL.Path)
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(audioData)
	}))
	defer ts.Close()

	channel := &model.Channel{
		Type: model.ChannelTypeGoogleGemini,
	}
	meta := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"gemini-2.5-flash",
		model.ModelConfig{},
	)

	openAIReq := map[string]any{
		"model": "gemini-2.5-flash",
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_audio",
						"input_audio": map[string]any{
							"data":   ts.URL + "/audio.wav",
							"format": "wav",
						},
					},
				},
			},
		},
	}

	jsonData, err := sonic.Marshal(openAIReq)
	assert.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)
	assert.NoError(t, err)

	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, err := io.ReadAll(result.Body)
	assert.NoError(t, err)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.Len(t, geminiReq.Contents, 1)
	assert.Len(t, geminiReq.Contents[0].Parts, 1)
	assert.NotNil(t, geminiReq.Contents[0].Parts[0].InlineData)
	assert.Equal(t, "audio/wav", geminiReq.Contents[0].Parts[0].InlineData.MimeType)
	assert.Equal(
		t,
		base64.StdEncoding.EncodeToString(audioData),
		geminiReq.Contents[0].Parts[0].InlineData.Data,
	)
	assert.Nil(t, geminiReq.Contents[0].Parts[0].FileData)
}

func TestConvertRequestCanDisableAudioAndVideoURLAutoBase64(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{
		Type: model.ChannelTypeGoogleGemini,
		Configs: model.ChannelConfigs{
			"disable_auto_audio_url_to_base64": true,
			"disable_auto_video_url_to_base64": true,
		},
	}
	meta := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"gemini-2.5-flash",
		model.ModelConfig{},
	)

	openAIReq := map[string]any{
		"model": "gemini-2.5-flash",
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_audio",
						"input_audio": map[string]any{
							"url": "https://example.com/audio.wav",
						},
					},
					{
						"type": "video_url",
						"video_url": map[string]any{
							"url": "https://example.com/video.mp4",
						},
					},
				},
			},
		},
	}

	jsonData, err := sonic.Marshal(openAIReq)
	assert.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)
	assert.NoError(t, err)

	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, err := io.ReadAll(result.Body)
	assert.NoError(t, err)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.Len(t, geminiReq.Contents, 1)
	assert.Len(t, geminiReq.Contents[0].Parts, 2)
	assert.Nil(t, geminiReq.Contents[0].Parts[0].InlineData)
	assert.NotNil(t, geminiReq.Contents[0].Parts[0].FileData)
	assert.Equal(t, "audio/wav", geminiReq.Contents[0].Parts[0].FileData.MimeType)
	assert.Equal(
		t,
		"https://example.com/audio.wav",
		geminiReq.Contents[0].Parts[0].FileData.FileURI,
	)
	assert.Nil(t, geminiReq.Contents[0].Parts[1].InlineData)
	assert.NotNil(t, geminiReq.Contents[0].Parts[1].FileData)
	assert.Equal(t, "video/mp4", geminiReq.Contents[0].Parts[1].FileData.MimeType)
	assert.Equal(
		t,
		"https://example.com/video.mp4",
		geminiReq.Contents[0].Parts[1].FileData.FileURI,
	)
}

func TestConvertRequestVertexAIAlwaysConvertsAudioAndVideoURLs(t *testing.T) {
	t.Parallel()

	audioData := []byte("audio bytes")
	videoData := []byte("video bytes")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio.wav":
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write(audioData)
		case "/video.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write(videoData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	channel := &model.Channel{
		Type: model.ChannelTypeVertexAI,
		Configs: model.ChannelConfigs{
			"disable_auto_audio_url_to_base64": true,
			"disable_auto_video_url_to_base64": true,
		},
	}
	meta := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"gemini-2.5-flash",
		model.ModelConfig{},
	)

	openAIReq := map[string]any{
		"model": "gemini-2.5-flash",
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_audio",
						"input_audio": map[string]any{
							"data":   ts.URL + "/audio.wav",
							"format": "wav",
						},
					},
					{
						"type": "video_url",
						"video_url": map[string]any{
							"url": ts.URL + "/video.mp4",
						},
					},
				},
			},
		},
	}

	jsonData, err := sonic.Marshal(openAIReq)
	assert.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(jsonData),
	)
	assert.NoError(t, err)

	result, err := gemini.ConvertRequest(meta, req)
	assert.NoError(t, err)

	bodyBytes, err := io.ReadAll(result.Body)
	assert.NoError(t, err)

	var geminiReq relaymodel.GeminiChatRequest

	err = json.Unmarshal(bodyBytes, &geminiReq)
	assert.NoError(t, err)
	assert.Len(t, geminiReq.Contents, 1)
	assert.Len(t, geminiReq.Contents[0].Parts, 2)
	assert.NotNil(t, geminiReq.Contents[0].Parts[0].InlineData)
	assert.Equal(t, "audio/wav", geminiReq.Contents[0].Parts[0].InlineData.MimeType)
	assert.Equal(
		t,
		base64.StdEncoding.EncodeToString(audioData),
		geminiReq.Contents[0].Parts[0].InlineData.Data,
	)
	assert.Nil(t, geminiReq.Contents[0].Parts[0].FileData)
	assert.NotNil(t, geminiReq.Contents[0].Parts[1].InlineData)
	assert.Equal(t, "video/mp4", geminiReq.Contents[0].Parts[1].InlineData.MimeType)
	assert.Equal(
		t,
		base64.StdEncoding.EncodeToString(videoData),
		geminiReq.Contents[0].Parts[1].InlineData.Data,
	)
	assert.Nil(t, geminiReq.Contents[0].Parts[1].FileData)
}

func TestProcessMediaTasksKeepsFileDataWhenConversionFails(t *testing.T) {
	t.Parallel()

	part := gemini.BuildMessagePartForTest(
		relaymodel.MessageContent{
			Type: relaymodel.ContentTypeVideoURL,
			VideoURL: &relaymodel.VideoURL{
				URL: "https://example.com/video.mp4",
			},
		},
	)

	gemini.ProcessMediaTasksForTest(context.Background(), "video", []*relaymodel.GeminiPart{part})
	assert.Nil(t, part.InlineData)
	assert.NotNil(t, part.FileData)
	assert.Equal(t, "https://example.com/video.mp4", part.FileData.FileURI)
}

func TestProcessMediaTasksKeepsFileDataWhenContextCanceled(t *testing.T) {
	t.Parallel()

	part := gemini.BuildMessagePartForTest(
		relaymodel.MessageContent{
			Type: relaymodel.ContentTypeVideoURL,
			VideoURL: &relaymodel.VideoURL{
				URL: "https://example.com/video.mp4",
			},
		},
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	gemini.ProcessMediaTasksForTest(ctx, "video", []*relaymodel.GeminiPart{part})
	assert.Nil(t, part.InlineData)
	assert.NotNil(t, part.FileData)
	assert.Equal(t, "https://example.com/video.mp4", part.FileData.FileURI)
}

func TestBuildMessagePartsUsesInlineDataForMediaDataURL(t *testing.T) {
	t.Parallel()

	t.Run("video_url", func(t *testing.T) {
		t.Parallel()

		videoData := base64.StdEncoding.EncodeToString([]byte("video bytes"))
		part := gemini.BuildMessagePartForTest(
			relaymodel.MessageContent{
				Type: relaymodel.ContentTypeVideoURL,
				VideoURL: &relaymodel.VideoURL{
					URL: "data:video/mp4;base64," + videoData,
				},
			},
		)

		assert.NotNil(t, part.InlineData)
		assert.Nil(t, part.FileData)
		assert.Equal(t, "video/mp4", part.InlineData.MimeType)
		assert.Equal(t, videoData, part.InlineData.Data)
	})

	t.Run("input_audio", func(t *testing.T) {
		t.Parallel()

		audioData := base64.StdEncoding.EncodeToString([]byte("audio bytes"))
		part := gemini.BuildMessagePartForTest(
			relaymodel.MessageContent{
				Type: relaymodel.ContentTypeInputAudio,
				InputAudio: &relaymodel.InputAudio{
					Data: "data:audio/wav;base64," + audioData,
				},
			},
		)

		assert.NotNil(t, part.InlineData)
		assert.Nil(t, part.FileData)
		assert.Equal(t, "audio/wav", part.InlineData.MimeType)
		assert.Equal(t, audioData, part.InlineData.Data)
	})
}

func TestResponseChat2OpenAIConvertsAudioInlineDataAndUsage(t *testing.T) {
	t.Parallel()

	meta := meta.NewMeta(
		&model.Channel{Type: model.ChannelTypeGoogleGemini},
		mode.ChatCompletions,
		"gemini-2.5-flash-tts",
		model.ModelConfig{},
	)
	audioData := base64.StdEncoding.EncodeToString([]byte("audio bytes"))
	response := &relaymodel.GeminiChatResponse{
		Candidates: []*relaymodel.GeminiChatCandidate{
			{
				Content: relaymodel.GeminiChatContent{
					Parts: []*relaymodel.GeminiPart{
						{
							InlineData: &relaymodel.GeminiInlineData{
								MimeType: "audio/wav",
								Data:     audioData,
							},
						},
					},
				},
				FinishReason: relaymodel.GeminiFinishReasonStop,
			},
		},
		UsageMetadata: &relaymodel.GeminiUsageMetadata{
			PromptTokenCount:     12,
			CandidatesTokenCount: 34,
			TotalTokenCount:      46,
			PromptTokensDetails: []relaymodel.GeminiTokensDetail{
				{Modality: relaymodel.GeminiModalityText, TokenCount: 12},
			},
			CandidatesTokensDetails: []relaymodel.GeminiTokensDetail{
				{Modality: relaymodel.GeminiModalityAudio, TokenCount: 34},
			},
		},
	}

	openAIResponse := gemini.ResponseChat2OpenAIForTest(meta, response)
	assert.Len(t, openAIResponse.Choices, 1)

	assert.Equal(t, "", openAIResponse.Choices[0].Message.Content)
	assert.NotNil(t, openAIResponse.Choices[0].Message.Audio)
	assert.Equal(t, audioData, openAIResponse.Choices[0].Message.Audio.Data)
	assert.NotNil(t, openAIResponse.Usage)
	assert.Equal(t, int64(34), openAIResponse.Usage.CompletionTokensDetails.AudioTokens)
	assert.Equal(
		t,
		model.ZeroNullInt64(34),
		openAIResponse.Usage.ToModelUsage().AudioOutputTokens,
	)
}

func TestStreamResponseChat2OpenAIConvertsAudioInlineDataToDeltaAudio(t *testing.T) {
	t.Parallel()

	meta := meta.NewMeta(
		&model.Channel{Type: model.ChannelTypeGoogleGemini},
		mode.ChatCompletions,
		"gemini-2.5-flash-tts",
		model.ModelConfig{},
	)
	audioData := base64.StdEncoding.EncodeToString([]byte("audio bytes"))
	response := &relaymodel.GeminiChatResponse{
		Candidates: []*relaymodel.GeminiChatCandidate{
			{
				Content: relaymodel.GeminiChatContent{
					Parts: []*relaymodel.GeminiPart{
						{
							InlineData: &relaymodel.GeminiInlineData{
								MimeType: "audio/wav",
								Data:     audioData,
							},
						},
					},
				},
			},
		},
	}

	openAIChunk := gemini.StreamResponseChat2OpenAIForTest(meta, response)
	assert.Len(t, openAIChunk.Choices, 1)
	assert.Equal(t, "", openAIChunk.Choices[0].Delta.Content)
	assert.NotNil(t, openAIChunk.Choices[0].Delta.Audio)
	assert.Equal(t, audioData, openAIChunk.Choices[0].Delta.Audio.Data)
}
