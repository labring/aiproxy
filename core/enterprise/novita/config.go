//go:build enterprise

package novita

import (
	"fmt"
	"strconv"

	"github.com/labring/aiproxy/core/model"
	"gorm.io/gorm"
)

const (
	optionKeyNovitaChannelID       = "NovitaChannelID"
	optionKeyNovitaAPIKey          = "NovitaAPIKey"
	optionKeyNovitaAPIBase         = "NovitaAPIBase"
	optionKeyNovitaMgmtToken       = "NovitaMgmtToken"
	optionKeyNovitaExchangeRate    = "NovitaExchangeRate"
	optionKeyNovitaAutoSyncEnabled = "NovitaAutoSyncEnabled"
	defaultNovitaExchangeRate      = 7.0
)

var novitaOptionKeys = []string{
	optionKeyNovitaChannelID,
	optionKeyNovitaAPIKey,
	optionKeyNovitaAPIBase,
	optionKeyNovitaMgmtToken,
	optionKeyNovitaExchangeRate,
	optionKeyNovitaAutoSyncEnabled,
}

// NovitaConfigResult holds the current Novita configuration.
type NovitaConfigResult struct {
	ChannelID       int     `json:"channel_id"`
	APIKey          string  `json:"api_key"`
	APIBase         string  `json:"api_base"`
	MgmtToken       string  `json:"mgmt_token,omitempty"`
	ExchangeRate    float64 `json:"exchange_rate"` // USD→CNY exchange rate for price conversion
	AutoSyncEnabled bool    `json:"auto_sync_enabled"`
}

// GetNovitaConfig reads Novita configuration from the option table.
func GetNovitaConfig() (cfg NovitaConfigResult) {
	var options []model.Option

	model.DB.Where("key IN ?", novitaOptionKeys).Find(&options)

	for _, opt := range options {
		switch opt.Key {
		case optionKeyNovitaChannelID:
			cfg.ChannelID, _ = strconv.Atoi(opt.Value)
		case optionKeyNovitaAPIKey:
			cfg.APIKey = opt.Value
		case optionKeyNovitaAPIBase:
			cfg.APIBase = opt.Value
		case optionKeyNovitaMgmtToken:
			cfg.MgmtToken = opt.Value
		case optionKeyNovitaExchangeRate:
			cfg.ExchangeRate, _ = strconv.ParseFloat(opt.Value, 64)
		case optionKeyNovitaAutoSyncEnabled:
			cfg.AutoSyncEnabled = opt.Value == "true"
		}
	}

	if cfg.ExchangeRate <= 0 {
		cfg.ExchangeRate = defaultNovitaExchangeRate
	}

	return cfg
}

// IsAutoSyncEnabled returns whether the daily auto-sync is enabled in the DB.
func IsAutoSyncEnabled() bool {
	var opt model.Option
	if err := model.DB.Where("key = ?", optionKeyNovitaAutoSyncEnabled).
		First(&opt).
		Error; err != nil {
		return false // not found → default off
	}

	return opt.Value == "true"
}

// SetAutoSyncEnabled persists the auto-sync toggle to the Option table.
func SetAutoSyncEnabled(enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}

	return model.DB.Where("key = ?", optionKeyNovitaAutoSyncEnabled).
		Assign(model.Option{Value: val}).
		FirstOrCreate(&model.Option{Key: optionKeyNovitaAutoSyncEnabled}).Error
}

// SetNovitaMgmtToken persists the management token to the Option table.
func SetNovitaMgmtToken(token string) error {
	return model.DB.Where("key = ?", optionKeyNovitaMgmtToken).
		Assign(model.Option{Value: token}).
		FirstOrCreate(&model.Option{Key: optionKeyNovitaMgmtToken}).Error
}

// SetNovitaExchangeRate persists the USD→CNY exchange rate to the Option table.
func SetNovitaExchangeRate(rate float64) error {
	return model.DB.Where("key = ?", optionKeyNovitaExchangeRate).
		Assign(model.Option{Value: strconv.FormatFloat(rate, 'f', -1, 64)}).
		FirstOrCreate(&model.Option{Key: optionKeyNovitaExchangeRate}).Error
}

// SetNovitaAPIKeyDirect persists an API key and base URL directly to the Option table,
// without requiring an existing channel. Used for bootstrap when no channels exist yet.
func SetNovitaAPIKeyDirect(apiKey, apiBase string) error {
	if apiBase == "" {
		apiBase = DefaultNovitaAPIBase
	}

	return model.DB.Transaction(func(tx *gorm.DB) error {
		for _, kv := range []struct{ key, val string }{
			{optionKeyNovitaAPIKey, apiKey},
			{optionKeyNovitaAPIBase, apiBase},
		} {
			if err := tx.Where("key = ?", kv.key).
				Assign(model.Option{Value: kv.val}).
				FirstOrCreate(&model.Option{Key: kv.key}).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// SetNovitaConfigFromChannel reads key/base_url from the given channel and persists them.
func SetNovitaConfigFromChannel(channelID int) error {
	var ch model.Channel
	if err := model.DB.First(&ch, channelID).Error; err != nil {
		return fmt.Errorf("channel %d not found: %w", channelID, err)
	}

	apiBase := ch.BaseURL
	if apiBase == "" {
		apiBase = DefaultNovitaAPIBase
	}

	return model.DB.Transaction(func(tx *gorm.DB) error {
		for _, kv := range []struct{ key, val string }{
			{optionKeyNovitaChannelID, strconv.Itoa(channelID)},
			{optionKeyNovitaAPIKey, ch.Key},
			{optionKeyNovitaAPIBase, apiBase},
		} {
			if err := tx.Where("key = ?", kv.key).
				Assign(model.Option{Value: kv.val}).
				FirstOrCreate(&model.Option{Key: kv.key}).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
