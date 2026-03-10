import React from 'react'
import { useTranslation } from 'react-i18next'
import {
    Activity,
    AlertTriangle,
    BarChart3,
    Zap,
    DollarSign,
    Clock,
    Coins,
    Database,
} from 'lucide-react'

import { Card, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from '@/components/ui/tooltip'
import type { DashboardV2Result } from '@/feature/monitor/hooks'
import { cn } from '@/lib/utils'


interface MetricsCardsProps {
    data: DashboardV2Result
    loading?: boolean
}

interface MetricCardProps {
    title: string
    value: number | string
    icon: React.ReactNode
    className?: string
    tooltip?: string
    bgColor?: string
    iconColor?: string
    subtitle?: string
}

function formatCompact(v: number): string {
    if (v >= 1000000000) return (v / 1000000000).toFixed(2).replace(/\.?0+$/, '') + 'B'
    if (v >= 1000000) return (v / 1000000).toFixed(2).replace(/\.?0+$/, '') + 'M'
    if (v >= 10000) return (v / 1000).toFixed(1).replace(/\.?0+$/, '') + 'K'
    return v.toLocaleString()
}

function MetricCard({ title, value, icon, className, tooltip, bgColor, iconColor, subtitle }: MetricCardProps) {
    const fullValue = typeof value === 'number' ? value.toLocaleString() : value
    const formattedValue = typeof value === 'number' ? formatCompact(value) : value
    const isAbbreviated = typeof value === 'number' && value >= 10000

    const cardContent = (
        <Card className={cn(
            "border-0 shadow-sm hover:shadow-md transition-all duration-200 h-28",
            "dark:bg-card dark:shadow-lg dark:hover:shadow-xl",
            bgColor,
            className
        )}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 pt-4">
                <div className={cn("p-2 rounded-lg", iconColor)}>
                    {icon}
                </div>
                <div className="text-right flex-1 ml-3">
                    <CardTitle className="text-xs font-medium text-muted-foreground mb-1 leading-tight">
                        {title}
                    </CardTitle>
                    <div className="text-2xl font-bold text-foreground truncate">
                        {formattedValue}
                    </div>
                    {subtitle && (
                        <div className="text-xs text-muted-foreground mt-0.5">
                            {subtitle}
                        </div>
                    )}
                </div>
            </CardHeader>
        </Card>
    )

    if (tooltip) {
        return (
            <TooltipProvider>
                <Tooltip>
                    <TooltipTrigger asChild>
                        {cardContent}
                    </TooltipTrigger>
                    <TooltipContent>
                        <p>{tooltip}</p>
                        {isAbbreviated && (
                            <p className="text-xs text-foreground mt-1">
                                {fullValue}
                            </p>
                        )}
                    </TooltipContent>
                </Tooltip>
            </TooltipProvider>
        )
    }

    return cardContent
}

export function MetricsCards({ data, loading = false }: MetricsCardsProps) {
    const { t } = useTranslation()

    if (loading) {
        return (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                {Array.from({ length: 9 }).map((_, index) => (
                    <Card key={index} className="border-0 shadow-sm h-28 dark:bg-card">
                        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 pt-4">
                            <div className="p-2">
                                <Skeleton className="h-5 w-5" />
                            </div>
                            <div className="text-right flex-1 ml-3">
                                <Skeleton className="h-3 w-16 mb-2" />
                                <Skeleton className="h-6 w-12" />
                            </div>
                        </CardHeader>
                    </Card>
                ))}
            </div>
        )
    }

    const agg = data?.aggregates

    const errorRate = (agg?.request_count || 0) > 0
        ? ((((agg?.exception_count || 0) / (agg?.request_count || 0)) * 100)).toFixed(1)
        : '0.0'

    const avgLatency = (agg?.request_count || 0) > 0
        ? Math.round((agg?.total_time_milliseconds || 0) / (agg?.request_count || 0))
        : 0

    const avgTtfb = (agg?.request_count || 0) > 0
        ? Math.round((agg?.total_ttfb_milliseconds || 0) / (agg?.request_count || 0))
        : 0

    const cacheHitRate = (agg?.request_count || 0) > 0
        ? ((((agg?.cache_hit_count || 0) / (agg?.request_count || 0)) * 100)).toFixed(1)
        : '0.0'

    return (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {/* Row 1: Core metrics */}
            <MetricCard
                title={t('monitor.metrics.totalRequests')}
                value={agg?.request_count || 0}
                icon={<Activity className="h-5 w-5 text-blue-600 dark:text-blue-400" />}
                bgColor="bg-blue-50 dark:bg-blue-950/30"
                iconColor="bg-blue-100 dark:bg-blue-900/50"
                tooltip={t('monitor.metrics.totalRequestsTooltip')}
            />
            <MetricCard
                title={t('monitor.metrics.errorCount')}
                value={agg?.exception_count || 0}
                subtitle={`${t('monitor.metrics.errorRate')}: ${errorRate}%`}
                icon={<AlertTriangle className="h-5 w-5 text-orange-600 dark:text-orange-400" />}
                bgColor="bg-orange-50 dark:bg-orange-950/30"
                iconColor="bg-orange-100 dark:bg-orange-900/50"
                tooltip={t('monitor.metrics.errorCountTooltip')}
            />
            <MetricCard
                title={t('monitor.metrics.usedAmount')}
                value={`$${(agg?.used_amount || 0).toFixed(4)}`}
                icon={<DollarSign className="h-5 w-5 text-green-600 dark:text-green-400" />}
                bgColor="bg-green-50 dark:bg-green-950/30"
                iconColor="bg-green-100 dark:bg-green-900/50"
                tooltip={t('monitor.metrics.usedAmountTooltip')}
            />
            {/* Row 2: Throughput & Latency */}
            <MetricCard
                title={t('monitor.metrics.currentRpm')}
                value={agg?.current_rpm || 0}
                subtitle={`${t('monitor.metrics.avgRpm')}: ${formatCompact(agg?.avg_rpm || 0)} | Max: ${formatCompact(agg?.max_rpm || 0)}`}
                icon={<BarChart3 className="h-5 w-5 text-indigo-600 dark:text-indigo-400" />}
                bgColor="bg-indigo-50 dark:bg-indigo-950/30"
                iconColor="bg-indigo-100 dark:bg-indigo-900/50"
                tooltip={t('monitor.metrics.currentRpmTooltip')}
            />
            <MetricCard
                title={t('monitor.metrics.currentTpm')}
                value={agg?.current_tpm || 0}
                subtitle={`${t('monitor.metrics.avgTpm')}: ${formatCompact(agg?.avg_tpm || 0)} | Max: ${formatCompact(agg?.max_tpm || 0)}`}
                icon={<Zap className="h-5 w-5 text-purple-600 dark:text-purple-400" />}
                bgColor="bg-purple-50 dark:bg-purple-950/30"
                iconColor="bg-purple-100 dark:bg-purple-900/50"
                tooltip={t('monitor.metrics.currentTpmTooltip')}
            />
            <MetricCard
                title={t('monitor.metrics.avgLatency')}
                value={`${avgLatency.toLocaleString()} ms`}
                icon={<Clock className="h-5 w-5 text-cyan-600 dark:text-cyan-400" />}
                bgColor="bg-cyan-50 dark:bg-cyan-950/30"
                iconColor="bg-cyan-100 dark:bg-cyan-900/50"
                tooltip={t('monitor.metrics.avgLatencyTooltip')}
            />
            {/* Row 3: Tokens & Cache */}
            <MetricCard
                title={t('monitor.metrics.totalTokens')}
                value={agg?.total_tokens || 0}
                subtitle={`${t('monitor.metrics.cachedTokens')}: ${formatCompact(agg?.cached_tokens || 0)}`}
                icon={<Coins className="h-5 w-5 text-amber-600 dark:text-amber-400" />}
                bgColor="bg-amber-50 dark:bg-amber-950/30"
                iconColor="bg-amber-100 dark:bg-amber-900/50"
                tooltip={t('monitor.metrics.totalTokensTooltip')}
            />
            <MetricCard
                title={t('monitor.metrics.cacheHitCount')}
                value={agg?.cache_hit_count || 0}
                subtitle={`${t('monitor.metrics.cacheHitRate')}: ${cacheHitRate}% | ${t('monitor.metrics.cacheCreationCount')}: ${formatCompact(agg?.cache_creation_count || 0)}`}
                icon={<Database className="h-5 w-5 text-teal-600 dark:text-teal-400" />}
                bgColor="bg-teal-50 dark:bg-teal-950/30"
                iconColor="bg-teal-100 dark:bg-teal-900/50"
                tooltip={t('monitor.metrics.cacheHitCountTooltip')}
            />
            <MetricCard
                title={t('monitor.metrics.avgTtfb')}
                value={`${avgTtfb.toLocaleString()} ms`}
                icon={<Zap className="h-5 w-5 text-sky-600 dark:text-sky-400" />}
                bgColor="bg-sky-50 dark:bg-sky-950/30"
                iconColor="bg-sky-100 dark:bg-sky-900/50"
                tooltip={t('monitor.metrics.avgTtfbTooltip')}
            />
        </div>
    )
}
