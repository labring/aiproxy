//go:build enterprise

package novita

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"gorm.io/gorm"
)

// syncMu prevents concurrent sync executions.
var syncMu sync.Mutex

// ErrSyncInProgress is returned when a sync is already running.
var ErrSyncInProgress = errors.New("a sync operation is already in progress")

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
	ctx context.Context,
	opts SyncOptions,
	progressCallback func(event SyncProgressEvent),
) (*SyncResult, error) {
	if !syncMu.TryLock() {
		return nil, ErrSyncInProgress
	}
	defer syncMu.Unlock()

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

	allModels, fetchErr := client.FetchAllModelsMerged(ctx, cfg.MgmtToken)
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

	exchangeRate := cfg.ExchangeRate

	sendProgress(progressCallback, "comparing", "对比本地和远程模型...", 30, nil)

	diff, err := CompareNovitaModelsV2(allModels, opts, exchangeRate)
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
		return executeSyncTransaction(tx, diff, opts, modelMap, result, progressCallback, exchangeRate)
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	sendProgress(progressCallback, "channels", "检查并更新 Channel 模型列表...", 85, nil)

	channelsInfo, err := EnsureNovitaChannels(opts.AutoCreateChannels, cfg)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("channel update: %v", err))
	}

	// If channels were auto-created, write the channel ID back to options
	// so the sync page can find it on next load.
	if channelsInfo.Novita.Exists && cfg.ChannelID == 0 && channelsInfo.Novita.ID > 0 {
		if err := SetNovitaConfigFromChannel(channelsInfo.Novita.ID); err != nil {
			log.Printf("failed to write back Novita channel config: %v", err)
		}
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
	exchangeRate float64,
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

		if err := createModelConfigV2(tx, m, exchangeRate); err != nil {
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

		if err := updateModelConfigV2(tx, m, exchangeRate); err != nil {
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
// exchangeRate converts USD prices to CNY before storing.
func createModelConfigV2(tx *gorm.DB, m *NovitaModelV2, exchangeRate float64) error {
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
		setPriceFromV2Model(&existing.Price, m, exchangeRate)

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

	setPriceFromV2Model(&mc.Price, m, exchangeRate)

	return tx.Create(&mc).Error
}

// updateModelConfigV2 updates an existing ModelConfig from a V2 Novita model.
func updateModelConfigV2(tx *gorm.DB, m *NovitaModelV2, exchangeRate float64) error {
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

	setPriceFromV2Model(&existing.Price, m, exchangeRate)

	return tx.Save(&existing).Error
}

// setPriceFromV2Model populates Price fields from a V2 model, including cache pricing.
// exchangeRate converts USD per-token prices to CNY before storing.
func setPriceFromV2Model(price *model.Price, m *NovitaModelV2, exchangeRate float64) {
	price.InputPrice = model.ZeroNullFloat64(m.GetInputPricePerToken() * exchangeRate)
	price.InputPriceUnit = model.ZeroNullInt64(1)
	price.OutputPrice = model.ZeroNullFloat64(m.GetOutputPricePerToken() * exchangeRate)
	price.OutputPriceUnit = model.ZeroNullInt64(1)

	if m.SupportPromptCache && m.CacheReadInputTokenPricePerM > 0 {
		price.CachedPrice = model.ZeroNullFloat64(m.GetCacheReadPricePerToken() * exchangeRate)
		price.CachedPriceUnit = model.ZeroNullInt64(1)
	}

	if m.SupportPromptCache && m.CacheCreationInputTokenPricePerM > 0 {
		price.CacheCreationPrice = model.ZeroNullFloat64(m.GetCacheCreationPricePerToken() * exchangeRate)
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
					InputPrice:      model.ZeroNullFloat64(tier.InputPricing.PricePerToken() * exchangeRate),
					InputPriceUnit:  model.ZeroNullInt64(1),
					OutputPrice:     model.ZeroNullFloat64(tier.OutputPricing.PricePerToken() * exchangeRate),
					OutputPriceUnit: model.ZeroNullInt64(1),
				},
			}

			if tier.CacheReadInputPricing.PricePerM > 0 {
				cp.Price.CachedPrice = model.ZeroNullFloat64(tier.CacheReadInputPricing.PricePerToken() * exchangeRate)
				cp.Price.CachedPriceUnit = model.ZeroNullInt64(1)
			}

			if tier.CacheCreationInputPricing.PricePerM > 0 {
				cp.Price.CacheCreationPrice = model.ZeroNullFloat64(tier.CacheCreationInputPricing.PricePerToken() * exchangeRate)
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

	// Derive capability flags from model metadata so the admin UI
	// can display "tool" / "vision" badges on the model table.
	if inferToolChoice(m.ModelType, m.Features) {
		cfg[string(model.ModelConfigToolChoiceKey)] = true
	}
	if slices.Contains(m.InputModalities, "image") {
		cfg[string(model.ModelConfigVisionKey)] = true
	}

	return cfg
}

// inferToolChoice returns true when the model is likely to support tool_choice.
// Signal priority: features list ("tool_use" / "function_calling") > model_type "chat".
func inferToolChoice(modelType string, features []string) bool {
	for _, f := range features {
		switch f {
		case "tool_use", "function_calling", "tools":
			return true
		}
	}
	return modelType == "chat"
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
// corresponding Novita channels. When autoCreate is true and no Novita channels
// exist, it creates them automatically using the API key from cfg.
func EnsureNovitaChannels(autoCreate bool, cfg NovitaConfigResult) (ChannelsInfo, error) {
	var localModels []model.ModelConfig

	if err := model.DB.Select("model", "config").
		Where("owner = ?", string(model.ModelOwnerNovita)).
		Find(&localModels).Error; err != nil {
		return ChannelsInfo{}, fmt.Errorf("failed to query local Novita models: %w", err)
	}

	var anthropicModels, openaiModels []string

	for _, mc := range localModels {
		// Skip models whose stored status indicates they are not available.
		// Safety net for models synced before the status filter was added.
		if status, ok := model.GetModelConfigInt(mc.Config, "status"); ok && status != NovitaModelStatusAvailable {
			continue
		}

		openaiModels = append(openaiModels, mc.Model)

		if eps, ok := model.GetModelConfigStringSlice(mc.Config, "endpoints"); ok {
			if slices.Contains(eps, "anthropic") {
				anthropicModels = append(anthropicModels, mc.Model)
			}
		}
	}

	slices.Sort(anthropicModels)
	slices.Sort(openaiModels)

	return ensureNovitaChannelsFromModels(anthropicModels, openaiModels, autoCreate, cfg)
}

func ensureNovitaChannelsFromModels(
	anthropicModels, openaiModels []string,
	autoCreate bool, cfg NovitaConfigResult,
) (ChannelsInfo, error) {
	info := ChannelsInfo{}

	var channels []model.Channel

	err := model.DB.Where(novitaChannelWhere(), novitaChannelArgs()...).Find(&channels).Error
	if err != nil {
		return info, fmt.Errorf("failed to query Novita channels: %w", err)
	}

	// Auto-create channels when none exist and the option is enabled.
	if len(channels) == 0 {
		if !autoCreate || cfg.APIKey == "" {
			return info, nil
		}

		created, createErr := createNovitaChannels(cfg, anthropicModels, openaiModels)
		if createErr != nil {
			return info, createErr
		}

		info.Novita.Exists = true
		info.Novita.ID = created[0].ID

		return info, nil
	}

	info.Novita.Exists = true
	info.Novita.ID = channels[0].ID

	for i := range channels {
		if strings.Contains(strings.ToLower(channels[i].BaseURL), "anthropic") {
			channels[i].Models = anthropicModels
			// Ensure recommended defaults for Novita's Anthropic endpoint:
			// skip_image_conversion — Novita natively supports URL image sources
			// disable_context_management — Novita rejects the beta field with 400
			if channels[i].Configs == nil {
				channels[i].Configs = make(model.ChannelConfigs)
			}
			if _, ok := channels[i].Configs["skip_image_conversion"]; !ok {
				channels[i].Configs["skip_image_conversion"] = true
			}
			if _, ok := channels[i].Configs["disable_context_management"]; !ok {
				channels[i].Configs["disable_context_management"] = true
			}
		} else {
			channels[i].Models = openaiModels
		}

		if err := model.DB.Save(&channels[i]).Error; err != nil {
			return info, fmt.Errorf("failed to update channel %d models: %w", channels[i].ID, err)
		}
	}

	return info, nil
}

// createNovitaChannels creates the OpenAI-compatible channel and, if there are
// anthropic-endpoint models, an Anthropic-compatible channel as well.
// Both channels share the same API key from the Novita config.
func createNovitaChannels(cfg NovitaConfigResult, anthropicModels, openaiModels []string) ([]model.Channel, error) {
	openaiBase := cfg.APIBase
	if openaiBase == "" {
		openaiBase = DefaultNovitaAPIBase
	}

	var created []model.Channel

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		openaiCh := model.Channel{
			Name:    "Novita (OpenAI)",
			Type:    model.ChannelTypeNovita,
			BaseURL: openaiBase,
			Key:     cfg.APIKey,
			Models:  openaiModels,
			Status:  model.ChannelStatusEnabled,
		}

		if err := tx.Create(&openaiCh).Error; err != nil {
			return fmt.Errorf("failed to create Novita OpenAI channel: %w", err)
		}

		created = append(created, openaiCh)

		if len(anthropicModels) > 0 {
			anthropicCh := model.Channel{
				Name:    "Novita (Anthropic)",
				Type:    model.ChannelTypeAnthropic,
				BaseURL: DefaultNovitaAnthropicBase,
				Key:     cfg.APIKey,
				Models:  anthropicModels,
				Status:  model.ChannelStatusEnabled,
				// See ensureNovitaChannelsFromModels for rationale on each key.
				Configs: model.ChannelConfigs{
					"skip_image_conversion":      true,
					"disable_context_management": true,
				},
			}

			if err := tx.Create(&anthropicCh).Error; err != nil {
				return fmt.Errorf("failed to create Novita Anthropic channel: %w", err)
			}

			created = append(created, anthropicCh)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	log.Printf("auto-created %d Novita channel(s)", len(created))

	return created, nil
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
