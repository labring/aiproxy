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
}).optional()

export const modelCreateSchema = z.object({
    model: z.string().min(1, 'Model name is required'),
    type: z.number().min(0, 'Type is required'),
    plugin: pluginSchema,
})

export type ModelCreateForm = z.infer<typeof modelCreateSchema>