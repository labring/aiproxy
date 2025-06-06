import { useQuery } from '@tanstack/react-query'
import { logApi } from '@/api/log'
import { LogFilters } from '@/types/log'

// 获取日志数据
export const useLogs = (filters?: LogFilters) => {
    const query = useQuery({
        queryKey: ['logs', filters],
        queryFn: () => logApi.getLogData(filters),
        // 5分钟刷新一次数据
        refetchInterval: 5 * 60 * 1000,
        // 窗口重新获得焦点时刷新
        refetchOnWindowFocus: true,
        // 禁用重试，避免错误时过多请求
        retry: false,
    })

    return {
        ...query,
    }
} 