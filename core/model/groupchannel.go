package model

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/monitor"
	"github.com/labring/aiproxy/core/relay/mode"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ErrGroupChannelNotFound = "group_channel"

type ChannelScope string

const (
	ChannelScopeGlobal ChannelScope = "global"
	ChannelScopeGroup  ChannelScope = "group"
)

func ParseChannelScope(value string) ChannelScope {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(ChannelScopeGlobal):
		return ChannelScopeGlobal
	case string(ChannelScopeGroup), "own", "group-only", "group_only":
		return ChannelScopeGroup
	default:
		return ""
	}
}

func NormalizeChannelScope(scope ChannelScope) ChannelScope {
	if scope == "" {
		return ChannelScopeGlobal
	}

	return scope
}

func ValidTokenChannelScope(scope ChannelScope) bool {
	return scope == "" || scope == ChannelScopeGlobal || scope == ChannelScopeGroup
}

func GroupChannelMonitorKey(group string, id int) string {
	return fmt.Sprintf("group_channel:%s:%d", group, id)
}

func GroupChannelMonitorPrefix(group string) string {
	return fmt.Sprintf("group_channel:%s:", group)
}

type GroupChannel struct {
	DeletedAt              gorm.DeletedAt      `gorm:"index"                                   json:"-"                             yaml:"-"`
	CreatedAt              time.Time           `gorm:"index"                                   json:"created_at"                    yaml:"-"`
	LastTestErrorAt        time.Time           `                                               json:"last_test_error_at"            yaml:"-"`
	GroupChannelTests      []*GroupChannelTest `gorm:"foreignKey:GroupChannelID;references:ID" json:"group_channel_tests,omitempty" yaml:"-"`
	ModelMapping           map[string]string   `gorm:"serializer:fastjson;type:text"           json:"model_mapping"                 yaml:"model_mapping,omitempty"`
	Key                    string              `gorm:"type:text;index:,length:191"             json:"key"                           yaml:"key,omitempty"`
	GroupID                string              `gorm:"size:64;not null;index"                  json:"group_id"                      yaml:"group_id,omitempty"`
	Name                   string              `gorm:"size:64;index"                           json:"name"                          yaml:"name,omitempty"`
	BaseURL                string              `gorm:"size:128;index"                          json:"base_url"                      yaml:"base_url,omitempty"`
	ProxyURL               string              `gorm:"size:255"                                json:"proxy_url"                     yaml:"proxy_url,omitempty"`
	Models                 []string            `gorm:"serializer:fastjson;type:text"           json:"models"                        yaml:"models,omitempty"`
	ID                     int                 `gorm:"primaryKey"                              json:"id"                            yaml:"id,omitempty"`
	UsedAmount             float64             `gorm:"index"                                   json:"used_amount"                   yaml:"-"`
	RequestCount           int                 `gorm:"index"                                   json:"request_count"                 yaml:"-"`
	RetryCount             int                 `gorm:"index"                                   json:"retry_count"                   yaml:"-"`
	Status                 int                 `gorm:"default:1;index"                         json:"status"                        yaml:"status,omitempty"`
	Type                   ChannelType         `gorm:"default:0;index"                         json:"type"                          yaml:"type,omitempty"`
	Priority               int32               `                                               json:"priority"                      yaml:"priority,omitempty"`
	SkipTLSVerify          bool                `                                               json:"skip_tls_verify"               yaml:"skip_tls_verify,omitempty"`
	EnabledNoPermissionBan bool                `                                               json:"enabled_no_permission_ban"     yaml:"enabled_no_permission_ban,omitempty"`
	MaxErrorRate           float64             `                                               json:"max_error_rate"                yaml:"max_error_rate,omitempty"`
	Configs                ChannelConfigs      `gorm:"serializer:fastjson;type:text"           json:"configs,omitempty"             yaml:"configs,omitempty"`
	Sets                   []string            `gorm:"serializer:fastjson;type:text"           json:"sets,omitempty"                yaml:"sets,omitempty"`
}

func (c *GroupChannel) GetSets() []string {
	return NormalizeAvailableSets(c.Sets)
}

