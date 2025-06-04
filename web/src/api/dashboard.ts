import { get } from './index'
import { DashboardData, DashboardFilters } from '@/types/dashboard'

export const dashboardApi = {
    getDashboard: async (filters?: DashboardFilters): Promise<DashboardData> => {
        const params = new URLSearchParams()
        
        if (filters?.model) {
            params.append('model', filters.model)
        }
        if (filters?.start_timestamp) {
            params.append('start_timestamp', filters.start_timestamp.toString())
        }
        if (filters?.end_timestamp) {
            params.append('end_timestamp', filters.end_timestamp.toString())
        }
        if (filters?.timezone) {
            params.append('timezone', filters.timezone)
        }
        if (filters?.timespan) {
            params.append('timespan', filters.timespan)
        }

        const queryString = params.toString()
        const url = queryString ? `dashboard/?${queryString}` : 'dashboard/'
        
        const response = await get<DashboardData>(url)
        return response
    },

    getDashboardByGroup: async (group: string, filters?: DashboardFilters): Promise<DashboardData> => {
        const params = new URLSearchParams()
        
        if (filters?.key) {
            params.append('token_name', filters.key)
        }
        if (filters?.model) {
            params.append('model', filters.model)
        }
        if (filters?.start_timestamp) {
            params.append('start_timestamp', filters.start_timestamp.toString())
        }
        if (filters?.end_timestamp) {
            params.append('end_timestamp', filters.end_timestamp.toString())
        }
        if (filters?.timezone) {
            params.append('timezone', filters.timezone)
        }
        if (filters?.timespan) {
            params.append('timespan', filters.timespan)
        }

        const queryString = params.toString()
        const url = queryString ? `dashboard/${group}?${queryString}` : `dashboard/${group}`
        
        const response = await get<DashboardData>(url)
        return response
    },

    getDashboardData: async (filters?: DashboardFilters): Promise<DashboardData> => {
        if (filters?.key) {
            return dashboardApi.getDashboardByGroup(filters.key, filters)
        } else {
            return dashboardApi.getDashboard(filters)
        }
    }
} 