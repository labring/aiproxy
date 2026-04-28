//nolint:testpackage
package qianfan

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
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptorSupportModeGemini(t *testing.T) {
	adaptor := &Adaptor{}

	if !adaptor.SupportMode(&meta.Meta{Mode: mode.Gemini}) {
		t.Fatal("expected Gemini mode to be supported")
	}
}

func TestAdaptorSupportModeResponses(t *testing.T) {
	adaptor := &Adaptor{}

	supportedModes := []mode.Mode{
		mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesInputItems,
	}
	for _, m := range supportedModes {
		if !adaptor.SupportMode(&meta.Meta{
			Mode:        m,
			OriginModel: "deepseek-v3.2",
			ActualModel: "mapped-model",
		}) {
			t.Fatalf("expected mode %s to be supported", m)
		}
	}

	if adaptor.SupportMode(&meta.Meta{Mode: mode.ResponsesCancel}) {
		t.Fatal("expected ResponsesCancel to be unsupported")
	}
}

func TestAdaptorSupportModeResponsesModelWhitelist(t *testing.T) {
	adaptor := &Adaptor{}

	if !adaptor.SupportMode(&meta.Meta{
		Mode:        mode.Responses,
		OriginModel: "unsupported-alias",
		ActualModel: "deepseek-v3.1-250821",
	}) {
		t.Fatal("expected Responses to be supported when actual model is whitelisted")
	}

	if adaptor.SupportMode(&meta.Meta{
		Mode:        mode.Responses,
		OriginModel: "ernie-4.5-turbo-128k",
		ActualModel: "ernie-4.5-turbo-128k",
	}) {
		t.Fatal("expected Responses to be unsupported for non-whitelisted model")
	}
}

func TestAdaptorSupportModeResponsesModelConfig(t *testing.T) {
	adaptor := &Adaptor{}

	if !adaptor.SupportMode(&meta.Meta{
		Mode:        mode.Responses,
		OriginModel: "custom-responses-model",
		ActualModel: "upstream-custom-model",
		ChannelConfigs: coremodel.ChannelConfigs{
			"response_models": []string{"custom-responses-model"},
		},
	}) {
		t.Fatal("expected Responses to be supported by channel response_models config")
	}
}

func TestAdaptorGetRequestURLResponses(t *testing.T) {
	adaptor := &Adaptor{}
	channel := &coremodel.Channel{BaseURL: "https://qianfan.baidubce.com/v2"}

	tests := []struct {
		name       string
		mode       mode.Mode
		responseID string
		wantMethod string
		wantURL    string
	}{
		{
			name:       "responses create",
			mode:       mode.Responses,
			wantMethod: http.MethodPost,
			wantURL:    "https://qianfan.baidubce.com/v2/responses",
		},
		{
			name:       "responses get",
			mode:       mode.ResponsesGet,
			responseID: "resp_123",
			wantMethod: http.MethodGet,
			wantURL:    "https://qianfan.baidubce.com/v2/responses/resp_123",
		},
		{
			name:       "responses delete",
			mode:       mode.ResponsesDelete,
			responseID: "resp_123",
			wantMethod: http.MethodDelete,
			wantURL:    "https://qianfan.baidubce.com/v2/responses/resp_123",
		},
		{
			name:       "responses input items",
			mode:       mode.ResponsesInputItems,
			responseID: "resp_123",
			wantMethod: http.MethodGet,
			wantURL:    "https://qianfan.baidubce.com/v2/responses/resp_123/input_items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := meta.NewMeta(
				channel,
				tt.mode,
				"ernie-4.5-turbo-128k",
				coremodel.ModelConfig{},
				meta.WithResponseID(tt.responseID),
			)

			got, err := adaptor.GetRequestURL(m, nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMethod, got.Method)
			assert.Equal(t, tt.wantURL, got.URL)
		})
	}
}

