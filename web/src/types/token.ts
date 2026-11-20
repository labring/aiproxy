// src/types/token.ts
export interface Token {
    key: string
    // 仅哈希模式的密钥返回预览，key 本身不可再取回
    key_preview?: string | null
    name: string
    group: string
    subnets: string[] | null
    models: string[] | null
    status: number
    id: number
    quota: number
    used_amount: number
    request_count: number
    created_at: number
    expired_at: number
    accessed_at: number
    // Period quota fields
    period_quota: number
    period_type: string | null
    period_last_update_time: number
    period_last_update_amount: number
}

export interface TokensResponse {
    tokens: Token[]
    total: number
}

export interface TokenCreateRequest {
    name: string
    hashed_key?: boolean
    subnets?: string[]
    quota?: number
    period_quota?: number
    period_type?: string
    // Unix 毫秒，不传表示永不过期
    expired_at?: number
}

export interface TokenUpdateRequest {
    name?: string
    subnets?: string[]
    models?: string[]
    quota?: number
    period_quota?: number
    period_type?: string
    // Unix 毫秒，0 表示清除到期时间
    expired_at?: number
}

export interface TokenStatusRequest {
    status: number
}

// 轮转模式：keep 保持当前存储模式，hash/plain 切换存储模式
export const TOKEN_ROTATE_MODES = {
    KEEP: '',
    HASH: 'hash',
    PLAIN: 'plain',
} as const

export type TokenRotateMode = typeof TOKEN_ROTATE_MODES[keyof typeof TOKEN_ROTATE_MODES]

export const isHashedToken = (token: Pick<Token, 'key_preview'>): boolean => !!token.key_preview

// Period type constants
export const PERIOD_TYPES = {
    DAILY: 'daily',
    WEEKLY: 'weekly',
    MONTHLY: 'monthly'
} as const

export type PeriodType = typeof PERIOD_TYPES[keyof typeof PERIOD_TYPES]