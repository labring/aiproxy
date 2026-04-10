package model

import (
	"context"
	"errors"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/oncall"
	"github.com/maruel/natural"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ModelConfigCache interface {
	GetModelConfig(model string) (ModelConfig, bool)
}

type ModelCaches struct {
	ModelConfig ModelConfigCache

	EnabledModelsBySet       map[string][]string
	EnabledModelConfigsBySet map[string][]ModelConfig
	EnabledModelConfigsMap   map[string]ModelConfig

	EnabledModel2ChannelsBySet  map[string]map[string][]*Channel
	DisabledModel2ChannelsBySet map[string]map[string][]*Channel
}

var modelCaches atomic.Pointer[ModelCaches]

func init() {
	modelCaches.Store(new(ModelCaches))
}

func LoadModelCaches() *ModelCaches {
	return modelCaches.Load()
}

func InitModelConfigAndChannelCache() error {
	modelConfig, err := initializeModelConfigCache()
	if err != nil {
		return err
	}

	modelConfig = applyYAMLConfigToModelConfigCache(modelConfig)

	enabledChannels, err := LoadEnabledChannels()
	if err != nil {
		return err
	}

	enabledModel2ChannelsBySet := buildModelToChannelsBySetMap(enabledChannels)
	sortChannelsByPriorityBySet(enabledModel2ChannelsBySet)

	enabledModelsBySet, enabledModelConfigsBySet, enabledModelConfigsMap := buildEnabledModelsBySet(
		enabledModel2ChannelsBySet,
		modelConfig,
	)

	disabledChannels, err := LoadDisabledChannels()
	if err != nil {
		return err
	}

	disabledModel2ChannelsBySet := buildModelToChannelsBySetMap(disabledChannels)

	modelCaches.Store(&ModelCaches{
		ModelConfig: modelConfig,

		EnabledModelsBySet:       enabledModelsBySet,
		EnabledModelConfigsBySet: enabledModelConfigsBySet,
		EnabledModelConfigsMap:   enabledModelConfigsMap,

		EnabledModel2ChannelsBySet:  enabledModel2ChannelsBySet,
		DisabledModel2ChannelsBySet: disabledModel2ChannelsBySet,
	})

	return nil
}

func LoadEnabledChannels() ([]*Channel, error) {
	var channels []*Channel

	err := DB.Where("status = ?", ChannelStatusEnabled).Find(&channels).Error
	if err != nil {
		return nil, err
	}

	configChannels := NewConfigChannels(LoadYAMLConfig(), ChannelStatusEnabled)
	if len(configChannels) != 0 {
		log.Infof("added %d channels from config", len(configChannels))
		channels = append(channels, configChannels...)
	}

	for _, channel := range channels {
		initializeChannelModels(channel)
		initializeChannelModelMapping(channel)
	}

	return channels, nil
}

func LoadDisabledChannels() ([]*Channel, error) {
	var channels []*Channel

	err := DB.Where("status = ?", ChannelStatusDisabled).Find(&channels).Error
	if err != nil {
		return nil, err
	}

	configChannels := NewConfigChannels(LoadYAMLConfig(), ChannelStatusDisabled)
	if len(configChannels) != 0 {
		log.Infof("added %d channels from config", len(configChannels))
		channels = append(channels, configChannels...)
	}

	for _, channel := range channels {
		initializeChannelModels(channel)
		initializeChannelModelMapping(channel)
	}

	return channels, nil
}

func LoadChannels() ([]*Channel, error) {
	var channels []*Channel

	err := DB.Find(&channels).Error
	if err != nil {
		return nil, err
	}

	configChannels := NewConfigChannels(LoadYAMLConfig(), 0)
	if len(configChannels) != 0 {
		log.Infof("added %d channels from config", len(configChannels))
		channels = append(channels, configChannels...)
	}

	for _, channel := range channels {
		initializeChannelModels(channel)
		initializeChannelModelMapping(channel)
	}

	return channels, nil
}

func LoadChannelByID(id int) (*Channel, error) {
	var channel Channel

	err := DB.First(&channel, id).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		chs, err := LoadChannels()
		if err != nil {
			return nil, err
		}

		for _, c := range chs {
			if c.ID == id {
				return c, nil
			}
		}

		return nil, gorm.ErrRecordNotFound
	}

	initializeChannelModels(&channel)
	initializeChannelModelMapping(&channel)

	return &channel, nil
}

var _ ModelConfigCache = (*modelConfigMapCache)(nil)

type modelConfigMapCache struct {
	modelConfigMap map[string]ModelConfig
}

func (m *modelConfigMapCache) GetModelConfig(model string) (ModelConfig, bool) {
	config, ok := m.modelConfigMap[model]
	return config, ok
}

