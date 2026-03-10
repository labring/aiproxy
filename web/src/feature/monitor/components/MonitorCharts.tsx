import { Fragment, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { EChartsOption } from 'echarts'

import { EChart } from '@/components/ui/echarts'
import { Skeleton } from '@/components/ui/skeleton'
import { useTheme } from '@/handler/ThemeContext'
import { ChartDataPoint, ModelSummary } from '@/types/dashboard'
import { cn } from '@/lib/utils'
import { ChevronRight } from 'lucide-react'
import { channelApi } from '@/api/channel'
import { useChannelTypeMetas } from '@/feature/channel/hooks'
import { ChannelLabel } from '@/components/common/ChannelLabel'
import { ChannelDialog } from '@/feature/channel/components/ChannelDialog'
import type { Channel } from '@/types/channel'

interface MonitorChartsProps {
    chartData: ChartDataPoint[]
    modelRanking: ModelSummary[]
    detailRanking?: ModelSummary[]
    hasModelFilter?: boolean
    isGroup?: boolean
    loading?: boolean
}

type DisplayMode = 'incremental' | 'cumulative'
type TokenChartMode = 'breakdown' | 'total'

function ToggleGroup({ value, onChange, options }: {
    value: string
    onChange: (v: string) => void
    options: { label: string; value: string }[]
}) {
    return (
        <div className="flex bg-muted rounded-md p-0.5 text-xs">
            {options.map((opt) => (
                <button
                    key={opt.value}
                    className={cn(
                        "px-2 py-0.5 rounded transition-colors",
                        value === opt.value
                            ? "bg-background shadow-sm text-foreground font-medium"
                            : "text-muted-foreground hover:text-foreground"
                    )}
                    onClick={() => onChange(opt.value)}
                >
                    {opt.label}
                </button>
            ))}
        </div>
    )
}

function ChartBox({ title, children, rightSlot, className }: {
    title: string
    children: React.ReactNode
    rightSlot?: React.ReactNode
    className?: string
}) {
    return (
        <div className={cn("bg-card rounded-lg border p-4 h-[300px] overflow-hidden", className)}>
            <div className="flex items-start justify-between mb-2">
                <span className="text-sm font-medium text-foreground">{title}</span>
                {rightSlot && <div className="flex items-center gap-2">{rightSlot}</div>}
            </div>
            <div className="h-[calc(100%-28px)]">
                {children}
            </div>
        </div>
    )
}

export function MonitorCharts({ chartData, modelRanking, detailRanking = [], hasModelFilter = false, isGroup = false, loading = false }: MonitorChartsProps) {
    const { t } = useTranslation()
    const { theme } = useTheme()
    const { data: typeMetas } = useChannelTypeMetas()

    // Channel edit dialog state
    const [channelDialogOpen, setChannelDialogOpen] = useState(false)
    const [editingChannel, setEditingChannel] = useState<Channel | null>(null)

    const openChannelEdit = (channelId: number) => {
        channelApi.getChannel(channelId)
            .then(channel => {
                setEditingChannel(channel)
                setChannelDialogOpen(true)
            })
            .catch(() => {})
    }

    const [requestsMode, setRequestsMode] = useState<DisplayMode>('incremental')
    const [tokensMode, setTokensMode] = useState<DisplayMode>('incremental')
    const [tokenChartMode, setTokenChartMode] = useState<TokenChartMode>('breakdown')
    const [costMode, setCostMode] = useState<DisplayMode>('incremental')
    const [costBreakdownMode, setCostBreakdownMode] = useState<DisplayMode>('incremental')

    const isDarkMode = useMemo(() => {
        if (theme === 'dark') return true
        if (theme === 'light') return false
        return window.matchMedia('(prefers-color-scheme: dark)').matches
    }, [theme])

    const themeColors = useMemo(() => ({
        textColor: isDarkMode ? '#e5e7eb' : '#666',
        axisLineColor: isDarkMode ? '#374151' : '#e1e4e8',
        splitLineColor: isDarkMode ? '#374151' : '#f0f0f0',
        tooltipBg: isDarkMode ? 'rgba(31, 41, 55, 0.95)' : 'rgba(255, 255, 255, 0.95)',
        tooltipBorder: isDarkMode ? '#4b5563' : '#e1e4e8',
        tooltipTextColor: isDarkMode ? '#f3f4f6' : '#333',
    }), [isDarkMode])

    const xLabels = useMemo(() => chartData.map(d => d.x), [chartData])

    const modeOptions = useMemo(() => [
        { label: t('monitor.charts.incremental'), value: 'incremental' },
        { label: t('monitor.charts.cumulative'), value: 'cumulative' },
    ], [t])

    const tokenChartOptions = useMemo(() => [
        { label: t('monitor.charts.tokenTypes.breakdown'), value: 'breakdown' },
        { label: t('monitor.charts.tokenTypes.total'), value: 'total' },
    ], [t])

    function makeData(key: keyof ChartDataPoint, mode: DisplayMode): number[] {
        const raw = chartData.map(d => d[key] as number)
        if (mode === 'incremental') return raw
        const cumulative: number[] = []
        raw.forEach((v, i) => cumulative.push(i === 0 ? v : cumulative[i - 1] + v))
        return cumulative
    }

    const defaultAxisFormatter = (v: number) => {
        if (v >= 1000000) return (v / 1000000).toFixed(1).replace(/\.0$/, '') + 'M'
        if (v >= 1000) return (v / 1000).toFixed(1).replace(/\.0$/, '') + 'K'
        return String(v)
    }

    const msAxisFormatter = (v: number) => {
        if (v >= 1000) return (v / 1000).toFixed(1).replace(/\.0$/, '') + 's'
        return v + 'ms'
    }

    function buildAreaChart(
        dataKey: keyof ChartDataPoint,
        color: string,
        mode: DisplayMode,
        opts?: {
            formatter?: (v: number) => string
            yAxisFormatter?: (v: number) => string
        }
    ): EChartsOption {
        const data = makeData(dataKey, mode)
        return {
            backgroundColor: 'transparent',
            tooltip: {
                trigger: 'axis',
                backgroundColor: themeColors.tooltipBg,
                borderColor: themeColors.tooltipBorder,
                borderWidth: 1,
                borderRadius: 8,
                textStyle: { color: themeColors.tooltipTextColor, fontSize: 12 },
                formatter: (params: any) => {
                    const p = Array.isArray(params) ? params[0] : params
                    const idx = p.dataIndex
                    const point = chartData[idx]
                    const val = opts?.formatter ? opts.formatter(p.value) : Number(p.value).toLocaleString()
                    return `<div style="font-size:12px"><div style="margin-bottom:4px">${point?.xLabel || point?.x}</div><div>${val}</div></div>`
                }
            },
            grid: { left: 10, right: 10, bottom: 0, top: 10, containLabel: true },
            xAxis: {
                type: 'category',
                boundaryGap: false,
                data: xLabels,
                axisLine: { lineStyle: { color: themeColors.axisLineColor } },
                axisLabel: { color: themeColors.textColor, fontSize: 11 },
                axisTick: { show: false },
            },
            yAxis: {
                type: 'value',
                axisLine: { show: false },
                axisLabel: {
                    color: themeColors.textColor,
                    fontSize: 11,
                    formatter: opts?.yAxisFormatter || defaultAxisFormatter,
                },
                axisTick: { show: false },
                splitLine: { lineStyle: { color: themeColors.splitLineColor, type: 'dashed' } },
            },
            series: [{
                type: 'line',
                smooth: true,
                showSymbol: false,
                lineStyle: { width: 2, color },
                itemStyle: { color },
                areaStyle: {
                    color: {
                        type: 'linear',
                        x: 0, y: 0, x2: 0, y2: 1,
                        colorStops: [
                            { offset: 0, color: color + (isDarkMode ? '50' : '40') },
                            { offset: 1, color: color + '05' },
                        ],
                    },
                },
                data,
            }],
            animation: true,
            animationDuration: 600,
        }
    }

    const [expandedModels, setExpandedModels] = useState<Set<string>>(new Set())

    const toggleExpand = (model: string) => {
        setExpandedModels(prev => {
            const next = new Set(prev)
            if (next.has(model)) next.delete(model)
            else next.add(model)
            return next
        })
    }

    interface TableRow {
        model: string
        tokenName: string
        channelId: number
        totalCalls: number
        errorCalls: number
        usedAmount: number
        avgResponseTime: number
        avgTtfb: number
    }

    const toRow = (m: ModelSummary): TableRow => {
        const successCalls = (m?.request_count || 0) - (m?.exception_count || 0)
        return {
            model: m?.model || '',
            tokenName: m?.token_name || '',
            channelId: m?.channel_id || 0,
            totalCalls: m?.request_count || 0,
            errorCalls: m?.exception_count || 0,
            usedAmount: m?.used_amount || 0,
            avgResponseTime: successCalls > 0 ? (m?.total_time_milliseconds || 0) / successCalls : 0,
            avgTtfb: successCalls > 0 ? (m?.total_ttfb_milliseconds || 0) / successCalls : 0,
        }
    }

    const tableData = useMemo(() => (modelRanking || []).map(toRow), [modelRanking])

    // Build detail rows grouped by model
    const detailByModel = useMemo(() => {
        const map = new Map<string, TableRow[]>()
        for (const m of detailRanking) {
            const modelKey = m?.model || ''
            const rows = map.get(modelKey) || []
            rows.push(toRow(m))
            map.set(modelKey, rows)
        }
        return map
    }, [detailRanking])

    const hasDetailData = detailRanking.length > 0

    // Batch fetch channel info for detail rows
    const [channelInfoMap, setChannelInfoMap] = useState<Record<number, { name: string; type: number }>>({})
    const detailChannelIds = useMemo(() => {
        const ids = new Set<number>()
        for (const m of detailRanking) {
            if (m?.channel_id) ids.add(m?.channel_id || 0)
        }
        return [...ids]
    }, [detailRanking])

    useEffect(() => {
        if (detailChannelIds.length === 0) return
        const missing = detailChannelIds.filter(id => !(id in channelInfoMap))
        if (missing.length === 0) return
        channelApi.getChannelBatchInfo(missing)
            .then(infos => {
                setChannelInfoMap(prev => {
                    const next = { ...prev }
                    for (const info of infos) next[info.id] = { name: info.name, type: info.type }
                    return next
                })
            })
            .catch(() => {
                setChannelInfoMap(prev => {
                    const next = { ...prev }
                    for (const id of missing) {
                        if (!(id in next)) next[id] = { name: `#${id}`, type: 0 }
                    }
                    return next
                })
            })
    }, [detailChannelIds]) // eslint-disable-line react-hooks/exhaustive-deps

    if (loading) {
        return (
            <div className="space-y-4">
                <Skeleton className="w-full h-[300px] rounded-lg" />
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                    <Skeleton className="h-[300px] rounded-lg" />
                    <Skeleton className="h-[300px] rounded-lg" />
                </div>
            </div>
        )
    }

    return (
        <div className="space-y-4">
            {/* Calls Overview + Error Rate - 2 columns */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <ChartBox
                    title={t('monitor.charts.callsOverview')}
                    rightSlot={<ToggleGroup value={requestsMode} onChange={(v) => setRequestsMode(v as DisplayMode)} options={modeOptions} />}
                >
                    <EChart
                        option={(() => {
                            const callsSeries = [
                                {
                                    name: t('monitor.charts.totalCalls'),
                                    color: '#3b82f6',
                                    data: makeData('totalCalls', requestsMode),
                                },
                                {
                                    name: t('monitor.charts.errorCalls'),
                                    color: '#ef4444',
                                    data: makeData('errorCalls', requestsMode),
                                },
                                ...(!isGroup ? [{
                                    name: t('monitor.charts.retryCount'),
                                    color: '#8b5cf6',
                                    data: makeData('retryCount', requestsMode),
                                }] : []),
                                {
                                    name: t('monitor.charts.cacheHitCount'),
                                    color: '#14b8a6',
                                    data: makeData('cacheHitCount', requestsMode),
                                },
                                {
                                    name: t('monitor.charts.cacheCreationCount'),
                                    color: '#6366f1',
                                    data: makeData('cacheCreationCount', requestsMode),
                                },
                                {
                                    name: t('monitor.charts.webSearchCount'),
                                    color: '#0ea5e9',
                                    data: makeData('webSearchCount', requestsMode),
                                },
                            ]
                            const colors = callsSeries.map(s => s.color)
                            return {
                                backgroundColor: 'transparent',
                                color: colors,
                                tooltip: {
                                    trigger: 'axis',
                                    backgroundColor: themeColors.tooltipBg,
                                    borderColor: themeColors.tooltipBorder,
                                    borderWidth: 1,
                                    borderRadius: 8,
                                    textStyle: { color: themeColors.tooltipTextColor, fontSize: 12 },
                                    formatter: (params: any) => {
                                        const ps = Array.isArray(params) ? params : [params]
                                        const idx = ps[0]?.dataIndex
                                        const point = chartData[idx]
                                        let html = `<div style="font-size:12px"><div style="margin-bottom:4px">${point?.xLabel || point?.x}</div>`
                                        for (const p of ps) {
                                            html += `<div>${p.marker} ${p.seriesName}: ${Number(p.value).toLocaleString()}</div>`
                                        }
                                        html += '</div>'
                                        return html
                                    }
                                },
                                legend: {
                                    bottom: 0,
                                    textStyle: { color: themeColors.textColor, fontSize: 11 },
                                    itemWidth: 12, itemHeight: 8,
                                },
                                grid: { left: 10, right: 10, bottom: 28, top: 10, containLabel: true },
                                xAxis: {
                                    type: 'category',
                                    boundaryGap: false,
                                    data: xLabels,
                                    axisLine: { lineStyle: { color: themeColors.axisLineColor } },
                                    axisLabel: { color: themeColors.textColor, fontSize: 11 },
                                    axisTick: { show: false },
                                },
                                yAxis: {
                                    type: 'value',
                                    axisLine: { show: false },
                                    axisLabel: { color: themeColors.textColor, fontSize: 11, formatter: defaultAxisFormatter },
                                    axisTick: { show: false },
                                    splitLine: { lineStyle: { color: themeColors.splitLineColor, type: 'dashed' } },
                                },
                                series: callsSeries.map(s => ({
                                    name: s.name,
                                    type: 'line' as const,
                                    smooth: true,
                                    showSymbol: false,
                                    lineStyle: { width: 2, color: s.color },
                                    itemStyle: { color: s.color },
                                    areaStyle: {
                                        color: {
                                            type: 'linear' as const, x: 0, y: 0, x2: 0, y2: 1,
                                            colorStops: [
                                                { offset: 0, color: s.color + (isDarkMode ? '30' : '20') },
                                                { offset: 1, color: s.color + '05' },
                                            ],
                                        },
                                    },
                                    data: s.data,
                                })),
                                animation: true,
                                animationDuration: 600,
                            }
                        })()}
                        style={{ width: '100%', height: '100%' }}
                    />
                </ChartBox>
                <ChartBox title={t('monitor.charts.errorRate')}>
                    <EChart
                        option={buildAreaChart('errorRate', '#ef4444', 'incremental', {
                            formatter: (v) => `${v}%`
                        })}
                        style={{ width: '100%', height: '100%' }}
                    />
                </ChartBox>
            </div>

            {/* HTTP Status - full width */}
            <ChartBox title={t('monitor.charts.httpStatus')}>
                <EChart
                        option={(() => {
                            const statusSeries: { name: string; color: string; data: number[] }[] = [
                                { name: '2xx', color: '#22c55e', data: chartData.map(d => d.status2xxCount) },
                                { name: '400', color: '#f59e0b', data: chartData.map(d => d.status400Count) },
                                { name: '429', color: '#f97316', data: chartData.map(d => d.status429Count) },
                                { name: t('monitor.charts.other4xx'), color: '#eab308', data: chartData.map(d => Math.max(0, d.status4xxCount - d.status400Count - d.status429Count)) },
                                { name: '500', color: '#ef4444', data: chartData.map(d => d.status500Count) },
                                { name: t('monitor.charts.other5xx'), color: '#dc2626', data: chartData.map(d => Math.max(0, d.status5xxCount - d.status500Count)) },
                                { name: t('monitor.charts.statusOther'), color: '#6b7280', data: chartData.map(d => d.statusOtherCount) },
                            ]
                            return {
                                backgroundColor: 'transparent',
                                color: statusSeries.map(s => s.color),
                                tooltip: {
                                    trigger: 'axis',
                                    backgroundColor: themeColors.tooltipBg,
                                    borderColor: themeColors.tooltipBorder,
                                    borderWidth: 1,
                                    borderRadius: 8,
                                    textStyle: { color: themeColors.tooltipTextColor, fontSize: 12 },
                                    formatter: (params: any) => {
                                        const ps = Array.isArray(params) ? params : [params]
                                        const idx = ps[0]?.dataIndex
                                        const point = chartData[idx]
                                        let html = `<div style="font-size:12px"><div style="margin-bottom:4px">${point?.xLabel || point?.x}</div>`
                                        for (const p of ps) {
                                            if (p.value > 0) {
                                                html += `<div>${p.marker} ${p.seriesName}: ${Number(p.value).toLocaleString()}</div>`
                                            }
                                        }
                                        html += '</div>'
                                        return html
                                    }
                                },
                                legend: {
                                    bottom: 0,
                                    textStyle: { color: themeColors.textColor, fontSize: 11 },
                                    itemWidth: 12, itemHeight: 8,
                                },
                                grid: { left: 10, right: 10, bottom: 28, top: 10, containLabel: true },
                                xAxis: {
                                    type: 'category',
                                    boundaryGap: false,
                                    data: xLabels,
                                    axisLine: { lineStyle: { color: themeColors.axisLineColor } },
                                    axisLabel: { color: themeColors.textColor, fontSize: 11 },
                                    axisTick: { show: false },
                                },
                                yAxis: {
                                    type: 'value',
                                    axisLine: { show: false },
                                    axisLabel: { color: themeColors.textColor, fontSize: 11, formatter: defaultAxisFormatter },
                                    axisTick: { show: false },
                                    splitLine: { lineStyle: { color: themeColors.splitLineColor, type: 'dashed' } },
                                },
                                series: statusSeries.map(s => ({
                                    name: s.name,
                                    type: 'line' as const,
                                    smooth: true,
                                    showSymbol: false,
                                    lineStyle: { width: 1.5, color: s.color },
                                    itemStyle: { color: s.color },
                                    areaStyle: { color: s.color + (isDarkMode ? '40' : '30') },
                                    data: s.data,
                                })),
                                animation: true,
                                animationDuration: 600,
                            }
                        })()}
                        style={{ width: '100%', height: '100%' }}
                    />
            </ChartBox>

            {/* Token Usage - full width with breakdown/total switcher */}
            <ChartBox
                title={t('monitor.charts.tokenUsage')}
                rightSlot={
                    <>
                        <ToggleGroup value={tokenChartMode} onChange={(v) => setTokenChartMode(v as TokenChartMode)} options={tokenChartOptions} />
                        <ToggleGroup value={tokensMode} onChange={(v) => setTokensMode(v as DisplayMode)} options={modeOptions} />
                    </>
                }
            >
                {tokenChartMode === 'total' ? (
                    <EChart
                        option={buildAreaChart('totalTokens', '#3b82f6', tokensMode)}
                        style={{ width: '100%', height: '100%' }}
                    />
                ) : (
                    <EChart
                        option={(() => {
                            const tokenSeries: { key: keyof ChartDataPoint; name: string; color: string }[] = [
                                { key: 'textInputTokens', name: t('monitor.charts.tokensBreakdown.input'), color: '#3b82f6' },
                                { key: 'cachedTokens', name: t('monitor.charts.tokensBreakdown.cached'), color: '#6366f1' },
                                { key: 'cacheCreationTokens', name: t('monitor.charts.tokensBreakdown.cacheCreation'), color: '#a78bfa' },
                                { key: 'imageInputTokens', name: t('monitor.charts.tokensBreakdown.imageInput'), color: '#06b6d4' },
                                { key: 'audioInputTokens', name: t('monitor.charts.tokensBreakdown.audioInput'), color: '#8b5cf6' },
                                { key: 'textOutputTokens', name: t('monitor.charts.tokensBreakdown.output'), color: '#10b981' },
                                { key: 'imageOutputTokens', name: t('monitor.charts.tokensBreakdown.imageOutput'), color: '#14b8a6' },
                            ]
                            // Filter out series that have no data at all
                            const activeSeries = tokenSeries.filter(s =>
                                chartData.some(d => (d[s.key] as number) > 0)
                            )
                            return {
                                backgroundColor: 'transparent',
                                color: activeSeries.map(s => s.color),
                                tooltip: {
                                    trigger: 'axis',
                                    backgroundColor: themeColors.tooltipBg,
                                    borderColor: themeColors.tooltipBorder,
                                    borderWidth: 1,
                                    borderRadius: 8,
                                    textStyle: { color: themeColors.tooltipTextColor, fontSize: 12 },
                                    formatter: (params: any) => {
                                        const ps = Array.isArray(params) ? params : [params]
                                        const idx = ps[0]?.dataIndex
                                        const point = chartData[idx]
                                        let html = `<div style="font-size:12px"><div style="margin-bottom:4px">${point?.xLabel || point?.x}</div>`
                                        for (const p of ps) {
                                            if (p.value > 0) {
                                                html += `<div>${p.marker} ${p.seriesName}: ${Number(p.value).toLocaleString()}</div>`
                                            }
                                        }
                                        html += '</div>'
                                        return html
                                    }
                                },
                                legend: {
                                    bottom: 0,
                                    textStyle: { color: themeColors.textColor, fontSize: 11 },
                                    itemWidth: 12, itemHeight: 8,
                                },
                                grid: { left: 10, right: 10, bottom: 28, top: 10, containLabel: true },
                                xAxis: {
                                    type: 'category',
                                    boundaryGap: false,
                                    data: xLabels,
                                    axisLine: { lineStyle: { color: themeColors.axisLineColor } },
                                    axisLabel: { color: themeColors.textColor, fontSize: 11 },
                                    axisTick: { show: false },
                                },
                                yAxis: {
                                    type: 'value',
                                    axisLine: { show: false },
                                    axisLabel: { color: themeColors.textColor, fontSize: 11, formatter: defaultAxisFormatter },
                                    axisTick: { show: false },
                                    splitLine: { lineStyle: { color: themeColors.splitLineColor, type: 'dashed' } },
                                },
                                series: activeSeries.map(s => ({
                                    name: s.name,
                                    type: 'line' as const,
                                    smooth: true,
                                    showSymbol: false,
                                    lineStyle: { width: 1.5, color: s.color },
                                    itemStyle: { color: s.color },
                                    areaStyle: { color: s.color + (isDarkMode ? '40' : '30') },
                                    data: makeData(s.key, tokensMode),
                                })),
                                animation: true,
                                animationDuration: 600,
                            }
                        })()}
                        style={{ width: '100%', height: '100%' }}
                    />
                )}
            </ChartBox>

            {/* Cost - full width */}
            <ChartBox
                title={t('monitor.charts.costTrend')}
                rightSlot={<ToggleGroup value={costMode} onChange={(v) => setCostMode(v as DisplayMode)} options={modeOptions} />}
            >
                <EChart
                    option={buildAreaChart('usedAmount', '#8b5cf6', costMode, {
                        formatter: (v) => `$${v.toFixed(4)}`
                    })}
                    style={{ width: '100%', height: '100%' }}
                />
            </ChartBox>

            {/* Cost Breakdown - full width */}
            <ChartBox
                title={t('monitor.charts.costBreakdown')}
                rightSlot={<ToggleGroup value={costBreakdownMode} onChange={(v) => setCostBreakdownMode(v as DisplayMode)} options={modeOptions} />}
            >
                <EChart
                    option={(() => {
                        const costBreakdownSeries: { key: keyof ChartDataPoint; name: string; color: string }[] = [
                            { key: 'inputAmount', name: t('monitor.charts.costBreakdownTypes.input'), color: '#3b82f6' },
                            { key: 'cachedAmount', name: t('monitor.charts.costBreakdownTypes.cached'), color: '#6366f1' },
                            { key: 'cacheCreationAmount', name: t('monitor.charts.costBreakdownTypes.cacheCreation'), color: '#a78bfa' },
                            { key: 'imageInputAmount', name: t('monitor.charts.costBreakdownTypes.imageInput'), color: '#06b6d4' },
                            { key: 'audioInputAmount', name: t('monitor.charts.costBreakdownTypes.audioInput'), color: '#8b5cf6' },
                            { key: 'outputAmount', name: t('monitor.charts.costBreakdownTypes.output'), color: '#10b981' },
                            { key: 'imageOutputAmount', name: t('monitor.charts.costBreakdownTypes.imageOutput'), color: '#14b8a6' },
                            { key: 'webSearchAmount', name: t('monitor.charts.costBreakdownTypes.webSearch'), color: '#0ea5e9' },
                        ]
                        // Filter out series that have no data at all
                        const activeSeries = costBreakdownSeries.filter(s =>
                            chartData.some(d => (d[s.key] as number) > 0)
                        )
                        return {
                            backgroundColor: 'transparent',
                            color: activeSeries.map(s => s.color),
                            tooltip: {
                                trigger: 'axis',
                                backgroundColor: themeColors.tooltipBg,
                                borderColor: themeColors.tooltipBorder,
                                borderWidth: 1,
                                borderRadius: 8,
                                textStyle: { color: themeColors.tooltipTextColor, fontSize: 12 },
                                formatter: (params: any) => {
                                    const ps = Array.isArray(params) ? params : [params]
                                    const idx = ps[0]?.dataIndex
                                    const point = chartData[idx]
                                    let html = `<div style="font-size:12px"><div style="margin-bottom:4px">${point?.xLabel || point?.x}</div>`
                                    for (const p of ps) {
                                        if (p.value > 0) {
                                            html += `<div>${p.marker} ${p.seriesName}: $${Number(p.value).toFixed(4)}</div>`
                                        }
                                    }
                                    html += '</div>'
                                    return html
                                }
                            },
                            legend: {
                                bottom: 0,
                                textStyle: { color: themeColors.textColor, fontSize: 11 },
                                itemWidth: 12, itemHeight: 8,
                            },
                            grid: { left: 10, right: 10, bottom: 28, top: 10, containLabel: true },
                            xAxis: {
                                type: 'category',
                                boundaryGap: false,
                                data: xLabels,
                                axisLine: { lineStyle: { color: themeColors.axisLineColor } },
                                axisLabel: { color: themeColors.textColor, fontSize: 11 },
                                axisTick: { show: false },
                            },
                            yAxis: {
                                type: 'value',
                                axisLine: { show: false },
                                axisLabel: {
                                    color: themeColors.textColor,
                                    fontSize: 11,
                                    formatter: (v: number) => `$${v.toFixed(4)}`,
                                },
                                axisTick: { show: false },
                                splitLine: { lineStyle: { color: themeColors.splitLineColor, type: 'dashed' } },
                            },
                            series: activeSeries.map(s => ({
                                name: s.name,
                                type: 'line' as const,
                                smooth: true,
                                showSymbol: false,
                                lineStyle: { width: 1.5, color: s.color },
                                itemStyle: { color: s.color },
                                areaStyle: { color: s.color + (isDarkMode ? '40' : '30') },
                                data: makeData(s.key, costBreakdownMode),
                            })),
                            animation: true,
                            animationDuration: 600,
                        }
                    })()}
                    style={{ width: '100%', height: '100%' }}
                />
            </ChartBox>

            {/* Response Time + TTFB - 2 columns */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <ChartBox title={t('monitor.charts.avgResponseTime')}>
                    <EChart
                        option={buildAreaChart('avgResponseTime', '#10b981', 'incremental', {
                            formatter: (v) => v >= 1000 ? `${(v / 1000).toFixed(2)}s` : `${v.toFixed(0)}ms`,
                            yAxisFormatter: msAxisFormatter,
                        })}
                        style={{ width: '100%', height: '100%' }}
                    />
                </ChartBox>
                <ChartBox title={t('monitor.charts.avgTtfb')}>
                    <EChart
                        option={buildAreaChart('avgTtfb', '#ef4444', 'incremental', {
                            formatter: (v) => v >= 1000 ? `${(v / 1000).toFixed(2)}s` : `${v.toFixed(0)}ms`,
                            yAxisFormatter: msAxisFormatter,
                        })}
                        style={{ width: '100%', height: '100%' }}
                    />
                </ChartBox>
            </div>

            {/* Max RPM + TPM - 2 columns, only when specific model is selected */}
            {hasModelFilter && (
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                    <ChartBox title={t('monitor.charts.maxRpm')}>
                        <EChart
                            option={buildAreaChart('maxRpm', '#6366f1', 'incremental')}
                            style={{ width: '100%', height: '100%' }}
                        />
                    </ChartBox>
                    <ChartBox title={t('monitor.charts.maxTpm')}>
                        <EChart
                            option={buildAreaChart('maxTpm', '#f97316', 'incremental')}
                            style={{ width: '100%', height: '100%' }}
                        />
                    </ChartBox>
                </div>
            )}

            {/* Model Data Table */}
            {tableData.length > 0 && (
                <div className="bg-card rounded-lg border overflow-hidden">
                    <div className="p-4 border-b">
                        <span className="text-sm font-medium text-foreground">{t('monitor.charts.modelRanking')}</span>
                    </div>
                    <div className="overflow-x-auto">
                        <table className="w-full text-sm">
                            <thead>
                                <tr className="border-b bg-muted/50">
                                    <th className="text-left p-3 font-medium text-muted-foreground">{t('monitor.table.model')}</th>
                                    <th className="text-right p-3 font-medium text-muted-foreground">{t('monitor.table.totalCalls')}</th>
                                    <th className="text-right p-3 font-medium text-muted-foreground">{t('monitor.table.errorCalls')}</th>
                                    <th className="text-right p-3 font-medium text-muted-foreground">{t('monitor.table.cost')}</th>
                                    <th className="text-right p-3 font-medium text-muted-foreground">{t('monitor.table.avgResponseTime')}</th>
                                    <th className="text-right p-3 font-medium text-muted-foreground">{t('monitor.table.avgTtfb')}</th>
                                </tr>
                            </thead>
                            <tbody>
                                {tableData.map((row) => {
                                    const details = detailByModel.get(row.model) || []
                                    const expandable = hasDetailData && details.length > 0
                                    const isExpanded = expandedModels.has(row.model)
                                    return (
                                        <Fragment key={row.model}>
                                            <tr
                                                className={cn(
                                                    "border-b last:border-b-0 transition-colors",
                                                    expandable ? "cursor-pointer hover:bg-muted/30" : "hover:bg-muted/30",
                                                    isExpanded && "bg-muted/20"
                                                )}
                                                onClick={expandable ? () => toggleExpand(row.model) : undefined}
                                            >
                                                <td className="p-3 font-medium truncate max-w-[200px]">
                                                    <div className="flex items-center gap-1.5">
                                                        {expandable && (
                                                            <ChevronRight className={cn(
                                                                "h-3.5 w-3.5 text-muted-foreground transition-transform shrink-0",
                                                                isExpanded && "rotate-90"
                                                            )} />
                                                        )}
                                                        {!expandable && hasDetailData && (
                                                            <span className="w-3.5 shrink-0" />
                                                        )}
                                                        {row.model}
                                                    </div>
                                                </td>
                                                <td className="p-3 text-right text-blue-600 dark:text-blue-400">{row.totalCalls.toLocaleString()}</td>
                                                <td className="p-3 text-right text-red-600 dark:text-red-400">{row.errorCalls.toLocaleString()}</td>
                                                <td className="p-3 text-right">${row.usedAmount.toFixed(4)}</td>
                                                <td className="p-3 text-right">{row.avgResponseTime > 0 ? `${row.avgResponseTime.toFixed(0)} ms` : '-'}</td>
                                                <td className="p-3 text-right">{row.avgTtfb > 0 ? `${row.avgTtfb.toFixed(0)} ms` : '-'}</td>
                                            </tr>
                                            {isExpanded && details.map((detail, idx) => (
                                                <tr
                                                    key={`${row.model}-${idx}`}
                                                    className={cn(
                                                        "border-b last:border-b-0 bg-muted/10",
                                                        detail.channelId && "cursor-pointer hover:bg-muted/30"
                                                    )}
                                                    onClick={detail.channelId ? () => openChannelEdit(detail.channelId) : undefined}
                                                >
                                                    <td className="p-3 pl-9 text-muted-foreground text-xs max-w-[280px]">
                                                        <span className="inline-flex items-center gap-1.5 flex-wrap">
                                                            {detail.channelId ? (
                                                                <ChannelLabel
                                                                    id={detail.channelId}
                                                                    info={channelInfoMap[detail.channelId]}
                                                                    typeName={typeMetas?.[channelInfoMap[detail.channelId]?.type]?.name}
                                                                    compact
                                                                />
                                                            ) : null}
                                                            {detail.channelId && detail.tokenName ? <span>/</span> : null}
                                                            {detail.tokenName || (!detail.channelId ? row.model : null)}
                                                        </span>
                                                    </td>
                                                    <td className="p-3 text-right text-xs text-blue-600 dark:text-blue-400">{detail.totalCalls.toLocaleString()}</td>
                                                    <td className="p-3 text-right text-xs text-red-600 dark:text-red-400">{detail.errorCalls.toLocaleString()}</td>
                                                    <td className="p-3 text-right text-xs">${detail.usedAmount.toFixed(4)}</td>
                                                    <td className="p-3 text-right text-xs">{detail.avgResponseTime > 0 ? `${detail.avgResponseTime.toFixed(0)} ms` : '-'}</td>
                                                    <td className="p-3 text-right text-xs">{detail.avgTtfb > 0 ? `${detail.avgTtfb.toFixed(0)} ms` : '-'}</td>
                                                </tr>
                                            ))}
                                        </Fragment>
                                    )
                                })}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}

            {/* Channel edit dialog */}
            <ChannelDialog
                open={channelDialogOpen}
                onOpenChange={setChannelDialogOpen}
                mode="update"
                channel={editingChannel}
            />
        </div>
    )
}
