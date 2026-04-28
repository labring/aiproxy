import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { dashboardApi } from '@/api/dashboard'
import { DashboardFilters, DashboardV2Response, TimeSeriesPoint, ModelSummary, ChartDataPoint, SummaryDataSet } from '@/types/dashboard'

export interface DashboardAggregates {
    request_count: number
    exception_count: number
    // Detailed amount fields
    input_amount: number
    image_input_amount: number
    audio_input_amount: number
    output_amount: number
    image_output_amount: number
    cached_amount: number
    cache_creation_amount: number
    web_search_amount: number
    used_amount: number
    total_time_milliseconds: number
    total_ttfb_milliseconds: number
    input_tokens: number
    image_input_tokens: number
    audio_input_tokens: number
    output_tokens: number
    image_output_tokens: number
    cached_tokens: number
    cache_creation_tokens: number
    cache_hit_count: number
    cache_creation_count: number
    reasoning_tokens: number
    total_tokens: number
    web_search_count: number
    current_rpm: number
    current_tpm: number
    avg_rpm: number
    avg_tpm: number
    max_rpm: number
    max_tpm: number
}

export interface DashboardV2Result {
    timeSeries: TimeSeriesPoint[]
    chartData: ChartDataPoint[]
    aggregates: DashboardAggregates
    modelRanking: ModelSummary[]
    detailRanking: ModelSummary[]
    channels: number[]
    models: string[]
    tokenNames: string[]
    // Summary breakdowns
    serviceTierFlex?: SummaryDataSet
    serviceTierPriority?: SummaryDataSet
    claudeLongContext?: SummaryDataSet
}

function withSummaryDefaults(summary?: ModelSummary): ModelSummary {
    return {
        ...summary,
        timestamp: summary?.timestamp || 0,
        channel_id: summary?.channel_id || 0,
        group_id: summary?.group_id || '',
        token_name: summary?.token_name || '',
        model: summary?.model || '',
        input_amount: summary?.input_amount || 0,
        image_input_amount: summary?.image_input_amount || 0,
        audio_input_amount: summary?.audio_input_amount || 0,
        output_amount: summary?.output_amount || 0,
        image_output_amount: summary?.image_output_amount || 0,
        cached_amount: summary?.cached_amount || 0,
        cache_creation_amount: summary?.cache_creation_amount || 0,
        web_search_amount: summary?.web_search_amount || 0,
        used_amount: summary?.used_amount || 0,
        total_time_milliseconds: summary?.total_time_milliseconds || 0,
        total_ttfb_milliseconds: summary?.total_ttfb_milliseconds || 0,
        request_count: summary?.request_count || 0,
        retry_count: summary?.retry_count || 0,
        exception_count: summary?.exception_count || 0,
        status_2xx_count: summary?.status_2xx_count || 0,
        status_4xx_count: summary?.status_4xx_count || 0,
        status_5xx_count: summary?.status_5xx_count || 0,
        status_other_count: summary?.status_other_count || 0,
        status_400_count: summary?.status_400_count || 0,
        status_429_count: summary?.status_429_count || 0,
        status_500_count: summary?.status_500_count || 0,
        cache_hit_count: summary?.cache_hit_count || 0,
        cache_creation_count: summary?.cache_creation_count || 0,
        input_tokens: summary?.input_tokens || 0,
        image_input_tokens: summary?.image_input_tokens || 0,
        audio_input_tokens: summary?.audio_input_tokens || 0,
        output_tokens: summary?.output_tokens || 0,
        image_output_tokens: summary?.image_output_tokens || 0,
        cached_tokens: summary?.cached_tokens || 0,
        cache_creation_tokens: summary?.cache_creation_tokens || 0,
        reasoning_tokens: summary?.reasoning_tokens || 0,
        total_tokens: summary?.total_tokens || 0,
        web_search_count: summary?.web_search_count || 0,
        max_rpm: summary?.max_rpm || 0,
        max_tpm: summary?.max_tpm || 0,
    }
}

function alignTimestamp(timestamp: number, timespan: string): number {
    const d = new Date(timestamp * 1000)
    if (timespan === 'month') {
        d.setDate(1)
        d.setHours(0, 0, 0, 0)
    } else if (timespan === 'day') {
        d.setHours(0, 0, 0, 0)
    } else if (timespan === 'hour') {
        d.setMinutes(0, 0, 0)
    } else if (timespan === 'minute') {
        d.setSeconds(0, 0)
    }
    return Math.floor(d.getTime() / 1000)
}

