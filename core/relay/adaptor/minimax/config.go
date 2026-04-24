package minimax

import "github.com/labring/aiproxy/core/relay/meta"

type Config struct {
	UseChatCompletionsPath bool `json:"use_chat_completions_path"`
}

func (a *Adaptor) loadConfig(meta *meta.Meta) (Config, error) {
	cfg := Config{}
	return a.configCache.Load(meta, cfg)
}
