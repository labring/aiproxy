//go:build enterprise

package ppio

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
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	"gorm.io/gorm"
)

// syncMu prevents concurrent sync executions.
var syncMu sync.Mutex

// ErrSyncInProgress is returned when a sync is already running.
var ErrSyncInProgress = errors.New("a sync operation is already in progress")

// ModelTypeToMode maps PPIO model_type strings to mode.Mode.
var ModelTypeToMode = map[string]mode.Mode{
	"chat":       mode.ChatCompletions,
	"embedding":  mode.Embeddings,
	"rerank":     mode.Rerank,
	"moderation": mode.Moderations,
	"tts":        mode.AudioSpeech,
	"stt":        mode.AudioTranscription,
	"image":      mode.ImagesGenerations,
	"video":      mode.VideoGenerationsJobs,
}

// endpointSlugToMode maps PPIO endpoint slugs to mode.Mode.
// Used as a fallback when ModelTypeToMode has no match for model_type.
var endpointSlugToMode = map[string]mode.Mode{
	"chat/completions":       mode.ChatCompletions,
	"completions":            mode.ChatCompletions,
	"responses":              mode.ChatCompletions,
	"anthropic":              mode.ChatCompletions,
	"embeddings":             mode.Embeddings,
	"rerank":                 mode.Rerank,
	"moderations":            mode.Moderations,
	"audio/speech":           mode.AudioSpeech,
	"audio/transcriptions":   mode.AudioTranscription,
	"images/generations":     mode.ImagesGenerations,
	"video/generations/jobs": mode.VideoGenerationsJobs,
}

// inferModeFromPPIO infers the mode.Mode from PPIO model_type and endpoints.
// Falls back to endpoint-based inference, then defaults to ChatCompletions.
func inferModeFromPPIO(modelType string, endpoints []string) mode.Mode {
	if m, ok := ModelTypeToMode[modelType]; ok {
		return m
	}

	for _, ep := range endpoints {
		if m, ok := endpointSlugToMode[ep]; ok {
			return m
		}
	}

	return mode.ChatCompletions
}

// modelCreator abstracts the create/update operations for V1 and V2 models
type modelCreator struct {
	create func(tx *gorm.DB, modelID string) error
	update func(tx *gorm.DB, modelID string) error
}

// ExecuteSync performs the actual sync operation with transaction
func ExecuteSync( //nolint:cyclop
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

	// Step 1: Fetch remote models
	sendProgress(progressCallback, "fetching", "正在获取 PPIO 模型列表...", 10, nil)

	client, err := NewPPIOClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create PPIO client: %w", err)
	}

	cfg := GetPPIOConfig()

	// Fetch models from both V1 (public) and V2 (mgmt) APIs, merged into V2 format.
	allModels, fetchErr := client.FetchAllModelsMerged(ctx, cfg.MgmtToken)
	if fetchErr != nil {
		return nil, fmt.Errorf("failed to fetch PPIO models: %w", fetchErr)
	}

	// Log unavailable models that will be filtered out
	unavailCount := 0
	for _, m := range allModels {
		if !m.IsAvailable() {
			unavailCount++
			log.Printf("PPIO sync: skipping unavailable model %s (status=%d)", m.ID, m.Status)
		}
	}

	if unavailCount > 0 {
		sendProgress(progressCallback, "filtering",
			fmt.Sprintf("已过滤 %d 个不可用模型（status≠1）", unavailCount), 20, nil)
	}

	sendProgress(progressCallback, "comparing", "对比本地和远程模型...", 30, nil)

	diff, err := ComparePPIOModelsV2(allModels, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to compare models: %w", err)
	}

	// Build a lookup map for create/update
	modelMap := make(map[string]*PPIOModelV2, len(allModels))
	for i := range allModels {
		modelMap[allModels[i].ID] = &allModels[i]
	}

	creator := modelCreator{
		create: func(tx *gorm.DB, modelID string) error {
			m := modelMap[modelID]
			if m == nil {
				return fmt.Errorf("model %s not found in remote models", modelID)
			}
			return createModelConfigV2(tx, m)
		},
		update: func(tx *gorm.DB, modelID string) error {
			m := modelMap[modelID]
			if m == nil {
				return fmt.Errorf("model %s not found in remote models", modelID)
			}
			return updateModelConfigV2(tx, m)
		},
	}

	result.Summary = diff.Summary

	// If dry run, return here
	if opts.DryRun {
		result.Success = true
		result.DurationMS = time.Since(startTime).Milliseconds()
		sendProgress(progressCallback, "complete", "预览完成", 100, result)
		return result, nil
	}

	// Step 3: Execute sync in transaction
	sendProgress(progressCallback, "syncing", "开始同步模型配置...", 50, nil)

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		return executeSyncTransaction(tx, diff, opts, creator, result, progressCallback)
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	// Step 4: Ensure channels exist (reads from local DB, not remote list)
	sendProgress(progressCallback, "channels", "检查并更新 Channel 模型列表...", 85, nil)

	channelsInfo, err := EnsurePPIOChannels(opts.AutoCreateChannels, cfg)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("channel creation: %v", err))
	}

	// If channels were auto-created, write the channel ID back to options
	// so the sync page can find it on next load.
	if channelsInfo.PPIO.Exists && cfg.ChannelID == 0 && channelsInfo.PPIO.ID > 0 {
		if err := SetPPIOConfigFromChannel(channelsInfo.PPIO.ID); err != nil {
			log.Printf("failed to write back PPIO channel config: %v", err)
		}
	}

	result.Channels = channelsInfo

	// Step 5: Finalize result
	result.Success = len(result.Errors) == 0
	result.DurationMS = time.Since(startTime).Milliseconds()

	// Step 5.5: Refresh global model+channel cache so new models are immediately visible
	if err := model.InitModelConfigAndChannelCache(); err != nil {
		log.Printf("failed to refresh model cache after sync: %v", err)
	}

	// Step 6: Record sync history (after result.Success is set)
	sendProgress(progressCallback, "recording", "记录同步历史...", 95, nil)

	if err := RecordSyncHistory(opts, result); err != nil {
		log.Printf("failed to record sync history: %v", err)
	}

	sendProgress(progressCallback, "complete", "同步完成", 100, result)

	return result, nil
}

