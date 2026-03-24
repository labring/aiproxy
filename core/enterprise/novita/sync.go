//go:build enterprise

package novita

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"gorm.io/gorm"
)

// toModelConfigKeys converts map[string]any to map[ModelConfigKey]any without JSON round-trip.
func toModelConfigKeys(m map[string]any) map[model.ModelConfigKey]any {
	out := make(map[model.ModelConfigKey]any, len(m))
	for k, v := range m {
		out[model.ModelConfigKey(k)] = v
	}

	return out
}

// ExecuteSync performs the actual sync operation with transaction.
// Always uses FetchAllModelsMerged (V1+V2 merged into V2 format).
func ExecuteSync(
	opts SyncOptions,
	progressCallback func(event SyncProgressEvent),
) (*SyncResult, error) {
	startTime := time.Now()
	result := &SyncResult{
		Success: false,
		Summary: SyncSummary{},
	}

	sendProgress(progressCallback, "fetching", "正在获取 Novita 模型列表...", 10, nil)

	client, err := NewNovitaClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Novita client: %w", err)
	}

	cfg := GetNovitaConfig()

	allModels, fetchErr := client.FetchAllModelsMerged(cfg.MgmtToken)
	if fetchErr != nil {
		return nil, fmt.Errorf("failed to fetch Novita models: %w", fetchErr)
	}

	unavailCount := 0
	for _, m := range allModels {
		if !m.IsAvailable() {
			unavailCount++
		}
	}

	if unavailCount > 0 {
		sendProgress(progressCallback, "filtering",
			fmt.Sprintf("已过滤 %d 个不可用模型（status≠1）", unavailCount), 20, nil)
	}

	sendProgress(progressCallback, "comparing", "对比本地和远程模型...", 30, nil)

	diff, err := CompareNovitaModelsV2(allModels, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to compare models: %w", err)
	}

	modelMap := make(map[string]*NovitaModelV2, len(allModels))
	for i := range allModels {
		modelMap[allModels[i].ID] = &allModels[i]
	}

	result.Summary = diff.Summary

	if opts.DryRun {
		result.Success = true
		result.DurationMS = time.Since(startTime).Milliseconds()
		sendProgress(progressCallback, "complete", "预览完成", 100, result)

		return result, nil
	}

	sendProgress(progressCallback, "syncing", "开始同步模型配置...", 50, nil)

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		return executeSyncTransaction(tx, diff, opts, modelMap, result, progressCallback)
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	sendProgress(progressCallback, "channels", "检查并更新 Channel 模型列表...", 85, nil)

	channelsInfo, err := EnsureNovitaChannels()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("channel update: %v", err))
	}

	result.Channels = channelsInfo
	result.Success = len(result.Errors) == 0
	result.DurationMS = time.Since(startTime).Milliseconds()

	if err := model.InitModelConfigAndChannelCache(); err != nil {
		log.Printf("failed to refresh model cache after novita sync: %v", err)
	}

	sendProgress(progressCallback, "recording", "记录同步历史...", 95, nil)

	if err := RecordSyncHistory(opts, result); err != nil {
		log.Printf("failed to record novita sync history: %v", err)
	}

	sendProgress(progressCallback, "complete", "同步完成", 100, result)

	return result, nil
}

// executeSyncTransaction runs add/update/delete inside a DB transaction.
func executeSyncTransaction(
	tx *gorm.DB,
	diff *SyncDiff,
	opts SyncOptions,
	modelMap map[string]*NovitaModelV2,
	result *SyncResult,
	progressCallback func(event SyncProgressEvent),
) error {
	totalAdd := max(len(diff.Changes.Add), 1)

	for i, modelDiff := range diff.Changes.Add {
		progress := 50 + (i * 15 / totalAdd)
		sendProgress(
			progressCallback, "adding",
			fmt.Sprintf("添加模型 %s (%d/%d)", modelDiff.ModelID, i+1, len(diff.Changes.Add)),
			progress, nil,
		)

		m := modelMap[modelDiff.ModelID]
		if m == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("model %s not found in remote models", modelDiff.ModelID))
			continue
		}

		if err := createModelConfigV2(tx, m); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to add %s: %v", modelDiff.ModelID, err))
			continue
		}

		result.Details.ModelsAdded = append(result.Details.ModelsAdded, modelDiff.ModelID)
	}

	totalUpdate := max(len(diff.Changes.Update), 1)

	for i, modelDiff := range diff.Changes.Update {
		progress := 65 + (i * 15 / totalUpdate)
		sendProgress(
			progressCallback, "updating",
			fmt.Sprintf("更新模型 %s (%d/%d)", modelDiff.ModelID, i+1, len(diff.Changes.Update)),
			progress, nil,
		)

		m := modelMap[modelDiff.ModelID]
		if m == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("model %s not found in remote models", modelDiff.ModelID))
			continue
		}

		if err := updateModelConfigV2(tx, m); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to update %s: %v", modelDiff.ModelID, err))
			continue
		}

		result.Details.ModelsUpdated = append(result.Details.ModelsUpdated, modelDiff.ModelID)
	}

	if opts.DeleteUnmatchedModel {
		totalDelete := max(len(diff.Changes.Delete), 1)

		for i, modelDiff := range diff.Changes.Delete {
			progress := 80 + (i * 5 / totalDelete)
			sendProgress(
				progressCallback, "deleting",
				fmt.Sprintf("删除模型 %s (%d/%d)", modelDiff.ModelID, i+1, len(diff.Changes.Delete)),
				progress, nil,
			)

			if err := tx.Where("model = ? AND owner = ?", modelDiff.ModelID, string(model.ModelOwnerNovita)).
				Delete(&model.ModelConfig{}).Error; err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to delete %s: %v", modelDiff.ModelID, err))
				continue
			}

			result.Details.ModelsDeleted = append(result.Details.ModelsDeleted, modelDiff.ModelID)
		}
	}

	return nil
}

