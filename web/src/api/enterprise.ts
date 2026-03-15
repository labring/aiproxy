import { get, post, put, del } from './index'
import type { EnterpriseUser } from '@/store/auth'
import apiClient from './index'

// Quota Policy types
export interface QuotaPolicy {
    id: number
    created_at: string
    updated_at: string
    name: string
    tier1_ratio: number
    tier2_ratio: number
    tier1_rpm_multiplier: number
    tier1_tpm_multiplier: number
    tier2_rpm_multiplier: number
    tier2_tpm_multiplier: number
    tier3_rpm_multiplier: number
    tier3_tpm_multiplier: number
    block_at_tier3: boolean
}

export type QuotaPolicyInput = Omit<QuotaPolicy, 'id' | 'created_at' | 'updated_at'>

export interface QuotaPolicyListResponse {
    policies: QuotaPolicy[]
    total: number
}

export interface GroupQuotaPolicy {
    id: number
    group_id: string
    quota_policy_id: number
    quota_policy?: QuotaPolicy
}

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

// Custom Report types
export interface CustomReportRequest {
    dimensions: string[]
    measures: string[]
    filters: {
        department_ids?: string[]
        models?: string[]
        user_names?: string[]
    }
    time_range: {
        start_timestamp: number
        end_timestamp: number
    }
    sort_by?: string
    sort_order?: string
    limit?: number
}

export interface CustomReportColumn {
    key: string
    label: string
    type: 'dimension' | 'measure' | 'computed'
}

export interface CustomReportResponse {
    columns: CustomReportColumn[]
    rows: Record<string, unknown>[]
    total: number
}

export interface FieldCatalog {
    dimensions: { key: string; label: string }[]
    measures: { key: string; label: string; type: string }[]
    computed_measures: { key: string; label: string; type: string }[]
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

    // Quota Policy APIs
    listQuotaPolicies: (page?: number, perPage?: number): Promise<QuotaPolicyListResponse> => {
        const params: Record<string, string> = {}
        if (page) params.page = String(page)
        if (perPage) params.per_page = String(perPage)
        return get<QuotaPolicyListResponse>('/enterprise/quota/policies', { params })
    },

    getQuotaPolicy: (id: number): Promise<QuotaPolicy> => {
        return get<QuotaPolicy>(`/enterprise/quota/policies/${id}`)
    },

    createQuotaPolicy: (policy: QuotaPolicyInput): Promise<QuotaPolicy> => {
        return post<QuotaPolicy>('/enterprise/quota/policies', policy)
    },

    updateQuotaPolicy: (id: number, policy: QuotaPolicyInput): Promise<QuotaPolicy> => {
        return put<QuotaPolicy>(`/enterprise/quota/policies/${id}`, policy)
    },

    deleteQuotaPolicy: (id: number): Promise<void> => {
        return del<void>(`/enterprise/quota/policies/${id}`)
    },

    bindQuotaPolicy: (groupId: string, quotaPolicyId: number): Promise<GroupQuotaPolicy> => {
        return post<GroupQuotaPolicy>('/enterprise/quota/bind', {
            group_id: groupId,
            quota_policy_id: quotaPolicyId,
        })
    },

    unbindQuotaPolicy: (groupId: string): Promise<void> => {
        return del<void>(`/enterprise/quota/bind/${groupId}`)
    },

    // Custom Report APIs
    getCustomReportFields: (): Promise<FieldCatalog> => {
        return get<FieldCatalog>('/enterprise/analytics/custom-report/fields')
    },

    generateCustomReport: (req: CustomReportRequest): Promise<CustomReportResponse> => {
        return post<CustomReportResponse>('/enterprise/analytics/custom-report', req)
    },
}