// executeSyncTransaction runs add/update/delete inside a DB transaction
func executeSyncTransaction(
	tx *gorm.DB,
	diff *SyncDiff,
	opts SyncOptions,
	creator modelCreator,
	result *SyncResult,
	progressCallback func(event SyncProgressEvent),
) error {
	// Add new models
	totalAdd := max(len(diff.Changes.Add), 1)

	for i, modelDiff := range diff.Changes.Add {
		progress := 50 + (i * 15 / totalAdd)
		sendProgress(
			progressCallback,
			"adding",
			fmt.Sprintf("添加模型 %s (%d/%d)", modelDiff.ModelID, i+1, len(diff.Changes.Add)),
			progress,
			nil,
		)

		if err := creator.create(tx, modelDiff.ModelID); err != nil {
			result.Errors = append(
				result.Errors,
				fmt.Sprintf("failed to add %s: %v", modelDiff.ModelID, err),
			)

			continue
		}

		result.Details.ModelsAdded = append(result.Details.ModelsAdded, modelDiff.ModelID)
	}

	// Update existing models
	totalUpdate := max(len(diff.Changes.Update), 1)

	for i, modelDiff := range diff.Changes.Update {
		progress := 65 + (i * 15 / totalUpdate)
		sendProgress(
			progressCallback,
			"updating",
			fmt.Sprintf("更新模型 %s (%d/%d)", modelDiff.ModelID, i+1, len(diff.Changes.Update)),
			progress,
			nil,
		)

		if err := creator.update(tx, modelDiff.ModelID); err != nil {
			result.Errors = append(
				result.Errors,
				fmt.Sprintf("failed to update %s: %v", modelDiff.ModelID, err),
			)

			continue
		}

		result.Details.ModelsUpdated = append(result.Details.ModelsUpdated, modelDiff.ModelID)
	}

	// Delete models (if enabled)
	if opts.DeleteUnmatchedModel {
		totalDelete := max(len(diff.Changes.Delete), 1)

		for i, modelDiff := range diff.Changes.Delete {
			progress := 80 + (i * 5 / totalDelete)
			sendProgress(
				progressCallback,
				"deleting",
				fmt.Sprintf(
					"删除模型 %s (%d/%d)",
					modelDiff.ModelID,
					i+1,
					len(diff.Changes.Delete),
				),
				progress,
				nil,
			)

			if err := tx.Where("model = ? AND owner = ?", modelDiff.ModelID, string(model.ModelOwnerPPIO)).
				Delete(&model.ModelConfig{}).
				Error; err != nil {
				result.Errors = append(
					result.Errors,
					fmt.Sprintf("failed to delete %s: %v", modelDiff.ModelID, err),
				)

				continue
			}

			result.Details.ModelsDeleted = append(
				result.Details.ModelsDeleted,
				modelDiff.ModelID,
			)
		}
	}

	return nil
}

