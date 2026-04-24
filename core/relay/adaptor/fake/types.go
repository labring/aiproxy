package fake

import (
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

const baseURL = "https://fake.local/v1"

type Adaptor struct {
	configCache utils.ChannelConfigCache[Config]
}

type Config struct {
	StaticText        string         `json:"static_text"`
	SystemFingerprint string         `json:"system_fingerprint"`
	ResponsePrefix    string         `json:"response_prefix"`
	ResponseSuffix    string         `json:"response_suffix"`
	ReasoningText     string         `json:"reasoning_text"`
	DelayMS           int            `json:"delay_ms"`
	StreamChunks      int            `json:"stream_chunks"`
	StreamChunkSize   int            `json:"stream_chunk_size"`
	Embedding         EmbeddingCfg   `json:"embedding"`
	Image             ImageCfg       `json:"image"`
	Rerank            RerankCfg      `json:"rerank"`
	Usage             UsageCfg       `json:"usage"`
	Response          ResponseCfg    `json:"response"`
	Anthropic         AnthropicCfg   `json:"anthropic"`
	Gemini            GeminiCfg      `json:"gemini"`
	OpenAPI           OpenAPICfg     `json:"openapi"`
	Metadata          map[string]any `json:"metadata"`
}

type EmbeddingCfg struct {
	Dimensions int    `json:"dimensions"`
	Base       string `json:"base"`
}

type ImageCfg struct {
	URL            string `json:"url"`
	B64JSON        string `json:"b64_json"`
	RevisedPrompt  string `json:"revised_prompt"`
	InputTokens    int64  `json:"input_tokens"`
	OutputTokens   int64  `json:"output_tokens"`
	ImageTokensIn  int64  `json:"image_tokens_in"`
	ImageTokensOut int64  `json:"image_tokens_out"`
}

type RerankCfg struct {
	BaseScore       float64 `json:"base_score"`
	Step            float64 `json:"step"`
	ReturnDocuments *bool   `json:"return_documents"`
}

type UsageCfg struct {
	InputTokens       int64 `json:"input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
	CachedTokens      int64 `json:"cached_tokens"`
	ReasoningTokens   int64 `json:"reasoning_tokens"`
	ImageInputTokens  int64 `json:"image_input_tokens"`
	ImageOutputTokens int64 `json:"image_output_tokens"`
	WebSearchCount    int64 `json:"web_search_count"`
}

type ResponseCfg struct {
	Store             *bool  `json:"store"`
	Status            string `json:"status"`
	ParallelToolCalls *bool  `json:"parallel_tool_calls"`
	IncludeOutputItem *bool  `json:"include_output_item"`
}

type AnthropicCfg struct {
	StopReason string `json:"stop_reason"`
	Type       string `json:"type"`
}

type GeminiCfg struct {
	FinishReason string `json:"finish_reason"`
	ModelVersion string `json:"model_version"`
}

type OpenAPICfg struct {
	SpecVersion string         `json:"spec_version"`
	Info        map[string]any `json:"info"`
	Components  map[string]any `json:"components"`
}

type requestContext struct {
	Text                string
	Model               string
	Stream              bool
	Stored              bool
	InputItems          []relaymodel.InputItem
	ImageResponseFormat string
	ImageSize           string
}
