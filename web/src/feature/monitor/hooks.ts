import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { dashboardApi } from '@/api/dashboard'
import { DashboardFilters, TimeSeriesPoint, ModelSummary, ChartDataPoint } from '@/types/dashboard'

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
    max_rpm: number
    max_tpm: number
}

export interface DashboardV2Result {
    timeSeries: TimeSeriesPoint[]
    chartData: ChartDataPoint[]
    aggregates: DashboardAggregates
    modelRanking: ModelSummary[]
    channels: number[]
    models: string[]
    tokenNames: string[]
}

function fillMissingPeriods(
    timeSeries: TimeSeriesPoint[],
    filters?: DashboardFilters,
): TimeSeriesPoint[] {
    if (!filters?.start_timestamp || !filters?.end_timestamp || timeSeries.length === 0) {
        return timeSeries
    }

    const timespan = filters.timespan || 'hour'
    const stepSeconds = timespan === 'day' ? 86400 : 3600

    const start = filters.start_timestamp
    const now = Math.floor(Date.now() / 1000)
    const end = Math.min(filters.end_timestamp, now)

    const existingMap = new Map<number, TimeSeriesPoint>()
    for (const ts of timeSeries) {
        existingMap.set(ts.timestamp, ts)
    }

    const result: TimeSeriesPoint[] = []
    for (let t = start; t <= end; t += stepSeconds) {
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
            if (timespan === 'day') {
                return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
            }
            return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
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
    timeSeries: TimeSeriesPoint[],
    filters?: DashboardFilters,
    isGroup?: boolean,
): DashboardV2Result {
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
        max_rpm: 0,
        max_tpm: 0,
    }

    const rankingMap = new Map<string, ModelSummary>()
    const channelSet = new Set<number>()
    const modelSet = new Set<string>()
    const tokenNameSet = new Set<string>()

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

            // Global dashboard: group by model (channel_id varies per model)
            // Group dashboard: group by token_name + model
            const rankKey = isGroup && s.token_name
                ? `${s.token_name}\0${s.model}`
                : s.model

            const existing = rankingMap.get(rankKey)
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
                rankingMap.set(rankKey, { ...s })
            }
        }
    }

    const modelRanking = [...rankingMap.values()].sort((a, b) => {
        if (b.used_amount !== a.used_amount) return b.used_amount - a.used_amount
        if (b.request_count !== a.request_count) return b.request_count - a.request_count
        return a.model.localeCompare(b.model)
    })

    const channels = [...channelSet].sort((a, b) => a - b)
    const models = [...new Set(modelRanking.map(m => m.model))]
    const tokenNames = [...tokenNameSet].sort()

    return { timeSeries: filled, chartData, aggregates: agg, modelRanking, channels, models, tokenNames }
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
        return computeDashboardResult(query.data, filters, false)
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
        return computeDashboardResult(query.data, filters, true)
    }, [query.data, filters])

    return {
        ...query,
        data: result,
    }
}
