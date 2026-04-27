package qianfan

import "github.com/labring/aiproxy/core/relay/meta"

type Config struct {
	AppID string `json:"appid"`
}

func (a *Adaptor) loadConfig(meta *meta.Meta) (Config, error) {
	cfg := Config{}
	return a.configCache.Load(meta, cfg)
}

func configSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"appid": map[string]any{
				"type":        "string",
				"title":       "AppID Header",
				"description": "Optional Qianfan appid header used to distinguish usage and billing by application.",
			},
		},
	}
}
