package anthropic

import "github.com/labring/aiproxy/core/relay/meta"

type Config struct {
	DisableContextManagement            bool     `json:"disable_context_management"`
	SupportedContextManagementEditsType []string `json:"supported_context_management_edits_type"`
	RemoveToolsExamples                 bool     `json:"remove_tools_examples"`
	RemoveToolsCustomDeferLoading       bool     `json:"remove_tools_custom_defer_loading"`
	DisableAutoImageURLToBase64         bool     `json:"disable_auto_image_url_to_base64"`
}

func loadConfig(meta *meta.Meta) (Config, error) {
	return (&Adaptor{}).loadConfig(meta)
}

func (a *Adaptor) loadConfig(meta *meta.Meta) (Config, error) {
	return a.configCache.Load(meta, Config{})
}
