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
        
        if (filters?.keyName) {
            params.append('token_name', filters.keyName)
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
        if (filters?.keyName) {
            return dashboardApi.getDashboardByGroup(filters.keyName, filters)
        } else {
            return dashboardApi.getDashboard(filters)
        }
    }
} 