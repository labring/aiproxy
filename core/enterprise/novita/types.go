//go:build enterprise

package novita

import (
	"time"

	"github.com/labring/aiproxy/core/enterprise/synccommon"
)

// novitaPricePerMDivisor converts Novita's raw price field (万分之一美元/百万token)
// to USD/token.  raw / novitaPricePerMDivisor = USD/token.
//
// Derivation: raw / 10_000 = USD/百万token, then / 1_000_000 for per-token.
// Cross-verified: llama-3.3-70b raw=1350 → 1350/10000=$0.135/M (matches novita.ai/pricing).
const novitaPricePerMDivisor = 10_000_000_000

// NovitaModelStatusAvailable is the status value for available models.
const NovitaModelStatusAvailable = 1

// NovitaModel represents a model from the standard Novita /v3/openai/models API.
type NovitaModel struct {
	ID                   string   `json:"id"`
	Object               string   `json:"object"`
	Created              int64    `json:"created"`
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	ContextSize          int64    `json:"context_size"`
	MaxOutputTokens      int64    `json:"max_output_tokens"`
	InputTokenPricePerM  int64    `json:"input_token_price_per_m"`
	OutputTokenPricePerM int64    `json:"output_token_price_per_m"`
	Endpoints            []string `json:"endpoints"`
	Features             []string `json:"features"`
	InputModalities      []string `json:"input_modalities"`
	OutputModalities     []string `json:"output_modalities"`
	ModelType            string   `json:"model_type"`
	Tags                 []any    `json:"tags"`
	Status               int      `json:"status"`
}

// NovitaModelsResponse is the response from GET /v3/openai/models.
type NovitaModelsResponse struct {
	Data []NovitaModel `json:"data"`
}

// IsAvailable reports whether the model is operational.
// V1 API typically returns only available models; treat missing status (0) as available.
func (m *NovitaModel) IsAvailable() bool {
	return m.Status == NovitaModelStatusAvailable || m.Status == 0
}

// GetInputPricePerToken converts raw price to USD/token.
func (m *NovitaModel) GetInputPricePerToken() float64 {
	return float64(m.InputTokenPricePerM) / novitaPricePerMDivisor
}

// GetOutputPricePerToken converts raw price to USD/token.
func (m *NovitaModel) GetOutputPricePerToken() float64 {
	return float64(m.OutputTokenPricePerM) / novitaPricePerMDivisor
}

// ToV2 converts a V1 NovitaModel to NovitaModelV2 format for unified processing.
// V2-only fields (tiered billing, cache, RPM/TPM) remain zero-valued,
// which the V2 create/update functions handle gracefully.
func (m *NovitaModel) ToV2() NovitaModelV2 {
	return NovitaModelV2{
		ID:                   m.ID,
		Title:                m.Title,
		Description:          m.Description,
		ModelType:            m.ModelType,
		ContextSize:          m.ContextSize,
		MaxOutputTokens:      m.MaxOutputTokens,
		InputTokenPricePerM:  m.InputTokenPricePerM,
		OutputTokenPricePerM: m.OutputTokenPricePerM,
		Endpoints:            m.Endpoints,
		Features:             m.Features,
		InputModalities:      m.InputModalities,
		OutputModalities:     m.OutputModalities,
		Status:               m.Status,
		Tags:                 m.Tags,
	}
}

// NovitaPricing represents a pricing entry from the management API (unit: 万分之一美元/百万token).
type NovitaPricing struct {
	OriginPricePerM int64 `json:"originPricePerM"`
	PricePerM       int64 `json:"pricePerM"`
}

// PricePerToken converts the raw PricePerM to USD/token.
func (p NovitaPricing) PricePerToken() float64 {
	return float64(p.PricePerM) / novitaPricePerMDivisor
}

// TieredBillingConfig represents a tiered billing tier from the management API.
type TieredBillingConfig struct {
	MinTokens                      int64          `json:"min_tokens"`
	MaxTokens                      int64          `json:"max_tokens"`
	InputPricing                   NovitaPricing  `json:"input_pricing"`
	OutputPricing                  NovitaPricing  `json:"output_pricing"`
	CacheReadInputPricing          NovitaPricing  `json:"cache_read_input_pricing"`
	CacheCreationInputPricing      NovitaPricing  `json:"cache_creation_input_pricing"`
	CacheCreation1HourInputPricing NovitaPricing  `json:"cache_creation_1_hour_input_pricing"`
}

// NovitaModelV2 represents a model from the Novita management API
// (api-server.novita.ai/v1/product/model/list).
type NovitaModelV2 struct {
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
	InputPricing                          NovitaPricing       `json:"input_pricing"`
	OutputPricing                         NovitaPricing       `json:"output_pricing"`
	Series                                string              `json:"series"`
	Quantization                          string              `json:"quantization"`
	RPM                                   int                 `json:"rpm"`
	TPM                                   int                 `json:"tpm"`
	Labels                                []map[string]string `json:"labels"`
}

