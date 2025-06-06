import { get } from './index'
import { LogResponse, LogFilters } from '@/types/log'

export const logApi = {
    // 获取全部日志数据
    getLogs: async (filters?: LogFilters): Promise<LogResponse> => {
        const params = new URLSearchParams()

        if (filters?.page) {
            params.append('page', filters.page.toString())
        }
        if (filters?.per_page) {
            params.append('per_page', filters.per_page.toString())
        }
        if (filters?.model) {
            params.append('model_name', filters.model)
        }
        if (filters?.start_timestamp) {
            params.append('start_timestamp', filters.start_timestamp.toString())
        }
        if (filters?.end_timestamp) {
            params.append('end_timestamp', filters.end_timestamp.toString())
        }
        if (filters?.code_type && filters.code_type !== 'all') {
            params.append('code_type', filters.code_type)
        }

        const queryString = params.toString()
        const url = queryString ? `logs/search?${queryString}` : 'logs/search'

        const response = await get<LogResponse>(url)
        return response
    },

    // 获取组级别日志数据
    getLogsByGroup: async (group: string, filters?: LogFilters): Promise<LogResponse> => {
        const params = new URLSearchParams()

        if (filters?.page) {
            params.append('page', filters.page.toString())
        }
        if (filters?.per_page) {
            params.append('per_page', filters.per_page.toString())
        }
        if (filters?.model) {
            params.append('model_name', filters.model)
        }
        if (filters?.keyName) {
            params.append('token_name', filters.keyName)
        }
        if (filters?.start_timestamp) {
            params.append('start_timestamp', filters.start_timestamp.toString())
        }
        if (filters?.end_timestamp) {
            params.append('end_timestamp', filters.end_timestamp.toString())
        }
        if (filters?.code_type && filters.code_type !== 'all') {
            params.append('code_type', filters.code_type)
        }

        const queryString = params.toString()
        const url = queryString ? `log/${group}/search?${queryString}` : `log/${group}/search`

        const response = await get<LogResponse>(url)
        return response
    },

    // 根据条件获取日志数据 - 统一入口
    getLogData: async (filters?: LogFilters): Promise<LogResponse> => {
        if (filters?.keyName) {
            // 有keyName时使用分组API
            return logApi.getLogsByGroup(filters.keyName, filters)
        } else {
            // 没有keyName时使用全局API
            return logApi.getLogs(filters)
        }
    }
} 