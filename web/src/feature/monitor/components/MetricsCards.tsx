import React from 'react'
import { useTranslation } from 'react-i18next'
import {
    Activity,
    AlertTriangle,
    BarChart3,
    Zap,
    DollarSign,
    Clock,
    TrendingUp,
    Gauge,
    Coins
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
                {Array.from({ length: 8 }).map((_, index) => (
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

    const agg = data.aggregates

    const errorRate = agg.request_count > 0
        ? ((agg.exception_count / agg.request_count) * 100).toFixed(1)
        : '0.0'

    const avgLatency = agg.request_count > 0
        ? Math.round(agg.total_time_milliseconds / agg.request_count)
        : 0

    return (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {/* Row 1 */}
            <MetricCard
                title={t('monitor.metrics.totalRequests')}
                value={agg.request_count}
                icon={<Activity className="h-5 w-5 text-blue-600 dark:text-blue-400" />}
                bgColor="bg-blue-50 dark:bg-blue-950/30"
                iconColor="bg-blue-100 dark:bg-blue-900/50"
                tooltip={t('monitor.metrics.totalRequestsTooltip')}

            />
            <MetricCard
                title={t('monitor.metrics.errorCount')}
                value={agg.exception_count}
                subtitle={`${t('monitor.metrics.errorRate')}: ${errorRate}%`}
                icon={<AlertTriangle className="h-5 w-5 text-orange-600 dark:text-orange-400" />}
                bgColor="bg-orange-50 dark:bg-orange-950/30"
                iconColor="bg-orange-100 dark:bg-orange-900/50"
                tooltip={t('monitor.metrics.errorCountTooltip')}

            />
            <MetricCard
                title={t('monitor.metrics.usedAmount')}
                value={`$${agg.used_amount.toFixed(4)}`}
                icon={<DollarSign className="h-5 w-5 text-green-600 dark:text-green-400" />}
                bgColor="bg-green-50 dark:bg-green-950/30"
                iconColor="bg-green-100 dark:bg-green-900/50"
                tooltip={t('monitor.metrics.usedAmountTooltip')}

            />
            <MetricCard
                title={t('monitor.metrics.totalTokens')}
                value={agg.total_tokens}
                icon={<Coins className="h-5 w-5 text-amber-600 dark:text-amber-400" />}
                bgColor="bg-amber-50 dark:bg-amber-950/30"
                iconColor="bg-amber-100 dark:bg-amber-900/50"
                tooltip={t('monitor.metrics.totalTokensTooltip')}
            />
            {/* Row 2 */}
            <MetricCard
                title={t('monitor.metrics.currentRpm')}
                value={agg.current_rpm}
                subtitle={`${t('monitor.metrics.avgRpm')}: ${formatCompact(agg.avg_rpm)}`}
                icon={<BarChart3 className="h-5 w-5 text-blue-600 dark:text-blue-400" />}
                bgColor="bg-blue-50 dark:bg-blue-950/30"
                iconColor="bg-blue-100 dark:bg-blue-900/50"
                tooltip={t('monitor.metrics.currentRpmTooltip')}

            />
            <MetricCard
                title={t('monitor.metrics.currentTpm')}
                value={agg.current_tpm}
                subtitle={`${t('monitor.metrics.avgTpm')}: ${formatCompact(agg.avg_tpm)}`}
                icon={<Zap className="h-5 w-5 text-purple-600 dark:text-purple-400" />}
                bgColor="bg-purple-50 dark:bg-purple-950/30"
                iconColor="bg-purple-100 dark:bg-purple-900/50"
                tooltip={t('monitor.metrics.currentTpmTooltip')}

            />
            <MetricCard
                title={t('monitor.metrics.maxRpm')}
                value={agg.max_rpm}
                icon={<TrendingUp className="h-5 w-5 text-indigo-600 dark:text-indigo-400" />}
                bgColor="bg-indigo-50 dark:bg-indigo-950/30"
                iconColor="bg-indigo-100 dark:bg-indigo-900/50"
                tooltip={t('monitor.metrics.maxRpmTooltip')}

            />
            <MetricCard
                title={t('monitor.metrics.maxTpm')}
                value={agg.max_tpm}
                icon={<Gauge className="h-5 w-5 text-rose-600 dark:text-rose-400" />}
                bgColor="bg-rose-50 dark:bg-rose-950/30"
                iconColor="bg-rose-100 dark:bg-rose-900/50"
                tooltip={t('monitor.metrics.maxTpmTooltip')}

            />
            <MetricCard
                title={t('monitor.metrics.avgLatency')}
                value={`${avgLatency.toLocaleString()} ms`}
                icon={<Clock className="h-5 w-5 text-cyan-600 dark:text-cyan-400" />}
                bgColor="bg-cyan-50 dark:bg-cyan-950/30"
                iconColor="bg-cyan-100 dark:bg-cyan-900/50"
                tooltip={t('monitor.metrics.avgLatencyTooltip')}

            />
        </div>
    )
}
