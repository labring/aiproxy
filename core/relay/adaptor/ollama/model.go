package ollama

type Options struct {
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
	Seed             int      `json:"seed,omitempty"`
	TopK             int      `json:"top_k,omitempty"`
	NumPredict       int      `json:"num_predict,omitempty"`
	NumCtx           int      `json:"num_ctx,omitempty"`
	Stop             any      `json:"stop,omitempty"`
}

type Message struct {
	Role       string   `json:"role,omitempty"`
	Content    string   `json:"content,omitempty"`
	ToolCallID string   `json:"tool_call_id,omitempty"`
	ToolCalls  []*Tool  `json:"tool_calls,omitempty"`
	Images     []string `json:"images,omitempty"`
}

type Tool struct {
	ID       string   `json:"id,omitempty"`
	Type     string   `json:"type,omitempty"`
	Function Function `json:"function"`
}

type Function struct {
	Parameters  any            `json:"parameters,omitempty"`
	Arguments   map[string]any `json:"arguments,omitempty"`
	Description string         `json:"description,omitempty"`
	Name        string         `json:"name,omitempty"`
}

type ChatRequest struct {
	Options  *Options  `json:"options,omitempty"`
	Model    string    `json:"model,omitempty"`
	Messages []Message `json:"messages,omitempty"`
	Prompt   any       `json:"prompt,omitempty"`
	Stream   bool      `json:"stream"`
	Format   any       `json:"format,omitempty"`
	Tools    []*Tool   `json:"tools,omitempty"`
}

type ChatResponse struct {
	Model           string   `json:"model,omitempty"`
	CreatedAt       string   `json:"created_at,omitempty"`
	Response        string   `json:"response,omitempty"`
	Message         *Message `json:"message,omitempty"`
	TotalDuration   int      `json:"total_duration,omitempty"`
	LoadDuration    int      `json:"load_duration,omitempty"`
	PromptEvalCount int64    `json:"prompt_eval_count,omitempty"`
	EvalCount       int64    `json:"eval_count,omitempty"`
	EvalDuration    int      `json:"eval_duration,omitempty"`
	Done            bool     `json:"done,omitempty"`
	DoneReason      string   `json:"done_reason,omitempty"`
}

type EmbeddingRequest struct {
	Options *Options `json:"options,omitempty"`
	Model   string   `json:"model"`
	Input   []string `json:"input"`
}

type EmbeddingResponse struct {
	Error           string      `json:"error,omitempty"`
	Model           string      `json:"model"`
	Embeddings      [][]float64 `json:"embeddings"`
	PromptEvalCount int64       `json:"prompt_eval_count,omitempty"`
}
