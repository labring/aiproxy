package gemini

import (
	"bufio"
	"bytes"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
)

func ConvertClaudeRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	adaptorConfig := Config{}

	err := meta.ChannelConfigs.LoadConfig(&adaptorConfig)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	textRequest, err := openai.ConvertClaudeRequestModel(meta, req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	textRequest.Model = meta.ActualModel
	meta.Set("stream", textRequest.Stream)

	systemContent, contents, imageTasks := buildContents(textRequest)

	// Process image tasks concurrently
	if len(imageTasks) > 0 {
		if err := processImageTasks(req.Context(), imageTasks); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	config := buildGenerationConfig(meta, textRequest, textRequest)

	// Build actual request
	geminiRequest := relaymodel.GeminiChatRequest{
		Contents:          contents,
		SystemInstruction: systemContent,
		SafetySettings:    buildSafetySettings(adaptorConfig.Safety),
		GenerationConfig:  config,
		Tools:             buildTools(textRequest),
		ToolConfig:        buildToolConfig(textRequest),
	}

	data, err := sonic.Marshal(geminiRequest)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(data))},
		},
		Body: bytes.NewReader(data),
	}, nil
}

// ClaudeHandler handles non-streaming Gemini responses and converts them to Claude format
func ClaudeHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	var geminiResponse relaymodel.GeminiChatResponse

	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&geminiResponse)
	if err != nil {
		return model.Usage{}, relaymodel.WrapperAnthropicError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Convert Gemini response to Claude response
	claudeResponse := geminiResponse2Claude(meta, &geminiResponse)

	jsonResponse, err := sonic.Marshal(claudeResponse)
	if err != nil {
		return claudeResponse.Usage.ToOpenAIUsage().
				ToModelUsage(),
			relaymodel.WrapperAnthropicError(
				err,
				"marshal_response_body_failed",
				http.StatusInternalServerError,
			)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	return claudeResponse.Usage.ToOpenAIUsage().ToModelUsage(), nil
}

// ClaudeStreamHandler handles streaming Gemini responses and converts them to Claude format
func ClaudeStreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return model.Usage{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner := bufio.NewScanner(resp.Body)
	if strings.Contains(meta.ActualModel, "image") {
		buf := GetImageScannerBuffer()
		defer PutImageScannerBuffer(buf)

		scanner.Buffer(*buf, cap(*buf))
	} else {
		buf := utils.GetScannerBuffer()
		defer utils.PutScannerBuffer(buf)

		scanner.Buffer(*buf, cap(*buf))
	}

	var (
		messageID           = "msg_" + common.ShortUUID()
		contentText         strings.Builder
		thinkingText        strings.Builder
		usage               relaymodel.ChatUsage
		stopReason          string
		currentContentIndex = -1
		currentContentType  = ""
		sentMessageStart    = false
		toolCallsBuffer     = make(map[int]*relaymodel.ClaudeContent)
	)

	// Helper function to close current content block
	closeCurrentBlock := func() {
		if currentContentIndex >= 0 {
			_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
				Type:  relaymodel.ClaudeStreamTypeContentBlockStop,
				Index: currentContentIndex,
			})
		}
	}

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		var geminiResponse relaymodel.GeminiChatResponse

		err := sonic.Unmarshal(data, &geminiResponse)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		// Send message_start event (only once)
		if !sentMessageStart {
			sentMessageStart = true

			messageStartResp := relaymodel.ClaudeStreamResponse{
				Type: relaymodel.ClaudeStreamTypeMessageStart,
				Message: &relaymodel.ClaudeResponse{
					ID:      messageID,
					Type:    "message",
					Role:    relaymodel.RoleAssistant,
					Model:   meta.ActualModel,
					Content: []relaymodel.ClaudeContent{},
				},
			}

			// Add initial usage if available
			if geminiResponse.UsageMetadata != nil {
				claudeUsage := geminiResponse.UsageMetadata.ToUsage().ToClaudeUsage()
				messageStartResp.Message.Usage = claudeUsage
			}

			_ = render.ClaudeObjectData(c, messageStartResp)

			// Send ping event
			_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{Type: "ping"})
		}

		// Update usage if available
		if geminiResponse.UsageMetadata != nil {
			usage = geminiResponse.UsageMetadata.ToUsage()
		}

		// Process each candidate
		for _, candidate := range geminiResponse.Candidates {
			// Handle finish reason
			if candidate.FinishReason != "" {
				stopReason = geminiFinishReason2Claude(candidate.FinishReason)
			}

			// Process content parts
			for _, part := range candidate.Content.Parts {
				switch {
				case part.Thought:
					// Handle thinking content
					if currentContentType != relaymodel.ClaudeContentTypeThinking {
						closeCurrentBlock()

						currentContentIndex++
						currentContentType = relaymodel.ClaudeContentTypeThinking

						_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
							Type:  relaymodel.ClaudeStreamTypeContentBlockStart,
							Index: currentContentIndex,
							ContentBlock: &relaymodel.ClaudeContent{
								Type:     relaymodel.ClaudeContentTypeThinking,
								Thinking: "",
							},
						})

						if part.ThoughtSignature != "" {
							_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
								Type:  relaymodel.ClaudeStreamTypeContentBlockDelta,
								Index: currentContentIndex,
								ContentBlock: &relaymodel.ClaudeContent{
									Type:      "signature_delta",
									Signature: part.ThoughtSignature,
								},
							})
						}
					}

					thinkingText.WriteString(part.Text)

					_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
						Type:  relaymodel.ClaudeStreamTypeContentBlockDelta,
						Index: currentContentIndex,
						Delta: &relaymodel.ClaudeDelta{
							Type:     "thinking_delta",
							Thinking: part.Text,
						},
					})
				case part.Text != "":
					// Handle text content
					if currentContentType != relaymodel.ClaudeContentTypeText {
						closeCurrentBlock()

						currentContentIndex++
						currentContentType = relaymodel.ClaudeContentTypeText

						_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
							Type:  relaymodel.ClaudeStreamTypeContentBlockStart,
							Index: currentContentIndex,
							ContentBlock: &relaymodel.ClaudeContent{
								Type: relaymodel.ClaudeContentTypeText,
								Text: "",
							},
						})
					}

					contentText.WriteString(part.Text)

					_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
						Type:  relaymodel.ClaudeStreamTypeContentBlockDelta,
						Index: currentContentIndex,
						Delta: &relaymodel.ClaudeDelta{
							Type: "text_delta",
							Text: part.Text,
						},
					})
				case part.FunctionCall != nil:
					// Handle tool/function calls
					closeCurrentBlock()

					currentContentIndex++
					currentContentType = relaymodel.ClaudeContentTypeToolUse

					toolContent := &relaymodel.ClaudeContent{
						Type:      relaymodel.ClaudeContentTypeToolUse,
						ID:        openai.CallID(),
						Name:      part.FunctionCall.Name,
						Input:     part.FunctionCall.Args,
						Signature: part.ThoughtSignature,
					}
					toolCallsBuffer[currentContentIndex] = toolContent

					// Send content_block_start for tool use
					_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
						Type:         relaymodel.ClaudeStreamTypeContentBlockStart,
						Index:        currentContentIndex,
						ContentBlock: toolContent,
					})

					// Send tool arguments as delta
					args, _ := sonic.MarshalString(part.FunctionCall.Args)
					_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
						Type:  relaymodel.ClaudeStreamTypeContentBlockDelta,
						Index: currentContentIndex,
						Delta: &relaymodel.ClaudeDelta{
							Type:        "input_json_delta",
							PartialJSON: args,
						},
					})
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	// Close the last open content block
	closeCurrentBlock()

	// Calculate final usage if not provided
	if usage.TotalTokens == 0 && (contentText.Len() > 0 || thinkingText.Len() > 0) {
		totalText := contentText.String()
		if thinkingText.Len() > 0 {
			totalText = thinkingText.String() + "\n" + totalText
		}

		usage = openai.ResponseText2Usage(
			totalText,
			meta.ActualModel,
			int64(meta.RequestUsage.InputTokens),
		)
	}

	claudeUsage := usage.ToClaudeUsage()

	if stopReason == "" {
		stopReason = relaymodel.ClaudeStopReasonEndTurn
	}

	// Send message_delta with final usage
	_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
		Type: relaymodel.ClaudeStreamTypeMessageDelta,
		Delta: &relaymodel.ClaudeDelta{
			StopReason: &stopReason,
		},
		Usage: &claudeUsage,
	})

	// Send message_stop
	_ = render.ClaudeObjectData(c, relaymodel.ClaudeStreamResponse{
		Type: "message_stop",
	})

	return usage.ToModelUsage(), nil
}

