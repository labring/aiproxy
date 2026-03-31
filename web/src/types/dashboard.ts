export interface ModelSummary {
    timestamp?: number
    channel_id?: number
    group_id?: string
    token_name?: string
    model?: string
    // Detailed amount fields
    input_amount?: number
    image_input_amount?: number
    audio_input_amount?: number
    output_amount?: number
    image_output_amount?: number
    cached_amount?: number
    cache_creation_amount?: number
    web_search_amount?: number
    used_amount?: number
    total_time_milliseconds?: number
    total_ttfb_milliseconds?: number
    request_count?: number
    retry_count?: number
    exception_count?: number
    status_2xx_count?: number
    status_4xx_count?: number
    status_5xx_count?: number
    status_other_count?: number
    status_400_count?: number
    status_429_count?: number
    status_500_count?: number
    cache_hit_count?: number
    cache_creation_count?: number
    input_tokens?: number
    image_input_tokens?: number
    audio_input_tokens?: number
    output_tokens?: number
    image_output_tokens?: number
    cached_tokens?: number
    cache_creation_tokens?: number
    reasoning_tokens?: number
    total_tokens?: number
    web_search_count?: number
    max_rpm?: number
    max_tpm?: number
    // Summary breakdowns (nested in each summary item)
    service_tier_flex?: SummaryDataSet
    service_tier_priority?: SummaryDataSet
    claude_long_context?: SummaryDataSet
}

export interface SummaryDataSet {
    request_count?: number
    retry_count?: number
    exception_count?: number
    status_2xx_count?: number
    status_4xx_count?: number
    status_5xx_count?: number
    status_other_count?: number
    status_400_count?: number
    status_429_count?: number
    status_500_count?: number
    cache_hit_count?: number
    cache_creation_count?: number
    input_tokens?: number
    image_input_tokens?: number
    audio_input_tokens?: number
    output_tokens?: number
    image_output_tokens?: number
    cached_tokens?: number
    cache_creation_tokens?: number
    reasoning_tokens?: number
    total_tokens?: number
    web_search_count?: number
    input_amount?: number
    image_input_amount?: number
    audio_input_amount?: number
    output_amount?: number
    image_output_amount?: number
    cached_amount?: number
    cache_creation_amount?: number
    web_search_amount?: number
    used_amount?: number
    total_time_milliseconds?: number
    total_ttfb_milliseconds?: number
}

export interface TimeSeriesPoint {
    timestamp?: number
    summary?: ModelSummary[]
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
    textInputTokens: number
    imageInputTokens: number
    audioInputTokens: number
    outputTokens: number
    textOutputTokens: number
    imageOutputTokens: number
    cachedTokens: number
    cacheCreationTokens: number
    cacheHitCount: number
    cacheCreationCount: number
    reasoningTokens: number
    totalTokens: number
    webSearchCount: number
    // Detailed amounts
    inputAmount: number
    totalInputAmount: number
    imageInputAmount: number
    audioInputAmount: number
    outputAmount: number
    totalOutputAmount: number
    imageOutputAmount: number
    cachedAmount: number
    cacheCreationAmount: number
    webSearchAmount: number
    usedAmount: number
    avgResponseTime: number
    avgTtfb: number
    maxRpm: number
    maxTpm: number
}

export interface DashboardV2Response {
    time_series?: TimeSeriesPoint[]
    rpm?: number
    tpm?: number
    total_count?: number
    request_count?: number
    retry_count?: number
    exception_count?: number
    input_tokens?: number
    output_tokens?: number
    total_tokens?: number
    used_amount?: number
    channels?: number[]
    models?: string[]
    token_names?: string[]
}

export interface DashboardFilters {
    model?: string
    channel?: number
    start_timestamp?: number
    end_timestamp?: number
    timezone?: string
    timespan?: 'minute' | 'hour' | 'day' | 'month'
}