function nextPeriod(timestamp: number, timespan: string): number {
    if (timespan === 'month') {
        const d = new Date(timestamp * 1000)
        d.setMonth(d.getMonth() + 1)
        return Math.floor(d.getTime() / 1000)
    }
    const stepSeconds = timespan === 'day' ? 86400 : timespan === 'minute' ? 60 : 3600
    return timestamp + stepSeconds
}

function fillMissingPeriods(
    timeSeries: TimeSeriesPoint[],
    filters?: DashboardFilters,
): TimeSeriesPoint[] {
    if (!filters?.start_timestamp || !filters?.end_timestamp || timeSeries.length === 0) {
        return timeSeries
    }

    const timespan = filters.timespan || 'hour'

    const start = alignTimestamp(filters.start_timestamp, timespan)
    const now = Math.floor(Date.now() / 1000)
    const end = Math.min(filters.end_timestamp, now)

    const existingMap = new Map<number, TimeSeriesPoint>()
    for (const ts of timeSeries) {
        existingMap.set(ts?.timestamp || 0, { timestamp: ts?.timestamp || 0, summary: ts?.summary || [] })
    }

    const result: TimeSeriesPoint[] = []
    for (let t = start; t <= end; t = nextPeriod(t, timespan)) {
        result.push(existingMap.get(t) || { timestamp: t, summary: [] })
    }

    return result
}

