package aws_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	coremodel "github.com/labring/aiproxy/core/model"
	awsa "github.com/labring/aiproxy/core/relay/adaptor/aws/claude"
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

	adaptor := &awsa.Adaptor{}
	_, err = adaptor.ConvertRequest(m, nil, req)
	require.NoError(t, err)

	converted, ok := m.Get(awsa.ConvertedRequest)
	require.True(t, ok)

	body, ok := converted.([]byte)
	require.True(t, ok)

	var awsReq map[string]any
	require.NoError(t, sonic.Unmarshal(body, &awsReq))

	thinking, ok := awsReq["thinking"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "enabled", thinking["type"])
	assert.Equal(t, float64(2048), thinking["budget_tokens"])

	_, hasOutputConfig := awsReq["output_config"]
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

	adaptor := &awsa.Adaptor{}
	_, err = adaptor.ConvertRequest(m, nil, req)
	require.NoError(t, err)

	converted, ok := m.Get(awsa.ConvertedRequest)
	require.True(t, ok)

	body, ok := converted.([]byte)
	require.True(t, ok)

	var awsReq map[string]any
	require.NoError(t, sonic.Unmarshal(body, &awsReq))

	thinking, ok := awsReq["thinking"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "adaptive", thinking["type"])

	outputConfig, hasOutputConfig := awsReq["output_config"].(map[string]any)
	require.True(t, hasOutputConfig)
	assert.Equal(t, "low", outputConfig["effort"])
}

func TestHandleAnthropicRequest_ContextManagementUsesResolvedModel(t *testing.T) {
	m := meta.NewMeta(nil, mode.Anthropic, "claude-sonnet-4-6", coremodel.ModelConfig{})
	m.ActualModel = "claude-opus-4-7"

	reqBody := map[string]any{
		"model":      "claude-sonnet-4-6",
		"max_tokens": 4096,
		"messages": []map[string]any{
			{"role": "user", "content": "hello"},
		},
		"context_management": map[string]any{
			"edits": []map[string]any{
				{"type": "clear_tool_uses_20250919"},
				{"type": "unsupported"},
			},
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

	adaptor := &awsa.Adaptor{}
	_, err = adaptor.ConvertRequest(m, nil, req)
	require.NoError(t, err)

	converted, ok := m.Get(awsa.ConvertedRequest)
	require.True(t, ok)

	body, ok := converted.([]byte)
	require.True(t, ok)

	var awsReq map[string]any
	require.NoError(t, sonic.Unmarshal(body, &awsReq))

	_, ok = awsReq["context_management"]
	assert.False(t, ok)
}
