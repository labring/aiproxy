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
	"github.com/labring/aiproxy/core/common/env"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/enterprise/synccommon"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	"gorm.io/gorm"
)

// syncMu prevents concurrent sync executions.
var syncMu sync.Mutex

// ErrSyncInProgress is returned when a sync is already running.
var ErrSyncInProgress = errors.New("a sync operation is already in progress")

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

	synccommon.SendProgress(progressCallback, "fetching", "正在获取海外模型列表...", 10, nil)

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
		synccommon.SendProgress(progressCallback, "filtering",
			fmt.Sprintf("已过滤 %d 个不可用模型（status≠1）", unavailCount), 20, nil)
	}

	exchangeRate := cfg.ExchangeRate

	synccommon.SendProgress(progressCallback, "comparing", "对比本地和远程模型...", 30, nil)

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
		synccommon.SendProgress(progressCallback, "complete", "预览完成", 100, result)

		return result, nil
	}

	synccommon.SendProgress(progressCallback, "syncing", "开始同步模型配置...", 50, nil)

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		return executeSyncTransaction(tx, diff, opts, modelMap, result, progressCallback, exchangeRate)
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	// Step 3.5: Sync multimodal models from the dedicated console API.
	if cfg.MgmtToken != "" {
		synccommon.SendProgress(progressCallback, "multimodal", "同步多模态模型（图像/视频/音频）...", 75, nil)

		mmAdded, mmUpdated, mmErr := syncMultimodalModels(ctx, client, cfg.MgmtToken, exchangeRate)
		if mmErr != nil {
			log.Printf("Novita sync: multimodal sync failed (non-fatal): %v", mmErr)
			result.Errors = append(result.Errors, fmt.Sprintf("multimodal sync: %v", mmErr))
		} else {
			if mmAdded > 0 || mmUpdated > 0 {
				log.Printf("Novita sync: multimodal models added=%d updated=%d", mmAdded, mmUpdated)
			}
		}
	}

	// Pass remote model IDs so cross-owner shared models are included in channels.
	synccommon.SendProgress(progressCallback, "channels", "检查并更新 Channel 模型列表...", 85, nil)

	remoteIDs := make([]string, 0, len(allModels))
	for i := range allModels {
		if allModels[i].IsAvailable() {
			remoteIDs = append(remoteIDs, allModels[i].ID)
		}
	}

	channelsInfo, err := EnsureNovitaChannels(opts.AutoCreateChannels, &opts.AnthropicPurePassthrough, opts.AllowPassthroughUnknown, cfg, remoteIDs)
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

	synccommon.SendProgress(progressCallback, "recording", "记录同步历史...", 95, nil)

	if err := RecordSyncHistory(opts, result); err != nil {
		log.Printf("failed to record novita sync history: %v", err)
	}

	synccommon.SendProgress(progressCallback, "complete", "同步完成", 100, result)

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
		synccommon.SendProgress(
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
		synccommon.SendProgress(
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
			synccommon.SendProgress(
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
	configData := synccommon.ToModelConfigKeys(buildConfigFromV2Model(m))

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
		if synccommon.ShouldSkipOwnership(existing.Owner, model.ModelOwnerNovita) {
			return nil
		}

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

	if synccommon.ShouldSkipOwnership(existing.Owner, model.ModelOwnerNovita) {
		return nil
	}

	existing.Owner = model.ModelOwnerNovita
	existing.Type = modeFromEndpoints(m.ModelType, m.Endpoints)
	existing.Config = synccommon.ToModelConfigKeys(buildConfigFromV2Model(m))

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

	if !m.IsTieredBilling || len(m.TieredBillingConfigs) == 0 {
		price.ConditionalPrices = nil
		return
	}

	conditionalPrices := make([]model.ConditionalPrice, 0, len(m.TieredBillingConfigs))

	var prevMax int64

	for _, tier := range m.TieredBillingConfigs {
		minTokens, maxTokens := synccommon.AdjustTierBounds(tier.MinTokens, tier.MaxTokens, prevMax)
		prevMax = tier.MaxTokens

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
	if synccommon.InferToolChoice(m.ModelType, m.Features) {
		cfg[string(model.ModelConfigToolChoiceKey)] = true
	}
	if slices.Contains(m.InputModalities, "image") {
		cfg[string(model.ModelConfigVisionKey)] = true
	}

	return cfg
}

// EnsureNovitaChannels queries local ModelConfig entries and partitions them
// by endpoint compatibility, then writes the lists into the corresponding
// Novita channels. When autoCreate is true and no Novita channels exist, it
// creates them automatically using the API key from cfg.
//
// remoteModelIDs is optional: when provided (during sync), the query includes
// both Novita-owned models AND cross-owner models whose name appears in the
// remote list. This ensures shared models (e.g. owned by PPIO) are still
// included in Novita channels. When omitted (startup refresh), only
// Novita-owned models are used.
//
// anthropicPurePassthrough controls the pure_passthrough config on the Anthropic
// channel. Pass nil to preserve the existing setting (only initializing the key
// to false if absent). Pass a non-nil pointer to always write the given value.
//
// allowPassthroughUnknown controls the allow_passthrough_unknown config on the
// OpenAI channel. When true, requests for models not in the model list are
// forwarded to this channel as a fallback (billed at zero cost).
func EnsureNovitaChannels(autoCreate bool, anthropicPurePassthrough *bool, allowPassthroughUnknown *bool, cfg NovitaConfigResult, remoteModelIDs ...[]string) (ChannelsInfo, error) {
	var localModels []model.ModelConfig

	query := model.DB.Select("model", "type", "config")
	if len(remoteModelIDs) > 0 && len(remoteModelIDs[0]) > 0 {
		// Include both Novita-owned models and cross-owner models in the remote list
		query = query.Where("owner = ? OR model IN ?", string(model.ModelOwnerNovita), remoteModelIDs[0])
	} else {
		query = query.Where("owner = ?", string(model.ModelOwnerNovita))
	}

	if err := query.Find(&localModels).Error; err != nil {
		return ChannelsInfo{}, fmt.Errorf("failed to query local Novita models: %w", err)
	}

	var anthropicModels, openaiModels, multimodalModels []string

	for _, mc := range localModels {
		// Skip models whose stored status indicates they are not available.
		// Safety net for models synced before the status filter was added.
		if status, ok := model.GetModelConfigInt(mc.Config, "status"); ok && status != NovitaModelStatusAvailable {
			continue
		}

		// PPIONative models (image/video/audio) go to the multimodal channel.
		if mc.Type == mode.PPIONative {
			multimodalModels = append(multimodalModels, mc.Model)
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
	slices.Sort(multimodalModels)

	return ensureNovitaChannelsFromModels(anthropicModels, openaiModels, multimodalModels, autoCreate, anthropicPurePassthrough, allowPassthroughUnknown, cfg)
}

func ensureNovitaChannelsFromModels(
	anthropicModels, openaiModels, multimodalModels []string,
	autoCreate bool, anthropicPurePassthrough *bool, allowPassthroughUnknown *bool, cfg NovitaConfigResult,
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

		purePassthrough := anthropicPurePassthrough != nil && *anthropicPurePassthrough
		allowUnknown := allowPassthroughUnknown != nil && *allowPassthroughUnknown
		created, createErr := createNovitaChannels(cfg, purePassthrough, allowUnknown, anthropicModels, openaiModels, multimodalModels)
		if createErr != nil {
			return info, createErr
		}

		info.Novita.Exists = true
		info.Novita.ID = created[0].ID

		return info, nil
	}

	info.Novita.Exists = true
	info.Novita.ID = channels[0].ID

	// Track whether a multimodal channel exists so we can create one if needed.
	hasMultimodal := false

	for i := range channels {
		switch channels[i].Type {
		case model.ChannelTypeAnthropic:
			channels[i].Models = synccommon.MergeModels(channels[i].Models, anthropicModels)
			if channels[i].Configs == nil {
				channels[i].Configs = make(model.ChannelConfigs)
			}
			if _, ok := channels[i].Configs["skip_image_conversion"]; !ok {
				channels[i].Configs["skip_image_conversion"] = true
			}
			if _, ok := channels[i].Configs["disable_context_management"]; !ok {
				channels[i].Configs["disable_context_management"] = true
			}
			channels[i].Configs.SetOrInit("pure_passthrough", anthropicPurePassthrough, false)

		case model.ChannelTypeNovitaMultimodal:
			hasMultimodal = true
			channels[i].Models = synccommon.MergeModels(channels[i].Models, multimodalModels)
			if channels[i].Configs == nil {
				channels[i].Configs = make(model.ChannelConfigs)
			}
			// Multimodal channel always allows passthrough for auto-discovery.
			channels[i].Configs[model.ChannelConfigAllowPassthroughUnknown] = true

		default:
			// ChannelTypeNovita (OpenAI-compatible)
			channels[i].Models = synccommon.MergeModels(channels[i].Models, openaiModels)
			if channels[i].Configs == nil {
				channels[i].Configs = make(model.ChannelConfigs)
			}
			channels[i].Configs[model.ChannelConfigPathBaseMapKey] = map[string]string{
				"/v1/responses":  novitaResponsesBase(channels[i].BaseURL),
				"/v1/web-search": novitaWebSearchBase(channels[i].BaseURL),
			}
			channels[i].Configs.SetOrInit(model.ChannelConfigAllowPassthroughUnknown, allowPassthroughUnknown, false)
		}

		if err := model.DB.Save(&channels[i]).Error; err != nil {
			return info, fmt.Errorf("failed to update channel %d models: %w", channels[i].ID, err)
		}
	}

	// If no multimodal channel exists yet, create one now.
	if !hasMultimodal && autoCreate && cfg.APIKey != "" {
		mlCh := newNovitaMultimodalChannel(cfg.APIKey, multimodalModels)
		if err := model.DB.Create(&mlCh).Error; err != nil {
			log.Printf("Novita sync: failed to create multimodal channel: %v", err)
		}
	}

	return info, nil
}

// createNovitaChannels creates the OpenAI-compatible channel, Anthropic channel
// (if there are anthropic-endpoint models), and a multimodal channel.
// All channels share the same API key from the Novita config.
func createNovitaChannels(cfg NovitaConfigResult, anthropicPurePassthrough bool, allowPassthroughUnknown bool, anthropicModels, openaiModels, multimodalModels []string) ([]model.Channel, error) {
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
			Configs: model.ChannelConfigs{
				model.ChannelConfigPathBaseMapKey: map[string]string{
					"/v1/responses":  novitaResponsesBase(openaiBase),
					"/v1/web-search": novitaWebSearchBase(openaiBase),
				},
				model.ChannelConfigAllowPassthroughUnknown: allowPassthroughUnknown,
			},
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
				Configs: model.ChannelConfigs{
					"skip_image_conversion":      true,
					"disable_context_management": true,
					"pure_passthrough":            anthropicPurePassthrough,
				},
			}

			if err := tx.Create(&anthropicCh).Error; err != nil {
				return fmt.Errorf("failed to create Novita Anthropic channel: %w", err)
			}

			created = append(created, anthropicCh)
		}

		// Always create the multimodal channel so auto-discovery can work even
		// before any multimodal models are synced from the management API.
		multimodalCh := newNovitaMultimodalChannel(cfg.APIKey, multimodalModels)

		if err := tx.Create(&multimodalCh).Error; err != nil {
			return fmt.Errorf("failed to create Novita multimodal channel: %w", err)
		}

		created = append(created, multimodalCh)

		return nil
	})
	if err != nil {
		return nil, err
	}

	log.Printf("auto-created %d Novita channel(s)", len(created))

	return created, nil
}

