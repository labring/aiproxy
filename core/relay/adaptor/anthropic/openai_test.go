package anthropic_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelDefaultMaxTokens(t *testing.T) {
	convey.Convey("ModelDefaultMaxTokens", t, func() {
		convey.So(
			anthropic.ModelDefaultMaxTokens("claude-sonnet-4-20250514"),
			convey.ShouldEqual,
			64000,
		)
		convey.So(
			anthropic.ModelDefaultMaxTokens("claude-sonnet-4-5-20250929"),
			convey.ShouldEqual,
			64000,
		)
		convey.So(
			anthropic.ModelDefaultMaxTokens("claude-opus-4-1-20250805"),
			convey.ShouldEqual,
			204800,
		)
	})
}

func TestOpenAIConvertRequest_DefaultMaxTokensForSonnet4(t *testing.T) {
	convey.Convey("OpenAIConvertRequest default max_tokens for Sonnet 4", t, func() {
		m := &meta.Meta{
			ActualModel: "claude-sonnet-4-20250514",
			OriginModel: "claude-sonnet-4-20250514",
			Mode:        mode.ChatCompletions,
		}

		reqBody := relaymodel.GeneralOpenAIRequest{
			Model: "claude-sonnet-4-20250514",
			Messages: []relaymodel.Message{
				{
					Role:    "user",
					Content: "hello",
				},
			},
		}

		data, err := sonic.Marshal(reqBody)
		convey.So(err, convey.ShouldBeNil)

		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://localhost/v1/chat/completions",
			bytes.NewBuffer(data),
		)
		convey.So(err, convey.ShouldBeNil)

		claudeReq, err := anthropic.OpenAIConvertRequest(m, req)
		convey.So(err, convey.ShouldBeNil)
		convey.So(claudeReq.MaxTokens, convey.ShouldEqual, 64000)

		marshaled, err := json.Marshal(claudeReq)
		convey.So(err, convey.ShouldBeNil)
		convey.So(string(marshaled), convey.ShouldContainSubstring, `"max_tokens":64000`)
	})
}

func TestStreamResponse2OpenAI(t *testing.T) {
	convey.Convey("StreamResponse2OpenAI", t, func() {
		streamState := anthropic.NewStreamState()
		m := &meta.Meta{
			OriginModel: "claude-3-7-sonnet-20250219",
		}

		convey.Convey("should handle signature_delta", func() {
			data := []byte(`{
				"type": "content_block_delta",
				"index": 0,
				"delta": {
					"type": "signature_delta",
					"signature": "test_signature"
				}
			}`)

			resp, err := streamState.StreamResponse2OpenAI(m, data)
			convey.So(err, convey.ShouldBeNil)
			convey.So(resp.Choices[0].Delta.Signature, convey.ShouldEqual, "test_signature")
		})

		convey.Convey("should handle thinking_delta", func() {
			data := []byte(`{
				"type": "content_block_delta",
				"index": 0,
				"delta": {
					"type": "thinking_delta",
					"thinking": "I am thinking"
				}
			}`)

			resp, err := streamState.StreamResponse2OpenAI(m, data)
			convey.So(err, convey.ShouldBeNil)
			convey.So(resp.Choices[0].Delta.ReasoningContent, convey.ShouldEqual, "I am thinking")
		})

		convey.Convey("should capture upstream ID from message_start", func() {
			data := []byte(`{
				"type": "message_start",
				"message": {
					"id": "msg_test_123",
					"type": "message",
					"role": "assistant",
					"model": "claude-3-7-sonnet-20250219",
					"usage": {
						"input_tokens": 10,
						"output_tokens": 0
					}
				}
			}`)

			resp, err := streamState.StreamResponse2OpenAI(m, data)
			convey.So(err, convey.ShouldBeNil)
			convey.So(resp.ID, convey.ShouldEqual, "msg_test_123")
		})
	})
}

