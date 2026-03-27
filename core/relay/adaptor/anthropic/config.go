package anthropic

type Config struct {
	DisableContextManagement            bool     `json:"disable_context_management"`
	SupportedContextManagementEditsType []string `json:"supported_context_management_edits_type"`
	RemoveToolsExamples                 bool     `json:"remove_tools_examples"`
	RemoveToolsCustomDeferLoading       bool     `json:"remove_tools_custom_defer_loading"`
	// SkipImageConversion when true skips converting image URLs to base64.
	// Anthropic-compatible upstreams (Anthropic API, PPIO, etc.) natively
	// support URL image sources, so the conversion only adds latency and
	// inflates the request body. Backends that require base64 (e.g. AWS
	// Bedrock) handle conversion in their own adaptor callbacks.
	SkipImageConversion bool `json:"skip_image_conversion"`
	// StripCacheTTL when true removes cache_control.ttl from the request.
	// The ttl field is part of the prompt-caching-scope beta. Set this to
	// true for upstreams that do not support cache TTL customization.
	StripCacheTTL bool `json:"strip_cache_ttl"`
}
