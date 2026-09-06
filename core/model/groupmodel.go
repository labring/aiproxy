package model

import (
	"errors"
	"fmt"

	"github.com/labring/aiproxy/core/common/config"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	GroupModelConfigCacheKey = "group_model_config"
)

var groupModelConfigZeroValueUpdateFields = []string{
	"override_retry_times",
	"retry_times",
	"override_retry_budget",
	"retry_budget",
	"override_max_image_generation_count",
	"max_image_generation_count",
	"override_max_video_generation_seconds",
	"max_video_generation_seconds",
	"override_max_video_generation_count",
	"max_video_generation_count",
}

type GroupModelConfig struct {
	GroupID string `gorm:"primaryKey"         json:"group_id"`
	Group   *Group `gorm:"foreignKey:GroupID" json:"-"`
	Model   string `gorm:"primaryKey"         json:"model"`

	OverrideLimit bool  `json:"override_limit"`
	RPM           int64 `json:"rpm"`
	TPM           int64 `json:"tpm"`

	OverridePrice bool  `json:"override_price"`
	Price         Price `json:"price,omitempty" gorm:"embedded"`

	OverrideRetryTimes bool  `json:"override_retry_times"`
	RetryTimes         int64 `json:"retry_times"`

	OverrideRetryBudget bool  `json:"override_retry_budget"`
	RetryBudget         int64 `json:"retry_budget"          binding:"gte=0,lte=180" minimum:"0" maximum:"180"`

	OverrideTimeoutConfig bool          `json:"override_timeout_config"`
	TimeoutConfig         TimeoutConfig `json:"timeout_config,omitempty" gorm:"embedded"`

	OverrideForceSaveDetail bool `json:"override_force_save_detail"`
	ForceSaveDetail         bool `json:"force_save_detail"`

	OverrideMaxImageGenerationCount bool `json:"override_max_image_generation_count"`
	MaxImageGenerationCount         int  `json:"max_image_generation_count"`

	OverrideMaxVideoGenerationSeconds bool `json:"override_max_video_generation_seconds"`
	MaxVideoGenerationSeconds         int  `json:"max_video_generation_seconds"`

	OverrideMaxVideoGenerationCount bool `json:"override_max_video_generation_count"`
	MaxVideoGenerationCount         int  `json:"max_video_generation_count"`

	OverrideRequestBodyStorageMaxSize bool  `json:"override_request_body_storage_max_size"`
	RequestBodyStorageMaxSize         int64 `json:"request_body_storage_max_size"`

	OverrideResponseBodyStorageMaxSize bool  `json:"override_response_body_storage_max_size"`
	ResponseBodyStorageMaxSize         int64 `json:"response_body_storage_max_size"`

	OverrideSummaryServiceTier bool `json:"override_summary_service_tier"`
	SummaryServiceTier         bool `json:"summary_service_tier"`

	OverrideSummaryClaudeLongContext bool `json:"override_summary_claude_long_context"`
	SummaryClaudeLongContext         bool `json:"summary_claude_long_context"`
}

func (g *GroupModelConfig) BeforeSave(_ *gorm.DB) (err error) {
	if g.Model == "" {
		return errors.New("model is required")
	}

	if g.RetryBudget < 0 || g.RetryBudget > config.MaxRetryBudgetSeconds {
		return fmt.Errorf(
			"retry_budget must be between 0 and %d seconds",
			config.MaxRetryBudgetSeconds,
		)
	}

	if err := g.Price.ValidateConditionalPrices(); err != nil {
		return err
	}

	return nil
}

func SaveGroupModelConfig(groupModelConfig GroupModelConfig) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeleteGroup(groupModelConfig.GroupID); err != nil {
				log.Error("cache delete group failed: " + err.Error())
			}
		}
	}()

	return DB.Save(&groupModelConfig).Error
}

func UpdateGroupModelConfig(groupModelConfig GroupModelConfig) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeleteGroup(groupModelConfig.GroupID); err != nil {
				log.Error("cache delete group failed: " + err.Error())
			}
		}
	}()

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := HandleNotFound(
			tx.Model(&groupModelConfig).Updates(groupModelConfig).Error,
			GroupModelConfigCacheKey,
		); err != nil {
			return err
		}

		return tx.Model(&groupModelConfig).
			Select(groupModelConfigZeroValueUpdateFields).
			Updates(groupModelConfig).
			Error
	})
}

func SaveGroupModelConfigs(groupID string, groupModelConfigs []GroupModelConfig) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeleteGroup(groupID); err != nil {
				log.Error("cache delete group failed: " + err.Error())
			}
		}
	}()

	return DB.Transaction(func(tx *gorm.DB) error {
		for _, groupModelConfig := range groupModelConfigs {
			groupModelConfig.GroupID = groupID
			if err := tx.Save(&groupModelConfig).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func UpdateGroupModelConfigs(groupID string, groupModelConfigs []GroupModelConfig) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeleteGroup(groupID); err != nil {
				log.Error("cache delete group failed: " + err.Error())
			}
		}
	}()

	return DB.Transaction(func(tx *gorm.DB) error {
		for _, groupModelConfig := range groupModelConfigs {
			groupModelConfig.GroupID = groupID
			if err := HandleNotFound(
				tx.Model(&groupModelConfig).Updates(groupModelConfig).Error,
				GroupModelConfigCacheKey,
			); err != nil {
				return err
			}

			if err := tx.Model(&groupModelConfig).
				Select(groupModelConfigZeroValueUpdateFields).
				Updates(groupModelConfig).
				Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func DeleteGroupModelConfig(groupID, model string) error {
	err := DB.
		Where("group_id = ? AND model = ?", groupID, model).
		Delete(&GroupModelConfig{}).
		Error

	return HandleNotFound(err, GroupModelConfigCacheKey)
}

func DeleteGroupModelConfigs(groupID string, models []string) error {
	return DB.Where("group_id = ? AND model IN ?", groupID, models).
		Delete(&GroupModelConfig{}).
		Error
}

func GetGroupModelConfigs(groupID string) ([]GroupModelConfig, error) {
	var groupModelConfigs []GroupModelConfig

	err := DB.Where("group_id = ?", groupID).Find(&groupModelConfigs).Error
	return groupModelConfigs, HandleNotFound(err, GroupModelConfigCacheKey)
}

func GetGroupModelConfig(groupID, model string) (*GroupModelConfig, error) {
	var groupModelConfig GroupModelConfig

	err := DB.Where("group_id = ? AND model = ?", groupID, model).First(&groupModelConfig).Error
	return &groupModelConfig, HandleNotFound(err, GroupModelConfigCacheKey)
}
