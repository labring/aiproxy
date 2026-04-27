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

	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptorSupportModeGemini(t *testing.T) {
	adaptor := &Adaptor{}

	if !adaptor.SupportMode(mode.Gemini) {
		t.Fatal("expected Gemini mode to be supported")
	}
}

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