function toChartData(timeSeries: TimeSeriesPoint[], timespan?: string, hasModelFilter?: boolean): ChartDataPoint[] {
    return timeSeries.map((ts) => {
        const summary = ts?.summary || []
        const totalCalls = summary.reduce((acc, s) => acc + (s?.request_count || 0), 0)
        const errorCalls = summary.reduce((acc, s) => acc + (s?.exception_count || 0), 0)
        const errorRate = totalCalls === 0 ? 0 : Number(((errorCalls / totalCalls) * 100).toFixed(1))

        const inputTokens = summary.reduce((acc, s) => acc + (s?.input_tokens || 0), 0)
        const imageInputTokens = summary.reduce((acc, s) => acc + (s?.image_input_tokens || 0), 0)
        const audioInputTokens = summary.reduce((acc, s) => acc + (s?.audio_input_tokens || 0), 0)
        const outputTokens = summary.reduce((acc, s) => acc + (s?.output_tokens || 0), 0)
        const imageOutputTokens = summary.reduce((acc, s) => acc + (s?.image_output_tokens || 0), 0)
        const cachedTokens = summary.reduce((acc, s) => acc + (s?.cached_tokens || 0), 0)
        const cacheCreationTokens = summary.reduce((acc, s) => acc + (s?.cache_creation_tokens || 0), 0)
        const cacheHitCount = summary.reduce((acc, s) => acc + (s?.cache_hit_count || 0), 0)
        const cacheCreationCount = summary.reduce((acc, s) => acc + (s?.cache_creation_count || 0), 0)
        const reasoningTokens = summary.reduce((acc, s) => acc + (s?.reasoning_tokens || 0), 0)
        const totalTokens = summary.reduce((acc, s) => acc + (s?.total_tokens || 0), 0)
        const webSearchCount = summary.reduce((acc, s) => acc + (s?.web_search_count || 0), 0)

        // Detailed amounts
        const inputAmount = summary.reduce((acc, s) => acc + (s?.input_amount || 0), 0)
        const imageInputAmount = summary.reduce((acc, s) => acc + (s?.image_input_amount || 0), 0)
        const audioInputAmount = summary.reduce((acc, s) => acc + (s?.audio_input_amount || 0), 0)
        const outputAmount = summary.reduce((acc, s) => acc + (s?.output_amount || 0), 0)
        const imageOutputAmount = summary.reduce((acc, s) => acc + (s?.image_output_amount || 0), 0)
        const cachedAmount = summary.reduce((acc, s) => acc + (s?.cached_amount || 0), 0)
        const cacheCreationAmount = summary.reduce((acc, s) => acc + (s?.cache_creation_amount || 0), 0)
        const webSearchAmount = summary.reduce((acc, s) => acc + (s?.web_search_amount || 0), 0)
        const usedAmount = summary.reduce((acc, s) => acc + (s?.used_amount || 0), 0)
        const totalInputAmount = inputAmount + imageInputAmount + audioInputAmount + cachedAmount + cacheCreationAmount
        const totalOutputAmount = outputAmount + imageOutputAmount

        // Non-overlapping text portions (subtract sub-categories from totals)
        const textInputTokens = Math.max(0, inputTokens - imageInputTokens - audioInputTokens - cachedTokens - cacheCreationTokens)
        const textOutputTokens = Math.max(0, outputTokens - imageOutputTokens)

        const status2xxCount = summary.reduce((acc, s) => acc + (s?.status_2xx_count || 0), 0)
        const status4xxCount = summary.reduce((acc, s) => acc + (s?.status_4xx_count || 0), 0)
        const status5xxCount = summary.reduce((acc, s) => acc + (s?.status_5xx_count || 0), 0)
        const statusOtherCount = summary.reduce((acc, s) => acc + (s?.status_other_count || 0), 0)
        const status400Count = summary.reduce((acc, s) => acc + (s?.status_400_count || 0), 0)
        const status429Count = summary.reduce((acc, s) => acc + (s?.status_429_count || 0), 0)
        const status500Count = summary.reduce((acc, s) => acc + (s?.status_500_count || 0), 0)
        const retryCount = summary.reduce((acc, s) => acc + (s?.retry_count || 0), 0)

        const successCalls = totalCalls - errorCalls
        const totalTime = summary.reduce((acc, s) => acc + (s?.total_time_milliseconds || 0), 0)
        const totalTtfb = summary.reduce((acc, s) => acc + (s?.total_ttfb_milliseconds || 0), 0)
        const avgResponseTime = successCalls > 0 ? Math.round((totalTime / successCalls) * 100) / 100 : 0
        const avgTtfb = successCalls > 0 ? Math.round((totalTtfb / successCalls) * 100) / 100 : 0

        const maxRpm = hasModelFilter
            ? summary.reduce((acc, s) => Math.max(acc, s?.max_rpm || 0), 0)
            : 0
        const maxTpm = hasModelFilter
            ? summary.reduce((acc, s) => Math.max(acc, s?.max_tpm || 0), 0)
            : 0

        const dateFormat = (() => {
            const d = new Date((ts?.timestamp || 0) * 1000)
            if (timespan === 'month') {
                return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
            }
            if (timespan === 'day') {
                return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
            }
            if (timespan === 'minute') {
                return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
            }
            return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:00`
        })()

        const d = new Date((ts?.timestamp || 0) * 1000)
        const xLabel = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`

        return {
            x: dateFormat,
            xLabel,
            timestamp: ts?.timestamp || 0,
            totalCalls,
            errorCalls,
            errorRate,
            status2xxCount,
            status4xxCount,
            status5xxCount,
            statusOtherCount,
            status400Count,
            status429Count,
            status500Count,
            retryCount,
            inputTokens,
            textInputTokens,
            imageInputTokens,
            audioInputTokens,
            outputTokens,
            textOutputTokens,
            imageOutputTokens,
            cachedTokens,
            cacheCreationTokens,
            cacheHitCount,
            cacheCreationCount,
            reasoningTokens,
            totalTokens,
            webSearchCount,
            // Detailed amounts
            inputAmount,
            totalInputAmount,
            imageInputAmount,
            audioInputAmount,
            outputAmount,
            totalOutputAmount,
            imageOutputAmount,
            cachedAmount,
            cacheCreationAmount,
            webSearchAmount,
            usedAmount,
            avgResponseTime,
            avgTtfb,
            maxRpm,
            maxTpm,
        }
    })
}

function computeDashboardResult(
    response: DashboardV2Response,
    filters?: DashboardFilters,
    dataSource?: 'total' | 'serviceTierFlex' | 'serviceTierPriority' | 'claudeLongContext',
): DashboardV2Result {
    const originalTimeSeries = response?.time_series || []

    // Transform time series based on data source
    let timeSeries = originalTimeSeries
    let isTransformedDataSource = dataSource && dataSource !== 'total'

    if (isTransformedDataSource) {
        timeSeries = originalTimeSeries.map(ts => {
            const transformedSummary = (ts?.summary || []).map(s => {
                let dataSet: SummaryDataSet | undefined
                switch (dataSource) {
                    case 'serviceTierFlex':
                        dataSet = s?.service_tier_flex
                        break
                    case 'serviceTierPriority':
                        dataSet = s?.service_tier_priority
                        break
                    case 'claudeLongContext':
                        dataSet = s?.claude_long_context
                        break
                }
                // Skip if no data set or no data (request_count is 0 or undefined)
                if (!dataSet || (dataSet.request_count || 0) === 0) return null
                // Map SummaryDataSet fields to ModelSummary format
                // Keep metadata from original but use data set values for metrics
                return {
                    timestamp: s?.timestamp,
                    channel_id: s?.channel_id,
                    group_id: s?.group_id,
                    token_name: s?.token_name,
                    model: s?.model,
                    // Use data set values for all metrics
                    request_count: dataSet.request_count || 0,
                    exception_count: dataSet.exception_count || 0,
                    retry_count: dataSet.retry_count || 0,
                    input_tokens: dataSet.input_tokens || 0,
                    output_tokens: dataSet.output_tokens || 0,
                    total_tokens: dataSet.total_tokens || 0,
                    used_amount: dataSet.used_amount || 0,
                    input_amount: dataSet.input_amount || 0,
                    output_amount: dataSet.output_amount || 0,
                    status_2xx_count: dataSet.status_2xx_count || 0,
                    status_4xx_count: dataSet.status_4xx_count || 0,
                    status_5xx_count: dataSet.status_5xx_count || 0,
                    cache_hit_count: dataSet.cache_hit_count || 0,
                    cached_tokens: dataSet.cached_tokens || 0,
                    // Reset other fields that aren't in SummaryDataSet
                    image_input_tokens: 0,
                    audio_input_tokens: 0,
                    image_output_tokens: 0,
                    cache_creation_tokens: 0,
                    reasoning_tokens: 0,
                    image_input_amount: 0,
                    audio_input_amount: 0,
                    image_output_amount: 0,
                    cached_amount: 0,
                    cache_creation_amount: 0,
                    web_search_amount: 0,
                    total_time_milliseconds: 0,
                    total_ttfb_milliseconds: 0,
                    status_other_count: 0,
                    status_400_count: 0,
                    status_429_count: 0,
                    status_500_count: 0,
                    cache_creation_count: 0,
                    web_search_count: 0,
                    max_rpm: 0,
                    max_tpm: 0,
                }
            }).filter(Boolean) as ModelSummary[]
            return { timestamp: ts?.timestamp, summary: transformedSummary }
        })
    }

    const filled = fillMissingPeriods(timeSeries, filters)
    const chartData = toChartData(filled, filters?.timespan, !!filters?.model)

    const agg: DashboardAggregates = {
        request_count: 0,
        exception_count: 0,
        // Detailed amounts
        input_amount: 0,
        image_input_amount: 0,
        audio_input_amount: 0,
        output_amount: 0,
        image_output_amount: 0,
        cached_amount: 0,
        cache_creation_amount: 0,
        web_search_amount: 0,
        used_amount: 0,
        total_time_milliseconds: 0,
        total_ttfb_milliseconds: 0,
        input_tokens: 0,
        image_input_tokens: 0,
        audio_input_tokens: 0,
        output_tokens: 0,
        image_output_tokens: 0,
        cached_tokens: 0,
        cache_creation_tokens: 0,
        cache_hit_count: 0,
        cache_creation_count: 0,
        reasoning_tokens: 0,
        total_tokens: 0,
        web_search_count: 0,
        current_rpm: 0,
        current_tpm: 0,
        avg_rpm: 0,
        avg_tpm: 0,
        max_rpm: 0,
        max_tpm: 0,
    }

    // Top-level ranking: always aggregate by model only
    const modelRankMap = new Map<string, ModelSummary>()
    // Detail ranking: aggregate by channel_id + token_name + model
    const detailRankMap = new Map<string, ModelSummary>()

    function mergeInto(map: Map<string, ModelSummary>, key: string, s: ModelSummary) {
        const normalized = withSummaryDefaults(s)
        const existing = map.get(key)
        if (existing) {
            existing.request_count = (existing?.request_count || 0) + (normalized?.request_count || 0)
            existing.exception_count = (existing?.exception_count || 0) + (normalized?.exception_count || 0)
            existing.used_amount = (existing?.used_amount || 0) + (normalized?.used_amount || 0)
            existing.input_amount = (existing?.input_amount || 0) + (normalized?.input_amount || 0)
            existing.image_input_amount = (existing?.image_input_amount || 0) + (normalized?.image_input_amount || 0)
            existing.audio_input_amount = (existing?.audio_input_amount || 0) + (normalized?.audio_input_amount || 0)
            existing.output_amount = (existing?.output_amount || 0) + (normalized?.output_amount || 0)
            existing.image_output_amount = (existing?.image_output_amount || 0) + (normalized?.image_output_amount || 0)
            existing.cached_amount = (existing?.cached_amount || 0) + (normalized?.cached_amount || 0)
            existing.cache_creation_amount = (existing?.cache_creation_amount || 0) + (normalized?.cache_creation_amount || 0)
            existing.web_search_amount = (existing?.web_search_amount || 0) + (normalized?.web_search_amount || 0)
            existing.total_time_milliseconds = (existing?.total_time_milliseconds || 0) + (normalized?.total_time_milliseconds || 0)
            existing.total_ttfb_milliseconds = (existing?.total_ttfb_milliseconds || 0) + (normalized?.total_ttfb_milliseconds || 0)
            existing.input_tokens = (existing?.input_tokens || 0) + (normalized?.input_tokens || 0)
            existing.output_tokens = (existing?.output_tokens || 0) + (normalized?.output_tokens || 0)
            existing.cached_tokens = (existing?.cached_tokens || 0) + (normalized?.cached_tokens || 0)
            existing.total_tokens = (existing?.total_tokens || 0) + (normalized?.total_tokens || 0)
            if ((normalized?.max_rpm || 0) > (existing?.max_rpm || 0)) existing.max_rpm = normalized?.max_rpm || 0
            if ((normalized?.max_tpm || 0) > (existing?.max_tpm || 0)) existing.max_tpm = normalized?.max_tpm || 0
        } else {
            map.set(key, normalized)
        }
    }

    // Helper function to merge SummaryDataSet
    const mergeSummaryDataSet = (target: SummaryDataSet | undefined, source: SummaryDataSet | undefined): SummaryDataSet => {
        if (!source) return target || { request_count: 0 }
        if (!target) return { ...source }
        return {
            request_count: (target.request_count || 0) + (source.request_count || 0),
            retry_count: (target.retry_count || 0) + (source.retry_count || 0),
            exception_count: (target.exception_count || 0) + (source.exception_count || 0),
            status_2xx_count: (target.status_2xx_count || 0) + (source.status_2xx_count || 0),
            status_4xx_count: (target.status_4xx_count || 0) + (source.status_4xx_count || 0),
            status_5xx_count: (target.status_5xx_count || 0) + (source.status_5xx_count || 0),
            input_tokens: (target.input_tokens || 0) + (source.input_tokens || 0),
            output_tokens: (target.output_tokens || 0) + (source.output_tokens || 0),
            total_tokens: (target.total_tokens || 0) + (source.total_tokens || 0),
            used_amount: (target.used_amount || 0) + (source.used_amount || 0),
            input_amount: (target.input_amount || 0) + (source.input_amount || 0),
            output_amount: (target.output_amount || 0) + (source.output_amount || 0),
        }
    }

    // Aggregate summary breakdowns from time series (only from original, non-transformed data)
    let serviceTierFlex: SummaryDataSet | undefined
    let serviceTierPriority: SummaryDataSet | undefined
    let claudeLongContext: SummaryDataSet | undefined

    // Use original time series for breakdown aggregation
    const tsForBreakdown = isTransformedDataSource ? originalTimeSeries : timeSeries
    for (const ts of tsForBreakdown) {
        for (const s of ts?.summary || []) {
            // Skip if s is null
            if (!s) continue

            // Aggregate summary breakdowns (only when not transformed)
            if (!isTransformedDataSource) {
                serviceTierFlex = mergeSummaryDataSet(serviceTierFlex, s?.service_tier_flex)
                serviceTierPriority = mergeSummaryDataSet(serviceTierPriority, s?.service_tier_priority)
                claudeLongContext = mergeSummaryDataSet(claudeLongContext, s?.claude_long_context)
            }
        }
    }

    // Aggregate from transformed time series for main stats
    for (const ts of timeSeries) {
        for (const s of ts?.summary || []) {
            // Skip if s is null (can happen with transformed data)
            if (!s) continue

            const normalized = withSummaryDefaults(s)
            agg.request_count += normalized?.request_count || 0
            agg.exception_count += normalized?.exception_count || 0
            agg.used_amount += normalized?.used_amount || 0
            agg.input_amount += normalized?.input_amount || 0
            agg.image_input_amount += normalized?.image_input_amount || 0
            agg.audio_input_amount += normalized?.audio_input_amount || 0
            agg.output_amount += normalized?.output_amount || 0
            agg.image_output_amount += normalized?.image_output_amount || 0
            agg.cached_amount += normalized?.cached_amount || 0
            agg.cache_creation_amount += normalized?.cache_creation_amount || 0
            agg.web_search_amount += normalized?.web_search_amount || 0
            agg.total_time_milliseconds += normalized?.total_time_milliseconds || 0
            agg.total_ttfb_milliseconds += normalized?.total_ttfb_milliseconds || 0
            agg.input_tokens += normalized?.input_tokens || 0
            agg.image_input_tokens += normalized?.image_input_tokens || 0
            agg.audio_input_tokens += normalized?.audio_input_tokens || 0
            agg.output_tokens += normalized?.output_tokens || 0
            agg.image_output_tokens += normalized?.image_output_tokens || 0
            agg.cached_tokens += normalized?.cached_tokens || 0
            agg.cache_creation_tokens += normalized?.cache_creation_tokens || 0
            agg.cache_hit_count += normalized?.cache_hit_count || 0
            agg.cache_creation_count += normalized?.cache_creation_count || 0
            agg.reasoning_tokens += normalized?.reasoning_tokens || 0
            agg.total_tokens += normalized?.total_tokens || 0
            agg.web_search_count += normalized?.web_search_count || 0
            if ((normalized?.max_rpm || 0) > agg.max_rpm) agg.max_rpm = normalized?.max_rpm || 0
            if ((normalized?.max_tpm || 0) > agg.max_tpm) agg.max_tpm = normalized?.max_tpm || 0

            // Top-level: by model only
            mergeInto(modelRankMap, normalized?.model || '', normalized)

            // Detail: by channel_id + token_name + model
            const detailKey = `${normalized?.channel_id || 0}\0${normalized?.token_name || ''}\0${normalized?.model || ''}`
            mergeInto(detailRankMap, detailKey, normalized)
        }
    }

    // Current RPM/TPM: from backend
    agg.current_rpm = response?.rpm || 0
    agg.current_tpm = response?.tpm || 0

    // Avg RPM/TPM: total / active minutes (only periods with data)
    const activePoints = timeSeries.filter(ts => (ts?.summary || []).length > 0).length
    if (activePoints > 0) {
        const timespan = filters?.timespan || 'hour'
        const minutesPerPoint = timespan === 'month' ? 43200 : timespan === 'day' ? 1440 : timespan === 'minute' ? 1 : 60
        const activeMinutes = Math.max(1, activePoints * minutesPerPoint)
        agg.avg_rpm = Math.round((agg.request_count || 0) / activeMinutes)
        agg.avg_tpm = Math.round((agg.total_tokens || 0) / activeMinutes)
    }

    const sortRanking = (arr: ModelSummary[]) => arr.sort((a, b) => {
        if ((b?.used_amount || 0) !== (a?.used_amount || 0)) return (b?.used_amount || 0) - (a?.used_amount || 0)
        if ((b?.request_count || 0) !== (a?.request_count || 0)) return (b?.request_count || 0) - (a?.request_count || 0)
        return (a?.model || '').localeCompare(b?.model || '')
    })

    const modelRanking = sortRanking([...modelRankMap.values()])
    const detailRanking = sortRanking([...detailRankMap.values()])

    const channels = response?.channels || []
    const models = response?.models || []
    const tokenNames = response?.token_names || []

    return { timeSeries: filled, chartData, aggregates: agg, modelRanking, detailRanking, channels, models, tokenNames, serviceTierFlex, serviceTierPriority, claudeLongContext }
}

export type DataSourceMode = 'total' | 'serviceTierFlex' | 'serviceTierPriority' | 'claudeLongContext'

export const useDashboard = (filters?: DashboardFilters, dataSource?: DataSourceMode) => {
    const query = useQuery({
        queryKey: ['dashboard', filters],
        queryFn: () => dashboardApi.getDashboardData(filters),
        refetchInterval: 5 * 60 * 1000,
        refetchOnWindowFocus: true,
        retry: false,
    })

    const result = useMemo(() => {
        if (!query.data) return undefined
        return computeDashboardResult(query.data, filters, dataSource)
    }, [query.data, filters, dataSource])

    return {
        ...query,
        data: result,
    }
}

export const useGroupDashboard = (group: string, filters?: DashboardFilters & { tokenName?: string }, dataSource?: DataSourceMode) => {
    const query = useQuery({
        queryKey: ['groupDashboard', group, filters],
        queryFn: () => dashboardApi.getDashboardByGroup(group, filters),
        refetchInterval: 5 * 60 * 1000,
        refetchOnWindowFocus: true,
        retry: false,
    })

    const result = useMemo(() => {
        if (!query.data) return undefined
        return computeDashboardResult(query.data, filters, dataSource)
    }, [query.data, filters, dataSource])

    return {
        ...query,
        data: result,
    }
}
