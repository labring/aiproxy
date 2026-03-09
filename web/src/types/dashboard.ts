export interface ModelSummary {
    timestamp?: number
    channel_id?: number
    group_id?: string
    token_name?: string
    model: string
    used_amount: number
    total_time_milliseconds: number
    total_ttfb_milliseconds: number
    request_count: number
    retry_count: number
    exception_count: number
    status_2xx_count: number
    status_4xx_count: number
    status_5xx_count: number
    status_other_count: number
    status_400_count: number
    status_429_count: number
    status_500_count: number
    cache_hit_count: number
    input_tokens: number
    image_input_tokens: number
    audio_input_tokens: number
    output_tokens: number
    image_output_tokens: number
    cached_tokens: number
    cache_creation_tokens: number
    reasoning_tokens: number
    total_tokens: number
    web_search_count: number
    max_rpm: number
    max_tpm: number
}

export interface TimeSeriesPoint {
    timestamp: number
    summary: ModelSummary[]
}

export interface ChartDataPoint {
    x: string
    xLabel: string
    timestamp: number
    totalCalls: number
    errorCalls: number
    errorRate: number
    status2xxCount: number
    status4xxCount: number
    status5xxCount: number
    statusOtherCount: number
    status400Count: number
    status429Count: number
    status500Count: number
    retryCount: number
    inputTokens: number
    outputTokens: number
    cachedTokens: number
    totalTokens: number
    usedAmount: number
    avgResponseTime: number
    avgTtfb: number
    maxRpm: number
    maxTpm: number
}

export interface DashboardV2Response {
    time_series: TimeSeriesPoint[]
    current_rpm: number
    current_tpm: number
    rpm: number
    tpm: number
}

export interface DashboardFilters {
    model?: string
    channel?: number
    start_timestamp?: number
    end_timestamp?: number
    timezone?: string
    timespan?: 'minute' | 'hour' | 'day' | 'month'
}
