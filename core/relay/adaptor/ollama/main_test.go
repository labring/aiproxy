package ollama_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/ollama"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
)

func TestConvertRequest_JsonObject(t *testing.T) {
	// Setup metadata
	meta := meta.NewMeta(
		&model.Channel{
			Type: model.ChannelTypeOllama,
		},
		0,
		"llama2",
		model.ModelConfig{},
	)

	// Create OpenAI request with response_format: {"type": "json_object"}
	openAIReq := relaymodel.GeneralOpenAIRequest{
		Model: "llama2",
		Messages: []relaymodel.Message{
			{
				Role:    "user",
				Content: "Hello, give me JSON",
			},
		},
		ResponseFormat: &relaymodel.ResponseFormat{
			Type: "json_object",
		},
	}

	jsonData, _ := sonic.Marshal(openAIReq)
	req, _ := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost:11434/api/chat",
		bytes.NewBuffer(jsonData),
	)

	// Convert request
	result, err := ollama.ConvertRequest(meta, req)
	assert.NoError(t, err)

	// Parse body to check format field
	bodyBytes, _ := io.ReadAll(result.Body)

	var ollamaReq ollama.ChatRequest

	err = json.Unmarshal(bodyBytes, &ollamaReq)
	assert.NoError(t, err)

	// Verify format is "json"
	assert.Equal(t, "json", ollamaReq.Format)
}
