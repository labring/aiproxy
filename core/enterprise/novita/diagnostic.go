//go:build enterprise

package novita

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

var modelTypeToMode = map[string]mode.Mode{
	"chat":       mode.ChatCompletions,
	"embedding":  mode.Embeddings,
	"rerank":     mode.Rerank,
	"moderation": mode.Moderations,
	"tts":        mode.AudioSpeech,
	"stt":        mode.AudioTranscription,
	"image":      mode.ImagesGenerations,
}

var endpointToMode = map[string]mode.Mode{
	"chat/completions":     mode.ChatCompletions,
	"completions":          mode.ChatCompletions,
	"responses":            mode.ChatCompletions,
	"anthropic":            mode.ChatCompletions,
	"embeddings":           mode.Embeddings,
	"rerank":               mode.Rerank,
	"moderations":          mode.Moderations,
	"audio/speech":         mode.AudioSpeech,
	"audio/transcriptions": mode.AudioTranscription,
	"images/generations":   mode.ImagesGenerations,
}

// modeFromEndpoints infers mode.Mode from Novita endpoint slugs and model_type.
// Falls back to ChatCompletions when no match is found.
func modeFromEndpoints(modelType string, endpoints []string) mode.Mode {
	if m, ok := modelTypeToMode[modelType]; ok {
		return m
	}

	for _, ep := range endpoints {
		if m, ok := endpointToMode[ep]; ok {
			return m
		}
	}

	return mode.ChatCompletions
}

// CompareNovitaModelsV2 compares remote V2 models with local database models.
func CompareNovitaModelsV2(remoteModels []NovitaModelV2, opts SyncOptions, exchangeRate float64) (*SyncDiff, error) {
	var localModels []model.ModelConfig

	err := model.DB.Where("owner = ?", string(model.ModelOwnerNovita)).Find(&localModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query local models: %w", err)
	}

	localModelMap := make(map[string]*model.ModelConfig)
	for i := range localModels {
		localModelMap[localModels[i].Model] = &localModels[i]
	}

	available := make([]NovitaModelV2, 0, len(remoteModels))
	for _, m := range remoteModels {
		if m.IsAvailable() {
			available = append(available, m)
		}
	}

	remoteModelMap := make(map[string]*NovitaModelV2, len(available))
	for i := range available {
		remoteModelMap[available[i].ID] = &available[i]
	}

	diff := &SyncDiff{
		Summary: SyncSummary{
			TotalModels: len(available),
		},
	}

	for _, remoteModel := range available {
		localModel, exists := localModelMap[remoteModel.ID]
		if !exists {
			diff.Changes.Add = append(diff.Changes.Add, ModelDiff{
				ModelID:   remoteModel.ID,
				Action:    "add",
				NewConfig: buildModelConfigMapV2(&remoteModel, exchangeRate),
			})
			diff.Summary.ToAdd++
		} else {
			changes := compareModelConfigsV2(localModel, &remoteModel, exchangeRate)
			if len(changes) > 0 {
				diff.Changes.Update = append(diff.Changes.Update, ModelDiff{
					ModelID:   remoteModel.ID,
					Action:    "update",
					OldConfig: buildLocalModelConfigMap(localModel),
					NewConfig: buildModelConfigMapV2(&remoteModel, exchangeRate),
					Changes:   changes,
				})
				diff.Summary.ToUpdate++
			}
		}
	}

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

	diff.Channels = checkChannelStatus(opts)

	return diff, nil
}

// compareModelConfigsV2 compares a local model config with a remote V2 model.
// Prices are compared after applying the exchange rate (USD→CNY).
func compareModelConfigsV2(local *model.ModelConfig, remote *NovitaModelV2, exchangeRate float64) []string {
	var changes []string

	newInputPrice := remote.GetInputPricePerToken() * exchangeRate
	if !floatEquals(float64(local.Price.InputPrice), newInputPrice) {
		changes = append(changes, fmt.Sprintf(
			"input_price: %.8f → %.8f",
			float64(local.Price.InputPrice),
			newInputPrice,
		))
	}

	newOutputPrice := remote.GetOutputPricePerToken() * exchangeRate
	if !floatEquals(float64(local.Price.OutputPrice), newOutputPrice) {
		changes = append(changes, fmt.Sprintf(
			"output_price: %.8f → %.8f",
			float64(local.Price.OutputPrice),
			newOutputPrice,
		))
	}

	if !configMapsEqual(local.Config, buildConfigFromV2Model(remote)) {
		changes = append(changes, "config updated")
	}

	return changes
}

