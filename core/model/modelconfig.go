package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-viper/mapstructure/v2"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/mode"
	"gorm.io/gorm"
)

const (
	// /1K tokens
	PriceUnit = 1000
)

type TimeoutConfig struct {
	RequestTimeout       int64 `json:"request_timeout,omitempty"        yaml:"request_timeout,omitempty"`
	StreamRequestTimeout int64 `json:"stream_request_timeout,omitempty" yaml:"stream_request_timeout,omitempty"`
}

type ModelConfig struct {
	CreatedAt        time.Time                 `gorm:"index;autoCreateTime"          json:"created_at"                     yaml:"-"`
	UpdatedAt        time.Time                 `gorm:"index;autoUpdateTime"          json:"updated_at"                     yaml:"-"`
	Config           map[ModelConfigKey]any    `gorm:"serializer:fastjson;type:text" json:"config,omitempty"               yaml:"config,omitempty"`
	Plugin           map[string]map[string]any `gorm:"serializer:fastjson;type:text" json:"plugin,omitempty"               yaml:"plugin,omitempty"`
	Model            string                    `gorm:"size:64;primaryKey"            json:"model"                          yaml:"model,omitempty"`
	Owner            ModelOwner                `gorm:"type:varchar(32);index"        json:"owner"                          yaml:"owner,omitempty"`
	Type             mode.Mode                 `                                     json:"type"                           yaml:"type,omitempty"`
	ExcludeFromTests bool                      `                                     json:"exclude_from_tests,omitempty"   yaml:"exclude_from_tests,omitempty"`
	RPM              int64                     `                                     json:"rpm,omitempty"                  yaml:"rpm,omitempty"`
	TPM              int64                     `                                     json:"tpm,omitempty"                  yaml:"tpm,omitempty"`
	// map[size]map[quality]price_per_image
	ImageQualityPrices map[string]map[string]float64 `gorm:"serializer:fastjson;type:text" json:"image_quality_prices,omitempty" yaml:"image_quality_prices,omitempty"`
	// map[size]price_per_image
	ImagePrices     map[string]float64 `gorm:"serializer:fastjson;type:text" json:"image_prices,omitempty"         yaml:"image_prices,omitempty"`
	Price           Price              `gorm:"embedded"                      json:"price,omitempty"                yaml:"price,omitempty"`
	RetryTimes      int64              `                                     json:"retry_times,omitempty"          yaml:"retry_times,omitempty"`
	TimeoutConfig   TimeoutConfig      `gorm:"embedded"                      json:"timeout_config,omitempty"       yaml:"timeout_config,omitempty"`
	WarnErrorRate   float64            `                                     json:"warn_error_rate,omitempty"      yaml:"warn_error_rate,omitempty"`
	MaxErrorRate    float64            `                                     json:"max_error_rate,omitempty"       yaml:"max_error_rate,omitempty"`
	ForceSaveDetail bool               `                                     json:"force_save_detail,omitempty"    yaml:"force_save_detail,omitempty"`
}

func (c *ModelConfig) BeforeSave(_ *gorm.DB) (err error) {
	if c.Model == "" {
		return errors.New("model is required")
	}

	if err := c.Price.ValidateConditionalPrices(); err != nil {
		return err
	}

	return nil
}

func NewDefaultModelConfig(model string) ModelConfig {
	return ModelConfig{
		Model: model,
	}
}

func (c *ModelConfig) RequestTimeout() time.Duration {
	return timeoutSecond(c.TimeoutConfig.RequestTimeout)
}

func (c *ModelConfig) StreamRequestTimeout() time.Duration {
	return timeoutSecond(c.TimeoutConfig.StreamRequestTimeout)
}

func timeoutSecond(second int64) time.Duration {
	if second == 0 {
		return 0
	}
	return time.Duration(second) * time.Second
}

func (c *ModelConfig) LoadPluginConfig(pluginName string, config any) error {
	if len(c.Plugin) == 0 {
		return nil
	}

	pluginConfig, ok := c.Plugin[pluginName]
	if !ok || len(pluginConfig) == 0 {
		return nil
	}

	return mapstructure.Decode(pluginConfig, config)
}

func (c *ModelConfig) LoadFromGroupModelConfig(groupModelConfig GroupModelConfig) ModelConfig {
	newC := *c
	if groupModelConfig.OverrideLimit {
		newC.RPM = groupModelConfig.RPM
		newC.TPM = groupModelConfig.TPM
	}

	if groupModelConfig.OverridePrice {
		newC.ImagePrices = groupModelConfig.ImagePrices
		newC.Price = groupModelConfig.Price
	}

	if groupModelConfig.OverrideRetryTimes {
		newC.RetryTimes = groupModelConfig.RetryTimes
	}

	if groupModelConfig.OverrideForceSaveDetail {
		newC.ForceSaveDetail = groupModelConfig.ForceSaveDetail
	}

	return newC
}

