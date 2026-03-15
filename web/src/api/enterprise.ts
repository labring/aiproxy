import { get } from './index'
import type { EnterpriseUser } from '@/store/auth'
import apiClient from './index'

// Enterprise API response types
export interface FeishuCallbackResponse {
    token_key: string
    user: {
        open_id: string
        name: string
        email: string
        avatar: string
    }
}

export interface DepartmentSummary {
    department_id: string
    department_name: string
    member_count: number
    active_users: number
    request_count: number
    used_amount: number
    total_tokens: number
    input_tokens: number
    output_tokens: number
    success_rate: number
    avg_cost: number
    unique_models: number
}

export interface DepartmentSummaryResponse {
    departments: DepartmentSummary[]
    total: number
}

export interface DepartmentTrendPoint {
    hour_timestamp: number
    request_count: number
    used_amount: number
    total_tokens: number
}

export interface DepartmentTrendResponse {
    department_id: string
    trend: DepartmentTrendPoint[]
}

export interface UserRankingItem {
    rank: number
    group_id: string
    user_name: string
    department_id: string
    department_name: string
    request_count: number
    used_amount: number
    total_tokens: number
    input_tokens: number
    output_tokens: number
    success_rate: number
    unique_models: number
}

export interface UserRankingResponse {
    ranking: UserRankingItem[]
    total: number
}

export interface ModelDistributionItem {
    model: string
    request_count: number
    total_tokens: number
    input_tokens: number
    output_tokens: number
    used_amount: number
    unique_users: number
    percentage: number
}

export interface ModelDistributionResponse {
    distribution: ModelDistributionItem[]
    total: number
}

export interface PeriodStats {
    request_count: number
    total_tokens: number
    used_amount: number
    active_users: number
}

export interface ComparisonData {
    period_type: string
    current_period: PeriodStats
    previous_period: PeriodStats
    changes: {
        request_count_pct: number
        total_tokens_pct: number
        used_amount_pct: number
        active_users_pct: number
    }
}

export interface DepartmentRankingItem {
    rank: number
    department_id: string
    department_name: string
    active_users: number
    used_amount: number
    request_count: number
    total_tokens: number
    input_tokens: number
    output_tokens: number
}

export interface DepartmentRankingResponse {
    ranking: DepartmentRankingItem[]
    total: number
}

function buildTimeParams(startTimestamp?: number, endTimestamp?: number) {
    const params: Record<string, string> = {}
    if (startTimestamp) params.start_timestamp = String(startTimestamp)
    if (endTimestamp) params.end_timestamp = String(endTimestamp)
    return params
}

export const enterpriseApi = {
    feishuCallback: (code: string): Promise<FeishuCallbackResponse> => {
        return get<FeishuCallbackResponse>('/enterprise/auth/feishu/callback', {
            params: { code },
        })
    },

    feishuLoginUrl: (): string => {
        const baseUrl = apiClient.defaults.baseURL || '/api'
        return `${baseUrl}/enterprise/auth/feishu/login`
    },

    getDepartmentSummary: (
        startTimestamp?: number,
        endTimestamp?: number,
    ): Promise<DepartmentSummaryResponse> => {
        return get<DepartmentSummaryResponse>('/enterprise/analytics/department', {
            params: buildTimeParams(startTimestamp, endTimestamp),
        })
    },

    getDepartmentTrend: (
        id: string,
        startTimestamp?: number,
        endTimestamp?: number,
    ): Promise<DepartmentTrendResponse> => {
        return get<DepartmentTrendResponse>(`/enterprise/analytics/department/${id}/trend`, {
            params: buildTimeParams(startTimestamp, endTimestamp),
        })
    },

    getDepartmentRanking: (
        limit?: number,
        startTimestamp?: number,
        endTimestamp?: number,
    ): Promise<DepartmentRankingResponse> => {
        const params: Record<string, string> = buildTimeParams(startTimestamp, endTimestamp)
        if (limit) params.limit = String(limit)
        return get<DepartmentRankingResponse>('/enterprise/analytics/department/ranking', { params })
    },

    getUserRanking: (
        departmentId?: string,
        limit?: number,
        startTimestamp?: number,
        endTimestamp?: number,
    ): Promise<UserRankingResponse> => {
        const params: Record<string, string> = buildTimeParams(startTimestamp, endTimestamp)
        if (departmentId) params.department_id = departmentId
        if (limit) params.limit = String(limit)
        return get<UserRankingResponse>('/enterprise/analytics/user/ranking', { params })
    },

    getModelDistribution: (
        departmentId?: string,
        startTimestamp?: number,
        endTimestamp?: number,
    ): Promise<ModelDistributionResponse> => {
        const params: Record<string, string> = buildTimeParams(startTimestamp, endTimestamp)
        if (departmentId) params.department_id = departmentId
        return get<ModelDistributionResponse>('/enterprise/analytics/model/distribution', { params })
    },

    getComparison: (
        period?: string,
        departmentId?: string,
    ): Promise<ComparisonData> => {
        const params: Record<string, string> = {}
        if (period) params.period = period
        if (departmentId) params.department_id = departmentId
        return get<ComparisonData>('/enterprise/analytics/comparison', { params })
    },

    exportReport: async (startTimestamp?: number, endTimestamp?: number): Promise<void> => {
        const params = buildTimeParams(startTimestamp, endTimestamp)
        const response = await apiClient.get('/enterprise/analytics/export', {
            params,
            responseType: 'blob',
        })
        const url = window.URL.createObjectURL(new Blob([response.data as BlobPart]))
        const link = document.createElement('a')
        link.href = url
        const disposition = response.headers['content-disposition']
        const filename = disposition
            ? disposition.split('filename=')[1]?.replace(/"/g, '')
            : 'enterprise_report.xlsx'
        link.setAttribute('download', filename)
        document.body.appendChild(link)
        link.click()
        link.remove()
        window.URL.revokeObjectURL(url)
    },

    toEnterpriseUser(resp: FeishuCallbackResponse): EnterpriseUser {
        return {
            name: resp.user.name,
            avatar: resp.user.avatar,
            openId: resp.user.open_id,
        }
    },
}
