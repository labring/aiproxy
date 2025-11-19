package gemini_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor/gemini"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/smartystreets/goconvey/convey"
)

func TestClaudeHandler(t *testing.T) {
	convey.Convey("ClaudeHandler", t, func() {
		convey.Convey("should handle thinking with signature", func() {
			meta := &meta.Meta{
				OriginModel: "claude-3-5-sonnet-20240620",
			}

			response := &gemini.ChatResponse{
				Candidates: []*gemini.ChatCandidate{
					{
						Content: gemini.ChatContent{
							Parts: []*gemini.Part{
								{
									Text:             "Thinking process...",
									Thought:          true,
									ThoughtSignature: "signature_123",
								},
								{
									Text: "Final answer",
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			}

			respBody, _ := json.Marshal(response)
			httpResp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(respBody)),
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

			usage, handlerErr := gemini.ClaudeHandler(meta, c, httpResp)
			convey.So(handlerErr, convey.ShouldBeNil)
			convey.So(usage, convey.ShouldNotBeNil)

			var claudeResponse relaymodel.ClaudeResponse

			err := json.Unmarshal(w.Body.Bytes(), &claudeResponse)
			convey.So(err, convey.ShouldBeNil)

			convey.So(len(claudeResponse.Content), convey.ShouldEqual, 2)

			// Check thinking block
			convey.So(claudeResponse.Content[0].Type, convey.ShouldEqual, "thinking")
			convey.So(claudeResponse.Content[0].Thinking, convey.ShouldEqual, "Thinking process...")
			convey.So(claudeResponse.Content[0].Signature, convey.ShouldEqual, "signature_123")

			// Check text block
			convey.So(claudeResponse.Content[1].Type, convey.ShouldEqual, "text")
			convey.So(claudeResponse.Content[1].Text, convey.ShouldEqual, "Final answer")
		})

		convey.Convey("should handle tool call with signature", func() {
			meta := &meta.Meta{
				OriginModel: "claude-3-5-sonnet-20240620",
			}

			response := &gemini.ChatResponse{
				Candidates: []*gemini.ChatCandidate{
					{
						Content: gemini.ChatContent{
							Parts: []*gemini.Part{
								{
									FunctionCall: &gemini.FunctionCall{
										Name: "get_weather",
										Args: map[string]any{"location": "London"},
									},
									ThoughtSignature: "tool_signature_456",
								},
							},
						},
						FinishReason: "TOOL_CALLS",
					},
				},
			}

			respBody, _ := json.Marshal(response)
			httpResp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(respBody)),
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

			usage, handlerErr := gemini.ClaudeHandler(meta, c, httpResp)
			convey.So(handlerErr, convey.ShouldBeNil)
			convey.So(usage, convey.ShouldNotBeNil)

			var claudeResponse relaymodel.ClaudeResponse

			err := json.Unmarshal(w.Body.Bytes(), &claudeResponse)
			convey.So(err, convey.ShouldBeNil)

			convey.So(len(claudeResponse.Content), convey.ShouldEqual, 1)

			// Check tool use block
			convey.So(claudeResponse.Content[0].Type, convey.ShouldEqual, "tool_use")
			convey.So(claudeResponse.Content[0].Name, convey.ShouldEqual, "get_weather")
			convey.So(claudeResponse.Content[0].Signature, convey.ShouldEqual, "tool_signature_456")
		})
	})
}

func TestClaudeStreamHandler(t *testing.T) {
	convey.Convey("ClaudeStreamHandler", t, func() {
		convey.Convey("should handle thinking with signature in stream", func() {
			meta := &meta.Meta{
				OriginModel: "claude-3-5-sonnet-20240620",
			}

			// Prepare SSE stream response
			response := &gemini.ChatResponse{
				Candidates: []*gemini.ChatCandidate{
					{
						Content: gemini.ChatContent{
							Parts: []*gemini.Part{
								{
									Text:             "Thinking process...",
									Thought:          true,
									ThoughtSignature: "signature_stream_123",
								},
							},
						},
					},
				},
			}

			respData, _ := json.Marshal(response)
			streamBody := "data: " + string(respData) + "\n\n" + "data: [DONE]\n\n"

			httpResp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(streamBody))),
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

			_, err := gemini.ClaudeStreamHandler(meta, c, httpResp)
			convey.So(err, convey.ShouldBeNil)

			// Check response body for signature
			body := w.Body.String()
			convey.So(body, convey.ShouldContainSubstring, `"type":"thinking"`)
			convey.So(
				body,
				convey.ShouldContainSubstring,
				`"signature":"signature_stream_123"`,
			)
		})
	})
}
