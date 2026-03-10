// src/types/token.ts
export interface Token {
    key: string
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
    quota?: number
    period_quota?: number
    period_type?: string
}

export interface TokenUpdateRequest {
    name?: string
    subnets?: string[]
    models?: string[]
    quota?: number
    period_quota?: number
    period_type?: string
}

export interface TokenStatusRequest {
    status: number
}

// Period type constants
export const PERIOD_TYPES = {
    DAILY: 'daily',
    WEEKLY: 'weekly',
    MONTHLY: 'monthly'
} as const

export type PeriodType = typeof PERIOD_TYPES[keyof typeof PERIOD_TYPES]