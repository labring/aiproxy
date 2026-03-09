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
    loading?: boolean
}

type DisplayMode = 'incremental' | 'cumulative'
type TokenType = 'totalTokens' | 'inputTokens' | 'outputTokens' | 'cachedTokens'

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

export function MonitorCharts({ chartData, modelRanking, detailRanking = [], hasModelFilter = false, loading = false }: MonitorChartsProps) {
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
    const [tokenType, setTokenType] = useState<TokenType>('totalTokens')
    const [costMode, setCostMode] = useState<DisplayMode>('incremental')

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

    const tokenOptions = useMemo(() => [
        { label: t('monitor.charts.tokenTypes.total'), value: 'totalTokens' },
        { label: t('monitor.charts.tokenTypes.input'), value: 'inputTokens' },
        { label: t('monitor.charts.tokenTypes.output'), value: 'outputTokens' },
        { label: t('monitor.charts.tokenTypes.cached'), value: 'cachedTokens' },
    ], [t])

    function makeData(key: keyof ChartDataPoint, mode: DisplayMode): number[] {
        const raw = chartData.map(d => d[key] as number)
        if (mode === 'incremental') return raw
        const cumulative: number[] = []
        raw.forEach((v, i) => cumulative.push(i === 0 ? v : cumulative[i - 1] + v))
        return cumulative
    }

    function buildAreaChart(
        dataKey: keyof ChartDataPoint,
        color: string,
        mode: DisplayMode,
        opts?: {
            formatter?: (v: number) => string
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
                    formatter: (v: number) => {
                        if (v >= 1000000) return (v / 1000000) + 'M'
                        if (v >= 1000) return (v / 1000) + 'K'
                        return String(v)
                    }
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
        const successCalls = m.request_count - m.exception_count
        return {
            model: m.model,
            tokenName: m.token_name || '',
            channelId: m.channel_id || 0,
            totalCalls: m.request_count,
            errorCalls: m.exception_count,
            usedAmount: m.used_amount,
            avgResponseTime: successCalls > 0 ? m.total_time_milliseconds / successCalls : 0,
            avgTtfb: successCalls > 0 ? m.total_ttfb_milliseconds / successCalls : 0,
        }
    }

    const tableData = useMemo(() => (modelRanking || []).map(toRow), [modelRanking])

    // Build detail rows grouped by model
    const detailByModel = useMemo(() => {
        const map = new Map<string, TableRow[]>()
        for (const m of detailRanking) {
            const rows = map.get(m.model) || []
            rows.push(toRow(m))
            map.set(m.model, rows)
        }
        return map
    }, [detailRanking])

    const hasDetailData = detailRanking.length > 0

    // Batch fetch channel info for detail rows
    const [channelInfoMap, setChannelInfoMap] = useState<Record<number, { name: string; type: number }>>({})
    const detailChannelIds = useMemo(() => {
        const ids = new Set<number>()
        for (const m of detailRanking) {
            if (m.channel_id) ids.add(m.channel_id)
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
            {/* Total Calls - full width */}
            <ChartBox
                title={t('monitor.charts.totalCalls')}
                rightSlot={<ToggleGroup value={requestsMode} onChange={(v) => setRequestsMode(v as DisplayMode)} options={modeOptions} />}
            >
                <EChart
                    option={buildAreaChart('totalCalls', '#3b82f6', requestsMode)}
                    style={{ width: '100%', height: '100%' }}
                />
            </ChartBox>

            {/* Error Calls + Error Rate - 2 columns */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <ChartBox title={t('monitor.charts.errorCalls')}>
                    <EChart
                        option={buildAreaChart('errorCalls', '#f59e0b', 'incremental')}
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

            {/* Token Usage - full width with type switcher */}
            <ChartBox
                title={t('monitor.charts.tokenUsage')}
                rightSlot={
                    <>
                        <ToggleGroup value={tokenType} onChange={(v) => setTokenType(v as TokenType)} options={tokenOptions} />
                        <ToggleGroup value={tokensMode} onChange={(v) => setTokensMode(v as DisplayMode)} options={modeOptions} />
                    </>
                }
            >
                <EChart
                    option={buildAreaChart(tokenType, '#3b82f6', tokensMode)}
                    style={{ width: '100%', height: '100%' }}
                />
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

            {/* Response Time + TTFB - 2 columns */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <ChartBox title={t('monitor.charts.avgResponseTime')}>
                    <EChart
                        option={buildAreaChart('avgResponseTime', '#10b981', 'incremental', {
                            formatter: (v) => `${v.toFixed(0)} ms`
                        })}
                        style={{ width: '100%', height: '100%' }}
                    />
                </ChartBox>
                <ChartBox title={t('monitor.charts.avgTtfb')}>
                    <EChart
                        option={buildAreaChart('avgTtfb', '#ef4444', 'incremental', {
                            formatter: (v) => `${v.toFixed(0)} ms`
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