// createModelConfigV2 creates a ModelConfig from a V2 Novita model.
func createModelConfigV2(tx *gorm.DB, m *NovitaModelV2) error {
	configData := toModelConfigKeys(buildConfigFromV2Model(m))

	rpm := int64(60)
	if m.RPM > 0 {
		rpm = int64(m.RPM)
	}

	tpm := int64(1000000)
	if m.TPM > 0 {
		tpm = int64(m.TPM)
	}

	var existing model.ModelConfig
	if err := tx.Where("model = ?", m.ID).First(&existing).Error; err == nil {
		existing.Owner = model.ModelOwnerNovita
		existing.Config = configData
		existing.Type = modeFromEndpoints(m.ModelType, m.Endpoints)
		existing.RPM = rpm
		existing.TPM = tpm
		setPriceFromV2Model(&existing.Price, m)

		return tx.Save(&existing).Error
	}

	mc := model.ModelConfig{
		Model:  m.ID,
		Owner:  model.ModelOwnerNovita,
		Type:   modeFromEndpoints(m.ModelType, m.Endpoints),
		RPM:    rpm,
		TPM:    tpm,
		Config: configData,
	}

	setPriceFromV2Model(&mc.Price, m)

	return tx.Create(&mc).Error
}

// updateModelConfigV2 updates an existing ModelConfig from a V2 Novita model.
func updateModelConfigV2(tx *gorm.DB, m *NovitaModelV2) error {
	var existing model.ModelConfig
	if err := tx.Where("model = ?", m.ID).
		First(&existing).Error; err != nil {
		return err
	}

	existing.Owner = model.ModelOwnerNovita
	existing.Type = modeFromEndpoints(m.ModelType, m.Endpoints)
	existing.Config = toModelConfigKeys(buildConfigFromV2Model(m))

	if m.RPM > 0 {
		existing.RPM = int64(m.RPM)
	}

	if m.TPM > 0 {
		existing.TPM = int64(m.TPM)
	}

	setPriceFromV2Model(&existing.Price, m)

	return tx.Save(&existing).Error
}

// setPriceFromV2Model populates Price fields from a V2 model, including cache pricing.
func setPriceFromV2Model(price *model.Price, m *NovitaModelV2) {
	price.InputPrice = model.ZeroNullFloat64(m.GetInputPricePerToken())
	price.InputPriceUnit = model.ZeroNullInt64(1)
	price.OutputPrice = model.ZeroNullFloat64(m.GetOutputPricePerToken())
	price.OutputPriceUnit = model.ZeroNullInt64(1)

	if m.SupportPromptCache && m.CacheReadInputTokenPricePerM > 0 {
		price.CachedPrice = model.ZeroNullFloat64(m.GetCacheReadPricePerToken())
		price.CachedPriceUnit = model.ZeroNullInt64(1)
	}

	if m.SupportPromptCache && m.CacheCreationInputTokenPricePerM > 0 {
		price.CacheCreationPrice = model.ZeroNullFloat64(m.GetCacheCreationPricePerToken())
		price.CacheCreationPriceUnit = model.ZeroNullInt64(1)
	}

	if m.IsTieredBilling && len(m.TieredBillingConfigs) > 0 {
		conditionalPrices := make([]model.ConditionalPrice, 0, len(m.TieredBillingConfigs))

		for i, tier := range m.TieredBillingConfigs {
			minTokens, maxTokens := adjustTierBounds(m.TieredBillingConfigs, i)
			if maxTokens > 0 && minTokens > maxTokens {
				continue
			}

			cp := model.ConditionalPrice{
				Condition: model.PriceCondition{
					InputTokenMin: minTokens,
					InputTokenMax: maxTokens,
				},
				Price: model.Price{
					InputPrice:      model.ZeroNullFloat64(tier.InputPricing.PricePerToken()),
					InputPriceUnit:  model.ZeroNullInt64(1),
					OutputPrice:     model.ZeroNullFloat64(tier.OutputPricing.PricePerToken()),
					OutputPriceUnit: model.ZeroNullInt64(1),
				},
			}

			if tier.CacheReadInputPricing.PricePerM > 0 {
				cp.Price.CachedPrice = model.ZeroNullFloat64(tier.CacheReadInputPricing.PricePerToken())
				cp.Price.CachedPriceUnit = model.ZeroNullInt64(1)
			}

			if tier.CacheCreationInputPricing.PricePerM > 0 {
				cp.Price.CacheCreationPrice = model.ZeroNullFloat64(tier.CacheCreationInputPricing.PricePerToken())
				cp.Price.CacheCreationPriceUnit = model.ZeroNullInt64(1)
			}

			conditionalPrices = append(conditionalPrices, cp)
		}

		price.ConditionalPrices = conditionalPrices
	}
}