func GroupChannelAccessModels(channel *GroupChannel) []string {
	if channel == nil {
		return nil
	}

	models := cloneStringSlice(channel.Models)
	for publicModel := range channel.ModelMapping {
		models = append(models, publicModel)
	}

	return models
}

func GroupChannelSupportsModel(channel *GroupChannel, modelName string) bool {
	return slices.ContainsFunc(GroupChannelAccessModels(channel), func(item string) bool {
		return strings.EqualFold(item, modelName)
	})
}

func (c *GroupChannel) GetPriority() int32 {
	if c.Priority == 0 {
		return DefaultPriority
	}

	if c.Priority > MaxPriority {
		return MaxPriority
	}

	return c.Priority
}

func (c *GroupChannel) BeforeCreate(_ *gorm.DB) error {
	if c.GroupID == "" {
		return errors.New("group id is required")
	}
	return nil
}

func (c *GroupChannel) BeforeUpdate(tx *gorm.DB) error {
	if tx.Statement.Changed("GroupID") && c.GroupID == "" {
		return errors.New("group id is required")
	}
	return nil
}

func (c *GroupChannel) BeforeDelete(tx *gorm.DB) error {
	return tx.Model(&GroupChannelTest{}).
		Where("group_channel_id = ?", c.ID).
		Delete(&GroupChannelTest{}).
		Error
}

func (c *GroupChannel) MarshalJSON() ([]byte, error) {
	type Alias GroupChannel

	return sonic.Marshal(&struct {
		*Alias
		CreatedAt       int64 `json:"created_at"`
		LastTestErrorAt int64 `json:"last_test_error_at"`
	}{
		Alias:           (*Alias)(c),
		CreatedAt:       c.CreatedAt.UnixMilli(),
		LastTestErrorAt: c.LastTestErrorAt.UnixMilli(),
	})
}

func (c *GroupChannel) ToChannel() *Channel {
	if c == nil {
		return nil
	}

	return &Channel{
		DeletedAt:              c.DeletedAt,
		CreatedAt:              c.CreatedAt,
		LastTestErrorAt:        c.LastTestErrorAt,
		ModelMapping:           cloneStringStringMap(c.ModelMapping),
		Key:                    c.Key,
		Name:                   c.Name,
		BaseURL:                c.BaseURL,
		ProxyURL:               c.ProxyURL,
		Models:                 cloneStringSlice(c.Models),
		ID:                     c.ID,
		UsedAmount:             c.UsedAmount,
		RequestCount:           c.RequestCount,
		RetryCount:             c.RetryCount,
		Status:                 c.Status,
		Type:                   c.Type,
		Priority:               c.Priority,
		SkipTLSVerify:          c.SkipTLSVerify,
		EnabledNoPermissionBan: c.EnabledNoPermissionBan,
		MaxErrorRate:           c.MaxErrorRate,
		Configs:                cloneChannelConfigs(c.Configs),
		Sets:                   cloneStringSlice(c.Sets),
	}
}

func initializeGroupChannelModels(channel *GroupChannel) {
	if len(channel.Models) == 0 {
		return
	}

	models := cloneStringSlice(channel.Models)
	slices.Sort(models)
	models = slices.Compact(models)
	channel.Models = models
}

func prepareGroupChannel(channel *GroupChannel) {
	initializeGroupChannelModels(channel)
}

func getGroupChannelOrder(order string) string {
	prefix, suffix, _ := strings.Cut(order, "-")
	switch prefix {
	case "name",
		"type",
		"created_at",
		"status",
		"used_amount",
		"request_count",
		"priority",
		"id":
		switch suffix {
		case "asc":
			return prefix + " asc"
		default:
			return prefix + " desc"
		}
	default:
		return "id desc"
	}
}

func buildGroupChannelsQuery(
	group string,
	id int,
	name, key string,
	channelType int,
	baseURL string,
) *gorm.DB {
	tx := DB.Model(&GroupChannel{})
	if group != "" {
		tx = tx.Where("group_id = ?", group)
	}

	if id != 0 {
		tx = tx.Where("id = ?", id)
	}

	if name != "" {
		tx = tx.Where("name = ?", name)
	}

	if key != "" {
		tx = tx.Where("key = ?", key)
	}

	if channelType != 0 {
		tx = tx.Where("type = ?", channelType)
	}

	if baseURL != "" {
		tx = tx.Where("base_url = ?", baseURL)
	}

	return tx
}

