//go:build enterprise

package ppio

import (
	"fmt"
	"strconv"

	"github.com/labring/aiproxy/core/model"
	"gorm.io/gorm"
)

const (
	optionKeyPPIOChannelID       = "PPIOChannelID"
	optionKeyPPIOAPIKey          = "PPIOAPIKey"
	optionKeyPPIOAPIBase         = "PPIOAPIBase"
	optionKeyPPIOMgmtToken       = "PPIOMgmtToken"
	optionKeyPPIOAutoSyncEnabled = "PPIOAutoSyncEnabled"
)

var ppioOptionKeys = []string{
	optionKeyPPIOChannelID,
	optionKeyPPIOAPIKey,
	optionKeyPPIOAPIBase,
	optionKeyPPIOMgmtToken,
	optionKeyPPIOAutoSyncEnabled,
}

// PPIOConfigResult holds the current PPIO configuration.
type PPIOConfigResult struct {
	ChannelID       int    `json:"channel_id"`
	APIKey          string `json:"api_key"`
	APIBase         string `json:"api_base"`
	MgmtToken       string `json:"mgmt_token,omitempty"`
	AutoSyncEnabled bool   `json:"auto_sync_enabled"`
}

// GetPPIOConfig reads PPIO configuration from the option table.
func GetPPIOConfig() (cfg PPIOConfigResult) {
	var options []model.Option

	model.DB.Where("key IN ?", ppioOptionKeys).Find(&options)

	for _, opt := range options {
		switch opt.Key {
		case optionKeyPPIOChannelID:
			cfg.ChannelID, _ = strconv.Atoi(opt.Value)
		case optionKeyPPIOAPIKey:
			cfg.APIKey = opt.Value
		case optionKeyPPIOAPIBase:
			cfg.APIBase = opt.Value
		case optionKeyPPIOMgmtToken:
			cfg.MgmtToken = opt.Value
		case optionKeyPPIOAutoSyncEnabled:
			cfg.AutoSyncEnabled = opt.Value == "true"
		}
	}

	return cfg
}

// IsAutoSyncEnabled returns whether the daily auto-sync is enabled in the DB.
func IsAutoSyncEnabled() bool {
	var opt model.Option
	if err := model.DB.Where("key = ?", optionKeyPPIOAutoSyncEnabled).
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

	return model.DB.Where("key = ?", optionKeyPPIOAutoSyncEnabled).
		Assign(model.Option{Value: val}).
		FirstOrCreate(&model.Option{Key: optionKeyPPIOAutoSyncEnabled}).Error
}

// SetPPIOMgmtToken persists the management console token to the Option table.
func SetPPIOMgmtToken(token string) error {
	return model.DB.Where("key = ?", optionKeyPPIOMgmtToken).
		Assign(model.Option{Value: token}).
		FirstOrCreate(&model.Option{Key: optionKeyPPIOMgmtToken}).Error
}

// SetPPIOAPIKeyDirect persists an API key and base URL directly to the Option table,
// without requiring an existing channel. Used for bootstrap when no channels exist yet.
func SetPPIOAPIKeyDirect(apiKey, apiBase string) error {
	if apiBase == "" {
		apiBase = DefaultPPIOAPIBase
	}

	return model.DB.Transaction(func(tx *gorm.DB) error {
		for _, kv := range []struct{ key, val string }{
			{optionKeyPPIOAPIKey, apiKey},
			{optionKeyPPIOAPIBase, apiBase},
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

// SetPPIOConfigFromChannel reads key/base_url from the given channel and persists them.
func SetPPIOConfigFromChannel(channelID int) error {
	var ch model.Channel
	if err := model.DB.First(&ch, channelID).Error; err != nil {
		return fmt.Errorf("channel %d not found: %w", channelID, err)
	}

	apiBase := ch.BaseURL
	if apiBase == "" {
		apiBase = DefaultPPIOAPIBase
	}

	return model.DB.Transaction(func(tx *gorm.DB) error {
		for _, kv := range []struct{ key, val string }{
			{optionKeyPPIOChannelID, strconv.Itoa(channelID)},
			{optionKeyPPIOAPIKey, ch.Key},
			{optionKeyPPIOAPIBase, apiBase},
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
