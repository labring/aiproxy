//go:build enterprise

package ppio

import (
	"slices"
	"time"
)

// PPIOModel represents a model from PPIO API
type PPIOModel struct {
	ID                   string         `json:"id"`
	Object               string         `json:"object"`
	Created              int64          `json:"created"`
	Title                string         `json:"title"`
	Description          string         `json:"description"`
	ContextSize          int64          `json:"context_size"`
	MaxOutputTokens      int64          `json:"max_output_tokens"`
	InputTokenPricePerM  float64        `json:"input_token_price_per_m"`  // Price per million tokens (CNY 厘)
	OutputTokenPricePerM float64        `json:"output_token_price_per_m"` // Price per million tokens (CNY 厘)
	Endpoints            []string       `json:"endpoints"`
	Features             []string       `json:"features"`
	InputModalities      []string       `json:"input_modalities"`
	OutputModalities     []string       `json:"output_modalities"`
	ModelType            string         `json:"model_type"`
	Tags                 []any          `json:"tags"`
	Status               int            `json:"status"`
	Config               map[string]any `json:"config,omitempty"`
}

// PPIOModelsResponse represents the response from PPIO /v1/models API
type PPIOModelsResponse struct {
	Data []PPIOModel `json:"data"`
}

func (m PPIOModel) GetID() string        { return m.ID }
func (m PPIOModel) GetEndpoints() []string { return m.Endpoints }

// IsAnthropicCompatible checks if the model supports Anthropic endpoint
func (m *PPIOModel) IsAnthropicCompatible() bool {
	return slices.Contains(m.Endpoints, "anthropic")
}

// IsOpenAICompatible checks if the model supports OpenAI chat/completions endpoint
func (m *PPIOModel) IsOpenAICompatible() bool {
	return slices.Contains(m.Endpoints, "chat/completions")
}

// GetInputPricePerToken returns input price per token (not per million)
func (m *PPIOModel) GetInputPricePerToken() float64 {
	return m.InputTokenPricePerM / 1000000
}

// GetOutputPricePerToken returns output price per token (not per million)
func (m *PPIOModel) GetOutputPricePerToken() float64 {
	return m.OutputTokenPricePerM / 1000000
}

// PPIOPricing represents a pricing entry from the management API (unit: 厘/百万token)
type PPIOPricing struct {
	OriginPricePerM int64 `json:"originPricePerM"`
	PricePerM       int64 `json:"pricePerM"`
}

// TieredBillingConfig represents a tiered billing tier from the management API
type TieredBillingConfig struct {
	MinTokens                      int64       `json:"min_tokens"`
	MaxTokens                      int64       `json:"max_tokens"`
	InputPricing                   PPIOPricing `json:"input_pricing"`
	OutputPricing                  PPIOPricing `json:"output_pricing"`
	CacheReadInputPricing          PPIOPricing `json:"cache_read_input_pricing"`
	CacheCreationInputPricing      PPIOPricing `json:"cache_creation_input_pricing"`
	CacheCreation1HourInputPricing PPIOPricing `json:"cache_creation_1_hour_input_pricing"`
}

// PPIOModelV2 represents a model from the PPIO management API (full catalog including pa/ models)
type PPIOModelV2 struct {
	ID                                    string              `json:"id"`
	Title                                 string              `json:"title"`
	Description                           string              `json:"description"`
	DisplayName                           string              `json:"display_name"`
	ModelType                             string              `json:"model_type"`
	ContextSize                           int64               `json:"context_size"`
	MaxOutputTokens                       int64               `json:"max_output_tokens"`
	InputTokenPricePerM                   int64               `json:"input_token_price_per_m"`
	OutputTokenPricePerM                  int64               `json:"output_token_price_per_m"`
	Endpoints                             []string            `json:"endpoints"`
	Features                              []string            `json:"features"`
	InputModalities                       []string            `json:"input_modalities"`
	OutputModalities                      []string            `json:"output_modalities"`
	Status                                int                 `json:"status"`
	Tags                                  []any               `json:"tags"`
	IsTieredBilling                       bool                `json:"is_tiered_billing"`
	TieredBillingConfigs                  []TieredBillingConfig `json:"tiered_billing_configs"`
	SupportPromptCache                    bool                `json:"support_prompt_cache"`
	CacheReadInputTokenPricePerM          int64               `json:"cache_read_input_token_price_per_m"`
	CacheCreationInputTokenPricePerM      int64               `json:"cache_creation_input_token_price_per_m"`
	CacheCreation1HourInputTokenPricePerM int64               `json:"cache_creation_1_hour_input_token_price_per_m"`
	InputPricing                          PPIOPricing         `json:"input_pricing"`
	OutputPricing                         PPIOPricing         `json:"output_pricing"`
	Series                                string              `json:"series"`
	Quantization                          string              `json:"quantization"`
	RPM                                   int                 `json:"rpm"`
	TPM                                   int                 `json:"tpm"`
	Labels                                []map[string]string `json:"labels"`
}

// PPIOMgmtModelsResponse represents the response from the PPIO management model list API
type PPIOMgmtModelsResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    []PPIOModelV2 `json:"data"`
}

// GetInputPricePerToken converts 厘/百万token → 元/token
func (m *PPIOModelV2) GetInputPricePerToken() float64 {
	return float64(m.InputTokenPricePerM) / 1_000_000_000
}

// GetOutputPricePerToken converts 厘/百万token → 元/token
func (m *PPIOModelV2) GetOutputPricePerToken() float64 {
	return float64(m.OutputTokenPricePerM) / 1_000_000_000
}

// GetCacheReadPricePerToken converts 厘/百万token → 元/token
func (m *PPIOModelV2) GetCacheReadPricePerToken() float64 {
	return float64(m.CacheReadInputTokenPricePerM) / 1_000_000_000
}

