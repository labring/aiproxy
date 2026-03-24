import { get } from './index'
import { LogResponse, LogFilters, LogListParams, LogRequestDetail } from '@/types/log'

// 构建日志搜索的通用查询参数
const buildLogSearchParams = (filters?: LogFilters): URLSearchParams => {
    const params = new URLSearchParams()
    if (filters?.page) params.append('page', filters.page.toString())
    if (filters?.per_page) params.append('per_page', filters.per_page.toString())
    if (filters?.model) params.append('model_name', filters.model)
    if (filters?.token_name) params.append('token_name', filters.token_name)
    if (filters?.channel) params.append('channel', filters.channel.toString())
    if (filters?.start_timestamp) params.append('start_timestamp', filters.start_timestamp.toString())
    if (filters?.end_timestamp) params.append('end_timestamp', filters.end_timestamp.toString())
    if (filters?.timezone) params.append('timezone', filters.timezone)
    if (filters?.code_type && filters.code_type !== 'all') params.append('code_type', filters.code_type)
    if (filters?.keyword) params.append('keyword', filters.keyword)
    return params
}

export const logApi = {
    // 获取全部日志数据
    getLogs: async (filters?: LogFilters): Promise<LogResponse> => {
        const params = buildLogSearchParams(filters)
        const queryString = params.toString()
        const url = queryString ? `logs/search?${queryString}` : 'logs/search'
        return get<LogResponse>(url)
    },

    // 获取组级别日志数据
    getLogsByGroup: async (group: string, filters?: LogFilters): Promise<LogResponse> => {
        const params = buildLogSearchParams(filters)
        const queryString = params.toString()
        const url = queryString ? `log/${group}/search?${queryString}` : `log/${group}/search`
        return get<LogResponse>(url)
    },

    // 获取日志数据（自动根据 group 选择 API）
    getLogData: async (filters?: LogListParams): Promise<LogResponse> => {
        if (filters?.group) {
            return logApi.getLogsByGroup(filters.group, filters)
        }
        return logApi.getLogs(filters)
    },
    
    // 获取日志详情
    getLogDetail: async (logId: number): Promise<LogRequestDetail> => {
        const response = await get<LogRequestDetail>(`logs/detail/${logId}`)
        return response
    }
} 
