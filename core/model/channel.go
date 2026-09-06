package model

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/monitor"
	"github.com/labring/aiproxy/core/relay/mode"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ErrChannelNotFound = "channel"
)

const (
	ChannelStatusUnknown  = 0
	ChannelStatusEnabled  = 1
	ChannelStatusDisabled = 2
)

const (
	ChannelDefaultSet = "default"
)

type Channel struct {
	DeletedAt               gorm.DeletedAt    `gorm:"index;index:idx_channels_backup_active_id,priority:2"         json:"-"                          yaml:"-"`
	CreatedAt               time.Time         `gorm:"index"                                                        json:"created_at"                 yaml:"-"`
	LastTestErrorAt         time.Time         `                                                                    json:"last_test_error_at"         yaml:"-"`
	ChannelTests            []*ChannelTest    `gorm:"foreignKey:ChannelID;references:ID"                           json:"channel_tests,omitempty"    yaml:"-"`
	BalanceUpdatedAt        time.Time         `                                                                    json:"balance_updated_at"         yaml:"-"`
	ModelMapping            map[string]string `gorm:"serializer:fastjson;type:text"                                json:"model_mapping"              yaml:"model_mapping,omitempty"`
	Key                     string            `gorm:"type:text;index:,length:191"                                  json:"key"                        yaml:"key,omitempty"`
	Name                    string            `gorm:"size:64;index"                                                json:"name"                       yaml:"name,omitempty"`
	Remark                  string            `gorm:"size:255;index"                                               json:"remark,omitempty"           yaml:"remark,omitempty"`
	BaseURL                 string            `gorm:"size:128;index"                                               json:"base_url"                   yaml:"base_url,omitempty"`
	ProxyURL                string            `gorm:"size:255"                                                     json:"proxy_url"                  yaml:"proxy_url,omitempty"`
	Models                  []string          `gorm:"serializer:fastjson;type:text"                                json:"models"                     yaml:"models,omitempty"`
	Balance                 float64           `                                                                    json:"balance"                    yaml:"balance,omitempty"`
	ID                      int               `gorm:"primaryKey;index:idx_channels_backup_active_id,priority:3"    json:"id"                         yaml:"id,omitempty"`
	UsedAmount              float64           `gorm:"index"                                                        json:"used_amount"                yaml:"-"`
	RequestCount            int               `gorm:"index"                                                        json:"request_count"              yaml:"-"`
	RetryCount              int               `gorm:"index"                                                        json:"retry_count"                yaml:"-"`
	Status                  int               `gorm:"default:1;index"                                              json:"status"                     yaml:"status,omitempty"`
	Type                    ChannelType       `gorm:"default:0;index"                                              json:"type"                       yaml:"type,omitempty"`
	Priority                int32             `                                                                    json:"priority"                   yaml:"priority,omitempty"`
	BackupOnly              bool              `gorm:"default:false;index:idx_channels_backup_active_id,priority:1" json:"backup_only"                yaml:"backup_only,omitempty"`
	EnabledAutoBalanceCheck bool              `                                                                    json:"enabled_auto_balance_check" yaml:"enabled_auto_balance_check,omitempty"`
	BalanceThreshold        float64           `                                                                    json:"balance_threshold"          yaml:"balance_threshold,omitempty"`
	SkipTLSVerify           bool              `                                                                    json:"skip_tls_verify"            yaml:"skip_tls_verify,omitempty"`
	EnabledNoPermissionBan  bool              `                                                                    json:"enabled_no_permission_ban"  yaml:"enabled_no_permission_ban,omitempty"`
	WarnErrorRate           float64           `                                                                    json:"warn_error_rate"            yaml:"warn_error_rate,omitempty"`
	MaxErrorRate            float64           `                                                                    json:"max_error_rate"             yaml:"max_error_rate,omitempty"`
	Configs                 ChannelConfigs    `gorm:"serializer:fastjson;type:text"                                json:"configs,omitempty"          yaml:"configs,omitempty"`
	Sets                    []string          `gorm:"serializer:fastjson;type:text"                                json:"sets,omitempty"             yaml:"sets,omitempty"`
}

