package gemini_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor/gemini"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/smartystreets/goconvey/convey"
)

func TestHandler(t *testing.T) {
	convey.Convey("Handler", t, func() {
		convey.Convey("should handle thinking with signature in OpenAI format", func() {
			meta := &meta.Meta{
				OriginModel: "gemini-1.5-pro",
			}

			response := &relaymodel.GeminiChatResponse{
				Candidates: []*relaymodel.GeminiChatCandidate{
					{
						Content: relaymodel.GeminiChatContent{
							Parts: []*relaymodel.GeminiPart{
								{
									Text:             "Thinking process...",
									Thought:          true,
									ThoughtSignature: "signature_openai_123",
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

			usage, handlerErr := gemini.Handler(meta, c, httpResp)
			convey.So(handlerErr, convey.ShouldBeNil)
			convey.So(usage, convey.ShouldNotBeNil)

			var textResponse relaymodel.TextResponse

			err := json.Unmarshal(w.Body.Bytes(), &textResponse)
			convey.So(err, convey.ShouldBeNil)

			convey.So(len(textResponse.Choices), convey.ShouldEqual, 1)

			// Check message content and signature
			convey.So(
				textResponse.Choices[0].Message.ReasoningContent,
				convey.ShouldEqual,
				"Thinking process...",
			)
			convey.So(textResponse.Choices[0].Message.Content, convey.ShouldEqual, "Final answer")
			convey.So(
				textResponse.Choices[0].Message.Signature,
				convey.ShouldEqual,
				"signature_openai_123",
			)
		})
	})
}

func TestStreamHandler(t *testing.T) {
	convey.Convey("StreamHandler", t, func() {
		convey.Convey("should handle thinking with signature in OpenAI stream format", func() {
			meta := &meta.Meta{
				OriginModel: "gemini-1.5-pro",
			}

			// Prepare SSE stream response
			response := &relaymodel.GeminiChatResponse{
				Candidates: []*relaymodel.GeminiChatCandidate{
					{
						Content: relaymodel.GeminiChatContent{
							Parts: []*relaymodel.GeminiPart{
								{
									Text:             "Thinking chunk...",
									Thought:          true,
									ThoughtSignature: "signature_stream_openai_456",
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

			_, err := gemini.StreamHandler(meta, c, httpResp)
			convey.So(err, convey.ShouldBeNil)

			// Check response body for signature
			body := w.Body.String()

			// Parse the SSE output manually to verify structure
			lines := strings.Split(body, "\n")
			foundSignature := false
			foundReasoning := false

			for _, line := range lines {
				if strings.HasPrefix(line, "data: {") {
					jsonStr := strings.TrimPrefix(line, "data: ")

					var streamResp relaymodel.ChatCompletionsStreamResponse

					_ = json.Unmarshal([]byte(jsonStr), &streamResp)

					if len(streamResp.Choices) > 0 {
						delta := streamResp.Choices[0].Delta
						if delta.ReasoningContent != "" {
							convey.So(
								delta.ReasoningContent,
								convey.ShouldEqual,
								"Thinking chunk...",
							)

							foundReasoning = true
						}

						if delta.Signature != "" {
							convey.So(
								delta.Signature,
								convey.ShouldEqual,
								"signature_stream_openai_456",
							)

							foundSignature = true
						}
					}
				}
			}

			convey.So(foundReasoning, convey.ShouldBeTrue)
			convey.So(foundSignature, convey.ShouldBeTrue)
		})
	})
}
