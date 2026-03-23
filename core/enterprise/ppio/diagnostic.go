//go:build enterprise

package ppio

import (
	"fmt"
	"math"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
)

// ComparePPIOModels compares remote PPIO models (V1) with local database models
func ComparePPIOModels(remoteModels []PPIOModel, opts SyncOptions) (*SyncDiff, error) {
	// Fetch local models owned by PPIO
	var localModels []model.ModelConfig

	err := model.DB.Where("owner = ?", string(model.ModelOwnerPPIO)).Find(&localModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query local models: %w", err)
	}

	// Build local model map for quick lookup
	localModelMap := make(map[string]*model.ModelConfig)
	for i := range localModels {
		localModelMap[localModels[i].Model] = &localModels[i]
	}

	// Filter: only keep available models (Status == 1).
	// Unavailable models (coming-soon, maintenance, deprecated) would cause
	// MODEL_NOT_AVAILABLE errors at inference time.
	available := make([]PPIOModel, 0, len(remoteModels))
	for _, m := range remoteModels {
		if m.IsAvailable() {
			available = append(available, m)
		}
	}

	// Build remote model map from available models only
	remoteModelMap := make(map[string]*PPIOModel, len(available))
	for i := range available {
		remoteModelMap[available[i].ID] = &available[i]
	}

	diff := &SyncDiff{
		Summary: SyncSummary{
			TotalModels: len(available),
		},
	}

	// Find models to add and update
	for _, remoteModel := range available {
		localModel, exists := localModelMap[remoteModel.ID]
		if !exists {
			// Model doesn't exist locally - needs to be added
			diff.Changes.Add = append(diff.Changes.Add, ModelDiff{
				ModelID:   remoteModel.ID,
				Action:    "add",
				NewConfig: buildModelConfigMap(&remoteModel),
			})
			diff.Summary.ToAdd++
		} else {
			// Model exists - check if needs update
			changes := compareModelConfigs(localModel, &remoteModel)
			if len(changes) > 0 {
				diff.Changes.Update = append(diff.Changes.Update, ModelDiff{
					ModelID:   remoteModel.ID,
					Action:    "update",
					OldConfig: buildLocalModelConfigMap(localModel),
					NewConfig: buildModelConfigMap(&remoteModel),
					Changes:   changes,
				})
				diff.Summary.ToUpdate++
			}
		}
	}

	// Find models to delete (only if DeleteUnmatchedModel is enabled)
	if opts.DeleteUnmatchedModel {
		for modelID := range localModelMap {
			if _, exists := remoteModelMap[modelID]; !exists {
				diff.Changes.Delete = append(diff.Changes.Delete, ModelDiff{
					ModelID:   modelID,
					Action:    "delete",
					OldConfig: buildLocalModelConfigMap(localModelMap[modelID]),
				})
				diff.Summary.ToDelete++
			}
		}
	}

	// Check channel status
	diff.Channels = checkChannelStatus(opts)

	return diff, nil
}

