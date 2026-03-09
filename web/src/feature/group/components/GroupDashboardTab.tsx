// src/feature/group/components/GroupDashboardTab.tsx
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { BarChart3 } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { dashboardApi } from '@/api/dashboard'
import { MetricsCards } from '@/feature/monitor/components/MetricsCards'
import { MonitorCharts } from '@/feature/monitor/components/MonitorCharts'
import type { DashboardFilters } from '@/types/dashboard'
import { Skeleton } from '@/components/ui/skeleton'

interface GroupDashboardTabProps {
    groupId: string
}

export function GroupDashboardTab({ groupId }: GroupDashboardTabProps) {
    const { t } = useTranslation()

    // Calculate default date range (7 days ago to today)
    const getDefaultFilters = (): DashboardFilters => {
        const today = new Date()
        const sevenDaysAgo = new Date()
        sevenDaysAgo.setDate(today.getDate() - 7)

        return {
            timespan: 'day',
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
            start_timestamp: Math.floor(sevenDaysAgo.getTime() / 1000),
            end_timestamp: Math.floor(today.setHours(23, 59, 59, 999) / 1000)
        }
    }

    const [filters] = useState<DashboardFilters>(getDefaultFilters())

    // Fetch group dashboard data
    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ['groupDashboard', groupId, filters],
        queryFn: () => dashboardApi.getDashboardByGroup(groupId, filters),
        refetchInterval: 5 * 60 * 1000, // 5 minutes
        refetchOnWindowFocus: true,
        retry: false,
    })

    // Auto refresh
    useEffect(() => {
        const interval = setInterval(() => {
            refetch()
        }, 5 * 60 * 1000)

        return () => clearInterval(interval)
    }, [refetch])

    const chartData = data?.chart_data || []
    const hasChartData = chartData.length > 0

    if (error) {
        return (
            <div className="flex items-center justify-center h-64 text-muted-foreground">
                <p>{t('error.loading')}</p>
            </div>
        )
    }

    return (
        <div className="space-y-4">
            {/* Metrics cards */}
            {isLoading ? (
                <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-5">
                    {Array.from({ length: 5 }).map((_, i) => (
                        <Skeleton key={i} className="h-28 rounded-lg" />
                    ))}
                </div>
            ) : (
                data && <MetricsCards data={data} loading={isLoading} />
            )}

            {/* Charts */}
            {isLoading ? (
                <div className="space-y-4">
                    <Skeleton className="h-64 rounded-lg" />
                    <Skeleton className="h-64 rounded-lg" />
                </div>
            ) : (
                data && hasChartData && <MonitorCharts chartData={chartData} loading={isLoading} />
            )}

            {/* Empty state */}
            {data && !hasChartData && !isLoading && (
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
