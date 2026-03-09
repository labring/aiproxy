import { useState, useCallback } from 'react'

import { useLogs } from '@/feature/log/hooks'
import { LogFilters } from '@/feature/log/components/LogFilters'
import { LogTable } from '@/feature/log/components/LogTable'
import { GroupDialog } from '@/feature/group/components/GroupDialog'
import { AdvancedErrorDisplay } from '@/components/common/error/errorDisplay'
import type { LogFilters as LogFiltersType } from '@/types/log'

export default function LogPage() {

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
        <div className="h-full flex flex-col">
            <div className="flex-shrink-0 p-6 pb-2">
                <LogFilters
                    onFiltersChange={handleFiltersChange}
                    loading={isLoading}
                    availableModels={logData?.models}
                    availableTokenNames={logData?.token_names}
                    availableChannels={logData?.channels}
                />

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

            <div className="flex-1 px-6 pb-6 min-h-0">
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