// EnsurePPIOChannels queries all local ModelConfig entries owned by PPIO,
// partitions them by endpoint compatibility, and writes the lists into the
// corresponding PPIO channels. When autoCreate is true and no PPIO channels
// exist, it creates them automatically using the API key from cfg.
func EnsurePPIOChannels(autoCreate bool, cfg PPIOConfigResult) (ChannelsInfo, error) {
	var localModels []model.ModelConfig

	if err := model.DB.Select("model", "config").
		Where("owner = ?", string(model.ModelOwnerPPIO)).
		Find(&localModels).Error; err != nil {
		return ChannelsInfo{}, fmt.Errorf("failed to query local PPIO models: %w", err)
	}

	var anthropicModels, openaiModels []string

	for _, mc := range localModels {
		// Skip models whose stored status indicates they are not available.
		// This acts as a safety net for models synced before the status filter
		// was added in ComparePPIOModels/V2.
		if status, ok := model.GetModelConfigInt(mc.Config, "status"); ok && status != PPIOModelStatusAvailable {
			continue
		}

		openaiModels = append(openaiModels, mc.Model)

		if slugs, ok := model.GetModelConfigStringSlice(mc.Config, "endpoints"); ok {
			if slices.Contains(slugs, "anthropic") {
				anthropicModels = append(anthropicModels, mc.Model)
			}
		}
	}

	slices.Sort(anthropicModels)
	slices.Sort(openaiModels)

	return ensurePPIOChannelsFromModels(anthropicModels, openaiModels, autoCreate, cfg)
}

