//go:build enterprise

package novita

import "time"

// novitaPricePerMDivisor converts Novita's raw price field (万分之一美元/百万token)
// to USD/token.  raw / novitaPricePerMDivisor = USD/token.
//
// Derivation: raw / 10_000 = USD/百万token, then / 1_000_000 for per-token.
// Cross-verified: llama-3.3-70b raw=1350 → 1350/10000=$0.135/M (matches novita.ai/pricing).
const novitaPricePerMDivisor = 10_000_000_000

// NovitaModelStatusAvailable is the status value for available models.
const NovitaModelStatusAvailable = 1

// NovitaModel represents a model from the standard Novita /v1/models API.
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

// NovitaModelsResponse is the response from GET /v1/models.
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

// NovitaModelV2 represents a model from the Novita management API
// (api-server.novita.ai/v1/user/info). Field structure is inferred from similar providers;
// verify against actual API response and update as needed.
type NovitaModelV2 struct {
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	DisplayName          string   `json:"display_name"`
	ModelType            string   `json:"model_type"`
	ContextSize          int64    `json:"context_size"`
	MaxOutputTokens      int64    `json:"max_output_tokens"`
	InputTokenPricePerM  int64    `json:"input_token_price_per_m"`
	OutputTokenPricePerM int64    `json:"output_token_price_per_m"`
	Endpoints            []string `json:"endpoints"`
	Features             []string `json:"features"`
	InputModalities      []string `json:"input_modalities"`
	OutputModalities     []string `json:"output_modalities"`
	Status               int      `json:"status"`
	Tags                 []any    `json:"tags"`
	Series               string   `json:"series"`
}

// NovitaMgmtModelsResponse is the response from the Novita management API.
// The exact wrapper key ("data") must be verified against real API response.
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
	AutoCreateChannels   bool `json:"auto_create_channels"`
	ChangesConfirmed     bool `json:"changes_confirmed"`
	DryRun               bool `json:"dry_run,omitempty"`
	DeleteUnmatchedModel bool `json:"delete_unmatched_model"`
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

// SyncProgressEvent represents a progress event sent via SSE.
type SyncProgressEvent struct {
	Type     string `json:"type"`    // "progress", "success", "error"
	Step     string `json:"step"`    // "fetching", "comparing", "syncing", "complete"
	Message  string `json:"message"` // Human-readable message
	Progress int    `json:"progress,omitempty"`
	Data     any    `json:"data,omitempty"`
}

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