// newNovitaMultimodalChannel builds the Channel struct for a Novita native
// multimodal channel, extracted to avoid duplicating the literal.
func newNovitaMultimodalChannel(apiKey string, models []string) model.Channel {
	return model.Channel{
		Name:    "Novita (Multimodal)",
		Type:    model.ChannelTypeNovitaMultimodal,
		BaseURL: DefaultNovitaMultimodalBase,
		Key:     apiKey,
		Models:  models,
		Status:  model.ChannelStatusEnabled,
		Configs: model.ChannelConfigs{
			model.ChannelConfigAllowPassthroughUnknown: true,
		},
	}
}

// syncMultimodalModels fetches multimodal models (image/video/audio) from
// the Novita console API and their SKU-based pricing from the batch-price API,
// then creates or updates ModelConfig entries with PerRequestPrice.
func syncMultimodalModels(ctx context.Context, client *NovitaClient, mgmtToken string, exchangeRate float64) (added, updated int, err error) {
	mmModels, err := client.FetchMultimodalModels(ctx, mgmtToken)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch multimodal models: %w", err)
	}

	if len(mmModels) == 0 {
		return 0, 0, nil
	}

	// Collect all SKU codes for batch price lookup
	var allSKUs []string
	for i := range mmModels {
		allSKUs = append(allSKUs, mmModels[i].collectSKUCodes()...)
	}

	// Fetch batch pricing
	skuPrices, priceErr := client.FetchMultimodalPrices(ctx, mgmtToken, allSKUs)
	if priceErr != nil {
		log.Printf("Novita sync: multimodal price fetch failed (non-fatal, using zero prices): %v", priceErr)

		skuPrices = make(map[string]int64)
	}

	// Create/update ModelConfig for each multimodal model
	for i := range mmModels {
		mm := &mmModels[i]
		modelName := mm.FusionConfig.Name

		if modelName == "" {
			continue
		}

		minPrice := mm.minSKUPrice(skuPrices, exchangeRate)
		modelType := multimodalCategoryToModelType(mm.ModelConfig.Config.Category)

		rawConfig := map[string]any{
			"model_type":   modelType,
			"category":     mm.ModelConfig.Config.Category,
			"display_name": mm.FusionConfig.DisplayName,
			"description":  mm.FusionConfig.Description,
		}

		if mm.FusionConfig.Series != "" {
			rawConfig["series"] = mm.FusionConfig.Series
		}

		configData := synccommon.ToModelConfigKeys(rawConfig)

		var existing model.ModelConfig
		if txErr := model.DB.Where("model = ?", modelName).First(&existing).Error; txErr == nil {
			// Only update if we own or can claim via priority
			if synccommon.ShouldSkipOwnership(existing.Owner, model.ModelOwnerNovita) {
				continue
			}

			existing.Owner = model.ModelOwnerNovita
			existing.Type = modeFromEndpoints(modelType, nil)
			existing.Config = configData

			if minPrice > 0 {
				existing.Price.PerRequestPrice = model.ZeroNullFloat64(minPrice)
			}

			if txErr := model.DB.Save(&existing).Error; txErr != nil {
				log.Printf("Novita sync: failed to update multimodal model %s: %v", modelName, txErr)

				continue
			}

			updated++
		} else {
			mc := model.ModelConfig{
				Model:  modelName,
				Owner:  model.ModelOwnerNovita,
				Type:   modeFromEndpoints(modelType, nil),
				RPM:    60,
				TPM:    1000000,
				Config: configData,
			}

			if minPrice > 0 {
				mc.Price.PerRequestPrice = model.ZeroNullFloat64(minPrice)
			}

			if txErr := model.DB.Create(&mc).Error; txErr != nil {
				log.Printf("Novita sync: failed to create multimodal model %s: %v", modelName, txErr)

				continue
			}

			added++
		}
	}

	return added, updated, nil
}

