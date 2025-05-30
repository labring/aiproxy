package websearch

import (
	"encoding/json"

	"github.com/bytedance/sonic"
)

// Configuration structures
type Config struct {
	EnablePlugin      bool           `json:"enable_plugin"`
	DefaultEnable     bool           `json:"default_enable"`
	MaxResults        int            `json:"max_results"`
	SearchRewrite     SearchRewrite  `json:"search_rewrite"`
	NeedReference     bool           `json:"need_reference"`
	ReferenceLocation string         `json:"reference_location"`
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
	Type       string          `json:"type"` // bing, google, arxiv
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
	APIKey string `json:"api_key"`
	CX     string `json:"cx"`
}

type BingSpec struct {
	APIKey string `json:"api_key"`
}

type ArxivSpec struct{}
