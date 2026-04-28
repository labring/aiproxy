package gemini

import "github.com/labring/aiproxy/core/relay/meta"

type Config struct {
	Safety                      string `json:"safety"`
	DisableAutoImageURLToBase64 bool   `json:"disable_auto_image_url_to_base64"`
}

func loadConfig(meta *meta.Meta) (Config, error) {
	return (&Adaptor{}).loadConfig(meta)
}

func (a *Adaptor) loadConfig(meta *meta.Meta) (Config, error) {
	return a.configCache.Load(meta, Config{})
}
