package fakeerror_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/fakeerror"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type closeTracker struct {
	io.Reader
	closed bool
}

func (c *closeTracker) Close() error {
	c.closed = true
	return nil
}

func TestFakeErrorChannelTypeNameToType(t *testing.T) {
	assert.Equal(t, int(model.ChannelTypeFakeError), model.ChannelTypeNameToType("fake-error"))
	assert.Equal(t, int(model.ChannelTypeFakeError), model.ChannelTypeNameToType("fake error"))
	assert.Equal(t, int(model.ChannelTypeFakeError), model.ChannelTypeNameToType("fakeerror"))
}

func TestFakeErrorAdaptorDoResponseOpenAI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	adaptor := &fakeerror.Adaptor{}
	m := meta.NewMeta(
		&model.Channel{Type: model.ChannelTypeFakeError, BaseURL: "http://fake-error.local"},
		mode.ChatCompletions,
		"fake-chat",
		model.ModelConfig{},
	)

	body := &closeTracker{Reader: bytes.NewReader([]byte(`{"ok":true}`))}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}

	result, err := adaptor.DoResponse(m, nil, ctx, resp)
	require.Empty(t, result)
	require.Error(t, err)
	require.True(t, body.closed)
	require.Equal(t, http.StatusBadRequest, err.StatusCode())

	data, marshalErr := err.MarshalJSON()
	require.NoError(t, marshalErr)

	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}

	require.NoError(t, json.Unmarshal(data, &payload))
	assert.Equal(t, "fake_error", payload.Error.Code)
	assert.Equal(t, "fake error", payload.Error.Message)
	assert.Equal(t, "upstream_error", payload.Error.Type)
}

func TestFakeErrorAdaptorDoResponseAnthropic(t *testing.T) {
	adaptor := &fakeerror.Adaptor{}
	m := meta.NewMeta(
		&model.Channel{Type: model.ChannelTypeFakeError, BaseURL: "http://fake-error.local"},
		mode.Anthropic,
		"fake-anthropic",
		model.ModelConfig{},
	)

	_, err := adaptor.DoResponse(m, nil, nil, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(nil)),
	})
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, err.StatusCode())

	data, marshalErr := err.MarshalJSON()
	require.NoError(t, marshalErr)

	var payload struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	require.NoError(t, json.Unmarshal(data, &payload))
	assert.Equal(t, "error", payload.Type)
	assert.Equal(t, "upstream_error", payload.Error.Type)
	assert.Equal(t, "fake error", payload.Error.Message)
}

func TestFakeErrorAdaptorDoResponseUsesConfiguredOpenAIError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	adaptor := &fakeerror.Adaptor{}
	m := meta.NewMeta(
		&model.Channel{
			Type:    model.ChannelTypeFakeError,
			BaseURL: "http://fake-error.local",
			Configs: model.ChannelConfigs{
				"error_status_code": http.StatusTooManyRequests,
				"error_message":     "configured fake error",
				"error_code":        "configured_fake_error",
			},
		},
		mode.ChatCompletions,
		"fake-chat",
		model.ModelConfig{},
	)

	_, err := adaptor.DoResponse(m, nil, ctx, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(nil)),
	})
	require.Error(t, err)
	require.Equal(t, http.StatusTooManyRequests, err.StatusCode())

	data, marshalErr := err.MarshalJSON()
	require.NoError(t, marshalErr)

	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}

	require.NoError(t, json.Unmarshal(data, &payload))
	assert.Equal(t, "configured_fake_error", payload.Error.Code)
	assert.Equal(t, "configured fake error", payload.Error.Message)
	assert.Equal(t, "upstream_error", payload.Error.Type)
}

func TestFakeErrorAdaptorDoResponseUsesConfiguredAnthropicError(t *testing.T) {
	adaptor := &fakeerror.Adaptor{}
	m := meta.NewMeta(
		&model.Channel{
			Type:    model.ChannelTypeFakeError,
			BaseURL: "http://fake-error.local",
			Configs: model.ChannelConfigs{
				"error_status_code": http.StatusForbidden,
				"error_message":     "anthropic fake error",
			},
		},
		mode.Anthropic,
		"fake-anthropic",
		model.ModelConfig{},
	)

	_, err := adaptor.DoResponse(m, nil, nil, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(nil)),
	})
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, err.StatusCode())

	data, marshalErr := err.MarshalJSON()
	require.NoError(t, marshalErr)

	var payload struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	require.NoError(t, json.Unmarshal(data, &payload))
	assert.Equal(t, "error", payload.Type)
	assert.Equal(t, "upstream_error", payload.Error.Type)
	assert.Equal(t, "anthropic fake error", payload.Error.Message)
}
