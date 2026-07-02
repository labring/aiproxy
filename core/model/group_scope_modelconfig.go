package model

import (
	"errors"
	"slices"

	"github.com/labring/aiproxy/core/common"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ErrGroupScopeModelConfigNotFound = "group_scope_model_config"

type GroupScopeModelConfig struct {
	ModelConfig
	GroupID string `gorm:"size:64;primaryKey" json:"group_id" yaml:"group_id,omitempty"`
	Group   *Group `gorm:"foreignKey:GroupID" json:"-"        yaml:"-"`
}

func (c *GroupScopeModelConfig) BeforeSave(_ *gorm.DB) error {
	if c.GroupID == "" {
		return errors.New("group id is required")
	}
	return c.ModelConfig.BeforeSave(nil)
}

func (c *GroupScopeModelConfig) ToModelConfig() ModelConfig {
	return c.ModelConfig
}

func SaveGroupScopeModelConfig(config GroupScopeModelConfig) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeleteGroupScopeModelConfig(config.GroupID); err != nil {
				log.Error("cache delete group scope model config failed: " + err.Error())
			}
		}
	}()

	return DB.Save(&config).Error
}

func SaveGroupScopeModelConfigs(group string, configs []GroupScopeModelConfig) (err error) {
	if group == "" {
		return errors.New("group id is required")
	}

	defer func() {
		if err == nil {
			if err := CacheDeleteGroupScopeModelConfig(group); err != nil {
				log.Error("cache delete group scope model config failed: " + err.Error())
			}
		}
	}()

	for i := range configs {
		configs[i].GroupID = group
	}

	return DB.Save(&configs).Error
}

func GetGroupScopeModelConfig(group, modelName string) (GroupScopeModelConfig, error) {
	var config GroupScopeModelConfig

	err := DB.Where("group_id = ? AND model = ?", group, modelName).First(&config).Error
	return config, HandleNotFound(err, ErrGroupScopeModelConfigNotFound)
}

func GetAllGroupScopeModelConfigs(group string) ([]GroupScopeModelConfig, error) {
	var configs []GroupScopeModelConfig

	err := DB.Where("group_id = ?", group).Find(&configs).Error
	return configs, err
}

func GetGroupScopeModelConfigs(
	group string,
	page, perPage int,
	modelName string,
) (configs []GroupScopeModelConfig, total int64, err error) {
	tx := DB.Model(&GroupScopeModelConfig{}).Where("group_id = ?", group)
	if modelName != "" {
		tx = tx.Where("model = ?", modelName)
	}

	if err = tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total <= 0 {
		return nil, 0, nil
	}

	limit, offset := toLimitOffset(page, perPage)
	err = tx.Order("model asc").Limit(limit).Offset(offset).Find(&configs).Error

	return configs, total, err
}

func GetGroupScopeModelConfigsByModels(
	group string,
	models []string,
) ([]GroupScopeModelConfig, error) {
	if len(models) == 0 {
		return nil, nil
	}

	var configs []GroupScopeModelConfig

	err := DB.Where("group_id = ? AND model IN ?", group, models).Find(&configs).Error

	return configs, err
}

func SearchGroupScopeModelConfigs(
	group, keyword, modelName string,
	page, perPage int,
	owner ModelOwner,
) (configs []GroupScopeModelConfig, total int64, err error) {
	tx := DB.Model(&GroupScopeModelConfig{}).Where("group_id = ?", group)
	if modelName != "" {
		tx = tx.Where("model = ?", modelName)
	}

	if owner != "" {
		tx = tx.Where("owner = ?", owner)
	}

	if keyword != "" {
		if !common.UsingSQLite {
			tx = tx.Where("model ILIKE ?", "%"+keyword+"%")
		} else {
			tx = tx.Where("model LIKE ?", "%"+keyword+"%")
		}
	}

	if err = tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total <= 0 {
		return nil, 0, nil
	}

	limit, offset := toLimitOffset(page, perPage)
	err = tx.Order("model asc").Limit(limit).Offset(offset).Find(&configs).Error

	return configs, total, err
}

func DeleteGroupScopeModelConfig(group, modelName string) error {
	result := DB.Where("group_id = ? AND model = ?", group, modelName).
		Delete(&GroupScopeModelConfig{})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		if err := CacheDeleteGroupScopeModelConfig(group); err != nil {
			log.Error("cache delete group scope model config failed: " + err.Error())
		}
	}

	return HandleUpdateResult(result, ErrGroupScopeModelConfigNotFound)
}

func DeleteGroupScopeModelConfigsByModels(group string, models []string) error {
	if len(models) == 0 {
		return nil
	}

	result := DB.Where("group_id = ? AND model IN ?", group, models).
		Delete(&GroupScopeModelConfig{})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		if err := CacheDeleteGroupScopeModelConfig(group); err != nil {
			log.Error("cache delete group scope model config failed: " + err.Error())
		}
	}

	return nil
}

func UpsertGroupScopeModelConfigs(group string, configs []GroupScopeModelConfig) (err error) {
	if group == "" {
		return errors.New("group id is required")
	}

	if len(configs) == 0 {
		return nil
	}

	defer func() {
		if err == nil {
			if err := CacheDeleteGroupScopeModelConfig(group); err != nil {
				log.Error("cache delete group scope model config failed: " + err.Error())
			}
		}
	}()

	for i := range configs {
		configs[i].GroupID = group
	}

	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "group_id"},
			{Name: "model"},
		},
		UpdateAll: true,
	}).Create(&configs).Error
}

func groupScopeModelConfigModels(configs []GroupScopeModelConfig) []string {
	models := make([]string, 0, len(configs))
	for _, config := range configs {
		models = append(models, config.Model)
	}

	slices.Sort(models)

	return models
}