func GetGlobalGroupChannels(
	group string,
	page, perPage, id int,
	name, key string,
	channelType int,
	baseURL, order string,
) (channels []*GroupChannel, total int64, err error) {
	tx := buildGroupChannelsQuery(group, id, name, key, channelType, baseURL)

	err = tx.Count(&total).Error
	if err != nil || total <= 0 {
		return channels, total, err
	}

	limit, offset := toLimitOffset(page, perPage)
	err = tx.Order(getGroupChannelOrder(order)).Limit(limit).Offset(offset).Find(&channels).Error

	return channels, total, err
}

func GetGroupChannels(
	group string,
	page, perPage, id int,
	name, key string,
	channelType int,
	baseURL, order string,
) (channels []*GroupChannel, total int64, err error) {
	if group == "" {
		return nil, 0, errors.New("group id is required")
	}

	return GetGlobalGroupChannels(
		group,
		page,
		perPage,
		id,
		name,
		key,
		channelType,
		baseURL,
		order,
	)
}

func SearchGlobalGroupChannels(
	group, keyword string,
	page, perPage, id int,
	name, key string,
	channelType int,
	baseURL, order string,
) (channels []*GroupChannel, total int64, err error) {
	tx := buildGroupChannelsQuery(group, id, name, key, channelType, baseURL)
	if keyword != "" {
		var (
			conditions []string
			values     []any
		)

		keywordInt := String2Int(keyword)
		if keywordInt != 0 && id == 0 {
			conditions = append(conditions, "id = ?")
			values = append(values, keywordInt)
		}

		if name == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "name ILIKE ?")
			} else {
				conditions = append(conditions, "name LIKE ?")
			}

			values = append(values, "%"+keyword+"%")
		}

		if key == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "key ILIKE ?")
			} else {
				conditions = append(conditions, "key LIKE ?")
			}

			values = append(values, "%"+keyword+"%")
		}

		if baseURL == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "base_url ILIKE ?")
			} else {
				conditions = append(conditions, "base_url LIKE ?")
			}

			values = append(values, "%"+keyword+"%")
		}

		if !common.UsingSQLite {
			conditions = append(conditions, "models ILIKE ?", "sets ILIKE ?")
		} else {
			conditions = append(conditions, "models LIKE ?", "sets LIKE ?")
		}

		values = append(values, "%"+keyword+"%", "%"+keyword+"%")
		if len(conditions) > 0 {
			tx = tx.Where(fmt.Sprintf("(%s)", strings.Join(conditions, " OR ")), values...)
		}
	}

	err = tx.Count(&total).Error
	if err != nil || total <= 0 {
		return channels, total, err
	}

	limit, offset := toLimitOffset(page, perPage)
	err = tx.Order(getGroupChannelOrder(order)).Limit(limit).Offset(offset).Find(&channels).Error

	return channels, total, err
}

func SearchGroupChannels(
	group, keyword string,
	page, perPage, id int,
	name, key string,
	channelType int,
	baseURL, order string,
) (channels []*GroupChannel, total int64, err error) {
	if group == "" {
		return nil, 0, errors.New("group id is required")
	}

	return SearchGlobalGroupChannels(
		group,
		keyword,
		page,
		perPage,
		id,
		name,
		key,
		channelType,
		baseURL,
		order,
	)
}

func GetAllGlobalGroupChannels(group string) ([]*GroupChannel, error) {
	var channels []*GroupChannel

	tx := DB.Model(&GroupChannel{})
	if group != "" {
		tx = tx.Where("group_id = ?", group)
	}

	err := tx.Order("id desc").Find(&channels).Error

	return channels, err
}

func GetAllGroupChannels(group string) ([]*GroupChannel, error) {
	if group == "" {
		return nil, errors.New("group id is required")
	}

	return GetAllGlobalGroupChannels(group)
}