var _ ModelConfigCache = (*disabledModelConfigCache)(nil)

type disabledModelConfigCache struct {
	modelConfigs ModelConfigCache
}

func (d *disabledModelConfigCache) GetModelConfig(model string) (ModelConfig, bool) {
	if config, ok := d.modelConfigs.GetModelConfig(model); ok {
		return config, true
	}
	return NewDefaultModelConfig(model), true
}

func initializeModelConfigCache() (ModelConfigCache, error) {
	modelConfigs, err := GetAllModelConfigs()
	if err != nil {
		return nil, err
	}

	newModelConfigMap := make(map[string]ModelConfig)
	for _, modelConfig := range modelConfigs {
		newModelConfigMap[modelConfig.Model] = modelConfig
	}

	configs := &modelConfigMapCache{modelConfigMap: newModelConfigMap}
	if config.DisableModelConfig {
		return &disabledModelConfigCache{modelConfigs: configs}, nil
	}

	return configs, nil
}

func initializeChannelModels(channel *Channel) {
	if len(channel.Models) == 0 {
		channel.Models = config.GetDefaultChannelModels()[int(channel.Type)]
		return
	}

	findedModels, missingModels, err := GetModelConfigWithModels(channel.Models)
	if err != nil {
		return
	}

	if len(missingModels) > 0 {
		slices.Sort(missingModels)
		log.Errorf("model config not found: %v", missingModels)
	}

	slices.Sort(findedModels)
	channel.Models = findedModels
}

func initializeChannelModelMapping(channel *Channel) {
	if len(channel.ModelMapping) == 0 {
		channel.ModelMapping = config.GetDefaultChannelModelMapping()[int(channel.Type)]
	}
}

func buildModelToChannelsBySetMap(channels []*Channel) map[string]map[string][]*Channel {
	modelMapBySet := make(map[string]map[string][]*Channel)

	for _, channel := range channels {
		sets := channel.GetSets()
		for _, set := range sets {
			if _, ok := modelMapBySet[set]; !ok {
				modelMapBySet[set] = make(map[string][]*Channel)
			}

			for _, model := range channel.Models {
				modelMapBySet[set][model] = append(modelMapBySet[set][model], channel)
			}
		}
	}

	return modelMapBySet
}

func sortChannelsByPriorityBySet(modelMapBySet map[string]map[string][]*Channel) {
	for _, modelMap := range modelMapBySet {
		for _, channels := range modelMap {
			sort.Slice(channels, func(i, j int) bool {
				return channels[i].GetPriority() > channels[j].GetPriority()
			})
		}
	}
}

func buildEnabledModelsBySet(
	modelMapBySet map[string]map[string][]*Channel,
	modelConfigCache ModelConfigCache,
) (
	map[string][]string,
	map[string][]ModelConfig,
	map[string]ModelConfig,
) {
	modelsBySet := make(map[string][]string)
	modelConfigsBySet := make(map[string][]ModelConfig)
	modelConfigsMap := make(map[string]ModelConfig)

	for set, modelMap := range modelMapBySet {
		models := make([]string, 0)
		configs := make([]ModelConfig, 0)
		appended := make(map[string]struct{})

		for model := range modelMap {
			if _, ok := appended[model]; ok {
				continue
			}

			if config, ok := modelConfigCache.GetModelConfig(model); ok {
				models = append(models, model)
				configs = append(configs, config)
				appended[model] = struct{}{}
				modelConfigsMap[model] = config
			}
		}

		slices.Sort(models)
		slices.SortStableFunc(configs, SortModelConfigsFunc)

		modelsBySet[set] = models
		modelConfigsBySet[set] = configs
	}

	return modelsBySet, modelConfigsBySet, modelConfigsMap
}

func SortModelConfigsFunc(i, j ModelConfig) int {
	if i.Owner != j.Owner {
		if natural.Less(string(i.Owner), string(j.Owner)) {
			return -1
		}
		return 1
	}

	if i.Type != j.Type {
		if i.Type < j.Type {
			return -1
		}
		return 1
	}

	if i.Model == j.Model {
		return 0
	}

	if natural.Less(i.Model, j.Model) {
		return -1
	}

	return 1
}

func SyncModelConfigAndChannelCache(
	ctx context.Context,
	wg *sync.WaitGroup,
	frequency time.Duration,
) {
	defer wg.Done()

	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := InitModelConfigAndChannelCache()
			if err != nil {
				notify.ErrorThrottle(
					"syncModelChannel",
					time.Minute*5,
					"failed to sync channels",
					err.Error(),
				)
				oncall.AlertDBError("SyncModelConfigAndChannelCache", err)
			} else {
				oncall.ClearDBError("SyncModelConfigAndChannelCache")
			}
		}
	}
}
