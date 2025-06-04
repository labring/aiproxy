import React from 'react'
import { useTranslation } from 'react-i18next'
import { 
    Activity, 
    AlertTriangle, 
    BarChart3, 
    Zap,
    MessageSquare
} from 'lucide-react'

import { Card, CardHeader, CardTitle } from '@/components/ui/card'
import { 
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from '@/components/ui/tooltip'
import { DashboardData } from '@/types/dashboard'
import { cn } from '@/lib/utils'

interface MetricsCardsProps {
    data: DashboardData
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
}

function MetricCard({ title, value, icon, className, tooltip, bgColor, iconColor }: MetricCardProps) {
    const formattedValue = typeof value === 'number' ? value.toLocaleString() : value

    const cardContent = (
        <Card className={cn(
            "border-0 shadow-sm hover:shadow-md transition-all duration-200 h-28",
            bgColor,
            className
        )}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 pt-4">
                <div className={cn("p-2 rounded-lg", iconColor)}>
                    {icon}
                </div>
                <div className="text-right flex-1 ml-3">
                    <CardTitle className="text-xs font-medium text-gray-600 mb-1 leading-tight">
                        {title}
                    </CardTitle>
                    <div className="text-2xl font-bold text-gray-900 truncate">
                        {formattedValue}
                    </div>
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
                        <p className="text-xs text-muted-foreground mt-1">
                            完整值: {formattedValue}
                        </p>
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
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-5">
                {Array.from({ length: 5 }).map((_, index) => (
                    <Card key={index} className="animate-pulse border-0 shadow-sm h-28">
                        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2 pt-4">
                            <div className="p-2">
                                <div className="h-5 w-5 bg-gray-200 rounded"></div>
                            </div>
                            <div className="text-right flex-1 ml-3">
                                <div className="h-3 bg-gray-200 rounded w-16 mb-2"></div>
                                <div className="h-6 bg-gray-200 rounded w-12"></div>
                            </div>
                        </CardHeader>
                    </Card>
                ))}
            </div>
        )
    }

    return (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-5">
            <MetricCard
                title={t('monitor.metrics.totalRequests')}
                value={data.total_count}
                icon={<Activity className="h-5 w-5 text-blue-600" />}
                bgColor="bg-blue-50"
                iconColor="bg-blue-100"
                tooltip={t('monitor.metrics.totalRequestsTooltip')}
            />
            <MetricCard
                title={t('monitor.metrics.errorCount')}
                value={data.exception_count}
                icon={<AlertTriangle className="h-5 w-5 text-orange-600" />}
                bgColor="bg-orange-50"
                iconColor="bg-orange-100"
                tooltip={t('monitor.metrics.errorCountTooltip')}
            />
            <MetricCard
                title={t('monitor.metrics.currentRpm')}
                value={data.rpm}
                icon={<BarChart3 className="h-5 w-5 text-blue-600" />}
                bgColor="bg-blue-50"
                iconColor="bg-blue-100"
                tooltip={t('monitor.metrics.currentRpmTooltip')}
            />
            <MetricCard
                title={t('monitor.metrics.currentTpm')}
                value={data.tpm}
                icon={<Zap className="h-5 w-5 text-purple-600" />}
                bgColor="bg-purple-50"
                iconColor="bg-purple-100"
                tooltip={t('monitor.metrics.currentTpmTooltip')}
            />
            <MetricCard
                title={t('monitor.metrics.outputTokens')}
                value={data.output_tokens}
                icon={<MessageSquare className="h-5 w-5 text-green-600" />}
                bgColor="bg-green-50"
                iconColor="bg-green-100"
                tooltip={t('monitor.metrics.outputTokensTooltip')}
            />
        </div>
    )
} 