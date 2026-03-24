package anthropic_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/smartystreets/goconvey/convey"
)

func TestModelDefaultMaxTokens(t *testing.T) {
	convey.Convey("ModelDefaultMaxTokens", t, func() {
		convey.So(anthropic.ModelDefaultMaxTokens("claude-sonnet-4-20250514"), convey.ShouldEqual, 64000)
		convey.So(anthropic.ModelDefaultMaxTokens("claude-sonnet-4-5-20250929"), convey.ShouldEqual, 64000)
		convey.So(anthropic.ModelDefaultMaxTokens("claude-opus-4-1-20250805"), convey.ShouldEqual, 204800)
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
