package openai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func TestConvertGeminiRequest_ToolResponse(t *testing.T) {
	// Reproduce the user's scenario:
	// 1. Setup message (User)
	// 2. Tool Call (Model)
	// 3. Tool Response (User/Function) - multiple parts
	requestJSON := `{
		"contents": [
            {
				"parts": [{"text": "List files in current directory"}],
				"role": "user"
			},
			{
				"parts": [
                    {
                        "functionCall": {
                            "name": "list_directory",
                            "args": {"dir_path": "."}
                        }
                    },
                    {
                        "functionCall": {
                            "name": "read_file",
                            "args": {"file_path": "README.md"}
                        }
                    }
                ],
				"role": "model"
			},
            {
                "parts": [
                    {
                        "functionResponse": {
                            "id": "client_provided_id_ignored",
                            "name": "list_directory",
                            "response": {"files": ["README.md", "main.go"]}
                        }
                    },
                    {
                        "functionResponse": {
                            "name": "read_file",
                            "response": {"content": "package main..."}
                        }
                    }
                ],
                "role": "user"
            }
		]
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/gemini-pro:generateContent",
		strings.NewReader(requestJSON),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	meta := &meta.Meta{
		ActualModel: "gpt-4o",
	}

	result, err := openai.ConvertGeminiRequest(meta, req)
	if err != nil {
		t.Fatalf("ConvertGeminiRequest failed: %v", err)
	}

	// Parse result body to GeneralOpenAIRequest
	bodyBytes, _ := io.ReadAll(result.Body)

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(bodyBytes, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal result body: %v", err)
	}

	// Verify Messages
	msgs := openAIReq.Messages
	if len(msgs) != 4 {
		// User, Assistant(2 calls), Tool(1), Tool(2)
		t.Errorf("Expected 4 messages, got %d", len(msgs))

		for i, m := range msgs {
			t.Logf(
				"Message %d: Role=%s, Content=%v, ToolCalls=%d",
				i,
				m.Role,
				m.Content,
				len(m.ToolCalls),
			)
		}
	}

	// Verify Assistant Message
	assistantMsg := msgs[1]
	if assistantMsg.Role != "assistant" {
		t.Errorf("Expected msg[1] role assistant, got %s", assistantMsg.Role)
	}

	if len(assistantMsg.ToolCalls) != 2 {
		t.Errorf("Expected 2 tool calls in assistant msg, got %d", len(assistantMsg.ToolCalls))
	}

	callID1 := assistantMsg.ToolCalls[0].ID
	callID2 := assistantMsg.ToolCalls[1].ID

	if callID1 == "" || callID2 == "" {
		t.Errorf("Expected non-empty tool call IDs")
	}

	// Verify Tool Messages
	toolMsg1 := msgs[2]
	if toolMsg1.Role != "tool" {
		t.Errorf("Expected msg[2] role tool, got %s", toolMsg1.Role)
	}
	// The ID should match the first call ID because we match by order/name
	// Call 1: list_directory. Call 2: read_file.
	// Response 1: list_directory. Response 2: read_file.

	if toolMsg1.ToolCallID != callID1 {
		t.Errorf(
			"Expected tool msg 1 ID to match call ID 1 (%s), got %s",
			callID1,
			toolMsg1.ToolCallID,
		)
	}

	// Verify that client provided ID was IGNORED in favor of consistency
	if toolMsg1.ToolCallID == "client_provided_id_ignored" {
		t.Errorf("Expected tool msg 1 ID to NOT be client provided ID, but it was")
	}

	toolMsg2 := msgs[3]
	if toolMsg2.Role != "tool" {
		t.Errorf("Expected msg[3] role tool, got %s", toolMsg2.Role)
	}

	if toolMsg2.ToolCallID != callID2 {
		t.Errorf(
			"Expected tool msg 2 ID to match call ID 2 (%s), got %s",
			callID2,
			toolMsg2.ToolCallID,
		)
	}
}

func TestConvertGeminiRequest_MissingModelCall(t *testing.T) {
	// Scenario: Client sends FunctionResponse but omits the preceding FunctionCall (Model) message.
	// We must synthesize a fake Assistant message to satisfy OpenAI protocol.
	requestJSON := `{
		"contents": [
            {
				"parts": [{"text": "Please read file"}],
				"role": "user"
			},
            {
                "parts": [
                    {
                        "functionResponse": {
                            "name": "read_file",
                            "response": {"content": "package main..."}
                        }
                    }
                ],
                "role": "user"
            }
		]
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/gemini-pro:generateContent",
		strings.NewReader(requestJSON),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	meta := &meta.Meta{
		ActualModel: "gpt-4o",
	}

	result, err := openai.ConvertGeminiRequest(meta, req)
	if err != nil {
		t.Fatalf("ConvertGeminiRequest failed: %v", err)
	}

	bodyBytes, _ := io.ReadAll(result.Body)

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(bodyBytes, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal result body: %v", err)
	}

	msgs := openAIReq.Messages
	// Expected: User, Synthetic Assistant, Tool
	if len(msgs) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(msgs))

		for i, m := range msgs {
			t.Logf("Message %d: Role=%s", i, m.Role)
		}
	}

	if msgs[1].Role != "assistant" {
		t.Errorf("Expected msg[1] to be synthetic assistant, got %s", msgs[1].Role)
	}

	if len(msgs[1].ToolCalls) == 0 {
		t.Errorf("Expected synthetic assistant to have tool calls")
	}

	toolID := msgs[1].ToolCalls[0].ID
	if msgs[2].Role != "tool" {
		t.Errorf("Expected msg[2] to be tool, got %s", msgs[2].Role)
	}

	if msgs[2].ToolCallID != toolID {
		t.Errorf(
			"Expected tool msg ID %s to match assistant call ID %s",
			msgs[2].ToolCallID,
			toolID,
		)
	}
}

