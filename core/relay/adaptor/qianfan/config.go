package qianfan

import "github.com/labring/aiproxy/core/relay/meta"

type Config struct {
	AppID          string   `json:"appid"`
	ResponseModels []string `json:"response_models"`
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
			"response_models": map[string]any{
				"type":        "array",
				"title":       "Additional Responses API Models",
				"description": "Additional Qianfan model names that support the Responses API beyond the built-in documented list.",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
	}
}
