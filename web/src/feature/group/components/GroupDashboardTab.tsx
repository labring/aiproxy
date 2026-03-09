// src/feature/group/components/GroupDashboardTab.tsx
import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { BarChart3 } from 'lucide-react'
import { useGroupDashboard } from '@/feature/monitor/hooks'
import { MetricsCards } from '@/feature/monitor/components/MetricsCards'
import { MonitorCharts } from '@/feature/monitor/components/MonitorCharts'
import { GroupDashboardFilters } from './GroupDashboardFilters'
import type { DashboardFilters } from '@/types/dashboard'
import { Skeleton } from '@/components/ui/skeleton'

interface GroupDashboardTabProps {
    groupId: string
    initialTokenName?: string
}

export function GroupDashboardTab({ groupId, initialTokenName }: GroupDashboardTabProps) {
    const { t } = useTranslation()

    const getDefaultFilters = (): DashboardFilters & { tokenName?: string } => {
        const today = new Date()
        const sevenDaysAgo = new Date()
        sevenDaysAgo.setDate(today.getDate() - 7)

        return {
            tokenName: initialTokenName || undefined,
            timespan: 'hour',
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
            start_timestamp: Math.floor(sevenDaysAgo.getTime() / 1000),
            end_timestamp: Math.floor(today.setHours(23, 59, 59, 999) / 1000)
        }
    }

    const [filters, setFilters] = useState<DashboardFilters & { tokenName?: string }>(getDefaultFilters())

    const { data, isLoading, error, refetch } = useGroupDashboard(groupId, filters)

    // Preserve the full list of available token names and models across filter changes
    const availableTokenNamesRef = useRef<string[]>([])
    const availableModelsRef = useRef<string[]>([])
    useEffect(() => {
        if (data?.tokenNames && data.tokenNames.length > 0) {
            availableTokenNamesRef.current = data.tokenNames
        }
        if (data?.models && data.models.length > 0) {
            availableModelsRef.current = data.models
        }
    }, [data?.tokenNames, data?.models])

    useEffect(() => {
        const interval = setInterval(() => {
            refetch()
        }, 5 * 60 * 1000)

        return () => clearInterval(interval)
    }, [refetch])

    const handleFiltersChange = (newFilters: DashboardFilters & { tokenName?: string }) => {
        setFilters(newFilters)
    }

    const hasData = (data?.chartData?.length ?? 0) > 0

    if (error) {
        return (
            <div className="flex items-center justify-center h-64 text-muted-foreground">
                <p>{t('error.loading')}</p>
            </div>
        )
    }

    return (
        <div className="space-y-4">
            <GroupDashboardFilters
                onFiltersChange={handleFiltersChange}
                loading={isLoading}
                availableModels={data?.models ?? availableModelsRef.current}
                availableTokenNames={data?.tokenNames ?? availableTokenNamesRef.current}
                defaultTokenName={initialTokenName}
            />

            {isLoading ? (
                <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
                    {Array.from({ length: 8 }).map((_, i) => (
                        <Skeleton key={i} className="h-28 rounded-lg" />
                    ))}
                </div>
            ) : (
                data && <MetricsCards data={data} loading={isLoading} />
            )}

            {isLoading ? (
                <div className="space-y-4">
                    <Skeleton className="h-[300px] rounded-lg" />
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                        <Skeleton className="h-[300px] rounded-lg" />
                        <Skeleton className="h-[300px] rounded-lg" />
                    </div>
                </div>
            ) : (
                data && hasData && (
                    <MonitorCharts
                        chartData={data.chartData}
                        modelRanking={data.modelRanking}
                        detailRanking={data.detailRanking}
                        hasModelFilter={!!filters.model}
                        loading={isLoading}
                    />
                )
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
