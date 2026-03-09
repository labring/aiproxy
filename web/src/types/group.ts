// src/types/group.ts

// Group status constants
export const GROUP_STATUS = {
    ENABLED: 1,
    DISABLED: 2,
    INTERNAL: 3,
} as const

export type GroupStatus = typeof GROUP_STATUS[keyof typeof GROUP_STATUS]

// Re-export price types from model
import type { ModelPrice } from './model'

// Group model config price (alias for ModelPrice)
export type GroupModelConfigPrice = ModelPrice

// Group model config
export interface GroupModelConfig {
    group_id: string
    model: string
    override_limit: boolean
    rpm: number
    tpm: number
    override_price: boolean
    price: GroupModelConfigPrice
    image_prices: Record<string, number>
    override_retry_times: boolean
    retry_times: number
    override_force_save_detail: boolean
    force_save_detail: boolean
}

// Group response from API
export interface Group {
    id: string
    status: GroupStatus
    rpm_ratio: number
    tpm_ratio: number
    used_amount: number
    request_count: number
    available_sets: string[]
    created_at: number
    accessed_at: number
    balance_alert_enabled: boolean
    balance_alert_threshold: number
}

// Groups list response
export interface GroupsResponse {
    groups: Group[]
    total: number
}

// Group create request
export interface GroupCreateRequest {
    rpm_ratio?: number
    tpm_ratio?: number
    available_sets?: string[]
    balance_alert_enabled?: boolean
    balance_alert_threshold?: number
}

// Group update request
export interface GroupUpdateRequest {
    status?: GroupStatus
    rpm_ratio?: number
    tpm_ratio?: number
    available_sets?: string[]
    balance_alert_enabled?: boolean
    balance_alert_threshold?: number
}

// Group status update request
export interface GroupStatusRequest {
    status: GroupStatus
}

// Group dashboard model (from /api/dashboard/:group/models)
export interface GroupDashboardModel {
    created_at?: number
    updated_at?: number
    config?: Record<string, unknown>
    model: string
    owner: string
    type: number
    rpm?: number
    tpm?: number
    image_quality_prices?: Record<string, Record<string, number>>
    image_prices?: Record<string, number>
    price?: ModelPrice
    enabled_plugins?: string[]
}

// Group model config save request
export interface GroupModelConfigSaveRequest {
    model: string
    override_limit?: boolean
    rpm?: number
    tpm?: number
    override_price?: boolean
    price?: Partial<GroupModelConfigPrice>
    image_prices?: Record<string, number>
    override_retry_times?: boolean
    retry_times?: number
    override_force_save_detail?: boolean
    force_save_detail?: boolean
}
