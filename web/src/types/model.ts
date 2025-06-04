// src/types/model.ts
export interface ModelConfigDetail {
    max_input_tokens?: number
    max_output_tokens?: number
    max_context_tokens?: number
    vision?: boolean
    tool_choice?: boolean
    support_formats?: string[]
    support_voices?: string[]
}

export interface ModelPrice {
    input_price: number
    output_price: number
    per_request_price: number
    cache_creation_price?: number
    cache_creation_price_unit?: number
    cached_price?: number
    cached_price_unit?: number
    image_input_price?: number
    image_input_price_unit?: number
    image_output_price?: number
    image_output_price_unit?: number
    web_search_price?: number
    web_search_price_unit?: number
}

export interface ModelConfig {
    config?: ModelConfigDetail
    created_at: number
    updated_at: number
    image_prices: number[] | null
    model: string
    owner: string
    image_batch_size?: number
    type: number
    price: ModelPrice
    rpm: number
    tpm?: number
    plugin: Plugin
}

type Plugin = {
    cache: CachePlugin // 缓存插件
    "web-search": WebSearchPlugin // 网络搜索插件
    "think-split": ThinkSplitPlugin // 思考拆分插件
}

type CachePlugin = {
    enable: boolean
    ttl?: number
    item_max_size?: number
    add_cache_hit_header?: boolean
    cache_hit_header?: string
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
    type: number
    plugin?: Plugin
}

// Export all types for use in other modules
export type {
    Plugin,
    CachePlugin,
    WebSearchPlugin,
    ThinkSplitPlugin,
    EngineConfig,
    GoogleSpec,
    BingSpec,
    ArxivSpec,
    SearchXNGSpec
}