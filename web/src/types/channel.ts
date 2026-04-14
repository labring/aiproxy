// src/types/channel.ts
export interface Channel {
    id: number
    type: number
    name: string
    key: string
    base_url?: string
    proxy_url?: string
    models: string[]
    model_mapping: Record<string, string> | null
    request_count: number
    retry_count: number
    status: number
    created_at: number
    accessed_at: number
    priority: number
    enabled_no_permission_ban?: boolean
    warn_error_rate?: number
    max_error_rate?: number
    balance?: number
    used_amount?: number
    sets?: string[]
    configs?: Record<string, unknown> | null
}

export const DEFAULT_PRIORITY = 10

export interface ChannelConfigSchema {
    type?: string
    title?: string
    description?: string
    enum?: string[]
    properties?: Record<string, ChannelConfigSchema>
    items?: ChannelConfigSchema
    required?: string[]
    default?: unknown
}

export interface ChannelTypeMeta {
    name: string
    keyHelp: string
    defaultBaseUrl: string
    readme?: string
    configSchema?: ChannelConfigSchema
}

export type ChannelTypeMetaMap = Record<string, ChannelTypeMeta>

export interface ChannelsResponse {
    channels: Channel[]
    total: number
}

export interface ChannelCreateRequest {
    type: number
    name: string
    key: string
    base_url?: string
    proxy_url?: string
    models: string[]
    model_mapping?: Record<string, string>
    sets?: string[]
    priority?: number
    enabled_no_permission_ban?: boolean
    warn_error_rate?: number
    max_error_rate?: number
    configs?: Record<string, unknown>
}

export interface ChannelUpdateRequest {
    type: number
    name: string
    key: string
    base_url?: string
    proxy_url?: string
    models: string[]
    model_mapping?: Record<string, string>
    sets?: string[]
    priority?: number
    enabled_no_permission_ban?: boolean
    warn_error_rate?: number
    max_error_rate?: number
    configs?: Record<string, unknown>
}

export interface ChannelStatusRequest {
    status: number
}