// novitaResponsesBase derives the Responses API base URL from an OpenAI channel's BaseURL.
// Mirrors the same logic in relay/adaptor/novita so both code paths stay in sync.
func novitaResponsesBase(channelBaseURL string) string {
	if r := strings.Replace(channelBaseURL, "/v3/openai", "/openai/v1", 1); r != channelBaseURL {
		return r
	}

	return "https://api.novita.ai/openai/v1"
}

// novitaWebSearchBase derives the web-search base URL by replacing /v3/openai with /v3.
func novitaWebSearchBase(channelBaseURL string) string {
	if r := strings.Replace(channelBaseURL, "/v3/openai", "/v3", 1); r != channelBaseURL {
		return r
	}

	return "https://api.novita.ai/v3"
}

// StartSyncScheduler starts a background goroutine that syncs Novita models daily at 02:15.
// Offset by 15 minutes from PPIO to avoid simultaneous heavy sync loads.
// A Feishu webhook notification is sent after each run summarising changes.
// Set DISABLE_NOVITA_AUTO_SYNC=true to disable.
func StartSyncScheduler(ctx context.Context) {
	if env.Bool("DISABLE_NOVITA_AUTO_SYNC", false) {
		log.Printf("Novita sync scheduler: disabled via DISABLE_NOVITA_AUTO_SYNC")
		return
	}

	go func() {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 2, 15, 0, 0, now.Location())

		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}

		delay := next.Sub(now)
		log.Printf("Novita sync scheduler: next run at %s (in %v)", next.Format("2006-01-02 15:04:05"), delay)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		runNovitaDailySync(ctx)

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runNovitaDailySync(ctx)
			}
		}
	}()
}

// runNovitaDailySync performs one Novita model sync and sends a Feishu notification with the outcome.
func runNovitaDailySync(ctx context.Context) {
	log.Printf("Novita auto sync: starting daily model sync")

	result, err := ExecuteSync(ctx, SyncOptions{AnthropicPurePassthrough: true}, nil)
	if err != nil {
		notify.ErrorThrottle(
			"novitaAutoSyncFailed",
			24*time.Hour,
			"Novita 每日模型同步失败",
			err.Error(),
		)
		log.Printf("Novita auto sync failed: %v", err)

		return
	}

	msg := fmt.Sprintf("新增: %d  更新: %d  删除: %d  耗时: %dms",
		len(result.Details.ModelsAdded),
		len(result.Details.ModelsUpdated),
		len(result.Details.ModelsDeleted),
		result.DurationMS,
	)

	if result.Success {
		notify.Info("Novita 每日模型同步完成", msg)
	} else {
		errSummary := strings.Join(result.Errors, "; ")
		notify.Warn("Novita 每日模型同步部分失败", msg+"\n错误: "+errSummary)
	}

	log.Printf("Novita auto sync completed: %s", msg)
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


