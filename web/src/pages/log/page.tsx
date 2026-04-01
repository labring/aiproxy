import { useState, useCallback, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'

import { useLogs } from '@/feature/log/hooks'
import { LogFilters } from '@/feature/log/components/LogFilters'
import { LogTable } from '@/feature/log/components/LogTable'
import { GroupDialog } from '@/feature/group/components/GroupDialog'
import { AdvancedErrorDisplay } from '@/components/common/error/errorDisplay'
import { groupApi } from '@/api/group'
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

    // GroupDialog state
    const [groupDialogOpen, setGroupDialogOpen] = useState(false)
    const [groupDialogGroupId, setGroupDialogGroupId] = useState<string | null>(null)
    const [groupDialogTokenName, setGroupDialogTokenName] = useState<string | undefined>()

    const {
        data: logData,
        isLoading,
        error,
        refetch
    } = useLogs(filters)

    // Extract unique group IDs from current page logs
    const groupIds = useMemo(() => {
        if (!logData?.logs) return []
        const ids = new Set<string>()
        for (const log of logData.logs) {
            if (log.group) ids.add(log.group)
        }
        return Array.from(ids)
    }, [logData?.logs])

    // Fetch group names for current page
    const { data: groupNames } = useQuery({
        queryKey: ['groupNames', groupIds],
        queryFn: () => groupApi.getGroupNames(groupIds),
        enabled: groupIds.length > 0,
        staleTime: 5 * 60 * 1000,
    })

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
                    groupNames={groupNames}
                />
            </div>

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
