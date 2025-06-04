package model

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	GroupModelConfigCacheKey = "group_model_config"
)

type GroupModelConfig struct {
	GroupID string `gorm:"primaryKey"         json:"group_id"`
	Group   *Group `gorm:"foreignKey:GroupID" json:"-"`
	Model   string `gorm:"primaryKey"         json:"model"`

	OverrideLimit bool  `json:"override_limit"`
	RPM           int64 `json:"rpm"`
	TPM           int64 `json:"tpm"`

	OverridePrice bool               `json:"override_price"`
	ImagePrices   map[string]float64 `json:"image_prices,omitempty" gorm:"serializer:fastjson;type:text"`
	Price         Price              `json:"price,omitempty"        gorm:"embedded"`

	OverrideRetryTimes bool  `json:"override_retry_times"`
	RetryTimes         int64 `json:"retry_times"`
}

func (g *GroupModelConfig) BeforeSave(_ *gorm.DB) (err error) {
	if g.Model == "" {
		return errors.New("model is required")
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
	return HandleNotFound(
		DB.Model(&groupModelConfig).Updates(groupModelConfig).Error,
		GroupModelConfigCacheKey,
	)
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