func ensurePPIOChannelsFromModels(
	anthropicModels, openaiModels []string,
	autoCreate bool, cfg PPIOConfigResult,
) (ChannelsInfo, error) {
	info := ChannelsInfo{}

	var channels []model.Channel

	err := model.DB.Where("base_url "+common.LikeOp()+" ?", "%ppio%").Find(&channels).Error
	if err != nil {
		return info, fmt.Errorf("failed to query PPIO channels: %w", err)
	}

	// Auto-create channels when none exist and the option is enabled.
	if len(channels) == 0 {
		if !autoCreate || cfg.APIKey == "" {
			return info, nil
		}

		created, createErr := createPPIOChannels(cfg, anthropicModels, openaiModels)
		if createErr != nil {
			return info, createErr
		}

		info.PPIO.Exists = true
		info.PPIO.ID = created[0].ID

		return info, nil
	}

	info.PPIO.Exists = true
	info.PPIO.ID = channels[0].ID

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

// createPPIOChannels creates the OpenAI-compatible channel and, if there are
// anthropic-endpoint models, an Anthropic-compatible channel as well.
func createPPIOChannels(cfg PPIOConfigResult, anthropicModels, openaiModels []string) ([]model.Channel, error) {
	openaiBase := cfg.APIBase
	if openaiBase == "" {
		openaiBase = DefaultPPIOAPIBase
	}

	var created []model.Channel

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		openaiCh := model.Channel{
			Name:    "PPIO (OpenAI)",
			Type:    model.ChannelTypePPIO,
			BaseURL: openaiBase,
			Key:     cfg.APIKey,
			Models:  openaiModels,
			Status:  model.ChannelStatusEnabled,
		}

		if err := tx.Create(&openaiCh).Error; err != nil {
			return fmt.Errorf("failed to create PPIO OpenAI channel: %w", err)
		}

		created = append(created, openaiCh)

		if len(anthropicModels) > 0 {
			anthropicCh := model.Channel{
				Name:    "PPIO (Anthropic)",
				Type:    model.ChannelTypeAnthropic,
				BaseURL: DefaultPPIOAnthropicBase,
				Key:     cfg.APIKey,
				Models:  anthropicModels,
				Status:  model.ChannelStatusEnabled,
			}

			if err := tx.Create(&anthropicCh).Error; err != nil {
				return fmt.Errorf("failed to create PPIO Anthropic channel: %w", err)
			}

			created = append(created, anthropicCh)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	log.Printf("auto-created %d PPIO channel(s)", len(created))

	return created, nil
}

// RecordSyncHistory records sync history to database
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

// Helper functions

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

// V1 model config creation (old public API)

func createModelConfig(tx *gorm.DB, ppioModel *PPIOModel) error {
	configData := toModelConfigKeys(buildConfigFromPPIOModel(ppioModel))

	// Check if model already exists (possibly with a different owner)
	var existing model.ModelConfig
	if err := tx.Where("model = ?", ppioModel.ID).First(&existing).Error; err == nil {
		existing.Owner = model.ModelOwnerPPIO
		existing.Config = configData
		existing.Type = inferModeFromPPIO(ppioModel.ModelType, ppioModel.Endpoints)
		existing.RPM = 60
		existing.TPM = 1000000
		existing.Price.InputPrice = model.ZeroNullFloat64(ppioModel.GetInputPricePerToken())
		existing.Price.InputPriceUnit = model.ZeroNullInt64(1)
		existing.Price.OutputPrice = model.ZeroNullFloat64(ppioModel.GetOutputPricePerToken())
		existing.Price.OutputPriceUnit = model.ZeroNullInt64(1)

		return tx.Save(&existing).Error
	}

	modelConfig := model.ModelConfig{
		Model:  ppioModel.ID,
		Owner:  model.ModelOwnerPPIO,
		Type:   inferModeFromPPIO(ppioModel.ModelType, ppioModel.Endpoints),
		RPM:    60,
		TPM:    1000000,
		Config: configData,
	}

	modelConfig.Price.InputPrice = model.ZeroNullFloat64(ppioModel.GetInputPricePerToken())
	modelConfig.Price.InputPriceUnit = model.ZeroNullInt64(1)
	modelConfig.Price.OutputPrice = model.ZeroNullFloat64(ppioModel.GetOutputPricePerToken())
	modelConfig.Price.OutputPriceUnit = model.ZeroNullInt64(1)

	return tx.Create(&modelConfig).Error
}

func updateModelConfig(tx *gorm.DB, ppioModel *PPIOModel) error {
	var existing model.ModelConfig
	if err := tx.Where("model = ? AND owner = ?", ppioModel.ID, string(model.ModelOwnerPPIO)).
		First(&existing).
		Error; err != nil {
		return err
	}

	existing.Type = inferModeFromPPIO(ppioModel.ModelType, ppioModel.Endpoints)
	existing.Config = toModelConfigKeys(buildConfigFromPPIOModel(ppioModel))
	existing.Price.InputPrice = model.ZeroNullFloat64(ppioModel.GetInputPricePerToken())
	existing.Price.OutputPrice = model.ZeroNullFloat64(ppioModel.GetOutputPricePerToken())
	existing.Price.InputPriceUnit = model.ZeroNullInt64(1)
	existing.Price.OutputPriceUnit = model.ZeroNullInt64(1)

	return tx.Save(&existing).Error
}

// V2 model config creation (management API with tiered & cache pricing)

func createModelConfigV2(tx *gorm.DB, m *PPIOModelV2) error {
	configData := toModelConfigKeys(buildConfigFromPPIOModelV2(m))

	rpm := int64(60)
	if m.RPM > 0 {
		rpm = int64(m.RPM)
	}

	tpm := int64(1000000)
	if m.TPM > 0 {
		tpm = int64(m.TPM)
	}

	// Check if model already exists (possibly with a different owner).
	// ModelConfig primary key is `model` alone (no composite with owner),
	// so we must handle the case where e.g. "deepseek/deepseek-r1" exists
	// with owner "deepseek" and the PPIO V2 API also returns it.
	var existing model.ModelConfig
	if err := tx.Where("model = ?", m.ID).First(&existing).Error; err == nil {
		// Model exists — update it in place and claim ownership for PPIO
		existing.Owner = model.ModelOwnerPPIO
		existing.Config = configData
		existing.Type = inferModeFromPPIO(m.ModelType, m.Endpoints)
		existing.RPM = rpm
		existing.TPM = tpm
		setPriceFromV2Model(&existing.Price, m)

		return tx.Save(&existing).Error
	}

	// Model doesn't exist — create new
	modelConfig := model.ModelConfig{
		Model:  m.ID,
		Owner:  model.ModelOwnerPPIO,
		Type:   inferModeFromPPIO(m.ModelType, m.Endpoints),
		RPM:    rpm,
		TPM:    tpm,
		Config: configData,
	}

	setPriceFromV2Model(&modelConfig.Price, m)

	return tx.Create(&modelConfig).Error
}

func updateModelConfigV2(tx *gorm.DB, m *PPIOModelV2) error {
	var existing model.ModelConfig
	if err := tx.Where("model = ?", m.ID).
		First(&existing).
		Error; err != nil {
		return err
	}

	existing.Owner = model.ModelOwnerPPIO
	existing.Type = inferModeFromPPIO(m.ModelType, m.Endpoints)
	existing.Config = toModelConfigKeys(buildConfigFromPPIOModelV2(m))

	if m.RPM > 0 {
		existing.RPM = int64(m.RPM)
	}

	if m.TPM > 0 {
		existing.TPM = int64(m.TPM)
	}

	setPriceFromV2Model(&existing.Price, m)

	return tx.Save(&existing).Error
}

// setPriceFromV2Model populates Price fields from a V2 model, including
// tiered billing (→ ConditionalPrices) and cache pricing.
func setPriceFromV2Model(price *model.Price, m *PPIOModelV2) {
	price.InputPrice = model.ZeroNullFloat64(m.GetInputPricePerToken())
	price.InputPriceUnit = model.ZeroNullInt64(1)
	price.OutputPrice = model.ZeroNullFloat64(m.GetOutputPricePerToken())
	price.OutputPriceUnit = model.ZeroNullInt64(1)

	// Cache pricing
	if m.SupportPromptCache && m.CacheReadInputTokenPricePerM > 0 {
		price.CachedPrice = model.ZeroNullFloat64(m.GetCacheReadPricePerToken())
		price.CachedPriceUnit = model.ZeroNullInt64(1)
	}

	if m.SupportPromptCache && m.CacheCreationInputTokenPricePerM > 0 {
		price.CacheCreationPrice = model.ZeroNullFloat64(m.GetCacheCreationPricePerToken())
		price.CacheCreationPriceUnit = model.ZeroNullInt64(1)
	}

	// Tiered billing → ConditionalPrices
	if m.IsTieredBilling && len(m.TieredBillingConfigs) > 0 {
		conditionalPrices := make([]model.ConditionalPrice, 0, len(m.TieredBillingConfigs))

		for i, tier := range m.TieredBillingConfigs {
			minTokens, maxTokens := adjustTierBounds(m.TieredBillingConfigs, i)
			if maxTokens > 0 && minTokens > maxTokens {
				continue // degenerate tier after boundary adjustment
			}

			cp := model.ConditionalPrice{
				Condition: model.PriceCondition{
					InputTokenMin: minTokens,
					InputTokenMax: maxTokens,
				},
				Price: model.Price{
					InputPrice:     model.ZeroNullFloat64(tier.InputPricing.PricePerToken()),
					InputPriceUnit: model.ZeroNullInt64(1),
					OutputPrice:     model.ZeroNullFloat64(tier.OutputPricing.PricePerToken()),
					OutputPriceUnit: model.ZeroNullInt64(1),
				},
			}

			// Tier-level cache pricing
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

// buildConfigFromPPIOModelV2 builds model config map from a V2 PPIO model
func buildConfigFromPPIOModelV2(m *PPIOModelV2) map[string]any {
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


// toModelConfigKeys converts map[string]any to map[ModelConfigKey]any without JSON round-trip.
func toModelConfigKeys(m map[string]any) map[model.ModelConfigKey]any {
	out := make(map[model.ModelConfigKey]any, len(m))
	for k, v := range m {
		out[model.ModelConfigKey(k)] = v
	}

	return out
}

// adjustTierBounds returns the effective [min, max] for tier i, bumping min by 1
// when it overlaps with the previous tier's max (PPIO uses inclusive boundaries
// like [0,128000],[128000,∞] but aiproxy requires non-overlapping ranges).
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

// countEffectiveTiers returns the number of non-degenerate tiers after boundary adjustment.
func countEffectiveTiers(tiers []TieredBillingConfig) int {
	count := 0

	for i := range tiers {
		minTokens, maxTokens := adjustTierBounds(tiers, i)
		if maxTokens > 0 && minTokens > maxTokens {
			continue
		}

		count++
	}

	return count
}
