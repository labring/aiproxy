import { useState } from 'react'

import { useLogs } from '@/feature/log/hooks'
import { LogFilters } from '@/feature/log/components/LogFilters'
import { LogTable } from '@/feature/log/components/LogTable'
import { AdvancedErrorDisplay } from '@/components/common/error/errorDisplay'
import type { LogFilters as LogFiltersType } from '@/types/log'

export default function LogPage() {

    // 初始化过滤器状态
    const getDefaultFilters = (): LogFiltersType => {
        const today = new Date()
        const sevenDaysAgo = new Date()
        sevenDaysAgo.setDate(today.getDate() - 7)

        return {
            code_type: 'all',
            page: 1,
            per_page: 10,
            start_timestamp: sevenDaysAgo.getTime(),
            end_timestamp: today.setHours(23, 59, 59, 999)
        }
    }

    const [filters, setFilters] = useState<LogFiltersType>(getDefaultFilters())

    // 获取日志数据
    const {
        data: logData,
        isLoading,
        error,
        refetch
    } = useLogs(filters)

    // 处理过滤器变化
    const handleFiltersChange = (newFilters: LogFiltersType) => {
        setFilters(newFilters)
    }

    // 处理分页变化
    const handlePageChange = (page: number) => {
        setFilters(prev => ({ ...prev, page }))
    }

    // 处理每页数量变化
    const handlePageSizeChange = (pageSize: number) => {
        setFilters(prev => ({ ...prev, per_page: pageSize, page: 1 }))
    }

    // 处理重试
    const handleRetry = () => {
        refetch()
    }

    return (
        <div className="h-screen flex flex-col">
            <div className="flex-shrink-0 p-6 pb-2">
                {/* 过滤器 */}
                <LogFilters
                    onFiltersChange={handleFiltersChange}
                    loading={isLoading}
                />

                {/* 错误提示 */}
                {error && (
                    <div className="mt-6">
                        <AdvancedErrorDisplay
                            error={error}
                            onRetry={handleRetry}
                            useCardStyle={true}
                        />
                    </div>
                )}
            </div>

            {/* 数据表格 - 占据剩余空间 */}
            <div className="flex-1 px-6 pb-6 min-h-0">
                <LogTable
                    data={logData?.logs || []}
                    total={logData?.total || 0}
                    loading={isLoading}
                    page={filters.page || 1}
                    pageSize={filters.per_page || 10}
                    onPageChange={handlePageChange}
                    onPageSizeChange={handlePageSizeChange}
                />
            </div>
        </div>
    )
}