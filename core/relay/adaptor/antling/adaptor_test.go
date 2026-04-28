package antling_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/antling"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAntLingChannelTypeNameToType(t *testing.T) {
	assert.Equal(t, int(model.ChannelTypeAntLing), model.ChannelTypeNameToType("antling"))
	assert.Equal(t, int(model.ChannelTypeAntLing), model.ChannelTypeNameToType("ant ling"))
	assert.Equal(t, int(model.ChannelTypeAntLing), model.ChannelTypeNameToType("蚂蚁百灵"))
}

func TestAntLingAdaptorMetadata(t *testing.T) {
	a := &antling.Adaptor{}
	meta := a.Metadata()

	assert.Equal(t, "https://api.tbox.cn/api", a.DefaultBaseURL())
	assert.NotEmpty(t, meta.Readme)
	assert.NotEmpty(t, meta.Models)
}

func TestAntLingAdaptorSupportMode(t *testing.T) {
	a := &antling.Adaptor{}

	assert.True(t, a.SupportMode(&meta.Meta{Mode: mode.ChatCompletions}))
	assert.True(t, a.SupportMode(&meta.Meta{Mode: mode.Anthropic}))
	assert.True(t, a.SupportMode(&meta.Meta{Mode: mode.Gemini}))
	assert.False(t, a.SupportMode(&meta.Meta{Mode: mode.Completions}))
	assert.False(t, a.SupportMode(&meta.Meta{Mode: mode.Embeddings}))
	assert.False(t, a.SupportMode(&meta.Meta{Mode: mode.Responses}))
}

func TestAntLingGetRequestURLAnthropic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	a := &antling.Adaptor{}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/messages?beta=code",
		nil,
	)

	reqURL, err := a.GetRequestURL(&meta.Meta{
		Mode: mode.Anthropic,
		Channel: meta.ChannelMeta{
			BaseURL: "https://api.tbox.cn/api",
		},
	}, nil, ctx)

	require.NoError(t, err)
	assert.Equal(t, "https://api.tbox.cn/api/anthropic/v1/messages?beta=code", reqURL.URL)
	assert.Equal(t, http.MethodPost, reqURL.Method)
}

func TestAntLingGetRequestURLChat(t *testing.T) {
	a := &antling.Adaptor{}
	reqURL, err := a.GetRequestURL(&meta.Meta{
		Mode: mode.ChatCompletions,
		Channel: meta.ChannelMeta{
			BaseURL: "https://api.tbox.cn/api",
		},
	}, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "https://api.tbox.cn/api/llm/v1/chat/completions", reqURL.URL)
	assert.Equal(t, http.MethodPost, reqURL.Method)
}

func TestAntLingSetupRequestHeaderAnthropic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	a := &antling.Adaptor{}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/messages",
		nil,
	)
	ctx.Request.Header.Set("Anthropic-Version", "2023-06-01")
	ctx.Request.Header.Set("Anthropic-Beta", "test-beta")

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://api.tbox.cn/api/anthropic/v1/messages",
		nil,
	)
	err := a.SetupRequestHeader(&meta.Meta{
		Mode:        mode.Anthropic,
		ActualModel: "Ling-1T",
		Channel: meta.ChannelMeta{
			Key: "test-key",
		},
	}, nil, ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))
	assert.Equal(t, "test-key", req.Header.Get("X-Api-Key"))
	assert.Equal(t, "2023-06-01", req.Header.Get("Anthropic-Version"))
	assert.Equal(t, "test-beta", req.Header.Get("Anthropic-Beta"))
}