func (c *Channel) GetSets() []string {
	if len(c.Sets) == 0 {
		return []string{ChannelDefaultSet}
	}
	return c.Sets
}

func (c *Channel) BeforeDelete(tx *gorm.DB) (err error) {
	return tx.Model(&ChannelTest{}).Where("channel_id = ?", c.ID).Delete(&ChannelTest{}).Error
}

func (c *Channel) GetBalanceThreshold() float64 {
	if c.BalanceThreshold < 0 {
		return 0
	}
	return c.BalanceThreshold
}

const (
	DefaultPriority = 10
	MaxPriority     = 1000000
)

func (c *Channel) GetPriority() int32 {
	if c.Priority == 0 {
		return DefaultPriority
	}

	if c.Priority > MaxPriority {
		return MaxPriority
	}

	return c.Priority
}

type ChannelConfigs map[string]any

func (c ChannelConfigs) LoadConfig(config any) error {
	if len(c) == 0 {
		return nil
	}

	v, err := sonic.Marshal(c)
	if err != nil {
		return err
	}

	return sonic.Unmarshal(v, config)
}

func GetModelConfigWithModels(models []string) ([]string, []string, error) {
	if len(models) == 0 || config.DisableModelConfig {
		return models, nil, nil
	}

	where := DB.Model(&ModelConfig{}).Where("model IN ?", models)

	var count int64
	if err := where.Count(&count).Error; err != nil {
		return nil, nil, err
	}

	if count == 0 {
		return nil, models, nil
	}

	if count == int64(len(models)) {
		return models, nil, nil
	}

	var foundModels []string
	if err := where.Pluck("model", &foundModels).Error; err != nil {
		return nil, nil, err
	}

	if len(foundModels) == len(models) {
		return models, nil, nil
	}

	foundModelsMap := make(map[string]struct{}, len(foundModels))
	for _, model := range foundModels {
		foundModelsMap[model] = struct{}{}
	}

	if len(models)-len(foundModels) > 0 {
		missingModels := make([]string, 0, len(models)-len(foundModels))
		for _, model := range models {
			if _, exists := foundModelsMap[model]; !exists {
				missingModels = append(missingModels, model)
			}
		}

		return foundModels, missingModels, nil
	}

	return foundModels, nil, nil
}

func CheckModelConfigExist(models []string) error {
	_, missingModels, err := GetModelConfigWithModels(models)
	if err != nil {
		return err
	}

	if len(missingModels) > 0 {
		slices.Sort(missingModels)
		return fmt.Errorf("model config not found: %v", missingModels)
	}

	return nil
}

func (c *Channel) MarshalJSON() ([]byte, error) {
	type Alias Channel

	return sonic.Marshal(&struct {
		*Alias
		CreatedAt        int64 `json:"created_at"`
		BalanceUpdatedAt int64 `json:"balance_updated_at"`
		LastTestErrorAt  int64 `json:"last_test_error_at"`
	}{
		Alias:            (*Alias)(c),
		CreatedAt:        c.CreatedAt.UnixMilli(),
		BalanceUpdatedAt: c.BalanceUpdatedAt.UnixMilli(),
		LastTestErrorAt:  c.LastTestErrorAt.UnixMilli(),
	})
}