func prepareGroupChannels(channels []*GroupChannel) {
	for _, channel := range channels {
		prepareGroupChannel(channel)
	}
}

func LoadGlobalGroupChannels(group string) ([]*GroupChannel, error) {
	channels, err := GetAllGlobalGroupChannels(group)
	if err != nil {
		return nil, err
	}

	prepareGroupChannels(channels)

	return channels, nil
}

func LoadGroupChannels(group string) ([]*GroupChannel, error) {
	channels, err := GetAllGroupChannels(group)
	if err != nil {
		return nil, err
	}

	prepareGroupChannels(channels)

	return channels, nil
}

func LoadEnabledGlobalGroupChannels(group string) ([]*GroupChannel, error) {
	var channels []*GroupChannel

	tx := DB.Where("status = ?", ChannelStatusEnabled)
	if group != "" {
		tx = tx.Where("group_id = ?", group)
	}

	err := tx.Find(&channels).Error
	if err != nil {
		return nil, err
	}

	prepareGroupChannels(channels)

	return channels, nil
}

func LoadEnabledGroupChannels(group string) ([]*GroupChannel, error) {
	if group == "" {
		return nil, errors.New("group id is required")
	}

	return LoadEnabledGlobalGroupChannels(group)
}

func GetGroupChannelByID(group string, id int) (*GroupChannel, error) {
	if group == "" {
		return nil, errors.New("group id is required")
	}

	channel := GroupChannel{}
	err := DB.Where("group_id = ? AND id = ?", group, id).First(&channel).Error

	return &channel, HandleNotFound(err, ErrGroupChannelNotFound)
}

func GetGlobalGroupChannelByID(id int) (*GroupChannel, error) {
	channel := GroupChannel{}
	err := DB.Where("id = ?", id).First(&channel).Error
	return &channel, HandleNotFound(err, ErrGroupChannelNotFound)
}

func LoadGroupChannelByID(group string, id int) (*GroupChannel, error) {
	channel, err := GetGroupChannelByID(group, id)
	if err != nil {
		return nil, err
	}

	prepareGroupChannel(channel)

	return channel, nil
}

func LoadGlobalGroupChannelByID(id int) (*GroupChannel, error) {
	channel, err := GetGlobalGroupChannelByID(id)
	if err != nil {
		return nil, err
	}

	prepareGroupChannel(channel)

	return channel, nil
}

type GroupChannelBasicInfo struct {
	GroupID string      `json:"group_id"`
	Name    string      `json:"name"`
	ID      int         `json:"id"`
	Type    ChannelType `json:"type"`
}

func GetGlobalGroupChannelsBasicInfoByIDs(ids []int) ([]GroupChannelBasicInfo, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var channels []GroupChannelBasicInfo

	err := DB.Model(&GroupChannel{}).
		Where("id IN ?", ids).
		Select("id", "group_id", "name", "type").
		Order("id desc").
		Find(&channels).
		Error

	return channels, err
}

func GetGroupChannelsBasicInfoByIDs(
	group string,
	ids []int,
) ([]GroupChannelBasicInfo, error) {
	if group == "" {
		return nil, errors.New("group id is required")
	}

	if len(ids) == 0 {
		return nil, nil
	}

	var channels []GroupChannelBasicInfo

	err := DB.Model(&GroupChannel{}).
		Where("group_id = ? AND id IN ?", group, ids).
		Select("id", "group_id", "name", "type").
		Order("id desc").
		Find(&channels).
		Error

	return channels, err
}

func GetGroupChannelTests(group string, id int) ([]*GroupChannelTest, error) {
	if group == "" {
		return nil, errors.New("group id is required")
	}

	var tests []*GroupChannelTest

	err := DB.
		Where("group_id = ? AND group_channel_id = ?", group, id).
		Order("test_at desc").
		Find(&tests).
		Error

	return tests, err
}

func BatchInsertGroupChannels(channels []*GroupChannel) (err error) {
	defer func() {
		if err == nil {
			groups := map[string]struct{}{}
			for _, channel := range channels {
				groups[channel.GroupID] = struct{}{}
			}

			for group := range groups {
				if err := CacheDeleteGroupChannels(group); err != nil {
					log.Error("cache delete group channels failed: " + err.Error())
				}
			}
		}
	}()

	return DB.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&channels).Error
	})
}

