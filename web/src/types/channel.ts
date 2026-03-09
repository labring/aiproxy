// src/types/channel.ts
export interface Channel {
    id: number
    type: number
    name: string
    key: string
    base_url?: string
    models: string[]
    model_mapping: Record<string, string> | null
    request_count: number
    retry_count: number
    status: number
    created_at: number
    priority: number
    balance?: number
    used_amount?: number
    sets?: string[]
}

export const DEFAULT_PRIORITY = 10

export interface ChannelTypeMeta {
    name: string
    keyHelp: string
    defaultBaseUrl: string
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
    models: string[]
    model_mapping?: Record<string, string>
    sets?: string[]
    priority?: number
}

export interface ChannelUpdateRequest {
    type: number
    name: string
    key: string
    base_url?: string
    models: string[]
    model_mapping?: Record<string, string>
    sets?: string[]
    priority?: number
}

export interface ChannelStatusRequest {
    status: number
}