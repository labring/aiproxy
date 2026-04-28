//nolint:testpackage
package deepseek

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepseekGetRequestURLAnthropic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	a := &Adaptor{}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/messages?beta=true",
		nil,
	)

	testCases := []struct {
		name    string
		baseURL string
		wantURL string
	}{
		{
			name:    "empty base uses official anthropic base",
			baseURL: "",
			wantURL: "https://api.deepseek.com/anthropic/v1/messages?beta=true",
		},
		{
			name:    "openai compatible base",
			baseURL: baseURL,
			wantURL: "https://api.deepseek.com/anthropic/v1/messages?beta=true",
		},
		{
			name:    "official root base",
			baseURL: "https://api.deepseek.com",
			wantURL: "https://api.deepseek.com/anthropic/v1/messages?beta=true",
		},
		{
			name:    "anthropic base appends v1",
			baseURL: anthropicBaseURL,
			wantURL: "https://api.deepseek.com/anthropic/v1/messages?beta=true",
		},
		{
			name:    "anthropic v1 base kept as is",
			baseURL: "https://api.deepseek.com/anthropic/v1",
			wantURL: "https://api.deepseek.com/anthropic/v1/messages?beta=true",
		},
		{
			name:    "proxy base preserves host",
			baseURL: "https://xxx.proxyxxx.com/v1",
			wantURL: "https://xxx.proxyxxx.com/anthropic/v1/messages?beta=true",
		},
		{
			name:    "proxy base preserves prefix",
			baseURL: "https://xxx.proxyxxx.com/deepseek/v1",
			wantURL: "https://xxx.proxyxxx.com/deepseek/anthropic/v1/messages?beta=true",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqURL, err := a.GetRequestURL(&meta.Meta{
				Mode: mode.Anthropic,
				Channel: meta.ChannelMeta{
					BaseURL: tc.baseURL,
				},
			}, nil, ctx)

			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, reqURL.Method)
			assert.Equal(t, tc.wantURL, reqURL.URL)
		})
	}
}

func TestDeepseekGetRequestURLOpenAIModes(t *testing.T) {
	a := &Adaptor{}

	testCases := []struct {
		name    string
		mode    mode.Mode
		baseURL string
		wantURL string
	}{
		{
			name:    "chat completions official base",
			mode:    mode.ChatCompletions,
			baseURL: baseURL,
			wantURL: "https://api.deepseek.com/v1/chat/completions",
		},
		{
			name:    "chat completions proxy base",
			mode:    mode.ChatCompletions,
			baseURL: "https://xxx.proxyxxx.com/deepseek/v1",
			wantURL: "https://xxx.proxyxxx.com/deepseek/v1/chat/completions",
		},
		{
			name:    "completions official base",
			mode:    mode.Completions,
			baseURL: baseURL,
			wantURL: "https://api.deepseek.com/v1/completions",
		},
		{
			name:    "gemini official base",
			mode:    mode.Gemini,
			baseURL: baseURL,
			wantURL: "https://api.deepseek.com/v1/chat/completions",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqURL, err := a.GetRequestURL(&meta.Meta{
				Mode: tc.mode,
				Channel: meta.ChannelMeta{
					BaseURL: tc.baseURL,
				},
			}, nil, nil)

			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, reqURL.Method)
			assert.Equal(t, tc.wantURL, reqURL.URL)
		})
	}
}

