import { useQuery } from '@tanstack/react-query'
import { dashboardApi } from '@/api/dashboard'
import { DashboardFilters } from '@/types/dashboard'

// 获取仪表盘数据
export const useDashboard = (filters?: DashboardFilters) => {
    const query = useQuery({
        queryKey: ['dashboard', filters],
        queryFn: () => dashboardApi.getDashboardData(filters),
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