func (c *ModelConfig) MarshalJSON() ([]byte, error) {
	type Alias ModelConfig

	a := &struct {
		*Alias
		CreatedAt int64 `json:"created_at,omitempty"`
		UpdatedAt int64 `json:"updated_at,omitempty"`
	}{
		Alias: (*Alias)(c),
	}
	if !c.CreatedAt.IsZero() {
		a.CreatedAt = c.CreatedAt.UnixMilli()
	}

	if !c.UpdatedAt.IsZero() {
		a.UpdatedAt = c.UpdatedAt.UnixMilli()
	}

	return sonic.Marshal(a)
}

func (c *ModelConfig) MaxContextTokens() (int, bool) {
	return GetModelConfigInt(c.Config, ModelConfigMaxContextTokensKey)
}

func (c *ModelConfig) MaxInputTokens() (int, bool) {
	return GetModelConfigInt(c.Config, ModelConfigMaxInputTokensKey)
}

func (c *ModelConfig) MaxOutputTokens() (int, bool) {
	return GetModelConfigInt(c.Config, ModelConfigMaxOutputTokensKey)
}

func (c *ModelConfig) SupportVision() (bool, bool) {
	return GetModelConfigBool(c.Config, ModelConfigVisionKey)
}

func (c *ModelConfig) SupportVoices() ([]string, bool) {
	return GetModelConfigStringSlice(c.Config, ModelConfigSupportVoicesKey)
}

func (c *ModelConfig) SupportToolChoice() (bool, bool) {
	return GetModelConfigBool(c.Config, ModelConfigToolChoiceKey)
}

func (c *ModelConfig) SupportFormats() ([]string, bool) {
	return GetModelConfigStringSlice(c.Config, ModelConfigSupportFormatsKey)
}

func GetModelConfigs(
	page, perPage int,
	model string,
) (configs []*ModelConfig, total int64, err error) {
	tx := DB.Model(&ModelConfig{})
	if model != "" {
		tx = tx.Where("model = ?", model)
	}

	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	if total <= 0 {
		return nil, 0, nil
	}

	limit, offset := toLimitOffset(page, perPage)
	err = tx.
		Order("created_at desc").
		Omit("created_at", "updated_at").
		Limit(limit).
		Offset(offset).
		Find(&configs).
		Error

	return configs, total, err
}

func GetAllModelConfigs() (configs []ModelConfig, err error) {
	tx := DB.Model(&ModelConfig{})
	err = tx.Order("created_at desc").
		Omit("created_at", "updated_at").
		Find(&configs).
		Error

	return configs, err
}

func GetModelConfigsByModels(models []string) (configs []ModelConfig, err error) {
	tx := DB.Model(&ModelConfig{}).Where("model IN (?)", models)
	err = tx.Order("created_at desc").
		Omit("created_at", "updated_at").
		Find(&configs).
		Error

	return configs, err
}

func GetModelConfig(model string) (ModelConfig, error) {
	config := ModelConfig{}
	err := DB.Model(&ModelConfig{}).
		Where("model = ?", model).
		Omit("created_at", "updated_at").
		First(config).
		Error

	return config, HandleNotFound(err, ErrModelConfigNotFound)
}

func SearchModelConfigs(
	keyword string,
	page, perPage int,
	model string,
	owner ModelOwner,
) (configs []ModelConfig, total int64, err error) {
	tx := DB.Model(&ModelConfig{}).Where("model LIKE ?", "%"+keyword+"%")
	if model != "" {
		tx = tx.Where("model = ?", model)
	}

	if owner != "" {
		tx = tx.Where("owner = ?", owner)
	}

	if keyword != "" {
		var (
			conditions []string
			values     []any
		)

		if model == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "model ILIKE ?")
			} else {
				conditions = append(conditions, "model LIKE ?")
			}

			values = append(values, "%"+keyword+"%")
		}

		if owner != "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "owner ILIKE ?")
			} else {
				conditions = append(conditions, "owner LIKE ?")
			}

			values = append(values, "%"+string(owner)+"%")
		}

		if len(conditions) > 0 {
			tx = tx.Where(fmt.Sprintf("(%s)", strings.Join(conditions, " OR ")), values...)
		}
	}

	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	if total <= 0 {
		return nil, 0, nil
	}

	limit, offset := toLimitOffset(page, perPage)
	err = tx.Order("created_at desc").
		Omit("created_at", "updated_at").
		Limit(limit).
		Offset(offset).
		Find(&configs).
		Error

	return configs, total, err
}

func SaveModelConfig(config ModelConfig) (err error) {
	defer func() {
		if err == nil {
			_ = InitModelConfigAndChannelCache()
		}
	}()

	return DB.Save(&config).Error
}

func SaveModelConfigs(configs []ModelConfig) (err error) {
	defer func() {
		if err == nil {
			_ = InitModelConfigAndChannelCache()
		}
	}()

	return DB.Transaction(func(tx *gorm.DB) error {
		for _, config := range configs {
			if err := tx.Save(&config).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

const ErrModelConfigNotFound = "model config"

func DeleteModelConfig(model string) error {
	result := DB.Where("model = ?", model).Delete(&ModelConfig{})
	return HandleUpdateResult(result, ErrModelConfigNotFound)
}

func DeleteModelConfigsByModels(models []string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return tx.
			Where("model IN (?)", models).
			Delete(&ModelConfig{}).
			Error
	})
}