func TestConvertGeminiRequest_EmptyFunctionName(t *testing.T) {
	// Scenario: Request contains a function call with empty name (user report)
	requestJSON := `{
		"contents": [
			{
				"parts": [
					{"functionCall": {"args": null, "name": "codebase_investigator"}},
					{"functionCall": {"args": {}, "name": ""}}
				],
				"role": "model"
			}
		]
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/gemini-pro:generateContent",
		strings.NewReader(requestJSON),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	meta := &meta.Meta{
		ActualModel: "gpt-4o",
	}

	result, err := openai.ConvertGeminiRequest(meta, req)
	if err != nil {
		t.Fatalf("ConvertGeminiRequest failed: %v", err)
	}

	bodyBytes, _ := io.ReadAll(result.Body)

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(bodyBytes, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal result body: %v", err)
	}

	msgs := openAIReq.Messages
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	if msgs[0].Role != "assistant" {
		t.Errorf("Expected role assistant, got %s", msgs[0].Role)
	}

	// Should only have 1 tool call (codebase_investigator), the empty one should be filtered
	if len(msgs[0].ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(msgs[0].ToolCalls))
	}

	if msgs[0].ToolCalls[0].Function.Name != "codebase_investigator" {
		t.Errorf(
			"Expected tool name codebase_investigator, got %s",
			msgs[0].ToolCalls[0].Function.Name,
		)
	}
}

func TestConvertGeminiRequest_LongToolResponseID(t *testing.T) {
	// Scenario: Client sends FunctionResponse with an ID that is too long (>40 chars).
	// OpenAI requires tool_call_id to be <= 40 chars.
	// If we are synthesizing the call (because original call not found), we must ensure the ID is valid.
	longID := "this_is_a_very_long_id_that_exceeds_the_limit_of_40_characters_created_by_gemini_client"
	requestJSON := `{
		"contents": [
            {
                "parts": [
                    {
                        "functionResponse": {
                            "id": "` + longID + `",
                            "name": "read_file",
                            "response": {"content": "package main..."}
                        }
                    }
                ],
                "role": "user"
            }
		]
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/gemini-pro:generateContent",
		strings.NewReader(requestJSON),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	meta := &meta.Meta{
		ActualModel: "gpt-4o",
	}

	result, err := openai.ConvertGeminiRequest(meta, req)
	if err != nil {
		t.Fatalf("ConvertGeminiRequest failed: %v", err)
	}

	bodyBytes, _ := io.ReadAll(result.Body)

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(bodyBytes, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal result body: %v", err)
	}

	msgs := openAIReq.Messages
	// Expected: Synthetic Assistant, Tool
	if len(msgs) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(msgs))
	}

	// Check Synthetic Assistant ToolCall ID
	syntheticToolCallID := msgs[0].ToolCalls[0].ID
	if len(syntheticToolCallID) > 40 {
		t.Errorf(
			"Synthetic ToolCall ID is too long: %d chars. ID: %s",
			len(syntheticToolCallID),
			syntheticToolCallID,
		)
	}

	// Check Tool Message ToolCallID
	toolMsgID := msgs[1].ToolCallID
	if len(toolMsgID) > 40 {
		t.Errorf("Tool Message ID is too long: %d chars. ID: %s", len(toolMsgID), toolMsgID)
	}

	if syntheticToolCallID != toolMsgID {
		t.Errorf(
			"Mismatch between synthetic call ID (%s) and tool msg ID (%s)",
			syntheticToolCallID,
			toolMsgID,
		)
	}
}

