// src/validation/model.ts
import { z } from 'zod'

// 不同搜索引擎的spec配置
const googleSpecSchema = z.object({
    api_key: z.string().optional(),
    cx: z.string().optional(),
})

const bingSpecSchema = z.object({
    api_key: z.string().optional(),
})

const arxivSpecSchema = z.object({})

const searchXNGSpecSchema = z.object({
    base_url: z.string().optional(),
})

// 搜索引擎配置验证
const engineConfigSchema = z.object({
    type: z.enum(['bing', 'google', 'arxiv', 'searchxng']),
    max_results: z.number().optional(),
    spec: z.union([googleSpecSchema, bingSpecSchema, arxivSpecSchema, searchXNGSpecSchema]).optional()
})

// 插件配置验证 - 根据用户需求调整
const pluginSchema = z.object({
    cache: z.object({
        enable: z.boolean(),
        ttl: z.number().optional(),
        item_max_size: z.number().optional(),
        add_cache_hit_header: z.boolean().optional(),
        cache_hit_header: z.string().optional(),
    }).optional(),
    "web-search": z.object({
        enable: z.boolean(),
        force_search: z.boolean().optional(),
        max_results: z.number().optional(),
        search_rewrite: z.object({
            enable: z.boolean().optional(),
            model_name: z.string().optional(),
            timeout_millisecond: z.number().optional(),
            max_count: z.number().optional(),
            add_rewrite_usage: z.boolean().optional(),
            rewrite_usage_field: z.string().optional(),
        }).optional(),
        need_reference: z.boolean().optional(),
        reference_location: z.string().optional(),
        reference_format: z.string().optional(),
        default_language: z.string().optional(),
        prompt_template: z.string().optional(),
        search_from: z.array(engineConfigSchema).optional()
    }).refine((data) => {
        // 如果 web-search 插件启用，则 search_from 必须至少有一个引擎
        if (data.enable && (!data.search_from || data.search_from.length === 0)) {
            return false
        }
        return true
    }, {
        message: '启用网络搜索插件时，必须至少配置一个搜索引擎',
        path: ['search_from']
    }).optional(),
    "think-split": z.object({
        enable: z.boolean(),
    }).optional(),
    "stream-fake": z.object({
        enable: z.boolean(),
    }).optional(),
}).optional()

// Price condition schema
const priceConditionSchema = z.object({
    input_token_min: z.number().optional(),
    input_token_max: z.number().optional(),
    output_token_min: z.number().optional(),
    output_token_max: z.number().optional(),
    start_time: z.number().optional(),
    end_time: z.number().optional(),
    service_tier: z.enum(['auto', 'default', 'flex', 'scale', 'priority']).or(z.literal('')).optional(),
})

// Price schema (used for conditional prices)
const basePriceSchema = z.object({
    input_price: z.number().optional(),
    input_price_unit: z.number().optional(),
    output_price: z.number().optional(),
    output_price_unit: z.number().optional(),
    per_request_price: z.number().optional(),
    cached_price: z.number().optional(),
    cached_price_unit: z.number().optional(),
    cache_creation_price: z.number().optional(),
    cache_creation_price_unit: z.number().optional(),
    image_input_price: z.number().optional(),
    image_input_price_unit: z.number().optional(),
    image_output_price: z.number().optional(),
    image_output_price_unit: z.number().optional(),
    audio_input_price: z.number().optional(),
    audio_input_price_unit: z.number().optional(),
    thinking_mode_output_price: z.number().optional(),
    thinking_mode_output_price_unit: z.number().optional(),
    web_search_price: z.number().optional(),
    web_search_price_unit: z.number().optional(),
})

const conditionalPriceSchema = z.object({
    condition: priceConditionSchema,
    price: basePriceSchema,
})

export const priceSchema = z.object({
    input_price: z.number().optional(),
    input_price_unit: z.number().optional(),
    output_price: z.number().optional(),
    output_price_unit: z.number().optional(),
    per_request_price: z.number().optional(),
    cached_price: z.number().optional(),
    cached_price_unit: z.number().optional(),
    cache_creation_price: z.number().optional(),
    cache_creation_price_unit: z.number().optional(),
    image_input_price: z.number().optional(),
    image_input_price_unit: z.number().optional(),
    image_output_price: z.number().optional(),
    image_output_price_unit: z.number().optional(),
    audio_input_price: z.number().optional(),
    audio_input_price_unit: z.number().optional(),
    thinking_mode_output_price: z.number().optional(),
    thinking_mode_output_price_unit: z.number().optional(),
    web_search_price: z.number().optional(),
    web_search_price_unit: z.number().optional(),
    conditional_prices: z.array(conditionalPriceSchema).optional(),
}).optional()

const modelConfigSchema = z.object({
    max_input_tokens: z.number().nonnegative('Max input tokens must be a non-negative number').optional(),
    max_output_tokens: z.number().nonnegative('Max output tokens must be a non-negative number').optional(),
    max_context_tokens: z.number().nonnegative('Max context tokens must be a non-negative number').optional(),
    vision: z.boolean().optional(),
    tool_choice: z.boolean().optional(),
    coder: z.boolean().optional(),
    limited_time_free: z.boolean().optional(),
    support_formats: z.array(z.string()).optional(),
    support_voices: z.array(z.string()).optional(),
}).optional()

export const modelCreateSchema = z.object({
    model: z.string().min(1, 'Model name is required'),
    config: modelConfigSchema,
    owner: z.string().optional(),
    type: z.number().min(0, 'Type is required'),
    rpm: z.number().nonnegative('RPM must be a non-negative number').optional(),
    tpm: z.number().nonnegative('TPM must be a non-negative number').optional(),
    retry_times: z.number().nonnegative('Retry times must be a non-negative number').optional(),
    timeout: z.number().nonnegative('Timeout must be a non-negative number').optional(),
    stream_timeout: z.number().nonnegative('Stream timeout must be a non-negative number').optional(),
    max_error_rate: z.number().min(0, 'Error rate must be at least 0').max(1, 'Error rate must be at most 1').optional(),
    force_save_detail: z.boolean().optional(),
    summary_service_tier: z.boolean().optional(),
    summary_claude_long_context: z.boolean().optional(),
    price: priceSchema,
    plugin: pluginSchema,
})

export type ModelCreateForm = z.infer<typeof modelCreateSchema>
