import { useQuery } from '@tanstack/react-query'
import { monitorApi } from '@/api/monitor'

export const useRuntimeMetrics = () => {
    return useQuery({
        queryKey: ['runtimeMetrics'],
        queryFn: monitorApi.getRuntimeMetrics,
        refetchInterval: 15000,
        staleTime: 10000,
    })
}
