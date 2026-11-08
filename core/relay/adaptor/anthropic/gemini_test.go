package anthropic_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
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

func TestConvertGeminiRequestToStruct_NormalizesThinkingForAdaptiveModels(t *testing.T) {
	requestJSON := `{
		"contents": [
			{
				"parts": [{"text": "test"}],
				"role": "user"
			}
		],
		"generationConfig": {
			"maxOutputTokens": 4096,
			"thinkingConfig": {
				"thinkingBudget": 2048,
				"includeThoughts": true
			}
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
		ActualModel: "claude-opus-4-7",
	}

	claudeReq, err := anthropic.ConvertGeminiRequestToStruct(meta, req)
	if err != nil {
		t.Fatalf("ConvertGeminiRequestToStruct failed: %v", err)
	}

	if claudeReq.Thinking == nil {
		t.Fatal("expected thinking to be set")
	}

	if claudeReq.Thinking.Type != relaymodel.ClaudeThinkingTypeAdaptive {
		t.Fatalf("expected adaptive thinking, got %s", claudeReq.Thinking.Type)
	}

	if claudeReq.Thinking.BudgetTokens != 0 {
		t.Fatalf("expected budget_tokens to be removed, got %d", claudeReq.Thinking.BudgetTokens)
	}
}

func TestClaudeGeminiUsageIncludesCache(t *testing.T) {
	m := &meta.Meta{ActualModel: "claude-sonnet-4"}
	usage := relaymodel.ClaudeUsage{
		InputTokens:              10,
		OutputTokens:             5,
		CacheReadInputTokens:     100,
		CacheCreationInputTokens: 20,
	}

	t.Run("non-stream", func(t *testing.T) {
		resp := anthropic.ConvertClaudeToGeminiResponse(m, &relaymodel.ClaudeResponse{Usage: usage})
		if resp.UsageMetadata == nil {
			t.Fatal("expected usage metadata")
		}

		if resp.UsageMetadata.PromptTokenCount != 130 ||
			resp.UsageMetadata.CachedContentTokenCount != 100 ||
			resp.UsageMetadata.TotalTokenCount != 135 {
			t.Fatalf("unexpected usage: %+v", resp.UsageMetadata)
		}
	})

	t.Run("stream message_delta", func(t *testing.T) {
		state := &anthropic.GeminiStreamState{}
		stop := relaymodel.ClaudeStopReasonEndTurn
		resp := state.ConvertClaudeStreamToGemini(m, &relaymodel.ClaudeStreamResponse{
			Type:  relaymodel.ClaudeStreamTypeMessageDelta,
			Delta: &relaymodel.ClaudeDelta{StopReason: &stop},
			Usage: &usage,
		})
		if resp == nil || resp.UsageMetadata == nil {
			t.Fatal("expected usage metadata")
		}

		mu := resp.UsageMetadata.ToModelUsage()
		if int64(mu.InputTokens) != 130 || int64(mu.CachedTokens) != 100 ||
			int64(mu.OutputTokens) != 5 {
			t.Fatalf("unexpected billed usage: %+v", mu)
		}
	})

	t.Run("stream start then output-only delta", func(t *testing.T) {
		state := &anthropic.GeminiStreamState{}
		state.ConvertClaudeStreamToGemini(m, &relaymodel.ClaudeStreamResponse{
			Type: relaymodel.ClaudeStreamTypeMessageStart,
			Message: &relaymodel.ClaudeResponse{Usage: relaymodel.ClaudeUsage{
				InputTokens:              10,
				CacheReadInputTokens:     100,
				CacheCreationInputTokens: 20,
			}},
		})

		stop := relaymodel.ClaudeStopReasonEndTurn
		resp := state.ConvertClaudeStreamToGemini(m, &relaymodel.ClaudeStreamResponse{
			Type:  relaymodel.ClaudeStreamTypeMessageDelta,
			Delta: &relaymodel.ClaudeDelta{StopReason: &stop},
			Usage: &relaymodel.ClaudeUsage{OutputTokens: 5},
		})
		if resp == nil || resp.UsageMetadata == nil {
			t.Fatal("expected usage metadata")
		}

		mu := resp.UsageMetadata.ToModelUsage()
		if int64(mu.InputTokens) != 130 || int64(mu.CachedTokens) != 100 ||
			int64(mu.OutputTokens) != 5 {
			t.Fatalf("unexpected billed usage: %+v", mu)
		}
	})
}

func TestGeminiStreamHandlerBillsLikeNonStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := &meta.Meta{ActualModel: "claude-sonnet-4"}
	start := `{"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":10,` +
		`"cache_read_input_tokens":100,"cache_creation_input_tokens":20,` +
		`"server_tool_use":{"web_search_requests":2}}}}`
	delta := `{"type":"message_delta","delta":{"stop_reason":"end_turn"},` +
		`"usage":{"output_tokens":5}}`
	body := "data: " + start + "\n\ndata: " + delta + "\n\ndata: [DONE]\n\n"

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)

	got, errResp := anthropic.GeminiStreamHandler(m, c, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	})
	if errResp != nil {
		t.Fatalf("unexpected error: %v", errResp)
	}

	want := relaymodel.ClaudeUsage{
		InputTokens:              10,
		OutputTokens:             5,
		CacheReadInputTokens:     100,
		CacheCreationInputTokens: 20,
		ServerToolUse:            &relaymodel.ClaudeServerToolUse{WebSearchRequests: 2},
	}
	exp := want.ToOpenAIUsage().ToModelUsage()

	if int64(got.Usage.CacheCreationTokens) != 20 || int64(got.Usage.WebSearchCount) != 2 {
		t.Fatalf("stream usage lost cache creation/web search: %+v", got.Usage)
	}

	if got.Usage != exp {
		t.Fatalf("stream usage %+v != non-stream usage %+v", got.Usage, exp)
	}
}
