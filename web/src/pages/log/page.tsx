import { useState, useCallback } from 'react'

import { useLogs } from '@/feature/log/hooks'
import { LogExportDialog } from '@/feature/log/components/LogExportDialog'
import { LogFilters } from '@/feature/log/components/LogFilters'
import { LogTable } from '@/feature/log/components/LogTable'
import { GroupDialog } from '@/feature/group/components/GroupDialog'
import { AdvancedErrorDisplay } from '@/components/common/error/errorDisplay'
import type { LogFilters as LogFiltersType } from '@/types/log'
import { DEFAULT_TIMEZONE, zonedBoundaryToUnixMs } from '@/utils/timezone'

export default function LogPage() {

    const getDefaultFilters = (): LogFiltersType => {
        const today = new Date()
        const oneDayAgo = new Date()
        oneDayAgo.setDate(today.getDate() - 1)

        return {
            code_type: 'all',
            page: 1,
            per_page: 10,
            timezone: DEFAULT_TIMEZONE,
            start_timestamp: zonedBoundaryToUnixMs(oneDayAgo, DEFAULT_TIMEZONE, false),
            end_timestamp: zonedBoundaryToUnixMs(today, DEFAULT_TIMEZONE, true)
        }
    }

    const [filters, setFilters] = useState<LogFiltersType>(getDefaultFilters())

    // GroupDialog 状态
    const [groupDialogOpen, setGroupDialogOpen] = useState(false)
    const [groupDialogGroupId, setGroupDialogGroupId] = useState<string | null>(null)
    const [groupDialogTokenName, setGroupDialogTokenName] = useState<string | undefined>()

    const {
        data: logData,
        isLoading,
        error,
        refetch
    } = useLogs(filters)

    const handleFiltersChange = (newFilters: LogFiltersType) => {
        setFilters(newFilters)
    }

    const handlePageChange = (page: number) => {
        setFilters(prev => ({ ...prev, page }))
    }

    const handlePageSizeChange = (pageSize: number) => {
        setFilters(prev => ({ ...prev, per_page: pageSize, page: 1 }))
    }

    const handleRetry = () => {
        refetch()
    }

    // 点击 group/token_name → 打开 GroupDialog 的日志标签
    const handleOpenGroupLog = useCallback((group: string, tokenName?: string) => {
        setGroupDialogGroupId(group)
        setGroupDialogTokenName(tokenName)
        setGroupDialogOpen(true)
    }, [])

    return (
        <div className="flex h-full min-h-0 flex-col gap-4 p-4 sm:p-6">
            <div className="flex-shrink-0 rounded-2xl border border-border/60 bg-card/80 p-4 shadow-sm sm:p-5">
                <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                        <h1 className="text-xl font-semibold tracking-tight">日志</h1>
                        <p className="mt-1 text-sm text-muted-foreground">按时间、模型和渠道检索请求记录</p>
                    </div>
                    <div className="flex justify-end">
                        <LogExportDialog
                            scope="global"
                            currentFilters={filters}
                        />
                    </div>

                    <LogFilters
                        onFiltersChange={handleFiltersChange}
                        loading={isLoading}
                        availableModels={logData?.models}
                        availableTokenNames={logData?.token_names}
                        availableChannels={logData?.channels}
                    />
                </div>

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

            <div className="min-h-0 flex-1 overflow-hidden rounded-2xl border border-border/60 bg-card/80 p-3 shadow-sm sm:p-5">
                <LogTable
                    data={logData?.logs || []}
                    total={logData?.total || 0}
                    loading={isLoading}
                    page={filters.page || 1}
                    pageSize={filters.per_page || 10}
                    onPageChange={handlePageChange}
                    onPageSizeChange={handlePageSizeChange}
                    onOpenGroupLog={handleOpenGroupLog}
                />
            </div>

            {/* 点击 group/token_name 打开 GroupDialog 日志标签 */}
            <GroupDialog
                open={groupDialogOpen}
                onOpenChange={setGroupDialogOpen}
                groupId={groupDialogGroupId}
                initialTab="logs"
                initialTokenName={groupDialogTokenName}
            />
        </div>
    )
}
