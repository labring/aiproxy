package gemini_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

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

	jsonData, _ := json.Marshal(openAIReq)
	req, _ := http.NewRequest(http.MethodPost, "http://localhost/v1/chat/completions", bytes.NewBuffer(jsonData))

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

	jsonData, _ := json.Marshal(openAIReq)
	req, _ := http.NewRequest(http.MethodPost, "http://localhost/v1/chat/completions", bytes.NewBuffer(jsonData))

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

	schema, ok := geminiReq.GenerationConfig.ResponseSchema.(map[string]any)
	assert.True(t, ok)

	// Check if unsupported fields are removed
	_, hasSchema := schema["$schema"]
	assert.False(t, hasSchema, "Expected $schema to be removed")
	_, hasAdditionalProperties := schema["additionalProperties"]
	assert.False(t, hasAdditionalProperties, "Expected additionalProperties to be removed")
}
