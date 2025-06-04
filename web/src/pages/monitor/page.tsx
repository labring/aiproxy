import { useState, useEffect } from 'react'
import { BarChart3 } from 'lucide-react'

import { useDashboard } from '@/feature/monitor/hooks'
import { MonitorFilters } from '@/feature/monitor/components/MonitorFilters'
import { MetricsCards } from '@/feature/monitor/components/MetricsCards'
import { MonitorCharts } from '@/feature/monitor/components/MonitorCharts'
import { DashboardFilters } from '@/types/dashboard'

export default function MonitorPage() {
    // 计算默认日期范围（当前时间往前7天）
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

    const [filters, setFilters] = useState<DashboardFilters>(getDefaultFilters())

    const { data, isLoading, error, refetch } = useDashboard(filters)

    // 自动刷新数据
    useEffect(() => {
        const interval = setInterval(() => {
            refetch()
        }, 5 * 60 * 1000) // 5分钟刷新一次

        return () => clearInterval(interval)
    }, [refetch])

    const handleFiltersChange = (newFilters: DashboardFilters) => {
        setFilters(newFilters)
    }

    // 安全地获取 chart_data
    const chartData = data?.chart_data || []
    const hasChartData = chartData.length > 0

    return (
        <div className="flex-1 space-y-4 p-6">
            {/* 过滤器 */}
            <MonitorFilters onFiltersChange={handleFiltersChange} loading={isLoading} />

            {error && (
                <div className="rounded-md bg-destructive/15 p-4 text-sm text-destructive">
                    数据加载失败: {error.message}
                </div>
            )}

            {/* 指标卡片 */}
            {data && (
                <MetricsCards data={data} loading={isLoading} />
            )}

            {/* 图表 */}
            {data && hasChartData && (
                <MonitorCharts chartData={chartData} loading={isLoading} />
            )}

            {/* 空状态 */}
            {data && !hasChartData && !isLoading && (
                <div className="flex flex-col items-center justify-center py-12 text-center">
                    <BarChart3 className="h-12 w-12 text-muted-foreground mb-4" />
                    <h3 className="text-lg font-medium text-muted-foreground mb-2">
                        暂无数据
                    </h3>
                    <p className="text-sm text-muted-foreground max-w-sm">
                        当前时间范围内没有找到任何数据。请尝试调整过滤条件或选择不同的时间范围。
                    </p>
                </div>
            )}
        </div>
    )
}