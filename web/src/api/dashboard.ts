import { get } from './index'
import { DashboardV2Response, DashboardFilters } from '@/types/dashboard'
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
    getDashboardData: async (filters?: DashboardFilters): Promise<DashboardV2Response> => {
        const queryString = buildParams(filters)
        const url = queryString ? `dashboardv2/?${queryString}` : 'dashboardv2/'
        return get<DashboardV2Response>(url)
    },

    getDashboardByGroup: async (group: string, filters?: DashboardFilters & { tokenName?: string }): Promise<DashboardV2Response> => {
        const queryString = buildParams(filters)
        const url = queryString ? `dashboardv2/${group}?${queryString}` : `dashboardv2/${group}`
        return get<DashboardV2Response>(url)
    },

    getGroupModels: async (group: string): Promise<GroupDashboardModel[]> => {
        const response = await get<GroupDashboardModel[]>(`dashboard/${group}/models`)
        return response
    }
}