// ComparePPIOModelsV2 compares remote PPIO models (V2) with local database models
func ComparePPIOModelsV2(remoteModels []PPIOModelV2, opts SyncOptions) (*SyncDiff, error) {
	// Fetch local models owned by PPIO
	var localModels []model.ModelConfig

	err := model.DB.Where("owner = ?", string(model.ModelOwnerPPIO)).Find(&localModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query local models: %w", err)
	}

	// Build local model map for quick lookup
	localModelMap := make(map[string]*model.ModelConfig)
	for i := range localModels {
		localModelMap[localModels[i].Model] = &localModels[i]
	}

	// Filter: only keep available models (Status == 1).
	available := make([]PPIOModelV2, 0, len(remoteModels))
	for _, m := range remoteModels {
		if m.IsAvailable() {
			available = append(available, m)
		}
	}

	// Build remote model map from available models only
	remoteModelMap := make(map[string]*PPIOModelV2, len(available))
	for i := range available {
		remoteModelMap[available[i].ID] = &available[i]
	}

	diff := &SyncDiff{
		Summary: SyncSummary{
			TotalModels: len(available),
		},
	}

	// Find models to add and update
	for _, remoteModel := range available {
		localModel, exists := localModelMap[remoteModel.ID]
		if !exists {
			diff.Changes.Add = append(diff.Changes.Add, ModelDiff{
				ModelID:   remoteModel.ID,
				Action:    "add",
				NewConfig: buildModelV2ConfigMap(&remoteModel),
			})
			diff.Summary.ToAdd++
		} else {
			changes := compareModelConfigsV2(localModel, &remoteModel)
			if len(changes) > 0 {
				diff.Changes.Update = append(diff.Changes.Update, ModelDiff{
					ModelID:   remoteModel.ID,
					Action:    "update",
					OldConfig: buildLocalModelConfigMap(localModel),
					NewConfig: buildModelV2ConfigMap(&remoteModel),
					Changes:   changes,
				})
				diff.Summary.ToUpdate++
			}
		}
	}

	// Find models to delete (only if DeleteUnmatchedModel is enabled)
	if opts.DeleteUnmatchedModel {
		for modelID := range localModelMap {
			if _, exists := remoteModelMap[modelID]; !exists {
				diff.Changes.Delete = append(diff.Changes.Delete, ModelDiff{
					ModelID:   modelID,
					Action:    "delete",
					OldConfig: buildLocalModelConfigMap(localModelMap[modelID]),
				})
				diff.Summary.ToDelete++
			}
		}
	}

	// Check channel status
	diff.Channels = checkChannelStatus(opts)

	return diff, nil
}

// compareModelConfigs compares local and remote model configs and returns list of changes
func compareModelConfigs(local *model.ModelConfig, remote *PPIOModel) []string {
	var changes []string

	// Compare prices
	remoteInputPrice := remote.GetInputPricePerToken()
	remoteOutputPrice := remote.GetOutputPricePerToken()

	if !floatEquals(float64(local.Price.InputPrice), remoteInputPrice) {
		changes = append(
			changes,
			fmt.Sprintf(
				"input_price: %.8f → %.8f",
				float64(local.Price.InputPrice),
				remoteInputPrice,
			),
		)
	}

	if !floatEquals(float64(local.Price.OutputPrice), remoteOutputPrice) {
		changes = append(
			changes,
			fmt.Sprintf(
				"output_price: %.8f → %.8f",
				float64(local.Price.OutputPrice),
				remoteOutputPrice,
			),
		)
	}

	// Compare config fields (normalize both through map[string]any to ensure
	// consistent key ordering — sonic sorts map[string]any keys but not map[ModelConfigKey]any)
	if !configMapsEqual(local.Config, buildConfigFromPPIOModel(remote)) {
		changes = append(changes, "config updated")
	}

	return changes
}