//nolint:goconst
func getChannelOrder(order string) string {
	prefix, suffix, _ := strings.Cut(order, "-")
	switch prefix {
	case "name",
		"type",
		"created_at",
		"status",
		"test_at",
		"balance_updated_at",
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

func GetAllChannels() (channels []*Channel, err error) {
	tx := DB.Model(&Channel{})
	err = tx.Order("id desc").Find(&channels).Error
	return channels, err
}

type ChannelFilter struct {
	ID         int
	Name       string
	Key        string
	Type       int
	BaseURL    string
	Remark     *string
	BackupOnly *bool
}

func (f ChannelFilter) apply(tx *gorm.DB) *gorm.DB {
	if f.ID != 0 {
		tx = tx.Where("id = ?", f.ID)
	}

	if f.Name != "" {
		tx = tx.Where("name = ?", f.Name)
	}

	if f.Key != "" {
		tx = tx.Where("key = ?", f.Key)
	}

	if f.Type != 0 {
		tx = tx.Where("type = ?", f.Type)
	}

	if f.BaseURL != "" {
		tx = tx.Where("base_url = ?", f.BaseURL)
	}

	if f.Remark != nil {
		if *f.Remark == "" {
			tx = tx.Where("(remark = ? OR remark IS NULL)", "")
		} else {
			tx = tx.Where("remark = ?", *f.Remark)
		}
	}

	if f.BackupOnly != nil {
		tx = tx.Where("backup_only = ?", *f.BackupOnly)
	}

	return tx
}

func GetChannels(
	page, perPage int,
	filter ChannelFilter,
	order string,
) (channels []*Channel, total int64, err error) {
	tx := filter.apply(DB.Model(&Channel{}))

	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	if total <= 0 {
		return nil, 0, nil
	}

	limit, offset := toLimitOffset(page, perPage)
	err = tx.Order(getChannelOrder(order)).Limit(limit).Offset(offset).Find(&channels).Error

	return channels, total, err
}

func SearchChannels(
	keyword string,
	page, perPage int,
	filter ChannelFilter,
	order string,
) (channels []*Channel, total int64, err error) {
	tx := filter.apply(DB.Model(&Channel{}))

	// Handle keyword search for zero value fields
	if keyword != "" {
		var (
			conditions []string
			values     []any
		)

		keywordInt := String2Int(keyword)

		if keywordInt != 0 {
			if filter.ID == 0 {
				conditions = append(conditions, "id = ?")
				values = append(values, keywordInt)
			}
		}

		if filter.Name == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "name ILIKE ?")
			} else {
				conditions = append(conditions, "name LIKE ?")
			}

			values = append(values, "%"+keyword+"%")
		}

		if !common.UsingSQLite {
			conditions = append(conditions, "remark ILIKE ?")
		} else {
			conditions = append(conditions, "remark LIKE ?")
		}

		values = append(values, "%"+keyword+"%")

		if filter.Key == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "key ILIKE ?")
			} else {
				conditions = append(conditions, "key LIKE ?")
			}

			values = append(values, "%"+keyword+"%")
		}

		if filter.BaseURL == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "base_url ILIKE ?")
			} else {
				conditions = append(conditions, "base_url LIKE ?")
			}

			values = append(values, "%"+keyword+"%")
		}

		if !common.UsingSQLite {
			conditions = append(conditions, "models ILIKE ?")
		} else {
			conditions = append(conditions, "models LIKE ?")
		}

		values = append(values, "%"+keyword+"%")

		if !common.UsingSQLite {
			conditions = append(conditions, "sets ILIKE ?")
		} else {
			conditions = append(conditions, "sets LIKE ?")
		}

		values = append(values, "%"+keyword+"%")

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
	err = tx.Order(getChannelOrder(order)).Limit(limit).Offset(offset).Find(&channels).Error

	return channels, total, err
}

func GetChannelByID(id int) (*Channel, error) {
	channel := Channel{ID: id}
	err := DB.First(&channel, "id = ?", id).Error
	return &channel, HandleNotFound(err, ErrChannelNotFound)
}

func BatchInsertChannels(channels []*Channel) (err error) {
	defer func() {
		if err == nil {
			_ = InitModelConfigAndChannelCache()
		}
	}()

	for _, channel := range channels {
		if err := CheckModelConfigExist(channel.Models); err != nil {
			return err
		}
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&channels).Error
	})
}

func UpdateChannel(channel *Channel, update *ChannelPatch) (err error) {
	defer func() {
		if err == nil {
			_ = InitModelConfigAndChannelCache()
			_ = monitor.ClearChannelAllModelErrors(context.Background(), channel.ID)
		}
	}()

	if update.Models != nil {
		if err := CheckModelConfigExist(*update.Models); err != nil {
			return err
		}
	}

	result := DB.Model(channel).
		Clauses(clause.Returning{}).
		Where("id = ?", channel.ID).
		Updates(update)
	if result.Error == nil && result.RowsAffected == 0 {
		current, err := GetChannelByID(channel.ID)
		if err != nil {
			return err
		}

		*channel = *current

		return nil
	}

	return HandleUpdateResult(result, ErrChannelNotFound)
}

