import { get } from './index'
import { TimeSeriesPoint, DashboardFilters } from '@/types/dashboard'
import type { GroupDashboardModel } from '@/types/group'

function buildParams(filters?: DashboardFilters & { tokenName?: string }): string {
    const params = new URLSearchParams()
    if (filters?.tokenName) params.append('token_name', filters.tokenName)
    if (filters?.model) params.append('model', filters.model)
    if (filters?.channel) params.append('channel', filters.channel.toString())
    if (filters?.start_timestamp) params.append('start_timestamp', filters.start_timestamp.toString())
    if (filters?.end_timestamp) params.append('end_timestamp', filters.end_timestamp.toString())
    if (filters?.timezone) params.append('timezone', filters.timezone)
    if (filters?.timespan) params.append('timespan', filters.timespan)
    return params.toString()
}

export const dashboardApi = {
    getDashboard: async (filters?: DashboardFilters): Promise<TimeSeriesPoint[]> => {
        const queryString = buildParams(filters)
        const url = queryString ? `dashboardv2/?${queryString}` : 'dashboardv2/'
        return get<TimeSeriesPoint[]>(url)
    },

    getDashboardByGroup: async (group: string, filters?: DashboardFilters & { tokenName?: string }): Promise<TimeSeriesPoint[]> => {
        const queryString = buildParams(filters)
        const url = queryString ? `dashboardv2/${group}?${queryString}` : `dashboardv2/${group}`
        return get<TimeSeriesPoint[]>(url)
    },

    getDashboardData: async (filters?: DashboardFilters): Promise<TimeSeriesPoint[]> => {
        return dashboardApi.getDashboard(filters)
    },

    getGroupModels: async (group: string): Promise<GroupDashboardModel[]> => {
        const response = await get<GroupDashboardModel[]>(`dashboard/${group}/models`)
        return response
    }
}
