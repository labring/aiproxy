// src/feature/group/components/GroupLogsTab.tsx
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { logApi } from '@/api/log'
import { LogTable } from '@/feature/log/components/LogTable'
import { LogFilters } from '@/feature/log/components/LogFilters'
import type { LogFilters as LogFiltersType } from '@/types/log'
import { DEFAULT_TIMEZONE, zonedBoundaryToUnixMs } from '@/utils/timezone'

interface GroupLogsTabProps {
    groupId: string
    initialTokenName?: string
}

export function GroupLogsTab({ groupId, initialTokenName }: GroupLogsTabProps) {
    const getDefaultFilters = (): LogFiltersType => {
        const today = new Date()
        const oneDayAgo = new Date()
        oneDayAgo.setDate(today.getDate() - 1)
        return {
            token_name: initialTokenName || undefined,
            code_type: 'all',
            page: 1,
            per_page: 10,
            timezone: DEFAULT_TIMEZONE,
            start_timestamp: zonedBoundaryToUnixMs(oneDayAgo, DEFAULT_TIMEZONE, false),
            end_timestamp: zonedBoundaryToUnixMs(today, DEFAULT_TIMEZONE, true),
        }
    }

    const [filters, setFilters] = useState<LogFiltersType>(getDefaultFilters())

    const { data, isLoading } = useQuery({
        queryKey: ['groupLogs', groupId, filters],
        queryFn: () => logApi.getLogsByGroup(groupId, filters),
        refetchOnWindowFocus: true,
        retry: false,
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

    return (
        <div className="flex flex-col h-full gap-2">
            <div className="flex-shrink-0">
                <LogFilters
                    onFiltersChange={handleFiltersChange}
                    loading={isLoading}
                    availableModels={data?.models}
                    availableTokenNames={data?.token_names}
                    availableChannels={data?.channels}
                    tokenNameFirst
                    defaultTokenName={initialTokenName}
                />
            </div>
            <div className="flex-1 min-h-0">
                <LogTable
                    data={data?.logs || []}
                    total={data?.total || 0}
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
