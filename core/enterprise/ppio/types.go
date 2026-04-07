//go:build enterprise

package ppio

import (
	"time"

	"github.com/labring/aiproxy/core/enterprise/synccommon"
)

// PPIO model status constants.
// Status 1 means the model is available for inference; other values indicate
// the model is visible in the catalog but not operational (e.g. coming soon,
// maintenance, deprecated). Unavailable models are filtered during sync to
// avoid "MODEL_NOT_AVAILABLE" 500 errors at request time.
const PPIOModelStatusAvailable = 1

// ppioPricePerMDivisor converts PPIO's raw price field (万分之一元/百万token)
// to 元/token.  raw / ppioPricePerMDivisor = 元/token.
//
// Derivation: raw / 10_000 = 元/百万token, then / 1_000_000 for per-token.
// Cross-verified against official pricing for GPT-4.1 nano, Claude Opus 4,
// DeepSeek-R1, DeepSeek-V3-Turbo (all exact ¥ match at CNY/USD 7.25).
const ppioPricePerMDivisor = 10_000_000_000

// PPIOModel represents a model from PPIO API
type PPIOModel struct {
	ID                   string         `json:"id"`
	Object               string         `json:"object"`
	Created              int64          `json:"created"`
	Title                string         `json:"title"`
	Description          string         `json:"description"`
	ContextSize          int64          `json:"context_size"`
	MaxOutputTokens      int64          `json:"max_output_tokens"`
	InputTokenPricePerM  float64        `json:"input_token_price_per_m"`  // 万分之一元/百万token (raw / 10000 = ¥/Mt)
	OutputTokenPricePerM float64        `json:"output_token_price_per_m"` // 万分之一元/百万token (raw / 10000 = ¥/Mt)
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

// IsAvailable reports whether the model is operational.
func (m *PPIOModel) IsAvailable() bool {
	return m.Status == PPIOModelStatusAvailable
}

// GetInputPricePerToken returns input price in 元/token.
func (m *PPIOModel) GetInputPricePerToken() float64 {
	return m.InputTokenPricePerM / ppioPricePerMDivisor
}

// GetOutputPricePerToken returns output price in 元/token.
func (m *PPIOModel) GetOutputPricePerToken() float64 {
	return m.OutputTokenPricePerM / ppioPricePerMDivisor
}

// PPIOPricing represents a pricing entry from the management API (unit: 万分之一元/百万token)
type PPIOPricing struct {
	OriginPricePerM int64 `json:"originPricePerM"`
	PricePerM       int64 `json:"pricePerM"`
}

// PricePerToken converts the raw PricePerM to 元/token.
func (p PPIOPricing) PricePerToken() float64 {
	return float64(p.PricePerM) / ppioPricePerMDivisor
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

// IsAvailable reports whether the model is operational.
func (m *PPIOModelV2) IsAvailable() bool {
	return m.Status == PPIOModelStatusAvailable
}

// PPIOMgmtModelsResponse represents the response from the PPIO management model list API
type PPIOMgmtModelsResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    []PPIOModelV2 `json:"data"`
}

// GetInputPricePerToken converts 万分之一元/百万token → 元/token.
func (m *PPIOModelV2) GetInputPricePerToken() float64 {
	return float64(m.InputTokenPricePerM) / ppioPricePerMDivisor
}

// GetOutputPricePerToken converts 万分之一元/百万token → 元/token.
func (m *PPIOModelV2) GetOutputPricePerToken() float64 {
	return float64(m.OutputTokenPricePerM) / ppioPricePerMDivisor
}

// GetCacheReadPricePerToken converts 万分之一元/百万token → 元/token.
func (m *PPIOModelV2) GetCacheReadPricePerToken() float64 {
	return float64(m.CacheReadInputTokenPricePerM) / ppioPricePerMDivisor
}

// GetCacheCreationPricePerToken converts 万分之一元/百万token → 元/token.
func (m *PPIOModelV2) GetCacheCreationPricePerToken() float64 {
	return float64(m.CacheCreationInputTokenPricePerM) / ppioPricePerMDivisor
}

// ToV2 converts a V1 PPIOModel to PPIOModelV2 format for unified processing.
// V2-only fields (tiered billing, cache, RPM/TPM) remain zero-valued,
// which the V2 create/update functions handle gracefully.
func (m *PPIOModel) ToV2() PPIOModelV2 {
	return PPIOModelV2{
		ID:                   m.ID,
		Title:                m.Title,
		Description:          m.Description,
		ModelType:            m.ModelType,
		ContextSize:          m.ContextSize,
		MaxOutputTokens:      m.MaxOutputTokens,
		InputTokenPricePerM:  int64(m.InputTokenPricePerM),
		OutputTokenPricePerM: int64(m.OutputTokenPricePerM),
		Endpoints:            m.Endpoints,
		Features:             m.Features,
		InputModalities:      m.InputModalities,
		OutputModalities:     m.OutputModalities,
		Status:               m.Status,
		Tags:                 m.Tags,
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
	AutoCreateChannels       bool `json:"auto_create_channels"`
	ChangesConfirmed         bool `json:"changes_confirmed"`          // User confirmed the changes
	DryRun                   bool `json:"dry_run,omitempty"`          // Preview only, don't execute
	DeleteUnmatchedModel     bool `json:"delete_unmatched_model"`     // Delete local models not in PPIO
	AnthropicPurePassthrough bool `json:"anthropic_pure_passthrough"` // Enable pure passthrough for Anthropic channel
	AllowPassthroughUnknown *bool `json:"allow_passthrough_unknown,omitempty"` // Route requests for models not in the model list to this channel; nil = preserve existing
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

// SyncProgressEvent is an alias for the shared synccommon type.
type SyncProgressEvent = synccommon.SyncProgressEvent

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

// ModelCoverageItem is a model that has a ModelConfig but is not in any enabled PPIO channel.
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

// ── Multimodal API types ──────────────────────────────────────────────────

// multimodalPriceDivisor converts the batch-price API's basePrice0 field
// to 元/次.  basePrice0 / multimodalPriceDivisor = ¥/request.
// Verified: seedream-4.5 = 2500 → ¥0.025/张, wan2.6-t2v 720P 5s = 30000 → ¥0.30/次.
const multimodalPriceDivisor = 100_000

// PPIOMultimodalModel represents a model from the multimodal-model/list API
// (api-server.ppio.com/v1/product/multimodal-model/list).
type PPIOMultimodalModel struct {
	FusionConfig PPIOMMFusionConfig `json:"fusionConfig"`
	ModelConfig  PPIOMMModelConfig  `json:"modelConfig"`
}

// PPIOMMFusionConfig holds display metadata for a multimodal model.
type PPIOMMFusionConfig struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Series      string `json:"series"`
	Description string `json:"description"`
}

// PPIOMMModelConfig holds billing and routing config for a multimodal model.
type PPIOMMModelConfig struct {
	Config      PPIOMMConfigDetail `json:"config"`
	SKUMappings []PPIOSKUMapping   `json:"skuMappings"`
}

// PPIOMMConfigDetail holds the inner config block (category, billing type).
type PPIOMMConfigDetail struct {
	Category string `json:"category"` // "video_gen", "image_gen", "audio_gen"
}

// PPIOSKUMapping maps a SKU code to its CEL matching expression.
// CELExpr is stored for future precise billing but not evaluated in the current implementation.
type PPIOSKUMapping struct {
	SKUCode string `json:"skuCode"`
	CELExpr string `json:"celExpr"`
}

// PPIOMultimodalListResponse is the response from the multimodal-model/list API.
type PPIOMultimodalListResponse struct {
	Configs []PPIOMultimodalModel `json:"configs"`
	Total   int                   `json:"total"`
}

// PPIOBatchPriceRequest is the request body for the batch-price API.
type PPIOBatchPriceRequest struct {
	BusinessType string   `json:"businessType"`
	ProductIDs   []string `json:"productIds"`
}

// PPIOBatchPriceResponse is the response from the batch-price API.
type PPIOBatchPriceResponse struct {
	Products []PPIOProductPrice `json:"products"`
}

// PPIOProductPrice represents a single product's pricing from the batch-price API.
type PPIOProductPrice struct {
	ProductID       string `json:"productId"`
	ProductCategory string `json:"productCategory"`
	BasePrice0      string `json:"basePrice0"` // string; parse to int64
}

// collectSKUCodes returns all SKU codes from a multimodal model's SKU mappings.
func (m *PPIOMultimodalModel) collectSKUCodes() []string {
	codes := make([]string, len(m.ModelConfig.SKUMappings))
	for i, s := range m.ModelConfig.SKUMappings {
		codes[i] = s.SKUCode
	}

	return codes
}

// minSKUPrice returns the minimum non-zero price across all of this model's SKUs.
// skuPrices maps skuCode → raw basePrice0 value from the batch-price API.
// Returns price in 元/次 (raw / multimodalPriceDivisor).
func (m *PPIOMultimodalModel) minSKUPrice(skuPrices map[string]int64) float64 {
	var minRaw int64

	for _, sku := range m.ModelConfig.SKUMappings {
		raw, ok := skuPrices[sku.SKUCode]
		if !ok || raw <= 0 {
			continue
		}

		if minRaw == 0 || raw < minRaw {
			minRaw = raw
		}
	}

	if minRaw == 0 {
		return 0
	}

	return float64(minRaw) / multimodalPriceDivisor
}

// multimodalCategoryToModelType maps the multimodal API's category field to
// the PPIO model_type strings used by ModelTypeToMode (sync.go).
func multimodalCategoryToModelType(category string) string {
	switch category {
	case "image_gen":
		return "image"
	case "video_gen":
		return "video"
	case "audio_gen":
		return "audio"
	default:
		return "video" // safe default for unknown multimodal categories
	}
}