func UpdateGroupChannel(channel *GroupChannel) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeleteGroupChannels(channel.GroupID); err != nil {
				log.Error("cache delete group channels failed: " + err.Error())
			}

			_ = monitor.ClearGroupChannelAllModelErrorsByKey(
				context.Background(),
				GroupChannelMonitorKey(channel.GroupID, channel.ID),
			)
		}
	}()

	selects := []string{
		"model_mapping",
		"key",
		"base_url",
		"proxy_url",
		"models",
		"priority",
		"configs",
		"skip_tls_verify",
		"enabled_no_permission_ban",
		"max_error_rate",
		"sets",
	}
	if channel.Type != 0 {
		selects = append(selects, "type")
	}

	if channel.Name != "" {
		selects = append(selects, "name")
	}

	if channel.Status != 0 {
		selects = append(selects, "status")
	}

	result := DB.
		Select(selects).
		Clauses(clause.Returning{}).
		Where("group_id = ? AND id = ?", channel.GroupID, channel.ID).
		Updates(channel)

	return HandleUpdateResult(result, ErrGroupChannelNotFound)
}

func UpdateGlobalGroupChannel(channel *GroupChannel) error {
	if channel.ID == 0 {
		return errors.New("group channel id is required")
	}

	if channel.GroupID == "" {
		existing, err := GetGlobalGroupChannelByID(channel.ID)
		if err != nil {
			return err
		}

		channel.GroupID = existing.GroupID
	}

	return UpdateGroupChannel(channel)
}

func DeleteGroupChannelByID(group string, id int) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeleteGroupChannels(group); err != nil {
				log.Error("cache delete group channels failed: " + err.Error())
			}

			_ = monitor.ClearGroupChannelAllModelErrorsByKey(
				context.Background(),
				GroupChannelMonitorKey(group, id),
			)
		}
	}()

	result := DB.Where("group_id = ?", group).Delete(&GroupChannel{ID: id})

	return HandleUpdateResult(result, ErrGroupChannelNotFound)
}

func DeleteGlobalGroupChannelByID(id int) error {
	channel, err := GetGlobalGroupChannelByID(id)
	if err != nil {
		return err
	}

	return DeleteGroupChannelByID(channel.GroupID, id)
}

func DeleteGroupChannelsByIDs(group string, ids []int) (err error) {
	if len(ids) == 0 {
		return nil
	}

	defer func() {
		if err == nil {
			if err := CacheDeleteGroupChannels(group); err != nil {
				log.Error("cache delete group channels failed: " + err.Error())
			}

			for _, id := range ids {
				_ = monitor.ClearGroupChannelAllModelErrorsByKey(
					context.Background(),
					GroupChannelMonitorKey(group, id),
				)
			}
		}
	}()

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("group_id = ? AND group_channel_id IN ?", group, ids).
			Delete(&GroupChannelTest{}).Error; err != nil {
			return err
		}

		return tx.Where("group_id = ? AND id IN ?", group, ids).Delete(&GroupChannel{}).Error
	})
}