func TestResponse2OpenAI(t *testing.T) {
	convey.Convey("Response2OpenAI", t, func() {
		m := &meta.Meta{
			OriginModel: "claude-3-7-sonnet-20250219",
		}

		convey.Convey("should handle thinking and signature in content", func() {
			data := []byte(`{
				"id": "msg_123",
				"type": "message",
				"role": "assistant",
				"model": "claude-3-7-sonnet-20250219",
				"content": [
					{
						"type": "thinking",
						"thinking": "I am thinking...",
						"signature": "test_signature_block"
					},
					{
						"type": "text",
						"text": "Hello"
					}
				],
				"usage": {
					"input_tokens": 10,
					"output_tokens": 20
				}
			}`)

			resp, err := anthropic.Response2OpenAI(m, data)
			convey.So(err, convey.ShouldBeNil)
			convey.So(resp.ID, convey.ShouldEqual, "msg_123")
			convey.So(
				resp.Choices[0].Message.ReasoningContent,
				convey.ShouldEqual,
				"I am thinking...",
			)
			convey.So(resp.Choices[0].Message.Signature, convey.ShouldEqual, "test_signature_block")
			convey.So(resp.Choices[0].Message.Content, convey.ShouldEqual, "Hello")
		})
	})
}

func TestOpenAIConvertRequest_DisableAutoImageURLToBase64(t *testing.T) {
	channel := &model.Channel{
		Configs: model.ChannelConfigs{
			"disable_auto_image_url_to_base64": true,
		},
	}
	m := meta.NewMeta(
		channel,
		mode.ChatCompletions,
		"claude-sonnet-4-20250514",
		model.ModelConfig{},
	)

	reqBody := relaymodel.GeneralOpenAIRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []relaymodel.Message{
			{
				Role: "user",
				Content: []relaymodel.MessageContent{
					{
						Type: relaymodel.ContentTypeImageURL,
						ImageURL: &relaymodel.ImageURL{
							URL: "https://example.com/test.png",
						},
					},
				},
			},
		},
	}

	data, err := sonic.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(data),
	)
	require.NoError(t, err)

	claudeReq, err := anthropic.OpenAIConvertRequest(m, req)
	require.NoError(t, err)
	require.Len(t, claudeReq.Messages, 1)
	require.Len(t, claudeReq.Messages[0].Content, 1)
	require.NotNil(t, claudeReq.Messages[0].Content[0].Source)
	require.Equal(
		t,
		relaymodel.ClaudeImageSourceTypeURL,
		claudeReq.Messages[0].Content[0].Source.Type,
	)
	require.Equal(t, "https://example.com/test.png", claudeReq.Messages[0].Content[0].Source.URL)
}

