//go:build enterprise

package novita

import (
	"fmt"
	"strconv"

	"gorm.io/gorm"

	"github.com/labring/aiproxy/core/model"
)

const (
	optionKeyNovitaChannelID = "NovitaChannelID"
	optionKeyNovitaAPIKey    = "NovitaAPIKey"
	optionKeyNovitaAPIBase   = "NovitaAPIBase"
	optionKeyNovitaMgmtToken = "NovitaMgmtToken"
)

var novitaOptionKeys = []string{
	optionKeyNovitaChannelID,
	optionKeyNovitaAPIKey,
	optionKeyNovitaAPIBase,
	optionKeyNovitaMgmtToken,
}

// NovitaConfigResult holds the current Novita configuration.
type NovitaConfigResult struct {
	ChannelID int    `json:"channel_id"`
	APIKey    string `json:"api_key"`
	APIBase   string `json:"api_base"`
	MgmtToken string `json:"mgmt_token,omitempty"`
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
		}
	}

	return cfg
}

// SetNovitaMgmtToken persists the management token to the Option table.
func SetNovitaMgmtToken(token string) error {
	return model.DB.Where("key = ?", optionKeyNovitaMgmtToken).
		Assign(model.Option{Value: token}).
		FirstOrCreate(&model.Option{Key: optionKeyNovitaMgmtToken}).Error
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
