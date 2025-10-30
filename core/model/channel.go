package model

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
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

type ChannelConfig struct {
	Spec json.RawMessage `json:"spec"`
}

// validate spec json is map[string]any
func (c *ChannelConfig) UnmarshalJSON(data []byte) error {
	type Alias ChannelConfig

	alias := (*Alias)(c)
	if err := sonic.Unmarshal(data, alias); err != nil {
		return err
	}

	if len(alias.Spec) > 0 {
		var spec map[string]any
		if err := sonic.Unmarshal(alias.Spec, &spec); err != nil {
			return fmt.Errorf("invalid spec json: %w", err)
		}
	}

	return nil
}

func (c *ChannelConfig) SpecConfig(obj any) error {
	if c == nil || len(c.Spec) == 0 {
		return nil
	}
	return sonic.Unmarshal(c.Spec, obj)
}

func (c *ChannelConfig) Get(key ...any) (ast.Node, error) {
	if c == nil || len(c.Spec) == 0 {
		return ast.Node{}, ast.ErrNotExist
	}
	return sonic.Get(c.Spec, key...)
}

type Channel struct {
	DeletedAt               gorm.DeletedAt    `gorm:"index"                              json:"-"`
	CreatedAt               time.Time         `gorm:"index"                              json:"created_at"`
	LastTestErrorAt         time.Time         `                                          json:"last_test_error_at"`
	ChannelTests            []*ChannelTest    `gorm:"foreignKey:ChannelID;references:ID" json:"channel_tests,omitempty"`
	BalanceUpdatedAt        time.Time         `                                          json:"balance_updated_at"`
	ModelMapping            map[string]string `gorm:"serializer:fastjson;type:text"      json:"model_mapping"`
	Key                     string            `gorm:"type:text;index:,length:191"        json:"key"`
	Name                    string            `gorm:"size:64;index"                      json:"name"`
	BaseURL                 string            `gorm:"size:128;index"                     json:"base_url"`
	Models                  []string          `gorm:"serializer:fastjson;type:text"      json:"models"`
	Balance                 float64           `                                          json:"balance"`
	ID                      int               `gorm:"primaryKey"                         json:"id"`
	UsedAmount              float64           `gorm:"index"                              json:"used_amount"`
	RequestCount            int               `gorm:"index"                              json:"request_count"`
	RetryCount              int               `gorm:"index"                              json:"retry_count"`
	Status                  int               `gorm:"default:1;index"                    json:"status"`
	Type                    ChannelType       `gorm:"default:0;index"                    json:"type"`
	Priority                int32             `                                          json:"priority"`
	EnabledAutoBalanceCheck bool              `                                          json:"enabled_auto_balance_check"`
	BalanceThreshold        float64           `                                          json:"balance_threshold"`
	Config                  *ChannelConfig    `gorm:"serializer:fastjson;type:text"      json:"config,omitempty"`
	Sets                    []string          `gorm:"serializer:fastjson;type:text"      json:"sets,omitempty"`
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
)

func (c *Channel) GetPriority() int32 {
	if c.Priority == 0 {
		return DefaultPriority
	}
	return c.Priority
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

func GetChannels(
	page, perPage, id int,
	name, key string,
	channelType int,
	baseURL, order string,
) (channels []*Channel, total int64, err error) {
	tx := DB.Model(&Channel{})
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
	page, perPage, id int,
	name, key string,
	channelType int,
	baseURL, order string,
) (channels []*Channel, total int64, err error) {
	tx := DB.Model(&Channel{})

	// Handle exact match conditions for non-zero values
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

	// Handle keyword search for zero value fields
	if keyword != "" {
		var (
			conditions []string
			values     []any
		)

		keywordInt := String2Int(keyword)

		if keywordInt != 0 {
			if id == 0 {
				conditions = append(conditions, "id = ?")
				values = append(values, keywordInt)
			}
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

func UpdateChannel(channel *Channel) (err error) {
	defer func() {
		if err == nil {
			_ = InitModelConfigAndChannelCache()
			_ = monitor.ClearChannelAllModelErrors(context.Background(), channel.ID)
		}
	}()

	if err := CheckModelConfigExist(channel.Models); err != nil {
		return err
	}

	selects := []string{
		"model_mapping",
		"key",
		"base_url",
		"models",
		"priority",
		"config",
		"enabled_auto_balance_check",
		"balance_threshold",
		"sets",
	}
	if channel.Type != 0 {
		selects = append(selects, "type")
	}

	if channel.Name != "" {
		selects = append(selects, "name")
	}

	result := DB.
		Select(selects).
		Clauses(clause.Returning{}).
		Where("id = ?", channel.ID).
		Updates(channel)

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
			result := tx.Model(&Channel{}).Where("id = ?", c.ID).Update("last_test_error_at", gorm.Expr("NULL"))
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