// compareModelConfigsV2 compares local and V2 remote model configs and returns list of changes
func compareModelConfigsV2(local *model.ModelConfig, remote *PPIOModelV2) []string {
	var changes []string

	// Compare prices
	remoteInputPrice := remote.GetInputPricePerToken()
	remoteOutputPrice := remote.GetOutputPricePerToken()

	if !floatEquals(float64(local.Price.InputPrice), remoteInputPrice) {
		changes = append(
			changes,
			fmt.Sprintf(
				"input_price: %.12f → %.12f",
				float64(local.Price.InputPrice),
				remoteInputPrice,
			),
		)
	}

	if !floatEquals(float64(local.Price.OutputPrice), remoteOutputPrice) {
		changes = append(
			changes,
			fmt.Sprintf(
				"output_price: %.12f → %.12f",
				float64(local.Price.OutputPrice),
				remoteOutputPrice,
			),
		)
	}

	// Compare tiered billing (count effective tiers, excluding degenerate ones
	// that are skipped during sync — see setPriceFromV2Model)
	remoteTieredCount := 0
	if remote.IsTieredBilling {
		remoteTieredCount = countEffectiveTiers(remote.TieredBillingConfigs)
	}

	localTieredCount := len(local.Price.ConditionalPrices)
	if localTieredCount != remoteTieredCount {
		changes = append(changes, fmt.Sprintf("tiered_billing_count: %d → %d", localTieredCount, remoteTieredCount))
	}

	// Compare cache pricing
	remoteCacheRead := remote.GetCacheReadPricePerToken()
	if remote.SupportPromptCache && !floatEquals(float64(local.Price.CachedPrice), remoteCacheRead) {
		changes = append(changes, fmt.Sprintf("cache_read_price: %.12f → %.12f",
			float64(local.Price.CachedPrice), remoteCacheRead))
	}

	// Compare config fields (normalize both through map[string]any to ensure
	// consistent key ordering — sonic sorts map[string]any keys but not map[ModelConfigKey]any)
	if !configMapsEqual(local.Config, buildConfigFromPPIOModelV2(remote)) {
		changes = append(changes, "config updated")
	}

	return changes
}

// configMapsEqual compares two config maps by normalizing both through map[string]any.
// This is needed because sonic serializes map[ModelConfigKey]any with non-deterministic
// key order, while map[string]any keys are sorted. Normalizing both to map[string]any
// ensures consistent JSON output for byte-level comparison.
func configMapsEqual(localConfig map[model.ModelConfigKey]any, remoteConfig map[string]any) bool {
	// Normalize local: map[ModelConfigKey]any → JSON → map[string]any → JSON (sorted keys)
	localJSON, _ := sonic.Marshal(localConfig)

	var normalizedLocal map[string]any

	_ = sonic.Unmarshal(localJSON, &normalizedLocal)

	// Use sonic.ConfigStd which sorts map keys (like encoding/json)
	normalizedLocalJSON, _ := sonic.ConfigStd.Marshal(normalizedLocal)

	// Normalize remote the same way: marshal → unmarshal → marshal
	// so numeric types are consistent (int64 → float64)
	remoteJSON, _ := sonic.Marshal(remoteConfig)

	var normalizedRemote map[string]any

	_ = sonic.Unmarshal(remoteJSON, &normalizedRemote)

	normalizedRemoteJSON, _ := sonic.ConfigStd.Marshal(normalizedRemote)

	return string(normalizedLocalJSON) == string(normalizedRemoteJSON)
}

// floatEquals compares two float64 values with tolerance
func floatEquals(a, b float64) bool {
	tolerance := 1e-10
	return math.Abs(a-b) < tolerance
}

// buildModelConfigMap builds a map representation of remote V1 model config
func buildModelConfigMap(m *PPIOModel) map[string]any {
	return map[string]any{
		"model":        m.ID,
		"title":        m.Title,
		"description":  m.Description,
		"input_price":  m.GetInputPricePerToken(),
		"output_price": m.GetOutputPricePerToken(),
		"context_size": m.ContextSize,
		"max_outputs":  m.MaxOutputTokens,
		"endpoints":    m.Endpoints,
		"features":     m.Features,
		"model_type":   m.ModelType,
		"tags":         m.Tags,
		"status":       m.Status,
	}
}

// buildModelV2ConfigMap builds a map representation of remote V2 model config
func buildModelV2ConfigMap(m *PPIOModelV2) map[string]any {
	return map[string]any{
		"model":           m.ID,
		"title":           m.Title,
		"description":     m.Description,
		"input_price":     m.GetInputPricePerToken(),
		"output_price":    m.GetOutputPricePerToken(),
		"context_size":    m.ContextSize,
		"max_outputs":     m.MaxOutputTokens,
		"endpoints":       m.Endpoints,
		"features":        m.Features,
		"model_type":      m.ModelType,
		"tags":            m.Tags,
		"status":          m.Status,
		"is_tiered":       m.IsTieredBilling,
		"support_cache":   m.SupportPromptCache,
		"cache_read":      m.GetCacheReadPricePerToken(),
		"cache_creation":  m.GetCacheCreationPricePerToken(),
	}
}