// geminiResponse2Claude converts a Gemini response to Claude format
func geminiResponse2Claude(
	meta *meta.Meta,
	response *relaymodel.GeminiChatResponse,
) *relaymodel.ClaudeResponse {
	claudeResponse := relaymodel.ClaudeResponse{
		ID:           "msg_" + common.ShortUUID(),
		Type:         "message",
		Role:         relaymodel.RoleAssistant,
		Model:        meta.OriginModel,
		Content:      []relaymodel.ClaudeContent{},
		StopReason:   "",
		StopSequence: nil,
	}

	// Convert usage
	if response.UsageMetadata != nil {
		usage := response.UsageMetadata.ToUsage()
		claudeResponse.Usage = usage.ToClaudeUsage()
	}

	// Convert content from candidates
	for _, candidate := range response.Candidates {
		// Map finish reason
		claudeResponse.StopReason = geminiFinishReason2Claude(candidate.FinishReason)

		// Extract content
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall != nil {
				// Convert function call to tool use
				claudeResponse.Content = append(claudeResponse.Content, relaymodel.ClaudeContent{
					Type:      relaymodel.ClaudeContentTypeToolUse,
					ID:        openai.CallID(),
					Name:      part.FunctionCall.Name,
					Input:     part.FunctionCall.Args,
					Signature: part.ThoughtSignature,
				})
			} else if part.Text != "" {
				if part.Thought {
					// Add thinking content
					claudeResponse.Content = append(claudeResponse.Content, relaymodel.ClaudeContent{
						Type:      relaymodel.ClaudeContentTypeThinking,
						Thinking:  part.Text,
						Signature: part.ThoughtSignature,
					})
				} else {
					// Add text content
					claudeResponse.Content = append(claudeResponse.Content, relaymodel.ClaudeContent{
						Type: relaymodel.ClaudeContentTypeText,
						Text: part.Text,
					})
				}
			}
		}
	}

	// If no content was added, ensure at least an empty text block
	// This can happen when Gemini returns empty content after receiving a tool result,
	// indicating it has nothing more to add beyond the tool's response
	if len(claudeResponse.Content) == 0 {
		claudeResponse.Content = append(claudeResponse.Content, relaymodel.ClaudeContent{
			Type: relaymodel.ClaudeContentTypeText,
			Text: "",
		})
	}

	return &claudeResponse
}

// geminiFinishReason2Claude converts Gemini finish reason to Claude stop reason
func geminiFinishReason2Claude(reason string) string {
	switch reason {
	case relaymodel.GeminiFinishReasonStop:
		return relaymodel.ClaudeStopReasonEndTurn
	case relaymodel.GeminiFinishReasonMaxTokens:
		return relaymodel.ClaudeStopReasonMaxTokens
	case relaymodel.GeminiFinishReasonToolCalls, relaymodel.GeminiFinishReasonFunctionCall:
		return relaymodel.ClaudeStopReasonToolUse
	case relaymodel.GeminiFinishReasonSafety:
		return relaymodel.ClaudeStopReasonStopSequence
	default:
		return relaymodel.ClaudeStopReasonEndTurn
	}
}
