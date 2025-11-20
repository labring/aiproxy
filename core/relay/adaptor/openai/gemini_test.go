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
		t.Errorf("Expected tool msg ID %s to match assistant call ID %s", msgs[2].ToolCallID, toolID)
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
		t.Errorf("Expected tool name codebase_investigator, got %s", msgs[0].ToolCalls[0].Function.Name)
	}
}