// buildLocalModelConfigMap builds a map representation of local model config
func buildLocalModelConfigMap(m *model.ModelConfig) map[string]any {
	return map[string]any{
		"model":        m.Model,
		"input_price":  float64(m.Price.InputPrice),
		"output_price": float64(m.Price.OutputPrice),
		"rpm":          m.RPM,
		"tpm":          m.TPM,
		"config":       m.Config,
	}
}

// buildConfigFromPPIOModel builds model config from V1 PPIO model
func buildConfigFromPPIOModel(m *PPIOModel) map[string]any {
	return map[string]any{
		"max_context_tokens": m.ContextSize,
		"max_output_tokens":  m.MaxOutputTokens,
		"title":              m.Title,
		"description":        m.Description,
		"features":           m.Features,
		"endpoints":          m.Endpoints,
		"input_modalities":   m.InputModalities,
		"output_modalities":  m.OutputModalities,
		"model_type":         m.ModelType,
		"tags":               m.Tags,
		"status":             m.Status,
	}
}

// checkChannelStatus checks if a PPIO channel exists (by base_url containing "ppio").
func checkChannelStatus(opts SyncOptions) ChannelsInfo {
	info := ChannelsInfo{}

	var ppioChannel model.Channel

	err := model.DB.Where("base_url "+likeOp()+" ?", "%ppio%").First(&ppioChannel).Error
	if err == nil {
		info.PPIO.Exists = true
		info.PPIO.ID = ppioChannel.ID
	} else {
		info.PPIO.WillCreate = opts.AutoCreateChannels
	}

	return info
}

// Diagnostic performs a diagnostic check without executing sync
func Diagnostic() (*DiagnosticResult, error) {
	client, err := NewPPIOClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create PPIO client: %w", err)
	}

	cfg := GetPPIOConfig()

	var remoteCount int

	var diff *SyncDiff

	if cfg.MgmtToken != "" {
		// Use management API for full model catalog
		v2Models, fetchErr := client.FetchAllModels(cfg.MgmtToken)
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch remote models (mgmt API): %w", fetchErr)
		}

		diff, err = ComparePPIOModelsV2(v2Models, SyncOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to compare models: %w", err)
		}

		// Use TotalModels from diff (which already excludes unavailable models)
		remoteCount = diff.Summary.TotalModels
	} else {
		// Fall back to public API
		remoteModels, fetchErr := client.FetchModels()
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch remote models: %w", fetchErr)
		}

		diff, err = ComparePPIOModels(remoteModels, SyncOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to compare models: %w", err)
		}

		remoteCount = diff.Summary.TotalModels
	}

	// Count local models
	var localCount int64

	err = model.DB.Model(&model.ModelConfig{}).
		Where("owner = ?", string(model.ModelOwnerPPIO)).
		Count(&localCount).
		Error
	if err != nil {
		return nil, fmt.Errorf("failed to count local models: %w", err)
	}

	// Get last sync time (from history if table exists)
	var (
		lastSyncAt *time.Time
		lastSync   SyncHistory
	)

	if model.DB.Migrator().HasTable(&SyncHistory{}) {
		err = model.DB.Order("synced_at DESC").First(&lastSync).Error
		if err == nil {
			lastSyncAt = &lastSync.SyncedAt
		}
	}

	result := &DiagnosticResult{
		LastSyncAt:   lastSyncAt,
		LocalModels:  int(localCount),
		RemoteModels: remoteCount,
		Diff:         diff,
		Channels:     diff.Channels,
	}

	return result, nil
}
