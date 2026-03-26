import { useQuery } from '@tanstack/react-query'
import { monitorApi } from '@/api/monitor'
import { BatchGroupTokenMetricsRequestItem, GroupSummaryMetricsQuery } from '@/types/runtime-metrics'

export const useRuntimeMetrics = () => {
    return useQuery({
        queryKey: ['runtimeMetrics'],
        queryFn: monitorApi.getRuntimeMetrics,
        refetchInterval: 15000,
        staleTime: 10000,
    })
}

export const useGroupSummaryMetrics = (query?: GroupSummaryMetricsQuery, enabled = true) => {
    return useQuery({
        queryKey: ['groupSummaryMetrics', query],
        queryFn: () => monitorApi.getGroupSummaryMetrics(query),
        enabled,
        refetchInterval: 15000,
        staleTime: 10000,
    })
}

export const useGroupTokenMetrics = (groupId?: string, enabled = true) => {
    return useQuery({
        queryKey: ['groupTokenMetrics', groupId],
        queryFn: () => monitorApi.getGroupTokenMetrics(groupId || ''),
        enabled: enabled && !!groupId,
        refetchInterval: 15000,
        staleTime: 10000,
    })
}

export const useGroupModelMetrics = (groupId?: string, enabled = true) => {
    return useQuery({
        queryKey: ['groupModelMetrics', groupId],
        queryFn: () => monitorApi.getGroupModelMetrics(groupId || ''),
        enabled: enabled && !!groupId,
        refetchInterval: 15000,
        staleTime: 10000,
    })
}

export const useGroupTokennameModelMetrics = (groupId?: string, enabled = true) => {
    return useQuery({
        queryKey: ['groupTokennameModelMetrics', groupId],
        queryFn: () => monitorApi.getGroupTokennameModelMetrics(groupId || ''),
        enabled: enabled && !!groupId,
        refetchInterval: 15000,
        staleTime: 10000,
    })
}

export const useBatchGroupTokenMetrics = (
    items: BatchGroupTokenMetricsRequestItem[] = [],
    enabled = true,
) => {
    return useQuery({
        queryKey: ['batchGroupTokenMetrics', items],
        queryFn: () => monitorApi.batchGetGroupTokenMetrics(items),
        enabled: enabled && items.length > 0,
        refetchInterval: 15000,
        staleTime: 10000,
    })
}