func ClearLastTestErrorAt(id int) error {
	result := DB.Model(&Channel{}).
		Where("id = ?", id).
		Update("last_test_error_at", gorm.Expr("NULL"))
	return HandleUpdateResult(result, ErrChannelNotFound)
}

func (c *Channel) UpdateModelTest(
	testAt time.Time,
	model, actualModel string,
	mode mode.Mode,
	took float64,
	success bool,
	response string,
	code int,
) (*ChannelTest, error) {
	var ct *ChannelTest

	err := DB.Transaction(func(tx *gorm.DB) error {
		if !success {
			result := tx.Model(&Channel{}).
				Where("id = ?", c.ID).
				Update("last_test_error_at", testAt)
			if err := HandleUpdateResult(result, ErrChannelNotFound); err != nil {
				return err
			}
		} else if !c.LastTestErrorAt.IsZero() && time.Since(c.LastTestErrorAt) > time.Hour {
			result := tx.Model(&Channel{}).
				Where("id = ?", c.ID).
				Update("last_test_error_at", gorm.Expr("NULL"))
			if err := HandleUpdateResult(result, ErrChannelNotFound); err != nil {
				return err
			}
		}

		ct = &ChannelTest{
			ChannelID:   c.ID,
			ChannelType: c.Type,
			ChannelName: c.Name,
			Model:       model,
			ActualModel: actualModel,
			Mode:        mode,
			TestAt:      testAt,
			Took:        took,
			Success:     success,
			Response:    response,
			Code:        code,
		}
		result := tx.Save(ct)

		return HandleUpdateResult(result, ErrChannelNotFound)
	})
	if err != nil {
		return nil, err
	}

	return ct, nil
}

func (c *Channel) UpdateBalance(balance float64) error {
	result := DB.Model(&Channel{}).
		Select("balance_updated_at", "balance").
		Where("id = ?", c.ID).
		Updates(Channel{
			BalanceUpdatedAt: time.Now(),
			Balance:          balance,
		})

	return HandleUpdateResult(result, ErrChannelNotFound)
}

func DeleteChannelByID(id int) (err error) {
	defer func() {
		if err == nil {
			_ = InitModelConfigAndChannelCache()
			_ = monitor.ClearChannelAllModelErrors(context.Background(), id)
		}
	}()

	result := DB.Delete(&Channel{ID: id})

	return HandleUpdateResult(result, ErrChannelNotFound)
}

func DeleteChannelsByIDs(ids []int) (err error) {
	defer func() {
		if err == nil {
			_ = InitModelConfigAndChannelCache()

			for _, id := range ids {
				_ = monitor.ClearChannelAllModelErrors(context.Background(), id)
			}
		}
	}()

	return DB.Transaction(func(tx *gorm.DB) error {
		return tx.
			Where("id IN (?)", ids).
			Delete(&Channel{}).
			Error
	})
}

func UpdateChannelStatusByID(id, status int) error {
	result := DB.Model(&Channel{}).
		Where("id = ?", id).
		Update("status", status)
	return HandleUpdateResult(result, ErrChannelNotFound)
}

func UpdateChannelUsedAmount(id int, amount float64, requestCount, retryCount int) error {
	result := DB.Model(&Channel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"used_amount":   gorm.Expr("used_amount + ?", amount),
			"request_count": gorm.Expr("request_count + ?", requestCount),
			"retry_count":   gorm.Expr("retry_count + ?", retryCount),
		})

	return HandleUpdateResult(result, ErrChannelNotFound)
}

type ChannelBasicInfo struct {
	ID         int         `json:"id"`
	Name       string      `json:"name"`
	Remark     string      `json:"remark,omitempty"`
	Type       ChannelType `json:"type"`
	Status     int         `json:"status"`
	BackupOnly bool        `json:"backup_only"`
}

func GetChannelsBasicInfoByIDs(ids []int) ([]*ChannelBasicInfo, error) {
	if len(ids) == 0 {
		return []*ChannelBasicInfo{}, nil
	}

	var result []*ChannelBasicInfo

	err := DB.Unscoped().
		Model(&Channel{}).
		Select("id", "name", "remark", "type", "status", "backup_only").
		Where("id IN ?", ids).
		Find(&result).
		Error

	return result, err
}
