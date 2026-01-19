package anthropic_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/meta"
)

func TestConvertGeminiRequestToStruct_Tools(t *testing.T) {
	// JSON from user issue report (simplified)
	requestJSON := `{
		"contents": [
			{
				"parts": [{"text": "test"}],
				"role": "user"
			}
		],
		"tools": [
			{
				"functionDeclarations": [
					{
						"name": "list_directory",
						"description": "Lists files",
						"parametersJsonSchema": {
							"type": "object",
							"properties": {
								"dir_path": {"type": "string", "description": "path"}
							},
							"required": ["dir_path"]
						}
					}
				]
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
		ActualModel: "claude-3-sonnet-20240229",
	}

	claudeReq, err := anthropic.ConvertGeminiRequestToStruct(meta, req)
	if err != nil {
		t.Fatalf("ConvertGeminiRequestToStruct failed: %v", err)
	}

	// Expectation: Tools should be populated
	if len(claudeReq.Tools) == 0 {
		t.Errorf("Expected tools to be populated, but got empty")
	} else {
		tool := claudeReq.Tools[0]
		if tool.Name != "list_directory" {
			t.Errorf("Expected tool name list_directory, got %s", tool.Name)
		}

		if tool.InputSchema == nil {
			t.Errorf(
				"Expected InputSchema to be populated (from parametersJsonSchema), but got nil",
			)
		}
	}
}

func TestConvertGeminiRequestToStruct_Tools_StandardParameters(t *testing.T) {
	// JSON with standard "parameters" field
	requestJSON := `{
		"contents": [
			{
				"parts": [{"text": "test"}],
				"role": "user"
			}
		],
		"tools": [
			{
				"functionDeclarations": [
					{
						"name": "get_weather",
						"description": "Get weather",
						"parameters": {
							"type": "object",
							"properties": {
								"location": {"type": "string", "description": "City"}
							},
							"required": ["location"]
						}
					}
				]
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
		ActualModel: "claude-3-sonnet-20240229",
	}

	claudeReq, err := anthropic.ConvertGeminiRequestToStruct(meta, req)
	if err != nil {
		t.Fatalf("ConvertGeminiRequestToStruct failed: %v", err)
	}

	if len(claudeReq.Tools) == 0 {
		t.Errorf("Expected tools to be populated, but got empty")
	} else {
		tool := claudeReq.Tools[0]
		if tool.Name != "get_weather" {
			t.Errorf("Expected tool name get_weather, got %s", tool.Name)
		}

		if tool.InputSchema == nil {
			t.Errorf("Expected InputSchema to be populated (from parameters), but got nil")
		}

		if tool.InputSchema.Type != "object" {
			t.Errorf("Expected InputSchema type object, got %s", tool.InputSchema.Type)
		}
	}
}

func TestConvertGeminiRequestToStruct_TemperatureAndTopP(t *testing.T) {
	requestJSON := `{
		"contents": [
			{
				"parts": [{"text": "test"}],
				"role": "user"
			}
		],
		"generationConfig": {
			"temperature": 0.5,
			"topP": 0.9
		}
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
		ActualModel: "claude-3-sonnet-20240229",
	}

	claudeReq, err := anthropic.ConvertGeminiRequestToStruct(meta, req)
	if err != nil {
		t.Fatalf("ConvertGeminiRequestToStruct failed: %v", err)
	}

	if claudeReq.Temperature != nil && claudeReq.TopP != nil {
		t.Errorf("Expected only one of Temperature or TopP to be set, but got both")
	}

	if claudeReq.Temperature == nil {
		t.Errorf("Expected Temperature to be set")
	}
}

func TestConvertGeminiRequestToStruct_TopPOnly(t *testing.T) {
	requestJSON := `{
		"contents": [
			{
				"parts": [{"text": "test"}],
				"role": "user"
			}
		],
		"generationConfig": {
			"topP": 0.9
		}
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
		ActualModel: "claude-3-sonnet-20240229",
	}

	claudeReq, err := anthropic.ConvertGeminiRequestToStruct(meta, req)
	if err != nil {
		t.Fatalf("ConvertGeminiRequestToStruct failed: %v", err)
	}

	if claudeReq.Temperature != nil {
		t.Errorf("Expected Temperature to be nil, but got %v", claudeReq.Temperature)
	}

	if claudeReq.TopP == nil {
		t.Errorf("Expected TopP to be set")
	}
}

func TestConvertGeminiRequestToStruct_TemperatureOnly(t *testing.T) {
	requestJSON := `{
		"contents": [
			{
				"parts": [{"text": "test"}],
				"role": "user"
			}
		],
		"generationConfig": {
			"temperature": 0.5
		}
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
		ActualModel: "claude-3-sonnet-20240229",
	}

	claudeReq, err := anthropic.ConvertGeminiRequestToStruct(meta, req)
	if err != nil {
		t.Fatalf("ConvertGeminiRequestToStruct failed: %v", err)
	}

	if claudeReq.Temperature == nil {
		t.Errorf("Expected Temperature to be set")
	}

	if claudeReq.TopP != nil {
		t.Errorf("Expected TopP to be nil")
	}
}