func TestConvertGeminiRequest_ParametersJsonSchema(t *testing.T) {
	// Scenario: Request uses parametersJsonSchema instead of parameters (new Gemini API format)
	requestJSON := `
	{
		"tools": [
			{
				"functionDeclarations": [
					{
						"name": "list_directory",
						"description": "List files",
						"parametersJsonSchema": {
							"type": "object",
							"properties": {
								"dir_path": {"type": "string"}
							},
							"required": ["dir_path"]
						}
					}
				]
			}
		],
		"contents": [{"parts": [{"text": "Hello"}], "role": "user"}]
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1beta/models/gemini-pro:generateContent",
		strings.NewReader(requestJSON),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	meta := &meta.Meta{
		ActualModel: "gpt-4o",
	}

	result, err := openai.ConvertGeminiRequest(meta, req)
	if err != nil {
		t.Fatalf("ConvertGeminiRequest failed: %v", err)
	}

	bodyBytes, _ := io.ReadAll(result.Body)

	var openAIReq relaymodel.GeneralOpenAIRequest
	if err := json.Unmarshal(bodyBytes, &openAIReq); err != nil {
		t.Fatalf("failed to unmarshal result body: %v", err)
	}

	if len(openAIReq.Tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(openAIReq.Tools))
	}

	tool := openAIReq.Tools[0]
	if tool.Function.Parameters == nil {
		t.Errorf("Expected tool parameters to be populated from parametersJsonSchema, but was nil")
	}
}

func TestConvertOpenAIStreamToGemini_PartialToolCalls(t *testing.T) {
	// This test reproduces the issue where split tool calls result in empty name function calls.
	meta := &meta.Meta{
		ActualModel: "gpt-4o",
	}

	// Buffer state
	streamState := openai.NewGeminiStreamState()

	// Chunk 1: Name and partial args
	chunk1 := &relaymodel.ChatCompletionsStreamResponse{
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: relaymodel.Message{
					ToolCalls: []relaymodel.ToolCall{
						{
							Index: 0,
							Function: relaymodel.Function{
								Name:      "list_directory",
								Arguments: `{"dir_path":`,
							},
						},
					},
				},
			},
		},
	}

	// Chunk 2: Remaining args
	chunk2 := &relaymodel.ChatCompletionsStreamResponse{
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: relaymodel.Message{
					ToolCalls: []relaymodel.ToolCall{
						{
							Index: 0,
							Function: relaymodel.Function{
								Arguments: ` "."}`,
							},
						},
					},
				},
			},
		},
	}

	// Chunk 3: Finish reason (trigger flush)
	chunk3 := &relaymodel.ChatCompletionsStreamResponse{
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: relaymodel.FinishReasonToolCalls,
				Delta:        relaymodel.Message{},
			},
		},
	}

	// Process chunk 1
	resp1 := streamState.ConvertOpenAIStreamToGemini(meta, chunk1)
	if resp1 != nil {
		// Should be nil or empty because we are buffering
		if len(resp1.Candidates) > 0 {
			t.Errorf(
				"Chunk 1 should not produce candidates yet (buffering). Got candidates: %d",
				len(resp1.Candidates),
			)
		}
	}

	// Verify buffer state
	if len(streamState.ToolCallBuffer) != 1 {
		t.Errorf("Buffer should have 1 entry, got %d", len(streamState.ToolCallBuffer))
	}

	if state, ok := streamState.ToolCallBuffer["0-0"]; !ok {
		t.Errorf("Buffer missing key 0-0")
	} else {
		if state.Name != "list_directory" {
			t.Errorf("Buffer state name mismatch. Got %s", state.Name)
		}

		if state.Arguments != `{"dir_path":` {
			t.Errorf("Buffer state args mismatch")
		}
	}

	// Process chunk 2
	resp2 := streamState.ConvertOpenAIStreamToGemini(meta, chunk2)
	if resp2 != nil {
		if len(resp2.Candidates) > 0 {
			t.Errorf("Chunk 2 should not produce candidates yet")
		}
	}

	// Verify buffer state update
	if state, ok := streamState.ToolCallBuffer["0-0"]; ok {
		expectedArgs := `{"dir_path": "."}`
		if state.Arguments != expectedArgs {
			t.Errorf(
				"Buffer state args mismatch after chunk 2. Got %s, want %s",
				state.Arguments,
				expectedArgs,
			)
		}
	}

	// Process chunk 3 (Flush)
	resp3 := streamState.ConvertOpenAIStreamToGemini(meta, chunk3)
	if resp3 == nil || len(resp3.Candidates) == 0 {
		t.Fatalf("Chunk 3 should produce candidates (flush)")
	}

	part := resp3.Candidates[0].Content.Parts[0]
	if part.FunctionCall == nil {
		t.Fatalf("Flushed part should have function call")
	}

	if part.FunctionCall.Name != "list_directory" {
		t.Errorf("Flushed function call name mismatch. Got: %s", part.FunctionCall.Name)
	}

	if part.FunctionCall.Args == nil {
		t.Errorf("Flushed function call args should not be nil")
	}
}

func TestConvertOpenAIStreamToGemini_MultipleToolCalls_Order(t *testing.T) {
	// Scenario: Multiple tool calls in the same choice.
	// We must ensure they are flushed in the correct order (by index).
	meta := &meta.Meta{
		ActualModel: "gpt-4o",
	}
	streamState := openai.NewGeminiStreamState()

	// Chunk 1: Two tool calls start
	chunk1 := &relaymodel.ChatCompletionsStreamResponse{
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: relaymodel.Message{
					ToolCalls: []relaymodel.ToolCall{
						{
							Index: 0,
							Function: relaymodel.Function{
								Name:      "func_zero",
								Arguments: `{}`,
							},
						},
						{
							Index: 1,
							Function: relaymodel.Function{
								Name:      "func_one",
								Arguments: `{}`,
							},
						},
					},
				},
			},
		},
	}

	// Chunk 2: Finish (Flush)
	chunk2 := &relaymodel.ChatCompletionsStreamResponse{
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: relaymodel.FinishReasonToolCalls,
				Delta:        relaymodel.Message{},
			},
		},
	}

	// Process
	streamState.ConvertOpenAIStreamToGemini(meta, chunk1)
	resp := streamState.ConvertOpenAIStreamToGemini(meta, chunk2)

	if resp == nil || len(resp.Candidates) == 0 {
		t.Fatalf("Expected candidates")
	}

	parts := resp.Candidates[0].Content.Parts
	if len(parts) != 2 {
		t.Fatalf("Expected 2 parts, got %d", len(parts))
	}

	// Check order
	// We expect func_zero then func_one because index 0 comes before index 1
	name0 := parts[0].FunctionCall.Name
	name1 := parts[1].FunctionCall.Name

	// Note: With current map iteration, this MIGHT fail randomly if we don't sort.
	// To verify the bug, we might need to run this multiple times or check implementation.
	// But we want to FIX it.
	if name0 != "func_zero" || name1 != "func_one" {
		t.Errorf("Expected order [func_zero, func_one], got [%s, %s]", name0, name1)
	}
}

func TestConvertGeminiRequest_ToolsWithRequiredField(t *testing.T) {
	tests := []struct {
		name      string
		request   string
		checkFunc func(*testing.T, relaymodel.GeneralOpenAIRequest)
	}{
		{
			name: "null required field should be removed",
			request: `{
				"tools": [{
					"functionDeclarations": [{
						"name": "get_weather",
						"description": "Get weather info",
						"parameters": {
							"type": "object",
							"properties": {
								"location": {"type": "string"}
							},
							"required": null
						}
					}]
				}],
				"contents": [{"parts": [{"text": "Hello"}], "role": "user"}]
			}`,
			checkFunc: func(t *testing.T, openAIReq relaymodel.GeneralOpenAIRequest) {
				t.Helper()

				if len(openAIReq.Tools) != 1 {
					t.Fatalf("Expected 1 tool, got %d", len(openAIReq.Tools))
				}

				// Check that required field is removed
				if params, ok := openAIReq.Tools[0].Function.Parameters.(map[string]any); ok {
					_, hasRequired := params["required"]
					if hasRequired {
						t.Errorf("required field should be removed when it's null")
					}
				} else {
					t.Errorf("Parameters should be a map, got %T", openAIReq.Tools[0].Function.Parameters)
				}
			},
		},
		{
			name: "empty required array should be removed",
			request: `{
				"tools": [{
					"functionDeclarations": [{
						"name": "get_weather",
						"description": "Get weather info",
						"parameters": {
							"type": "object",
							"properties": {
								"location": {"type": "string"}
							},
							"required": []
						}
					}]
				}],
				"contents": [{"parts": [{"text": "Hello"}], "role": "user"}]
			}`,
			checkFunc: func(t *testing.T, openAIReq relaymodel.GeneralOpenAIRequest) {
				t.Helper()

				if len(openAIReq.Tools) != 1 {
					t.Fatalf("Expected 1 tool, got %d", len(openAIReq.Tools))
				}

				// Check that required field is removed
				if params, ok := openAIReq.Tools[0].Function.Parameters.(map[string]any); ok {
					_, hasRequired := params["required"]
					if hasRequired {
						t.Errorf("required field should be removed when it's empty array")
					}
				}
			},
		},
		{
			name: "valid required array should be kept",
			request: `{
				"tools": [{
					"functionDeclarations": [{
						"name": "get_weather",
						"description": "Get weather info",
						"parameters": {
							"type": "object",
							"properties": {
								"location": {"type": "string"}
							},
							"required": ["location"]
						}
					}]
				}],
				"contents": [{"parts": [{"text": "Hello"}], "role": "user"}]
			}`,
			checkFunc: func(t *testing.T, openAIReq relaymodel.GeneralOpenAIRequest) {
				t.Helper()

				if len(openAIReq.Tools) != 1 {
					t.Fatalf("Expected 1 tool, got %d", len(openAIReq.Tools))
				}

				// Check that required field is kept
				if params, ok := openAIReq.Tools[0].Function.Parameters.(map[string]any); ok {
					required, hasRequired := params["required"]
					if !hasRequired {
						t.Errorf("required field should be kept when it has values")
					}

					if reqArray, ok := required.([]any); ok {
						if len(reqArray) != 1 {
							t.Errorf("Expected 1 required field, got %d", len(reqArray))
						}

						if reqArray[0] != "location" {
							t.Errorf("Expected required field 'location', got %v", reqArray[0])
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				"/v1beta/models/gemini-pro:generateContent",
				strings.NewReader(tt.request),
			)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			req.Header.Set("Content-Type", "application/json")

			meta := &meta.Meta{
				ActualModel: "gpt-4o",
			}

			result, err := openai.ConvertGeminiRequest(meta, req)
			if err != nil {
				t.Fatalf("ConvertGeminiRequest failed: %v", err)
			}

			bodyBytes, _ := io.ReadAll(result.Body)

			var openAIReq relaymodel.GeneralOpenAIRequest
			if err := json.Unmarshal(bodyBytes, &openAIReq); err != nil {
				t.Fatalf("failed to unmarshal result body: %v", err)
			}

			tt.checkFunc(t, openAIReq)
		})
	}
}
