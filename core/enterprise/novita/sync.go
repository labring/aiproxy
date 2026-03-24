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

// modelCreator abstracts the create/update operations for V1 and V2 models.
type modelCreator struct {
	create func(tx *gorm.DB, modelID string) error
	update func(tx *gorm.DB, modelID string) error
}

// ExecuteSync performs the actual sync operation with transaction.
func ExecuteSync( //nolint:cyclop
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
	useV2 := cfg.MgmtToken != ""

	var (
		diff    *SyncDiff
		creator modelCreator
	)

	if useV2 {
		sendProgress(progressCallback, "fetching", "正在通过管理接口获取全量模型（含闭源）...", 10, nil)

		v2Models, fetchErr := client.FetchAllModels(cfg.MgmtToken)
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch Novita models (mgmt API): %w", fetchErr)
		}

		unavailCount := 0
		for _, m := range v2Models {
			if !m.IsAvailable() {
				unavailCount++
			}
		}

		if unavailCount > 0 {
			sendProgress(progressCallback, "filtering",
				fmt.Sprintf("已过滤 %d 个不可用模型（status≠1）", unavailCount), 20, nil)
		}

		sendProgress(progressCallback, "comparing", "对比本地和远程模型...", 30, nil)

		diff, err = CompareNovitaModelsV2(v2Models, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to compare models: %w", err)
		}

		v2Map := make(map[string]*NovitaModelV2, len(v2Models))
		for i := range v2Models {
			v2Map[v2Models[i].ID] = &v2Models[i]
		}

		creator = modelCreator{
			create: func(tx *gorm.DB, modelID string) error {
				m := v2Map[modelID]
				if m == nil {
					return fmt.Errorf("model %s not found in remote models", modelID)
				}

				return createModelConfigV2(tx, m)
			},
			update: func(tx *gorm.DB, modelID string) error {
				m := v2Map[modelID]
				if m == nil {
					return fmt.Errorf("model %s not found in remote models", modelID)
				}

				return updateModelConfigV2(tx, m)
			},
		}
	} else {
		remoteModels, fetchErr := client.FetchModels()
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch Novita models: %w", fetchErr)
		}

		sendProgress(progressCallback, "comparing", "对比本地和远程模型...", 30, nil)

		diff, err = CompareNovitaModels(remoteModels, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to compare models: %w", err)
		}

		v1Map := make(map[string]*NovitaModel, len(remoteModels))
		for i := range remoteModels {
			v1Map[remoteModels[i].ID] = &remoteModels[i]
		}

		creator = modelCreator{
			create: func(tx *gorm.DB, modelID string) error {
				m := v1Map[modelID]
				if m == nil {
					return fmt.Errorf("model %s not found in remote models", modelID)
				}

				return createModelConfigV1(tx, m)
			},
			update: func(tx *gorm.DB, modelID string) error {
				m := v1Map[modelID]
				if m == nil {
					return fmt.Errorf("model %s not found in remote models", modelID)
				}

				return updateModelConfigV1(tx, m)
			},
		}
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
		return executeSyncTransaction(tx, diff, opts, creator, result, progressCallback)
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
	creator modelCreator,
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

		if err := creator.create(tx, modelDiff.ModelID); err != nil {
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

		if err := creator.update(tx, modelDiff.ModelID); err != nil {
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

// createModelConfigV1 creates a ModelConfig from a V1 Novita model.
func createModelConfigV1(tx *gorm.DB, m *NovitaModel) error {
	configData := toModelConfigKeys(buildConfigFromV1Model(m))

	var existing model.ModelConfig
	if err := tx.Where("model = ?", m.ID).First(&existing).Error; err == nil {
		existing.Owner = model.ModelOwnerNovita
		existing.Config = configData
		existing.Type = modeFromEndpoints(m.ModelType, m.Endpoints)
		existing.RPM = 60
		existing.TPM = 1000000
		existing.Price.InputPrice = model.ZeroNullFloat64(m.GetInputPricePerToken())
		existing.Price.InputPriceUnit = model.ZeroNullInt64(1)
		existing.Price.OutputPrice = model.ZeroNullFloat64(m.GetOutputPricePerToken())
		existing.Price.OutputPriceUnit = model.ZeroNullInt64(1)

		return tx.Save(&existing).Error
	}

	mc := model.ModelConfig{
		Model:  m.ID,
		Owner:  model.ModelOwnerNovita,
		Type:   modeFromEndpoints(m.ModelType, m.Endpoints),
		RPM:    60,
		TPM:    1000000,
		Config: configData,
	}

	mc.Price.InputPrice = model.ZeroNullFloat64(m.GetInputPricePerToken())
	mc.Price.InputPriceUnit = model.ZeroNullInt64(1)
	mc.Price.OutputPrice = model.ZeroNullFloat64(m.GetOutputPricePerToken())
	mc.Price.OutputPriceUnit = model.ZeroNullInt64(1)

	return tx.Create(&mc).Error
}

// updateModelConfigV1 updates an existing ModelConfig from a V1 Novita model.
func updateModelConfigV1(tx *gorm.DB, m *NovitaModel) error {
	var existing model.ModelConfig
	if err := tx.Where("model = ? AND owner = ?", m.ID, string(model.ModelOwnerNovita)).
		First(&existing).Error; err != nil {
		return err
	}

	existing.Type = modeFromEndpoints(m.ModelType, m.Endpoints)
	existing.Config = toModelConfigKeys(buildConfigFromV1Model(m))
	existing.Price.InputPrice = model.ZeroNullFloat64(m.GetInputPricePerToken())
	existing.Price.InputPriceUnit = model.ZeroNullInt64(1)
	existing.Price.OutputPrice = model.ZeroNullFloat64(m.GetOutputPricePerToken())
	existing.Price.OutputPriceUnit = model.ZeroNullInt64(1)

	return tx.Save(&existing).Error
}

// createModelConfigV2 creates a ModelConfig from a V2 Novita model.
func createModelConfigV2(tx *gorm.DB, m *NovitaModelV2) error {
	configData := toModelConfigKeys(buildConfigFromV2Model(m))

	var existing model.ModelConfig
	if err := tx.Where("model = ?", m.ID).First(&existing).Error; err == nil {
		existing.Owner = model.ModelOwnerNovita
		existing.Config = configData
		existing.Type = modeFromEndpoints(m.ModelType, m.Endpoints)
		existing.RPM = 60
		existing.TPM = 1000000
		existing.Price.InputPrice = model.ZeroNullFloat64(m.GetInputPricePerToken())
		existing.Price.InputPriceUnit = model.ZeroNullInt64(1)
		existing.Price.OutputPrice = model.ZeroNullFloat64(m.GetOutputPricePerToken())
		existing.Price.OutputPriceUnit = model.ZeroNullInt64(1)

		return tx.Save(&existing).Error
	}

	mc := model.ModelConfig{
		Model:  m.ID,
		Owner:  model.ModelOwnerNovita,
		Type:   modeFromEndpoints(m.ModelType, m.Endpoints),
		RPM:    60,
		TPM:    1000000,
		Config: configData,
	}

	mc.Price.InputPrice = model.ZeroNullFloat64(m.GetInputPricePerToken())
	mc.Price.InputPriceUnit = model.ZeroNullInt64(1)
	mc.Price.OutputPrice = model.ZeroNullFloat64(m.GetOutputPricePerToken())
	mc.Price.OutputPriceUnit = model.ZeroNullInt64(1)

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
	existing.Price.InputPrice = model.ZeroNullFloat64(m.GetInputPricePerToken())
	existing.Price.InputPriceUnit = model.ZeroNullInt64(1)
	existing.Price.OutputPrice = model.ZeroNullFloat64(m.GetOutputPricePerToken())
	existing.Price.OutputPriceUnit = model.ZeroNullInt64(1)

	return tx.Save(&existing).Error
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

	err := model.DB.Where("base_url "+likeOp()+" ?", "%novita%").Find(&channels).Error
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

		if err := model.DB.Model(&channels[i]).Update("models", channels[i].Models).Error; err != nil {
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