func TestAdaptorGetRequestURLResponsesCancelUnsupported(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://qianfan.baidubce.com/v2"},
		mode.ResponsesCancel,
		"ernie-4.5-turbo-128k",
		coremodel.ModelConfig{},
		meta.WithResponseID("resp_123"),
	)

	_, err := adaptor.GetRequestURL(m, nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported mode")
}

func TestAdaptorConvertRequestResponses(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Responses,
		"ernie-4.5-turbo-128k",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/responses",
		strings.NewReader(`{"model":"ernie-4.5-turbo-128k","input":"hello","stream":true}`),
	)
	require.NoError(t, err)

	result, err := adaptor.ConvertRequest(m, nil, req)
	require.NoError(t, err)

	body, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	var responseReq relaymodel.CreateResponseRequest
	require.NoError(t, json.Unmarshal(body, &responseReq))
	assert.Equal(t, "ernie-4.5-turbo-128k", responseReq.Model)
	assert.True(t, responseReq.Stream)
}

func TestAdaptorDoResponseResponsesDeleteNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	resp := &http.Response{
		StatusCode: http.StatusNoContent,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
	}

	_, err := adaptor.DoResponse(
		&meta.Meta{Mode: mode.ResponsesDelete},
		nil,
		ctx,
		resp,
	)

	require.Nil(t, err)
	assert.Equal(t, http.StatusNoContent, ctx.Writer.Status())
}

func TestAdaptorConvertRequestResponsesCancelUnsupported(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.ResponsesCancel,
		"ernie-4.5-turbo-128k",
		coremodel.ModelConfig{},
	)

	_, err := adaptor.ConvertRequest(m, nil, httptest.NewRequest(
		http.MethodPost,
		"/v1/responses/resp_123/cancel",
		nil,
	))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported mode")
}

var _ adaptor.Adaptor = (*Adaptor)(nil)

func TestAdaptorSetupRequestHeaderWithAppID(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{
			Key: "test-key",
			Configs: coremodel.ChannelConfigs{
				"appid": " app-test ",
			},
		},
		mode.ChatCompletions,
		"ernie-4.5-turbo-128k",
		coremodel.ModelConfig{},
	)
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://qianfan.baidubce.com/v2/chat/completions",
		nil,
	)

	err := adaptor.SetupRequestHeader(m, nil, nil, req)

	require.NoError(t, err)
	assert.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))
	assert.Equal(t, "app-test", req.Header.Get("Appid"))
}

func TestAdaptorSetupRequestHeaderWithoutAppID(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{
			Key: "test-key",
		},
		mode.ChatCompletions,
		"ernie-4.5-turbo-128k",
		coremodel.ModelConfig{},
	)
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://qianfan.baidubce.com/v2/chat/completions",
		nil,
	)

	err := adaptor.SetupRequestHeader(m, nil, nil, req)

	require.NoError(t, err)
	assert.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))
	assert.Empty(t, req.Header.Get("Appid"))
}

func TestAdaptorMetadataConfigSchema(t *testing.T) {
	adaptor := &Adaptor{}
	metaInfo := adaptor.Metadata()

	properties, ok := metaInfo.ConfigSchema["properties"].(map[string]any)
	require.True(t, ok)

	field, ok := properties["appid"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "string", field["type"])

	field, ok = properties["response_models"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "array", field["type"])
}

func TestAdaptorConvertRequestGemini(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		nil,
		mode.Gemini,
		"ernie-4.5-turbo-128k",
		coremodel.ModelConfig{},
	)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/ernie-4.5-turbo-128k:streamGenerateContent",
		strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	result, err := adaptor.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest returned error: %v", err)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read converted body: %v", err)
	}

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(body, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal converted body: %v", err)
	}

	if openAIReq.Model != "ernie-4.5-turbo-128k" {
		t.Fatalf("expected model ernie-4.5-turbo-128k, got %s", openAIReq.Model)
	}

	if !openAIReq.Stream {
		t.Fatal("expected stream to be enabled")
	}
}