func DeleteGlobalGroupChannelsByIDs(ids []int) (err error) {
	if len(ids) == 0 {
		return nil
	}

	channels, err := GetGlobalGroupChannelsBasicInfoByIDs(ids)
	if err != nil {
		return err
	}

	idsByGroup := make(map[string][]int, len(channels))
	for _, channel := range channels {
		idsByGroup[channel.GroupID] = append(idsByGroup[channel.GroupID], channel.ID)
	}

	defer func() {
		if err != nil {
			return
		}

		for group, groupIDs := range idsByGroup {
			if err := CacheDeleteGroupChannels(group); err != nil {
				log.Error("cache delete group channels failed: " + err.Error())
			}

			for _, id := range groupIDs {
				_ = monitor.ClearGroupChannelAllModelErrorsByKey(
					context.Background(),
					GroupChannelMonitorKey(group, id),
				)
			}
		}
	}()

	return DB.Transaction(func(tx *gorm.DB) error {
		for group, groupIDs := range idsByGroup {
			if err := tx.
				Where("group_id = ? AND group_channel_id IN ?", group, groupIDs).
				Delete(&GroupChannelTest{}).Error; err != nil {
				return err
			}

			if err := tx.Where("group_id = ? AND id IN ?", group, groupIDs).
				Delete(&GroupChannel{}).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func UpdateGroupChannelStatusByID(group string, id, status int) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeleteGroupChannels(group); err != nil {
				log.Error("cache delete group channels failed: " + err.Error())
			}

			_ = monitor.ClearGroupChannelAllModelErrorsByKey(
				context.Background(),
				GroupChannelMonitorKey(group, id),
			)
		}
	}()

	result := DB.Model(&GroupChannel{}).
		Where("group_id = ? AND id = ?", group, id).
		Update("status", status)

	return HandleUpdateResult(result, ErrGroupChannelNotFound)
}

func UpdateGlobalGroupChannelStatusByID(id, status int) error {
	channel, err := GetGlobalGroupChannelByID(id)
	if err != nil {
		return err
	}

	return UpdateGroupChannelStatusByID(channel.GroupID, id, status)
}

func UpdateGroupChannelUsedAmount(
	group string,
	id int,
	amount float64,
	requestCount, retryCount int,
) (err error) {
	defer func() {
		if err == nil {
			if cacheErr := CacheDeleteGroupChannels(group); cacheErr != nil {
				log.Error("cache delete group channels failed: " + cacheErr.Error())
			}
		}
	}()

	result := DB.Model(&GroupChannel{}).
		Where("group_id = ? AND id = ?", group, id).
		Updates(func() map[string]any {
			updates := map[string]any{
				"request_count": gorm.Expr("request_count + ?", requestCount),
				"retry_count":   gorm.Expr("retry_count + ?", retryCount),
			}
			if amount > 0 {
				updates["used_amount"] = gorm.Expr("used_amount + ?", amount)
			}

			return updates
		}())

	return HandleUpdateResult(result, ErrGroupChannelNotFound)
}

func ClearGroupChannelLastTestErrorAt(group string, id int) error {
	result := DB.Model(&GroupChannel{}).
		Where("group_id = ? AND id = ?", group, id).
		Update("last_test_error_at", gorm.Expr("NULL"))
	return HandleUpdateResult(result, ErrGroupChannelNotFound)
}

func (c *GroupChannel) UpdateModelTest(
	testAt time.Time,
	model, actualModel string,
	mode mode.Mode,
	took float64,
	success bool,
	response string,
	code int,
) (*GroupChannelTest, error) {
	var ct *GroupChannelTest

	err := DB.Transaction(func(tx *gorm.DB) error {
		if !success {
			result := tx.Model(&GroupChannel{}).
				Where("group_id = ? AND id = ?", c.GroupID, c.ID).
				Update("last_test_error_at", testAt)
			if err := HandleUpdateResult(result, ErrGroupChannelNotFound); err != nil {
				return err
			}
		} else if !c.LastTestErrorAt.IsZero() && time.Since(c.LastTestErrorAt) > time.Hour {
			result := tx.Model(&GroupChannel{}).
				Where("group_id = ? AND id = ?", c.GroupID, c.ID).
				Update("last_test_error_at", gorm.Expr("NULL"))
			if err := HandleUpdateResult(result, ErrGroupChannelNotFound); err != nil {
				return err
			}
		}

		ct = &GroupChannelTest{
			GroupID:        c.GroupID,
			GroupChannelID: c.ID,
			ChannelType:    c.Type,
			ChannelName:    c.Name,
			Model:          model,
			ActualModel:    actualModel,
			Mode:           mode,
			TestAt:         testAt,
			Took:           took,
			Success:        success,
			Response:       response,
			Code:           code,
		}
		result := tx.Save(ct)

		return HandleUpdateResult(result, ErrGroupChannelNotFound)
	})
	if err != nil {
		return nil, err
	}

	if err := CacheDeleteGroupChannels(c.GroupID); err != nil {
		log.Error("cache delete group channels failed: " + err.Error())
	}

	return ct, nil
}