// configMapsEqual compares two config maps by normalizing through JSON.
func configMapsEqual(localConfig map[model.ModelConfigKey]any, remoteConfig map[string]any) bool {
	localJSON, _ := sonic.Marshal(localConfig)

	var normalizedLocal map[string]any

	_ = sonic.Unmarshal(localJSON, &normalizedLocal)

	normalizedLocalJSON, _ := sonic.ConfigStd.Marshal(normalizedLocal)

	remoteJSON, _ := sonic.Marshal(remoteConfig)

	var normalizedRemote map[string]any

	_ = sonic.Unmarshal(remoteJSON, &normalizedRemote)

	normalizedRemoteJSON, _ := sonic.ConfigStd.Marshal(normalizedRemote)

	return string(normalizedLocalJSON) == string(normalizedRemoteJSON)
}

// floatEquals compares two float64 values with tolerance.
func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < 1e-10
}

// buildModelConfigMapV2 builds a diff-display map for a V2 model.
// Prices shown are after exchange rate conversion (USD→CNY).
func buildModelConfigMapV2(m *NovitaModelV2, exchangeRate float64) map[string]any {
	return map[string]any{
		"model":        m.ID,
		"title":        m.Title,
		"description":  m.Description,
		"input_price":  m.GetInputPricePerToken() * exchangeRate,
		"output_price": m.GetOutputPricePerToken() * exchangeRate,
		"context_size": m.ContextSize,
		"endpoints":    m.Endpoints,
		"model_type":   m.ModelType,
		"status":       m.Status,
	}
}

// buildLocalModelConfigMap builds a map representation of a local model config.
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

// checkChannelStatus checks if a Novita channel exists.
func checkChannelStatus(opts SyncOptions) ChannelsInfo {
	info := ChannelsInfo{}

	var novitaChannel model.Channel

	err := model.DB.Where(novitaChannelWhere(), novitaChannelArgs()...).First(&novitaChannel).Error
	if err == nil {
		info.Novita.Exists = true
		info.Novita.ID = novitaChannel.ID
	} else {
		info.Novita.WillCreate = opts.AutoCreateChannels
	}

	return info
}

// Diagnostic performs a diagnostic check without executing sync.
// Always uses FetchAllModelsMerged (V1+V2 merged into V2 format).
func Diagnostic(ctx context.Context) (*DiagnosticResult, error) {
	client, err := NewNovitaClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Novita client: %w", err)
	}

	cfg := GetNovitaConfig()

	allModels, fetchErr := client.FetchAllModelsMerged(ctx, cfg.MgmtToken)
	if fetchErr != nil {
		return nil, fmt.Errorf("failed to fetch remote models: %w", fetchErr)
	}

	diff, err := CompareNovitaModelsV2(allModels, SyncOptions{}, cfg.ExchangeRate)
	if err != nil {
		return nil, fmt.Errorf("failed to compare models: %w", err)
	}

	remoteCount := diff.Summary.TotalModels

	var localCount int64

	err = model.DB.Model(&model.ModelConfig{}).
		Where("owner = ?", string(model.ModelOwnerNovita)).
		Count(&localCount).
		Error
	if err != nil {
		return nil, fmt.Errorf("failed to count local models: %w", err)
	}

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

	return &DiagnosticResult{
		LastSyncAt:   lastSyncAt,
		LocalModels:  int(localCount),
		RemoteModels: remoteCount,
		Diff:         diff,
		Channels:     diff.Channels,
	}, nil
}