func TestOpenAIConvertRequest_AdaptiveThinkingModels(t *testing.T) {
	t.Run("opus 4.7 rewrites enabled thinking to adaptive", func(t *testing.T) {
		m := &meta.Meta{
			ActualModel: "claude-opus-4-7",
			OriginModel: "claude-opus-4-7",
			Mode:        mode.ChatCompletions,
		}

		reqBody := relaymodel.GeneralOpenAIRequest{
			Model: "claude-opus-4-7",
			Messages: []relaymodel.Message{
				{Role: relaymodel.RoleUser, Content: "hello"},
			},
			ReasoningEffort: new("low"),
		}

		data, err := sonic.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://localhost/v1/chat/completions",
			bytes.NewBuffer(data),
		)
		require.NoError(t, err)

		claudeReq, err := anthropic.OpenAIConvertRequest(m, req)
		require.NoError(t, err)
		require.NotNil(t, claudeReq.Thinking)
		assert.Equal(t, relaymodel.ClaudeThinkingTypeAdaptive, claudeReq.Thinking.Type)
		assert.Zero(t, claudeReq.Thinking.BudgetTokens)
		require.NotNil(t, claudeReq.OutputConfig)
		require.NotNil(t, claudeReq.OutputConfig.Effort)
		assert.Equal(t, "low", *claudeReq.OutputConfig.Effort)
	})

	t.Run("sonnet 4.5 keeps budget thinking", func(t *testing.T) {
		m := &meta.Meta{
			ActualModel: "claude-sonnet-4-5",
			OriginModel: "claude-sonnet-4-5",
			Mode:        mode.ChatCompletions,
		}

		reqBody := relaymodel.GeneralOpenAIRequest{
			Model: "claude-sonnet-4-5",
			Messages: []relaymodel.Message{
				{Role: relaymodel.RoleUser, Content: "hello"},
			},
			ReasoningEffort: new("low"),
		}

		data, err := sonic.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://localhost/v1/chat/completions",
			bytes.NewBuffer(data),
		)
		require.NoError(t, err)

		claudeReq, err := anthropic.OpenAIConvertRequest(m, req)
		require.NoError(t, err)
		require.NotNil(t, claudeReq.Thinking)
		assert.Equal(t, relaymodel.ClaudeThinkingTypeEnabled, claudeReq.Thinking.Type)
		assert.Equal(t, 2048, claudeReq.Thinking.BudgetTokens)
		assert.Nil(t, claudeReq.OutputConfig)
	})

	t.Run("claude 3.7 sonnet keeps budget thinking", func(t *testing.T) {
		m := &meta.Meta{
			ActualModel: "claude-3-7-sonnet-20250219",
			OriginModel: "claude-3-7-sonnet-20250219",
			Mode:        mode.ChatCompletions,
		}

		reqBody := relaymodel.GeneralOpenAIRequest{
			Model: "claude-3-7-sonnet-20250219",
			Messages: []relaymodel.Message{
				{Role: relaymodel.RoleUser, Content: "hello"},
			},
			ReasoningEffort: new("medium"),
		}

		data, err := sonic.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://localhost/v1/chat/completions",
			bytes.NewBuffer(data),
		)
		require.NoError(t, err)

		claudeReq, err := anthropic.OpenAIConvertRequest(m, req)
		require.NoError(t, err)
		require.NotNil(t, claudeReq.Thinking)
		assert.Equal(t, relaymodel.ClaudeThinkingTypeEnabled, claudeReq.Thinking.Type)
		assert.Equal(t, 8192, claudeReq.Thinking.BudgetTokens)
		assert.Nil(t, claudeReq.OutputConfig)
	})

	t.Run("future sonnet defaults to adaptive", func(t *testing.T) {
		m := &meta.Meta{
			ActualModel: "claude-sonnet-5-0",
			OriginModel: "claude-sonnet-5-0",
			Mode:        mode.ChatCompletions,
		}

		reqBody := relaymodel.GeneralOpenAIRequest{
			Model: "claude-sonnet-5-0",
			Messages: []relaymodel.Message{
				{Role: relaymodel.RoleUser, Content: "hello"},
			},
			ReasoningEffort: new("high"),
		}

		data, err := sonic.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://localhost/v1/chat/completions",
			bytes.NewBuffer(data),
		)
		require.NoError(t, err)

		claudeReq, err := anthropic.OpenAIConvertRequest(m, req)
		require.NoError(t, err)
		require.NotNil(t, claudeReq.Thinking)
		assert.Equal(t, relaymodel.ClaudeThinkingTypeAdaptive, claudeReq.Thinking.Type)
		assert.Zero(t, claudeReq.Thinking.BudgetTokens)
		require.NotNil(t, claudeReq.OutputConfig)
		require.NotNil(t, claudeReq.OutputConfig.Effort)
		assert.Equal(t, "high", *claudeReq.OutputConfig.Effort)
	})

	t.Run("mythos removes unsupported disabled thinking", func(t *testing.T) {
		m := &meta.Meta{
			ActualModel: "claude-mythos-preview",
			OriginModel: "claude-mythos-preview",
			Mode:        mode.ChatCompletions,
		}

		reqBody := relaymodel.GeneralOpenAIRequest{
			Model: "claude-mythos-preview",
			Messages: []relaymodel.Message{
				{Role: relaymodel.RoleUser, Content: "hello"},
			},
			ReasoningEffort: new("none"),
		}

		data, err := sonic.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://localhost/v1/chat/completions",
			bytes.NewBuffer(data),
		)
		require.NoError(t, err)

		claudeReq, err := anthropic.OpenAIConvertRequest(m, req)
		require.NoError(t, err)
		assert.Nil(t, claudeReq.Thinking)
	})
}