// GetCacheCreationPricePerToken converts 厘/百万token → 元/token
func (m *PPIOModelV2) GetCacheCreationPricePerToken() float64 {
	return float64(m.CacheCreationInputTokenPricePerM) / 1_000_000_000
}

func (m PPIOModelV2) GetID() string        { return m.ID }
func (m PPIOModelV2) GetEndpoints() []string { return m.Endpoints }

// IsAnthropicCompatible checks if the V2 model supports Anthropic endpoint
func (m *PPIOModelV2) IsAnthropicCompatible() bool {
	return slices.Contains(m.Endpoints, "anthropic")
}

// IsOpenAICompatible checks if the V2 model supports OpenAI chat/completions endpoint
func (m *PPIOModelV2) IsOpenAICompatible() bool {
	return slices.Contains(m.Endpoints, "chat/completions")
}

// ToV1 converts a PPIOModelV2 to PPIOModel for backward compatibility
func (m *PPIOModelV2) ToV1() PPIOModel {
	return PPIOModel{
		ID:                   m.ID,
		Title:                m.Title,
		Description:          m.Description,
		ContextSize:          m.ContextSize,
		MaxOutputTokens:      m.MaxOutputTokens,
		InputTokenPricePerM:  float64(m.InputTokenPricePerM) / 1000, // 厘→元
		OutputTokenPricePerM: float64(m.OutputTokenPricePerM) / 1000,
		Endpoints:            m.Endpoints,
		Features:             m.Features,
		InputModalities:      m.InputModalities,
		OutputModalities:     m.OutputModalities,
		ModelType:            m.ModelType,
		Tags:                 m.Tags,
		Status:               m.Status,
	}
}

// ModelDiff represents the difference for a single model
type ModelDiff struct {
	ModelID   string         `json:"model_id"`
	Action    string         `json:"action"` // "add", "update", "delete"
	OldConfig map[string]any `json:"old_config,omitempty"`
	NewConfig map[string]any `json:"new_config,omitempty"`
	Changes   []string       `json:"changes,omitempty"` // List of changed fields
}

// SyncDiff represents the comparison between remote and local models
type SyncDiff struct {
	Summary SyncSummary `json:"summary"`
	Changes struct {
		Add    []ModelDiff `json:"add"`
		Update []ModelDiff `json:"update"`
		Delete []ModelDiff `json:"delete"`
	} `json:"changes"`
	Channels ChannelsInfo `json:"channels"`
}

// SyncSummary provides a summary of sync changes
type SyncSummary struct {
	TotalModels int `json:"total_models"`
	ToAdd       int `json:"to_add"`
	ToUpdate    int `json:"to_update"`
	ToDelete    int `json:"to_delete"`
}

// ChannelsInfo contains information about PPIO channels
type ChannelsInfo struct {
	PPIO ChannelInfo `json:"ppio"`
}

// ChannelInfo represents channel status and info
type ChannelInfo struct {
	Exists     bool `json:"exists"`
	ID         int  `json:"id,omitempty"`
	WillCreate bool `json:"will_create,omitempty"`
}

// SyncOptions represents options for sync operation
type SyncOptions struct {
	AutoCreateChannels   bool `json:"auto_create_channels"`
	ChangesConfirmed     bool `json:"changes_confirmed"`      // User confirmed the changes
	DryRun               bool `json:"dry_run,omitempty"`      // Preview only, don't execute
	DeleteUnmatchedModel bool `json:"delete_unmatched_model"` // Delete local models not in PPIO
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Success    bool        `json:"success"`
	Summary    SyncSummary `json:"summary"`
	DurationMS int64       `json:"duration_ms"`
	Errors     []string    `json:"errors,omitempty"`
	Details    struct {
		ModelsAdded   []string `json:"models_added,omitempty"`
		ModelsUpdated []string `json:"models_updated,omitempty"`
		ModelsDeleted []string `json:"models_deleted,omitempty"`
	} `json:"details,omitempty"`
	Channels ChannelsInfo `json:"channels,omitempty"`
}

// SyncProgressEvent represents a progress event sent via SSE
type SyncProgressEvent struct {
	Type     string `json:"type"`    // "progress", "success", "error"
	Step     string `json:"step"`    // "fetching", "comparing", "updating", "complete"
	Message  string `json:"message"` // Human-readable message
	Progress int    `json:"progress,omitempty"`
	Data     any    `json:"data,omitempty"` // Additional data (e.g., SyncResult on success)
}

// DiagnosticResult represents the result of diagnostic check
type DiagnosticResult struct {
	LastSyncAt   *time.Time   `json:"last_sync_at,omitempty"`
	LocalModels  int          `json:"local_models"`
	RemoteModels int          `json:"remote_models"`
	Diff         *SyncDiff    `json:"diff,omitempty"`
	Channels     ChannelsInfo `json:"channels"`
}

// SyncHistory represents a sync history record
type SyncHistory struct {
	ID           int64       `json:"id"                      gorm:"primaryKey"`
	SyncedAt     time.Time   `json:"synced_at"               gorm:"autoCreateTime;index"`
	Operator     string      `json:"operator,omitempty"`
	SyncOptions  string      `json:"sync_options"` // JSON
	Result       string      `json:"result"`       // JSON
	Status       string      `json:"status"`       // "success", "partial", "failed"
	CreatedAt    time.Time   `json:"created_at"              gorm:"autoCreateTime"`
	ResultParsed *SyncResult `json:"result_parsed,omitempty" gorm:"-"` // Parsed result for API response
}

// TableName overrides the table name for SyncHistory
func (SyncHistory) TableName() string {
	return "ppio_sync_history"
}