// buildConfigFromV2Model builds the model config map stored in ModelConfig.Config from a V2 model.
func buildConfigFromV2Model(m *NovitaModelV2) map[string]any {
	cfg := map[string]any{
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

	if m.DisplayName != "" {
		cfg["display_name"] = m.DisplayName
	}

	if m.Series != "" {
		cfg["series"] = m.Series
	}

	if m.IsTieredBilling {
		cfg["is_tiered_billing"] = true
	}

	if m.SupportPromptCache {
		cfg["support_prompt_cache"] = true
	}

	return cfg
}

// adjustTierBounds returns the effective [min, max] for tier i, bumping min by 1
// when it overlaps with the previous tier's max.
func adjustTierBounds(tiers []TieredBillingConfig, i int) (minTokens, maxTokens int64) {
	minTokens = tiers[i].MinTokens
	maxTokens = tiers[i].MaxTokens

	if i > 0 && minTokens > 0 {
		prevMax := tiers[i-1].MaxTokens
		if prevMax > 0 && minTokens <= prevMax {
			minTokens = prevMax + 1
		}
	}

	return minTokens, maxTokens
}

// EnsureNovitaChannels queries all local ModelConfig entries owned by Novita,
// partitions them by endpoint compatibility, and writes the lists into the
// corresponding Novita channels.
func EnsureNovitaChannels() (ChannelsInfo, error) {
	var localModels []model.ModelConfig

	if err := model.DB.Select("model", "config").
		Where("owner = ?", string(model.ModelOwnerNovita)).
		Find(&localModels).Error; err != nil {
		return ChannelsInfo{}, fmt.Errorf("failed to query local Novita models: %w", err)
	}

	var anthropicModels, openaiModels []string

	for _, mc := range localModels {
		openaiModels = append(openaiModels, mc.Model)

		if eps, ok := model.GetModelConfigStringSlice(mc.Config, "endpoints"); ok {
			if slices.Contains(eps, "anthropic") {
				anthropicModels = append(anthropicModels, mc.Model)
			}
		}
	}

	slices.Sort(anthropicModels)
	slices.Sort(openaiModels)

	return ensureNovitaChannelsFromModels(anthropicModels, openaiModels)
}

func ensureNovitaChannelsFromModels(anthropicModels, openaiModels []string) (ChannelsInfo, error) {
	info := ChannelsInfo{}

	var channels []model.Channel

	err := model.DB.Where(novitaChannelWhere(), novitaChannelArgs()...).Find(&channels).Error
	if err != nil || len(channels) == 0 {
		return info, nil
	}

	info.Novita.Exists = true
	info.Novita.ID = channels[0].ID

	for i := range channels {
		if strings.Contains(strings.ToLower(channels[i].BaseURL), "anthropic") {
			channels[i].Models = anthropicModels
		} else {
			channels[i].Models = openaiModels
		}

		if err := model.DB.Save(&channels[i]).Error; err != nil {
			return info, fmt.Errorf("failed to update channel %d models: %w", channels[i].ID, err)
		}
	}

	return info, nil
}

// RecordSyncHistory records sync history to database.
func RecordSyncHistory(opts SyncOptions, result *SyncResult) error {
	optsJSON, _ := sonic.Marshal(opts)
	resultJSON, _ := sonic.Marshal(result)

	status := "success"
	if !result.Success {
		if len(result.Errors) == result.Summary.TotalModels {
			status = "failed"
		} else {
			status = "partial"
		}
	}

	history := SyncHistory{
		Operator:    "admin",
		SyncOptions: string(optsJSON),
		Result:      string(resultJSON),
		Status:      status,
	}

	return model.DB.Create(&history).Error
}

// sendProgress sends a progress event to the callback if not nil.
func sendProgress(
	callback func(event SyncProgressEvent),
	step, message string,
	progress int,
	data any,
) {
	if callback != nil {
		eventType := "progress"
		if step == "complete" {
			eventType = "success"
		}

		callback(SyncProgressEvent{
			Type:     eventType,
			Step:     step,
			Message:  message,
			Progress: progress,
			Data:     data,
		})
	}
}
