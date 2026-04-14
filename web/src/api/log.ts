import apiClient, { get } from './index'
import {
    LogResponse,
    LogFilters,
    LogListParams,
    LogRequestDetail,
    LogExportParams,
} from '@/types/log'

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

const buildLogExportParams = (filters?: LogExportParams): URLSearchParams => {
    const params = new URLSearchParams()
    if (filters?.model) params.append('model_name', filters.model)
    if (filters?.token_name) params.append('token_name', filters.token_name)
    if (typeof filters?.channel === 'number') params.append('channel', filters.channel.toString())
    if (typeof filters?.start_timestamp === 'number') params.append('start_timestamp', filters.start_timestamp.toString())
    if (typeof filters?.end_timestamp === 'number') params.append('end_timestamp', filters.end_timestamp.toString())
    if (filters?.timezone) params.append('timezone', filters.timezone)
    if (filters?.code_type && filters.code_type !== 'all') params.append('code_type', filters.code_type)
    if (typeof filters?.code === 'number') params.append('code', filters.code.toString())
    if (filters?.request_id) params.append('request_id', filters.request_id)
    if (filters?.upstream_id) params.append('upstream_id', filters.upstream_id)
    if (filters?.ip) params.append('ip', filters.ip)
    if (filters?.user) params.append('user', filters.user)
    if (filters?.include_detail) params.append('include_detail', 'true')
    if (filters?.include_channel) params.append('include_channel', 'true')
    if (typeof filters?.max_entries === 'number') params.append('max_entries', filters.max_entries.toString())
    if (filters?.chunk_interval) params.append('chunk_interval', filters.chunk_interval)
    if (filters?.order) params.append('order', filters.order)
    return params
}

const getFilenameFromDisposition = (contentDisposition?: string, fallback = 'logs-export.csv') => {
    if (!contentDisposition) return fallback

    const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i)
    if (utf8Match?.[1]) {
        try {
            return decodeURIComponent(utf8Match[1])
        } catch {
            return utf8Match[1]
        }
    }

    const asciiMatch = contentDisposition.match(/filename="?([^";]+)"?/i)
    return asciiMatch?.[1] || fallback
}

const downloadBlob = (blob: Blob, filename: string) => {
    const url = window.URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = filename
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    window.URL.revokeObjectURL(url)
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
    },

    exportLogs: async (filters?: LogExportParams): Promise<string> => {
        const params = buildLogExportParams(filters)
        const queryString = params.toString()
        const url = queryString ? `logs/export?${queryString}` : 'logs/export'
        const response = await apiClient.get<Blob>(url, {
            responseType: 'blob',
        })

        const filename = getFilenameFromDisposition(
            response.headers['content-disposition'],
            'global-logs.csv'
        )
        downloadBlob(response.data, filename)
        return filename
    },

    exportGroupLogs: async (group: string, filters?: LogExportParams): Promise<string> => {
        const params = buildLogExportParams(filters)
        const queryString = params.toString()
        const url = queryString ? `log/${group}/export?${queryString}` : `log/${group}/export`
        const response = await apiClient.get<Blob>(url, {
            responseType: 'blob',
        })

        const filename = getFilenameFromDisposition(
            response.headers['content-disposition'],
            `group-${group}-logs.csv`
        )
        downloadBlob(response.data, filename)
        return filename
    },
} 
