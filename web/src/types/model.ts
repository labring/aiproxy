// src/types/model.ts
export interface ModelConfigDetail {
    max_input_tokens?: number
    max_output_tokens?: number
    max_context_tokens?: number
    vision?: boolean
    tool_choice?: boolean
    coder?: boolean
    limited_time_free?: boolean
    support_formats?: string[]
    support_voices?: string[]
    [key: string]: unknown
}

export interface PriceCondition {
    input_token_min?: number
    input_token_max?: number
    output_token_min?: number
    output_token_max?: number
    start_time?: number
    end_time?: number
    daily_start_time?: string
    daily_end_time?: string
    timezone?: string
    resolution?: string[]
    quality?: string[]
    service_tier?: '' | 'auto' | 'default' | 'flex' | 'scale' | 'priority'
    input_media?: boolean
    input_video?: boolean
    output_audio?: boolean
}

export interface ConditionalPrice {
    condition: PriceCondition
    price: ModelPrice
}

export interface TimeoutConfig {
    request_timeout?: number
    stream_request_timeout?: number
}

export interface ModelPrice {
    input_price?: number
    input_price_unit?: number
    output_price?: number
    output_price_unit?: number
    per_request_price?: number
    cache_creation_price?: number
    cache_creation_price_unit?: number
    cached_price?: number
    cached_price_unit?: number
    image_input_price?: number
    image_input_price_unit?: number
    image_output_price?: number
    image_output_price_unit?: number
    audio_input_price?: number
    audio_input_price_unit?: number
    video_input_price?: number
    video_input_price_unit?: number
    audio_output_price?: number
    audio_output_price_unit?: number
    thinking_mode_output_price?: number
    thinking_mode_output_price_unit?: number
    web_search_price?: number
    web_search_price_unit?: number
    conditional_prices?: ConditionalPrice[]
}

export interface ModelConfig {
    config?: ModelConfigDetail
    created_at?: number
    updated_at?: number
    model: string
    owner?: string
    image_batch_size?: number
    type: number
    exclude_from_tests?: boolean
    price?: ModelPrice
    rpm?: number
    tpm?: number
    retry_times?: number
    retry_budget?: number
    timeout_config?: TimeoutConfig
    force_save_detail?: boolean
    max_image_generation_count?: number
    max_video_generation_seconds?: number
    max_video_generation_count?: number
    allowed_resolutions?: string[]
    request_body_storage_max_size?: number
    response_body_storage_max_size?: number
    summary_service_tier?: boolean
    summary_claude_long_context?: boolean
    disable_resolution_fuzzy_match?: boolean
    plugin?: Plugin
}

export type ModelSaveRequest = Omit<ModelConfig, 'created_at' | 'updated_at'>

export const MODEL_TYPE_OPTIONS = [
    1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
    11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
    21, 22, 23, 24, 25, 26, 27, 28, 29, 30,
    31, 32, 33, 34, 35, 36, 37, 38,
] as const

export const STREAM_TIMEOUT_SUPPORTED_MODEL_TYPES = [1, 2, 12, 16, 21] as const
export const IMAGE_GENERATION_COUNT_LIMIT_SUPPORTED_MODEL_TYPES = [5, 6, 30] as const
export const VIDEO_GENERATION_SECONDS_LIMIT_SUPPORTED_MODEL_TYPES = [13, 22, 26, 27, 32, 33, 34, 36] as const
export const VIDEO_GENERATION_COUNT_LIMIT_SUPPORTED_MODEL_TYPES = [13, 27] as const
export const RESOLUTION_FUZZY_MATCH_SUPPORTED_MODEL_TYPES = [5, 6, 13, 22, 26, 27, 28, 30, 32, 33, 34, 36] as const

export type ModelTypeOption = (typeof MODEL_TYPE_OPTIONS)[number]

type Plugin = {
    cache: CachePlugin // 缓存插件
    cachefollow: CacheFollowPlugin // 缓存跟随插件
    "web-search": WebSearchPlugin // 网络搜索插件
    "think-split": ThinkSplitPlugin // 思考拆分插件
    "stream-fake": StreamFakePlugin // 流式伪装插件
}

type CachePlugin = {
    enable: boolean
    ttl?: number
    item_max_size?: number
    add_cache_hit_header?: boolean
    cache_hit_header?: string
}

type CacheFollowPlugin = {
    enable: boolean
    enable_generic_follow?: boolean
    followed_channel_ttl_seconds?: number
    recent_channel_update_debounce_seconds?: number
}

type WebSearchPlugin = {
    enable: boolean
    force_search?: boolean
    max_results?: number
    search_rewrite?: {
        enable?: boolean
        model_name?: string
        timeout_millisecond?: number
        max_count?: number
        add_rewrite_usage?: boolean
        rewrite_usage_field?: string
    }
    need_reference?: boolean
    reference_location?: string
    reference_format?: string
    default_language?: string
    prompt_template?: string
    search_from: EngineConfig[]
}

type ThinkSplitPlugin = {
    enable: boolean
}

type StreamFakePlugin = {
    enable: boolean
}

type EngineConfig = {
    type: 'bing' | 'google' | 'arxiv' | 'searchxng'
    max_results?: number
    spec?: GoogleSpec | BingSpec | ArxivSpec | SearchXNGSpec
}

type GoogleSpec = {
    api_key?: string
    cx?: string
}

type BingSpec = {
    api_key?: string
}

type ArxivSpec = object

type SearchXNGSpec = {
    base_url?: string
}

export interface ModelCreateRequest {
    model: string
    config?: ModelConfigDetail
    owner?: string
    type: number
    exclude_from_tests?: boolean
    rpm?: number
    tpm?: number
    retry_times?: number
    retry_budget?: number
    timeout_config?: TimeoutConfig
    force_save_detail?: boolean
    max_image_generation_count?: number
    max_video_generation_seconds?: number
    max_video_generation_count?: number
    allowed_resolutions?: string[]
    request_body_storage_max_size?: number
    response_body_storage_max_size?: number
    summary_service_tier?: boolean
    summary_claude_long_context?: boolean
    disable_resolution_fuzzy_match?: boolean
    price?: ModelPrice | Record<string, unknown>
    plugin?: Plugin
}

// Export all types for use in other modules
export type {
    Plugin,
    CachePlugin,
    CacheFollowPlugin,
    WebSearchPlugin,
    ThinkSplitPlugin,
    StreamFakePlugin,
    EngineConfig,
    GoogleSpec,
    BingSpec,
    ArxivSpec,
    SearchXNGSpec
}
