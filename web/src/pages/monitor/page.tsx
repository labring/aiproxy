import { useState, useEffect } from 'react'
import { BarChart3 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'

import { useDashboard } from '@/feature/monitor/hooks'
import { MonitorFilters } from '@/feature/monitor/components/MonitorFilters'
import { MetricsCards } from '@/feature/monitor/components/MetricsCards'
import { MonitorCharts } from '@/feature/monitor/components/MonitorCharts'
import { AdvancedErrorDisplay } from '@/components/common/error/errorDisplay'
import { DashboardFilters } from '@/types/dashboard'

export default function MonitorPage() {
    const { t } = useTranslation()
    const [searchParams] = useSearchParams()

    const initialChannel = searchParams.get('channel') ? Number(searchParams.get('channel')) : undefined

    const getDefaultFilters = (): DashboardFilters => {
        const today = new Date()
        const oneDayAgo = new Date()
        oneDayAgo.setDate(today.getDate() - 1)

        return {
            channel: initialChannel,
            timespan: 'hour',
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
            start_timestamp: Math.floor(oneDayAgo.getTime() / 1000),
            end_timestamp: Math.floor(today.setHours(23, 59, 59, 999) / 1000)
        }
    }

    const [filters, setFilters] = useState<DashboardFilters>(getDefaultFilters())

    const { data, isLoading, error, refetch } = useDashboard(filters)

    useEffect(() => {
        const interval = setInterval(() => {
            refetch()
        }, 5 * 60 * 1000)

        return () => clearInterval(interval)
    }, [refetch])

    const handleFiltersChange = (newFilters: DashboardFilters) => {
        setFilters(newFilters)
    }

    const hasData = (data?.chartData?.length ?? 0) > 0

    return (
        <div className="flex-1 space-y-4 p-6">
            <MonitorFilters
                onFiltersChange={handleFiltersChange}
                loading={isLoading}
                availableModels={data?.models}
                availableChannels={data?.channels}
                defaultChannel={initialChannel}
            />

            {error && (
                <AdvancedErrorDisplay
                    error={error}
                    onRetry={refetch}
                    useCardStyle={true}
                />
            )}

            {data && (
                <MetricsCards data={data} loading={isLoading} />
            )}

            {data && hasData && (
                <MonitorCharts
                    chartData={data.chartData}
                    modelRanking={data.modelRanking}
                    detailRanking={data.detailRanking}
                    hasModelFilter={!!filters.model}
                    loading={isLoading}
                />
            )}

            {data && !hasData && !isLoading && (
                <div className="flex flex-col items-center justify-center py-12 text-center">
                    <BarChart3 className="h-12 w-12 text-muted-foreground mb-4" />
                    <h3 className="text-lg font-medium text-muted-foreground mb-2">
                        {t('monitor.noData')}
                    </h3>
                    <p className="text-sm text-muted-foreground max-w-sm">
                        {t('monitor.noDataDescription')}
                    </p>
                </div>
            )}
        </div>
    )
}
