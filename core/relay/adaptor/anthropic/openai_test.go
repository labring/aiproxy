package anthropic_test

import (
	"testing"

	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/smartystreets/goconvey/convey"
)

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
