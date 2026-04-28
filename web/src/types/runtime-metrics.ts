export interface RuntimeChannelModelMetric {
    rpm: number
    tpm: number
    rps: number
    tps: number
    requests: number
    errors: number
    error_rate: number
    banned: boolean
}

export interface RuntimeModelChannelMetric {
    rpm: number
    tpm: number
    rps: number
    tps: number
    requests: number
    errors: number
    error_rate: number
    banned_models: number
}

export interface RuntimeChannelModelSummary {
    rpm: number
    tpm: number
    rps: number
    tps: number
    requests: number
    errors: number
    error_rate: number
    banned_channels: number
    accessible_sets: string[]
    accessible_groups: number
    channels: Record<string, RuntimeModelChannelMetric>
}

export interface RuntimeChannelSummary {
    rpm: number
    tpm: number
    rps: number
    tps: number
    requests: number
    errors: number
    error_rate: number
    banned_models: number
    models: Record<string, RuntimeChannelModelSummary>
}

export interface RuntimeMetricsResponse {
    models: Record<string, RuntimeChannelModelSummary>
    channels: Record<string, RuntimeChannelSummary>
    channel_models: Record<string, Record<string, RuntimeChannelModelMetric>>
}

export interface GroupSummaryMetricsResponse {
    groups: Record<string, RuntimeRateMetric>
}

export interface GroupTokenMetricsResponse {
    tokens: Record<string, RuntimeRateMetric>
}

export interface GroupModelMetricsResponse {
    models: Record<string, RuntimeRateMetric>
}

export interface GroupTokennameModelMetricsResponseItem extends RuntimeRateMetric {
    group: string
    token_name: string
    model: string
}

export interface GroupTokennameModelMetricsResponse {
    items: GroupTokennameModelMetricsResponseItem[]
}

export interface BatchGroupTokenMetricsRequestItem {
    group: string
    token_name: string
}

export interface BatchGroupTokenMetricsResponseItem extends RuntimeRateMetric {
    group: string
    token_name: string
}

export interface BatchGroupTokenMetricsResponse {
    items: BatchGroupTokenMetricsResponseItem[]
}

export interface RuntimeRateMetric {
    rpm: number
    tpm: number
    rps: number
    tps: number
}

export interface GroupSummaryMetricsQuery {
    groups?: string[]
}
