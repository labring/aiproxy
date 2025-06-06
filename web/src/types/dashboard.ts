export interface ChartDataPoint {
    cache_creation_tokens: number
    cached_tokens: number
    exception_count: number
    input_tokens: number
    max_rpm: number
    max_rps: number
    max_tpm: number
    max_tps: number
    output_tokens: number
    request_count: number
    timestamp: number
    total_tokens: number
    used_amount: number
    web_search_count: number
}

export interface DashboardData {
    cache_creation_tokens: number
    cached_tokens: number
    channels: number[]
    chart_data: ChartDataPoint[]
    exception_count: number
    input_tokens: number
    max_rpm: number
    max_rps: number
    max_tpm: number
    max_tps: number
    models: string[]
    output_tokens: number
    rpm: number
    total_count: number
    total_tokens: number
    tpm: number
    used_amount: number
    web_search_count: number
}

export interface DashboardFilters {
    keyName?: string
    model?: string
    start_timestamp?: number
    end_timestamp?: number
    timezone?: string
    timespan?: 'day' | 'hour'
} 