func TestConvertRequestToBytes_PreservesNativeThinking(t *testing.T) {
	m := &meta.Meta{
		ActualModel: "claude-opus-4-7",
		OriginModel: "claude-opus-4-7",
		Mode:        mode.Anthropic,
	}

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

	normalized, err := anthropic.ConvertRequestToBytes(m, req)
	require.NoError(t, err)

	var converted map[string]any
	require.NoError(t, sonic.Unmarshal(normalized, &converted))

	thinking, ok := converted["thinking"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "enabled", thinking["type"])
	assert.Equal(t, float64(2048), thinking["budget_tokens"])
}

func TestOpenAIConvertRequest_Claude3SonnetOldIDKeepsBudgetThinking(t *testing.T) {
	m := &meta.Meta{
		ActualModel: "claude-3-sonnet-20240229",
		OriginModel: "claude-3-sonnet-20240229",
		Mode:        mode.ChatCompletions,
	}

	reqBody := relaymodel.GeneralOpenAIRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []relaymodel.Message{
			{Role: relaymodel.RoleUser, Content: "hello"},
		},
		ReasoningEffort: new("low"),
	}

	data, err := sonic.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(data),
	)
	require.NoError(t, err)

	claudeReq, err := anthropic.OpenAIConvertRequest(m, req)
	require.NoError(t, err)
	require.NotNil(t, claudeReq.Thinking)
	assert.Equal(t, relaymodel.ClaudeThinkingTypeEnabled, claudeReq.Thinking.Type)
	assert.Equal(t, 2048, claudeReq.Thinking.BudgetTokens)
	assert.Nil(t, claudeReq.OutputConfig)
}

func TestOpenAIConvertRequest_ThinkingBudgetRaisesMaxTokens(t *testing.T) {
	m := &meta.Meta{
		ActualModel: "claude-3-7-sonnet-20250219",
		OriginModel: "claude-3-7-sonnet-20250219",
		Mode:        mode.ChatCompletions,
	}

	reqBody := relaymodel.ClaudeOpenAIRequest{
		Model:           "claude-3-7-sonnet-20250219",
		MaxTokens:       1000,
		ReasoningEffort: new("minimal"),
		Messages: []*relaymodel.ClaudeOpenaiMessage{
			{
				Message: relaymodel.Message{
					Role:    relaymodel.RoleUser,
					Content: "hello",
				},
			},
		},
	}

	data, err := sonic.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(data),
	)
	require.NoError(t, err)

	claudeReq, err := anthropic.OpenAIConvertRequest(m, req)
	require.NoError(t, err)
	require.NotNil(t, claudeReq.Thinking)
	assert.Equal(t, relaymodel.ClaudeThinkingTypeEnabled, claudeReq.Thinking.Type)
	assert.Equal(t, 1024, claudeReq.Thinking.BudgetTokens)
	assert.Equal(t, 2048, claudeReq.MaxTokens)
}

func TestOpenAIConvertRequest_UsesOriginModelNameFirst(t *testing.T) {
	m := &meta.Meta{
		ActualModel: "mapped-upstream-model",
		OriginModel: "claude-opus-4-7",
		Mode:        mode.ChatCompletions,
	}

	reqBody := relaymodel.ClaudeOpenAIRequest{
		Model:           "claude-opus-4-7",
		ReasoningEffort: new("low"),
		Messages: []*relaymodel.ClaudeOpenaiMessage{
			{
				Message: relaymodel.Message{
					Role:    relaymodel.RoleUser,
					Content: "hello",
				},
			},
		},
	}

	data, err := sonic.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1/chat/completions",
		bytes.NewBuffer(data),
	)
	require.NoError(t, err)

	claudeReq, err := anthropic.OpenAIConvertRequest(m, req)
	require.NoError(t, err)
	require.NotNil(t, claudeReq.Thinking)
	assert.Equal(t, relaymodel.ClaudeThinkingTypeAdaptive, claudeReq.Thinking.Type)
	require.NotNil(t, claudeReq.OutputConfig)
	require.NotNil(t, claudeReq.OutputConfig.Effort)
	assert.Equal(t, "low", *claudeReq.OutputConfig.Effort)
}
