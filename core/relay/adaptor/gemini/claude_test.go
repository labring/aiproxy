package gemini_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/gemini"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/require"
)

func TestClaudeHandler(t *testing.T) {
	convey.Convey("ClaudeHandler", t, func() {
		convey.Convey("should handle thinking with signature", func() {
			meta := &meta.Meta{
				OriginModel: "claude-3-5-sonnet-20240620",
			}

			response := &relaymodel.GeminiChatResponse{
				Candidates: []*relaymodel.GeminiChatCandidate{
					{
						Content: relaymodel.GeminiChatContent{
							Parts: []*relaymodel.GeminiPart{
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
			c.Request, _ = http.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				"/",
				nil,
			)

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

			response := &relaymodel.GeminiChatResponse{
				Candidates: []*relaymodel.GeminiChatCandidate{
					{
						Content: relaymodel.GeminiChatContent{
							Parts: []*relaymodel.GeminiPart{
								{
									FunctionCall: &relaymodel.GeminiFunctionCall{
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
			c.Request, _ = http.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				"/",
				nil,
			)

			usage, handlerErr := gemini.ClaudeHandler(meta, c, httpResp)
			convey.So(handlerErr, convey.ShouldBeNil)
			convey.So(usage, convey.ShouldNotBeNil)

			var claudeResponse relaymodel.ClaudeResponse

			err := json.Unmarshal(w.Body.Bytes(), &claudeResponse)
			convey.So(err, convey.ShouldBeNil)

			convey.So(len(claudeResponse.Content), convey.ShouldEqual, 1)

			// Check tool use block
			convey.So(
				claudeResponse.Content[0].Type,
				convey.ShouldEqual,
				relaymodel.ClaudeContentTypeToolUse,
			)
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
			response := &relaymodel.GeminiChatResponse{
				Candidates: []*relaymodel.GeminiChatCandidate{
					{
						Content: relaymodel.GeminiChatContent{
							Parts: []*relaymodel.GeminiPart{
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
			c.Request, _ = http.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				"/",
				nil,
			)

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

func TestConvertClaudeRequest_DisableAutoImageURLToBase64(t *testing.T) {
	channel := &model.Channel{
		Configs: model.ChannelConfigs{
			"disable_auto_image_url_to_base64": true,
		},
	}
	meta := meta.NewMeta(channel, 0, "gemini-1.5-pro", model.ModelConfig{})

	reqBody := relaymodel.ClaudeAnyContentRequest{
		Model: "gemini-1.5-pro",
		Messages: []relaymodel.ClaudeAnyContentMessage{
			{
				Role: relaymodel.RoleUser,
				Content: []relaymodel.ClaudeContent{
					{
						Type: relaymodel.ClaudeContentTypeImage,
						Source: &relaymodel.ClaudeImageSource{
							Type: relaymodel.ClaudeImageSourceTypeURL,
							URL:  "https://example.com/test.png",
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(reqBody)
	require.NoError(t, err)
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/messages",
		bytes.NewReader(data),
	)

	result, err := gemini.ConvertClaudeRequest(meta, req)
	require.NoError(t, err)

	body, _ := io.ReadAll(result.Body)

	var geminiReq relaymodel.GeminiChatRequest
	require.NoError(t, json.Unmarshal(body, &geminiReq))
	require.Len(t, geminiReq.Contents, 1)
	require.Len(t, geminiReq.Contents[0].Parts, 1)
	require.Nil(t, geminiReq.Contents[0].Parts[0].InlineData)
	require.NotNil(t, geminiReq.Contents[0].Parts[0].FileData)
	require.Equal(
		t,
		"https://example.com/test.png",
		geminiReq.Contents[0].Parts[0].FileData.FileURI,
	)
	require.Equal(t, "", geminiReq.Contents[0].Parts[0].FileData.MimeType)
}
