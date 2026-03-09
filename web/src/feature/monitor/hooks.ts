import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { dashboardApi } from '@/api/dashboard'
import { DashboardFilters, DashboardV2Response, TimeSeriesPoint, ModelSummary, ChartDataPoint } from '@/types/dashboard'

export interface DashboardAggregates {
    request_count: number
    exception_count: number
    used_amount: number
    total_time_milliseconds: number
    total_ttfb_milliseconds: number
    input_tokens: number
    output_tokens: number
    cached_tokens: number
    total_tokens: number
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
        existingMap.set(ts.timestamp, ts)
    }

    const result: TimeSeriesPoint[] = []
    for (let t = start; t <= end; t = nextPeriod(t, timespan)) {
        result.push(existingMap.get(t) || { timestamp: t, summary: [] })
    }

    return result
}

function toChartData(timeSeries: TimeSeriesPoint[], timespan?: string, hasModelFilter?: boolean): ChartDataPoint[] {
    return timeSeries.map((ts) => {
        const summary = ts.summary || []
        const totalCalls = summary.reduce((acc, s) => acc + (s.request_count || 0), 0)
        const errorCalls = summary.reduce((acc, s) => acc + (s.exception_count || 0), 0)
        const errorRate = totalCalls === 0 ? 0 : Number(((errorCalls / totalCalls) * 100).toFixed(1))

        const inputTokens = summary.reduce((acc, s) => acc + (s.input_tokens || 0), 0)
        const outputTokens = summary.reduce((acc, s) => acc + (s.output_tokens || 0), 0)
        const cachedTokens = summary.reduce((acc, s) => acc + (s.cached_tokens || 0), 0)
        const totalTokens = summary.reduce((acc, s) => acc + (s.total_tokens || 0), 0)
        const usedAmount = summary.reduce((acc, s) => acc + (s.used_amount || 0), 0)

        const status2xxCount = summary.reduce((acc, s) => acc + (s.status_2xx_count || 0), 0)
        const status4xxCount = summary.reduce((acc, s) => acc + (s.status_4xx_count || 0), 0)
        const status5xxCount = summary.reduce((acc, s) => acc + (s.status_5xx_count || 0), 0)
        const statusOtherCount = summary.reduce((acc, s) => acc + (s.status_other_count || 0), 0)
        const status400Count = summary.reduce((acc, s) => acc + (s.status_400_count || 0), 0)
        const status429Count = summary.reduce((acc, s) => acc + (s.status_429_count || 0), 0)
        const status500Count = summary.reduce((acc, s) => acc + (s.status_500_count || 0), 0)
        const retryCount = summary.reduce((acc, s) => acc + (s.retry_count || 0), 0)

        const successCalls = totalCalls - errorCalls
        const totalTime = summary.reduce((acc, s) => acc + (s.total_time_milliseconds || 0), 0)
        const totalTtfb = summary.reduce((acc, s) => acc + (s.total_ttfb_milliseconds || 0), 0)
        const avgResponseTime = successCalls > 0 ? Math.round((totalTime / successCalls) * 100) / 100 : 0
        const avgTtfb = successCalls > 0 ? Math.round((totalTtfb / successCalls) * 100) / 100 : 0

        const maxRpm = hasModelFilter
            ? summary.reduce((acc, s) => Math.max(acc, s.max_rpm || 0), 0)
            : 0
        const maxTpm = hasModelFilter
            ? summary.reduce((acc, s) => Math.max(acc, s.max_tpm || 0), 0)
            : 0

        const dateFormat = (() => {
            const d = new Date(ts.timestamp * 1000)
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

        const d = new Date(ts.timestamp * 1000)
        const xLabel = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`

        return {
            x: dateFormat,
            xLabel,
            timestamp: ts.timestamp,
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
            outputTokens,
            cachedTokens,
            totalTokens,
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
): DashboardV2Result {
    const timeSeries = response.time_series || []
    const filled = fillMissingPeriods(timeSeries, filters)
    const chartData = toChartData(filled, filters?.timespan, !!filters?.model)

    const agg: DashboardAggregates = {
        request_count: 0,
        exception_count: 0,
        used_amount: 0,
        total_time_milliseconds: 0,
        total_ttfb_milliseconds: 0,
        input_tokens: 0,
        output_tokens: 0,
        cached_tokens: 0,
        total_tokens: 0,
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
    const channelSet = new Set<number>()
    const modelSet = new Set<string>()
    const tokenNameSet = new Set<string>()

    function mergeInto(map: Map<string, ModelSummary>, key: string, s: ModelSummary) {
        const existing = map.get(key)
        if (existing) {
            existing.request_count += s.request_count
            existing.exception_count += s.exception_count
            existing.used_amount += s.used_amount
            existing.total_time_milliseconds += s.total_time_milliseconds
            existing.total_ttfb_milliseconds += s.total_ttfb_milliseconds
            existing.input_tokens += s.input_tokens
            existing.output_tokens += s.output_tokens
            existing.cached_tokens += s.cached_tokens
            existing.total_tokens += s.total_tokens
            if (s.max_rpm > existing.max_rpm) existing.max_rpm = s.max_rpm
            if (s.max_tpm > existing.max_tpm) existing.max_tpm = s.max_tpm
        } else {
            map.set(key, { ...s })
        }
    }

    for (const ts of timeSeries) {
        for (const s of ts.summary) {
            agg.request_count += s.request_count
            agg.exception_count += s.exception_count
            agg.used_amount += s.used_amount
            agg.total_time_milliseconds += s.total_time_milliseconds
            agg.total_ttfb_milliseconds += s.total_ttfb_milliseconds
            agg.input_tokens += s.input_tokens
            agg.output_tokens += s.output_tokens
            agg.cached_tokens += s.cached_tokens
            agg.total_tokens += s.total_tokens
            if (s.max_rpm > agg.max_rpm) agg.max_rpm = s.max_rpm
            if (s.max_tpm > agg.max_tpm) agg.max_tpm = s.max_tpm

            if (s.channel_id) channelSet.add(s.channel_id)
            if (s.model) modelSet.add(s.model)
            if (s.token_name) tokenNameSet.add(s.token_name)

            // Top-level: by model only
            mergeInto(modelRankMap, s.model, s)

            // Detail: by channel_id + token_name + model
            const detailKey = `${s.channel_id || 0}\0${s.token_name || ''}\0${s.model}`
            mergeInto(detailRankMap, detailKey, s)
        }
    }

    // Current RPM/TPM: from backend
    agg.current_rpm = response.current_rpm || 0
    agg.current_tpm = response.current_tpm || 0

    // Avg RPM/TPM: total / minutes in range
    if (filters?.start_timestamp && filters?.end_timestamp) {
        const minutes = Math.max(1, (filters.end_timestamp - filters.start_timestamp) / 60)
        agg.avg_rpm = Math.round(agg.request_count / minutes)
        agg.avg_tpm = Math.round(agg.total_tokens / minutes)
    }

    const sortRanking = (arr: ModelSummary[]) => arr.sort((a, b) => {
        if (b.used_amount !== a.used_amount) return b.used_amount - a.used_amount
        if (b.request_count !== a.request_count) return b.request_count - a.request_count
        return a.model.localeCompare(b.model)
    })

    const modelRanking = sortRanking([...modelRankMap.values()])
    const detailRanking = sortRanking([...detailRankMap.values()])

    const channels = [...channelSet].sort((a, b) => a - b)
    const models = [...new Set(modelRanking.map(m => m.model))]
    const tokenNames = [...tokenNameSet].sort()

    return { timeSeries: filled, chartData, aggregates: agg, modelRanking, detailRanking, channels, models, tokenNames }
}

export const useDashboard = (filters?: DashboardFilters) => {
    const query = useQuery({
        queryKey: ['dashboard', filters],
        queryFn: () => dashboardApi.getDashboardData(filters),
        refetchInterval: 5 * 60 * 1000,
        refetchOnWindowFocus: true,
        retry: false,
    })

    const result = useMemo(() => {
        if (!query.data) return undefined
        return computeDashboardResult(query.data, filters)
    }, [query.data, filters])

    return {
        ...query,
        data: result,
    }
}

export const useGroupDashboard = (group: string, filters?: DashboardFilters & { tokenName?: string }) => {
    const query = useQuery({
        queryKey: ['groupDashboard', group, filters],
        queryFn: () => dashboardApi.getDashboardByGroup(group, filters),
        refetchInterval: 5 * 60 * 1000,
        refetchOnWindowFocus: true,
        retry: false,
    })

    const result = useMemo(() => {
        if (!query.data) return undefined
        return computeDashboardResult(query.data, filters)
    }, [query.data, filters])

    return {
        ...query,
        data: result,
    }
}
