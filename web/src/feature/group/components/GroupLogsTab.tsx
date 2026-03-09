// src/feature/group/components/GroupLogsTab.tsx
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { logApi } from '@/api/log'
import { LogTable } from '@/feature/log/components/LogTable'
import { LogFilters } from '@/feature/log/components/LogFilters'
import type { LogFilters as LogFiltersType } from '@/types/log'

interface GroupLogsTabProps {
    groupId: string
}

export function GroupLogsTab({ groupId }: GroupLogsTabProps) {
    const getDefaultFilters = (): LogFiltersType => {
        const today = new Date()
        const sevenDaysAgo = new Date()
        sevenDaysAgo.setDate(today.getDate() - 7)
        return {
            code_type: 'all',
            page: 1,
            per_page: 10,
            start_timestamp: sevenDaysAgo.getTime(),
            end_timestamp: new Date(today.setHours(23, 59, 59, 999)).getTime(),
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
