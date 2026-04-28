//nolint:testpackage
package minimax

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAPIKeyAndGroupID(t *testing.T) {
	t.Run("api key only", func(t *testing.T) {
		apiKey, groupID, err := GetAPIKeyAndGroupID("test-key")
		require.NoError(t, err)
		assert.Equal(t, "test-key", apiKey)
		assert.Empty(t, groupID)
	})

	t.Run("api key and group id", func(t *testing.T) {
		apiKey, groupID, err := GetAPIKeyAndGroupID("test-key|test-group")
		require.NoError(t, err)
		assert.Equal(t, "test-key", apiKey)
		assert.Equal(t, "test-group", groupID)
	})

	t.Run("empty api key rejected", func(t *testing.T) {
		_, _, err := GetAPIKeyAndGroupID("|test-group")
		require.Error(t, err)
	})
}

func TestMinimaxAdaptorSupportMode(t *testing.T) {
	a := &Adaptor{}

	assert.True(t, a.SupportMode(&meta.Meta{Mode: mode.ChatCompletions}))
	assert.True(t, a.SupportMode(&meta.Meta{Mode: mode.Embeddings}))
	assert.True(t, a.SupportMode(&meta.Meta{Mode: mode.AudioSpeech}))
	assert.True(t, a.SupportMode(&meta.Meta{Mode: mode.Anthropic}))
	assert.True(t, a.SupportMode(&meta.Meta{Mode: mode.Gemini}))
	assert.False(t, a.SupportMode(&meta.Meta{Mode: mode.Completions}))
	assert.False(t, a.SupportMode(&meta.Meta{Mode: mode.Responses}))
}

func TestMinimaxGetRequestURLAnthropic(t *testing.T) {
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
			name:    "new openai compatible base",
			baseURL: baseURL,
			wantURL: "https://api.minimaxi.com/anthropic/v1/messages?beta=true",
		},
		{
			name:    "legacy openai compatible base",
			baseURL: legacyBaseURL,
			wantURL: "https://api.minimaxi.com/anthropic/v1/messages?beta=true",
		},
		{
			name:    "anthropic base kept as is",
			baseURL: anthropicBaseURL,
			wantURL: "https://api.minimaxi.com/anthropic/v1/messages?beta=true",
		},
		{
			name:    "proxy base preserves host",
			baseURL: "https://xxx.proxyxxx.com/v1",
			wantURL: "https://xxx.proxyxxx.com/anthropic/v1/messages?beta=true",
		},
		{
			name:    "proxy base preserves prefix",
			baseURL: "https://xxx.proxyxxx.com/minimax/v1",
			wantURL: "https://xxx.proxyxxx.com/minimax/anthropic/v1/messages?beta=true",
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

func TestMinimaxGetRequestURLChat(t *testing.T) {
	a := &Adaptor{}

	testCases := []struct {
		name           string
		baseURL        string
		channelConfigs map[string]any
		wantURL        string
	}{
		{
			name:    "default base",
			baseURL: baseURL,
			wantURL: baseURL + "/text/chatcompletion_v2",
		},
		{
			name:    "legacy base",
			baseURL: legacyBaseURL,
			wantURL: "https://api.minimax.chat/v1/text/chatcompletion_v2",
		},
		{
			name:    "proxy base preserves host",
			baseURL: "https://xxx.proxyxxx.com/v1",
			wantURL: "https://xxx.proxyxxx.com/v1/text/chatcompletion_v2",
		},
		{
			name:    "proxy base preserves prefix",
			baseURL: "https://xxx.proxyxxx.com/minimax/v1",
			wantURL: "https://xxx.proxyxxx.com/minimax/v1/text/chatcompletion_v2",
		},
		{
			name:    "config switches to chat completions path",
			baseURL: "https://xxx.proxyxxx.com/minimax/v1",
			channelConfigs: map[string]any{
				"use_chat_completions_path": true,
			},
			wantURL: "https://xxx.proxyxxx.com/minimax/v1/chat/completions",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqURL, err := a.GetRequestURL(&meta.Meta{
				Mode: mode.ChatCompletions,
				Channel: meta.ChannelMeta{
					BaseURL: tc.baseURL,
				},
				ChannelConfigs: tc.channelConfigs,
			}, nil, nil)

			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, reqURL.Method)
			assert.Equal(t, tc.wantURL, reqURL.URL)
		})
	}
}

func TestMinimaxMetadataConfigSchema(t *testing.T) {
	a := &Adaptor{}
	metaInfo := a.Metadata()

	properties, ok := metaInfo.ConfigSchema["properties"].(map[string]any)
	require.True(t, ok)

	field, ok := properties["use_chat_completions_path"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "boolean", field["type"])
}

func TestMinimaxGetRequestURLEmbeddingsGroupIDOptional(t *testing.T) {
	a := &Adaptor{}

	t.Run("without group id", func(t *testing.T) {
		reqURL, err := a.GetRequestURL(&meta.Meta{
			Mode: mode.Embeddings,
			Channel: meta.ChannelMeta{
				BaseURL: baseURL,
				Key:     "test-key",
			},
		}, nil, nil)

		require.NoError(t, err)
		assert.Equal(t, baseURL+"/embeddings", reqURL.URL)
	})

	t.Run("with group id", func(t *testing.T) {
		reqURL, err := a.GetRequestURL(&meta.Meta{
			Mode: mode.Embeddings,
			Channel: meta.ChannelMeta{
				BaseURL: baseURL,
				Key:     "test-key|test-group",
			},
		}, nil, nil)

		require.NoError(t, err)
		assert.Equal(t, baseURL+"/embeddings?GroupId=test-group", reqURL.URL)
	})
}

func TestMinimaxSetupRequestHeaderAnthropic(t *testing.T) {
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
	ctx.Request.Header.Add("Anthropic-Beta", "token-efficient-tools-2025-02-19")
	ctx.Request.Header.Add("Anthropic-Beta", "context-management-2025-06-27")

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://api.minimaxi.com/anthropic/v1/messages",
		nil,
	)

	err := a.SetupRequestHeader(&meta.Meta{
		Mode:        mode.Anthropic,
		ActualModel: "MiniMax-M2.1",
		Channel: meta.ChannelMeta{
			Key: "test-key|test-group",
		},
	}, nil, ctx, req)

	require.NoError(t, err)
	assert.Empty(t, req.Header.Get("Authorization"))
	assert.Equal(t, "test-key", req.Header.Get(anthropic.AnthropicTokenHeader))
	assert.Equal(t, "2023-06-01", req.Header.Get("Anthropic-Version"))
	assert.Equal(
		t,
		"token-efficient-tools-2025-02-19,context-management-2025-06-27",
		req.Header.Get("Anthropic-Beta"),
	)
}
