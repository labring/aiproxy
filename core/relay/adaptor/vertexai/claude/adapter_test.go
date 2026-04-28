package vertexai_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	coremodel "github.com/labring/aiproxy/core/model"
	vertexai "github.com/labring/aiproxy/core/relay/adaptor/vertexai/claude"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleAnthropicRequest_PreservesNativeThinking(t *testing.T) {
	m := meta.NewMeta(nil, mode.Anthropic, "claude-opus-4-7", coremodel.ModelConfig{})

	reqBody := map[string]any{
		"model":      "claude-opus-4-7",
		"max_tokens": 4096,
		"messages": []map[string]any{
			{"role": "user", "content": "hello"},
		},
		"thinking": map[string]any{
			"type":          "enabled",
			"budget_tokens": 2048,
		},
	}

	data, err := sonic.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/messages",
		bytes.NewBuffer(data),
	)
	require.NoError(t, err)

	adaptor := &vertexai.Adaptor{}
	result, err := adaptor.ConvertRequest(m, nil, req)
	require.NoError(t, err)

	converted, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	var vertexReq map[string]any
	require.NoError(t, sonic.Unmarshal(converted, &vertexReq))

	thinking, ok := vertexReq["thinking"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "enabled", thinking["type"])
	assert.Equal(t, float64(2048), thinking["budget_tokens"])

	_, hasOutputConfig := vertexReq["output_config"]
	assert.False(t, hasOutputConfig)
}

func TestHandleAnthropicRequest_PreservesLegacyThinkingForOldModels(t *testing.T) {
	m := meta.NewMeta(nil, mode.Anthropic, "claude-sonnet-4-5", coremodel.ModelConfig{})

	reqBody := map[string]any{
		"model":      "claude-sonnet-4-5",
		"max_tokens": 4096,
		"messages": []map[string]any{
			{"role": "user", "content": "hello"},
		},
		"thinking": map[string]any{
			"type": "adaptive",
		},
		"output_config": map[string]any{
			"effort": "low",
		},
	}

	data, err := sonic.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/messages",
		bytes.NewBuffer(data),
	)
	require.NoError(t, err)

	adaptor := &vertexai.Adaptor{}
	result, err := adaptor.ConvertRequest(m, nil, req)
	require.NoError(t, err)

	converted, err := io.ReadAll(result.Body)
	require.NoError(t, err)

	var vertexReq map[string]any
	require.NoError(t, sonic.Unmarshal(converted, &vertexReq))

	thinking, ok := vertexReq["thinking"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "adaptive", thinking["type"])

	outputConfig, hasOutputConfig := vertexReq["output_config"].(map[string]any)
	require.True(t, hasOutputConfig)
	assert.Equal(t, "low", outputConfig["effort"])
}
