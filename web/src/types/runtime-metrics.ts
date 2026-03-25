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