func TestDeepseekSetupRequestHeaderAnthropic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	a := &Adaptor{}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/messages",
		nil,
	)
	ctx.Request.Header.Set("Anthropic-Version", "2023-06-01")
	ctx.Request.Header.Add(anthropic.AnthropicBeta, "token-efficient-tools-2025-02-19")
	ctx.Request.Header.Add(anthropic.AnthropicBeta, "context-management-2025-06-27")

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://api.deepseek.com/anthropic/v1/messages",
		nil,
	)

	err := a.SetupRequestHeader(&meta.Meta{
		Mode: mode.Anthropic,
		Channel: meta.ChannelMeta{
			Key: "test-key",
		},
	}, nil, ctx, req)

	require.NoError(t, err)
	assert.Empty(t, req.Header.Get("Authorization"))
	assert.Equal(t, "test-key", req.Header.Get(anthropic.AnthropicTokenHeader))
	assert.Equal(t, "2023-06-01", req.Header.Get("Anthropic-Version"))
	assert.Equal(
		t,
		"token-efficient-tools-2025-02-19,context-management-2025-06-27",
		req.Header.Get(anthropic.AnthropicBeta),
	)
}

func TestDeepseekConvertRequestReasoning(t *testing.T) {
	adaptor := &Adaptor{}

	t.Run("chat reasoning_effort maps to thinking", func(t *testing.T) {
		m := meta.NewMeta(nil, mode.ChatCompletions, "deepseek-chat", coremodel.ModelConfig{})
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"/v1/chat/completions",
			strings.NewReader(`{
				"model":"deepseek-chat",
				"reasoning_effort":"high",
				"messages":[{"role":"user","content":"hello"}]
			}`),
		)
		require.NoError(t, err)

		result, err := adaptor.ConvertRequest(m, nil, req)
		require.NoError(t, err)

		body, err := io.ReadAll(result.Body)
		require.NoError(t, err)

		var openAIReq relaymodel.GeneralOpenAIRequest
		require.NoError(t, json.Unmarshal(body, &openAIReq))
		require.NotNil(t, openAIReq.Thinking)
		assert.Equal(t, relaymodel.ClaudeThinkingTypeEnabled, openAIReq.Thinking.Type)
		assert.Nil(t, openAIReq.ReasoningEffort)
	})

	t.Run("chat reasoning_effort none disables thinking", func(t *testing.T) {
		m := meta.NewMeta(nil, mode.ChatCompletions, "deepseek-chat", coremodel.ModelConfig{})
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"/v1/chat/completions",
			strings.NewReader(`{
				"model":"deepseek-chat",
				"reasoning_effort":"none",
				"messages":[{"role":"user","content":"hello"}]
			}`),
		)
		require.NoError(t, err)

		result, err := adaptor.ConvertRequest(m, nil, req)
		require.NoError(t, err)

		body, err := io.ReadAll(result.Body)
		require.NoError(t, err)

		var openAIReq relaymodel.GeneralOpenAIRequest
		require.NoError(t, json.Unmarshal(body, &openAIReq))
		require.NotNil(t, openAIReq.Thinking)
		assert.Equal(t, relaymodel.ClaudeThinkingTypeDisabled, openAIReq.Thinking.Type)
		assert.Nil(t, openAIReq.ReasoningEffort)
	})

	t.Run("gemini thinking maps to deepseek thinking", func(t *testing.T) {
		m := meta.NewMeta(nil, mode.Gemini, "deepseek-chat", coremodel.ModelConfig{})
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"/v1beta/models/gemini-pro:generateContent",
			strings.NewReader(`{
				"generationConfig": {
					"thinkingConfig": {
						"thinkingBudget": 2048,
						"includeThoughts": true
					}
				},
				"contents":[{"role":"user","parts":[{"text":"hello"}]}]
			}`),
		)
		require.NoError(t, err)

		result, err := adaptor.ConvertRequest(m, nil, req)
		require.NoError(t, err)

		body, err := io.ReadAll(result.Body)
		require.NoError(t, err)

		var openAIReq relaymodel.GeneralOpenAIRequest
		require.NoError(t, json.Unmarshal(body, &openAIReq))
		require.NotNil(t, openAIReq.Thinking)
		assert.Equal(t, relaymodel.ClaudeThinkingTypeEnabled, openAIReq.Thinking.Type)
		assert.Nil(t, openAIReq.ReasoningEffort)
	})
}
