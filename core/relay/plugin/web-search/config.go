package websearch

import (
	"encoding/json"

	"github.com/bytedance/sonic"
)

// Configuration structures
type Config struct {
	Enable            bool           `json:"enable"`
	ForceSearch       bool           `json:"force_search"`
	MaxResults        int            `json:"max_results"`
	SearchRewrite     SearchRewrite  `json:"search_rewrite"`
	NeedReference     bool           `json:"need_reference"`
	ReferenceLocation string         `json:"reference_location"` // content or other field
	ReferenceFormat   string         `json:"reference_format"`
	DefaultLanguage   string         `json:"default_language"`
	PromptTemplate    string         `json:"prompt_template"`
	SearchFrom        []EngineConfig `json:"search_from"`
}

type SearchRewrite struct {
	Enable             bool   `json:"enable"`
	ModelName          string `json:"model_name"`
	TimeoutMillisecond uint32 `json:"timeout_millisecond"`
	MaxCount           int    `json:"max_count"`
	AddRewriteUsage    bool   `json:"add_rewrite_usage"`
	RewriteUsageField  string `json:"rewrite_usage_field"`
}

type EngineConfig struct {
	Type       string          `json:"type"` // bing, google, arxiv, searchxng
	MaxResults int             `json:"max_results"`
	Spec       json.RawMessage `json:"spec"`
}

func (e *EngineConfig) SpecExists() bool {
	return len(e.Spec) > 0
}

func (e *EngineConfig) LoadSpec(spec any) error {
	if !e.SpecExists() {
		return nil
	}
	return sonic.Unmarshal(e.Spec, spec)
}

// Engine-specific configuration structures
type GoogleSpec struct {
	APIKey  string `json:"api_key"`
	CX      string `json:"cx"`
	BaseURL string `json:"base_url,omitempty"`
}

type BingSpec struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url,omitempty"`
}

type ArxivSpec struct{}

type SearchXNGSpec struct {
	BaseURL string `json:"base_url"`
}
