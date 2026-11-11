package openai_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatUpstreamProtocolRejectsInvalidConfig(t *testing.T) {
	for _, tt := range []struct {
		name    string
		value   any
		message string
	}{
		{"unknown protocol", "unsupported", "invalid chat_upstream_protocol"},
		{"invalid type", true, "load channel config"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := &openai.Adaptor{}
			mt := meta.NewMeta(&model.Channel{
				BaseURL: "https://example.com/v1",
				Configs: model.ChannelConfigs{"chat_upstream_protocol": tt.value},
			}, mode.ChatCompletions, "gpt-4o", model.ModelConfig{})
			request := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/v1/chat/completions",
				strings.NewReader(
					`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`,
				),
			)
			_, err := a.ConvertRequest(mt, nil, request)
			require.ErrorContains(t, err, tt.message)
			_, err = a.GetRequestURL(mt, nil, nil)
			require.ErrorContains(t, err, tt.message)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())

			response := &http.Response{
				Header: make(http.Header),
				Body:   io.NopCloser(strings.NewReader(`{}`)),
			}
			defer response.Body.Close()

			_, responseErr := a.DoResponse(mt, nil, c, response)
			require.NotNil(t, responseErr)
		})
	}
}

func TestChatUpstreamProtocolPreservesOtherEndpoints(t *testing.T) {
	for _, tt := range []struct {
		name string
		mode mode.Mode
		body string
		path string
	}{
		{"legacy completions", mode.Completions, `{"model":"gpt-4o","prompt":"hello"}`, "/completions"},
		{"native responses", mode.Responses, `{"model":"gpt-4o","input":"hello"}`, "/responses"},
		{"compact", mode.ResponsesCompact, `{"model":"gpt-4o","input":[]}`, "/responses/compact"},
		{"embeddings", mode.Embeddings, `{"model":"gpt-4o","input":"hello"}`, "/embeddings"},
		{"anthropic", mode.Anthropic, `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"max_tokens":10}`, "/chat/completions"},
		{"gemini", mode.Gemini, `{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`, "/chat/completions"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := &openai.Adaptor{}

			var baseline any
			for _, protocol := range []string{"auto", "responses", "chat_completions"} {
				mt := meta.NewMeta(&model.Channel{
					BaseURL: "https://example.com/v1",
					Configs: model.ChannelConfigs{"chat_upstream_protocol": protocol},
				}, tt.mode, "gpt-4o", model.ModelConfig{})
				url, err := a.GetRequestURL(mt, nil, nil)
				require.NoError(t, err)
				assert.Equal(t, "https://example.com/v1"+tt.path, url.URL)
				request := httptest.NewRequestWithContext(
					t.Context(),
					http.MethodPost,
					"/",
					strings.NewReader(tt.body),
				)
				request.Header.Set("Content-Type", "application/json")
				converted, err := a.ConvertRequest(mt, nil, request)
				require.NoError(t, err)

				var body any
				require.NoError(t, json.NewDecoder(converted.Body).Decode(&body))

				if protocol == "auto" {
					baseline = body
				} else {
					assert.Equal(t, baseline, body)
				}
			}
		})
	}
}

func TestChatUpstreamProtocolSchema(t *testing.T) {
	metadata := (&openai.Adaptor{}).Metadata()
	properties, ok := metadata.ConfigSchema["properties"].(map[string]any)
	require.True(t, ok)
	field, ok := properties["chat_upstream_protocol"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "auto", field["default"])
	assert.Equal(t, []string{"auto", "responses", "chat_completions"}, field["enum"])
}

func TestSharedConfigSchemaDoesNotAdvertiseChatUpstreamProtocol(t *testing.T) {
	properties, ok := openai.ConfigSchema()["properties"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, properties, "chat_upstream_protocol")
}