// NovitaMgmtModelsResponse is the response from the Novita management model list API.
type NovitaMgmtModelsResponse struct {
	Data []NovitaModelV2 `json:"data"`
}

// IsAvailable reports whether the model is operational.
func (m *NovitaModelV2) IsAvailable() bool {
	return m.Status == NovitaModelStatusAvailable
}

// GetInputPricePerToken converts raw price to USD/token.
func (m *NovitaModelV2) GetInputPricePerToken() float64 {
	return float64(m.InputTokenPricePerM) / novitaPricePerMDivisor
}

// GetOutputPricePerToken converts raw price to USD/token.
func (m *NovitaModelV2) GetOutputPricePerToken() float64 {
	return float64(m.OutputTokenPricePerM) / novitaPricePerMDivisor
}

// GetCacheReadPricePerToken converts raw cache read price to USD/token.
func (m *NovitaModelV2) GetCacheReadPricePerToken() float64 {
	return float64(m.CacheReadInputTokenPricePerM) / novitaPricePerMDivisor
}

// GetCacheCreationPricePerToken converts raw cache creation price to USD/token.
func (m *NovitaModelV2) GetCacheCreationPricePerToken() float64 {
	return float64(m.CacheCreationInputTokenPricePerM) / novitaPricePerMDivisor
}

// ModelDiff represents the difference for a single model.
type ModelDiff struct {
	ModelID   string         `json:"model_id"`
	Action    string         `json:"action"` // "add", "update", "delete"
	OldConfig map[string]any `json:"old_config,omitempty"`
	NewConfig map[string]any `json:"new_config,omitempty"`
	Changes   []string       `json:"changes,omitempty"`
}

// SyncDiff represents the comparison between remote and local models.
type SyncDiff struct {
	Summary  SyncSummary `json:"summary"`
	Changes  struct {
		Add    []ModelDiff `json:"add"`
		Update []ModelDiff `json:"update"`
		Delete []ModelDiff `json:"delete"`
	} `json:"changes"`
	Channels ChannelsInfo `json:"channels"`
}

// SyncSummary provides a summary of sync changes.
type SyncSummary struct {
	TotalModels int `json:"total_models"`
	ToAdd       int `json:"to_add"`
	ToUpdate    int `json:"to_update"`
	ToDelete    int `json:"to_delete"`
}

// ChannelsInfo contains information about Novita channels.
type ChannelsInfo struct {
	Novita ChannelInfo `json:"novita"`
}

// ChannelInfo represents channel status.
type ChannelInfo struct {
	Exists     bool `json:"exists"`
	ID         int  `json:"id,omitempty"`
	WillCreate bool `json:"will_create,omitempty"`
}

// SyncOptions represents options for a sync operation.
type SyncOptions struct {
	AutoCreateChannels       bool `json:"auto_create_channels"`
	ChangesConfirmed         bool `json:"changes_confirmed"`
	DryRun                   bool `json:"dry_run,omitempty"`
	DeleteUnmatchedModel     bool `json:"delete_unmatched_model"`
	AnthropicPurePassthrough bool `json:"anthropic_pure_passthrough"` // Enable pure passthrough for Anthropic channel
}

// SyncResult represents the result of a sync operation.
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

// SyncProgressEvent is an alias for the shared synccommon type.
type SyncProgressEvent = synccommon.SyncProgressEvent

// DiagnosticResult represents the result of a diagnostic check.
type DiagnosticResult struct {
	LastSyncAt   *time.Time   `json:"last_sync_at,omitempty"`
	LocalModels  int          `json:"local_models"`
	RemoteModels int          `json:"remote_models"`
	Diff         *SyncDiff    `json:"diff,omitempty"`
	Channels     ChannelsInfo `json:"channels"`
}

// SyncHistory represents a sync history record.
type SyncHistory struct {
	ID           int64       `json:"id"                      gorm:"primaryKey"`
	SyncedAt     time.Time   `json:"synced_at"               gorm:"autoCreateTime;index"`
	Operator     string      `json:"operator,omitempty"`
	SyncOptions  string      `json:"sync_options"` // JSON
	Result       string      `json:"result"`       // JSON
	Status       string      `json:"status"`       // "success", "partial", "failed"
	CreatedAt    time.Time   `json:"created_at"              gorm:"autoCreateTime"`
	ResultParsed *SyncResult `json:"result_parsed,omitempty" gorm:"-"`
}

// TableName overrides the table name for SyncHistory.
func (SyncHistory) TableName() string {
	return "novita_sync_history"
}

// ModelCoverageItem is a model that has a ModelConfig but is not in any enabled Novita channel.
type ModelCoverageItem struct {
	Model     string   `json:"model"`
	Endpoints []string `json:"endpoints,omitempty"`
	ModelType string   `json:"model_type,omitempty"`
}

// ModelCoverageResult is returned by ModelCoverageHandler.
type ModelCoverageResult struct {
	Total     int                 `json:"total"`
	Covered   int                 `json:"covered"`
	Uncovered []ModelCoverageItem `json:"uncovered"`
}
