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
    tier2_blocked_models: string
    tier3_blocked_models: string
    tier2_price_input_threshold: number
    tier2_price_output_threshold: number
    tier2_price_condition: "and" | "or"
    tier3_price_input_threshold: number
    tier3_price_output_threshold: number
    tier3_price_condition: "and" | "or"
    period_quota: number
    period_type: number // 1=daily, 2=weekly, 3=monthly
}

export type QuotaPolicyInput = Omit<QuotaPolicy, 'id' | 'created_at' | 'updated_at'>

export interface DepartmentQuotaPolicyBinding {
    id: number
    department_id: string
    quota_policy_id: number
    quota_policy?: QuotaPolicy
    level1_name?: string
    level2_name?: string
    member_count?: number
    override_count?: number
    created_at: string
    updated_at: string
}

export interface DepartmentPolicyBindingsResponse {
    bindings: DepartmentQuotaPolicyBinding[]
    total: number
}

export interface UserPolicyBindingsResponse {
    bindings: UserQuotaPolicy[]
    total: number
}

export interface BatchBindResponse {
    bindings: DepartmentQuotaPolicyBinding[]
    errors: string[]
}

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

// Feishu Department Path types
export interface DepartmentPath {
    level1_id: string
    level1_name: string
    level2_id: string
    level2_name: string
    level3_id: string
    level3_name: string
    full_path: string
}

// Feishu User types
export interface FeishuUser {
    id: number
    open_id: string
    union_id: string
    user_id: string
    tenant_id: string
    name: string
    email: string
    avatar: string
    department_id: string
    department_ids: string
    level1_dept_id: string
    level1_dept_name: string
    level2_dept_id: string
    level2_dept_name: string
    dept_full_path: string
    group_id: string
    token_id: number
    role: 'viewer' | 'analyst' | 'admin'
    status: number
    created_at: string
    updated_at: string
    department_path?: DepartmentPath
    effective_policy?: string
    policy_source?: 'user' | 'department'
    quota_usage_percent?: number
    period_quota?: number
    period_used?: number
}

export interface FeishuUsersResponse {
    users: FeishuUser[]
    total: number
}

// Feishu Department types
export interface FeishuDepartment {
    id: number
    department_id: string
    parent_id: string
    name: string
    open_department_id: string
    member_count: number
    order: number
    status: number
    created_at: string
    updated_at: string
}

export interface FeishuDepartmentsResponse {
    departments: FeishuDepartment[]
    total: number
}

// User Quota Policy Assignment
export interface UserQuotaPolicy {
    id: number
    open_id: string
    quota_policy_id: number
    quota_policy?: QuotaPolicy
    user_name?: string
    created_at: string
    updated_at: string
}

// Enterprise API response types
export interface FeishuCallbackResponse {
    session_token?: string   // JWT session token (new, preferred)
    token_key: string        // Legacy API key token (backward compat)
    user: {
        open_id: string
        name: string
        email: string
        avatar: string
        role: 'viewer' | 'analyst' | 'admin'
    }
}

export interface DepartmentSummary {
    department_id: string
    department_name: string
    level1_dept_id: string
    level2_dept_id: string
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
    cached_tokens: number
    cache_creation_tokens: number
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

// Feishu Sync Status
export interface SyncStatus {
    last_sync_at: string
    status: string
    total_depts: number
    depts_with_name: number
    total_users: number
    users_with_name: number
    users_with_email: number
    departed_users: number
    duration_ms: number
    error?: string
}

// Feishu Sync History record (extends SyncStatus with DB metadata)
export interface FeishuSyncHistory extends SyncStatus {
    id: number
    synced_at: string
    created_at: string
}

export interface FeishuSyncHistoryResponse {
    records: FeishuSyncHistory[]
    total: number
}

// Tenant Summary types
export interface TenantSummaryItem {
    tenant_id: string
    name: string
    is_whitelisted: boolean
    whitelist_id?: number
    added_by?: string
    successful_members: number
    rejected_attempts: number
    rejected_record_id?: number
}

export interface TenantSummaryResponse {
    tenants: TenantSummaryItem[]
}

// Rejected Tenant Login types
export interface RejectedTenantLogin {
    id: number
    tenant_id: string
    user_name: string
    user_email: string
    attempt_count: number
    last_attempt_at: string
    created_at: string
}

export interface RejectedTenantLoginsResponse {
    rejected: RejectedTenantLogin[]
}

// Permission types
export interface PermissionKeyInfo {
    key: string
    display_name: string
}

export interface PermissionModuleInfo {
    module: string
    display_name: string
    view_key: string
    manage_key: string
}

// My Stats types
export interface ModelUsage {
    model: string
    used_amount: number
    request_count: number
    total_tokens: number
    success_rate: number
    avg_response_ms: number
    avg_ttfb_ms: number
    avg_cost_per_req: number
}

export interface MetricComparison {
    dept_avg: number
    enterprise_avg: number
}

export interface UsageComparisons {
    total_amount: MetricComparison
    total_tokens: MetricComparison
    total_requests: MetricComparison
    unique_models: MetricComparison
    avg_cost_per_req: MetricComparison
    success_rate: MetricComparison
    avg_response_ms: MetricComparison
    avg_ttfb_ms: MetricComparison
}

export interface MyUsageStats {
    total_amount: number
    total_tokens: number
    total_requests: number
    unique_models: number
    avg_cost_per_req: number
    success_rate: number
    avg_response_ms: number
    avg_ttfb_ms: number
    top_models: ModelUsage[]
    comparisons?: UsageComparisons
}

export interface MyQuotaStatus {
    period_quota: number
    period_used: number
    period_type: string
    period_start: number
    policy_name: string
    policy_id: number
    current_tier: number
    tier1_ratio: number
    tier2_ratio: number
    block_at_tier3: boolean
}

export interface MyStatsResponse {
    usage: MyUsageStats
    quota: MyQuotaStatus | null
}

// My Access types
export interface MyTokenInfo {
    id: number
    name: string
    key: string
    status: number
    created_at: string
    used_amount: number
    request_count: number
}

export interface ModelAccessInfo {
    model: string
    type: number
    type_name: string
    rpm: number
    tpm: number
    input_price: number
    output_price: number
    price_unit: number
    supported_endpoints: string[]
    max_context?: number
    max_output?: number
}

export interface ModelGroupInfo {
    owner: string
    models: ModelAccessInfo[]
}

export interface MyAccessResponse {
    base_url: string
    owner_base_urls?: Record<string, string>
    local_owner?: string
    group_id: string
    tokens: MyTokenInfo[]
    model_groups: ModelGroupInfo[]
}

export interface TokenPeriodStats {
    token_name: string
    used_amount: number
    request_count: number
    total_tokens: number
    success_rate: number
}

export interface UserLog {
    id: number
    request_at: number
    created_at: number
    request_id?: string
    token_name?: string
    model: string
    endpoint: string
    content?: string
    code: number
    usage: {
        input_tokens: number
        output_tokens: number
        total_tokens: number
    }
    used_amount?: number
    ttfb_milliseconds: number
    upstream_id?: string
    has_detail: boolean
}

export interface GetMyLogsResult {
    logs: UserLog[]
    has_more: boolean
}

export interface RequestDetail {
    request_body: string
    response_body: string
    request_body_truncated: boolean
    response_body_truncated: boolean
}

// Quota Notification Config
export interface QuotaNotifConfig {
    enabled: boolean
    tier2_title: string
    tier2_body: string
    tier3_title: string
    tier3_body: string
    exhaust_title: string
    exhaust_body: string
    admin_alert_enabled: boolean
    admin_alert_threshold: number
    admin_alert_title: string
    admin_alert_body: string
    policy_change_title: string
    policy_change_body: string
}

export interface QuotaNotifConfigResponse extends QuotaNotifConfig {
    p2p_available: boolean
}

// Quota Alert History
export interface QuotaAlertHistory {
    id: number
    created_at: string
    open_id: string
    user_name: string
    tier: number
    usage_ratio: number
    period_quota: number
    period_type: string
    title: string
    body: string
    status: string
    error?: string
}

export interface QuotaAlertHistoryResponse {
    records: QuotaAlertHistory[]
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
        if (limit !== undefined) params.limit = String(limit)
        return get<UserRankingResponse>('/enterprise/analytics/user/ranking', { params })
    },

    getModelDistribution: (
        departmentIds?: string[],
        startTimestamp?: number,
        endTimestamp?: number,
    ): Promise<ModelDistributionResponse> => {
        const params = new URLSearchParams()
        const tp = buildTimeParams(startTimestamp, endTimestamp)
        for (const [k, v] of Object.entries(tp)) params.append(k, v)
        if (departmentIds) {
            for (const id of departmentIds) params.append('department_id', id)
        }
        return get<ModelDistributionResponse>('/enterprise/analytics/model/distribution', { params })
    },

    getComparison: (
        departmentIds?: string[],
        startTimestamp?: number,
        endTimestamp?: number,
    ): Promise<ComparisonData> => {
        const params = new URLSearchParams()
        const tp = buildTimeParams(startTimestamp, endTimestamp)
        for (const [k, v] of Object.entries(tp)) params.append(k, v)
        if (departmentIds) {
            for (const id of departmentIds) params.append('department_id', id)
        }
        return get<ComparisonData>('/enterprise/analytics/comparison', { params })
    },

    exportReport: async (
        startTimestamp?: number,
        endTimestamp?: number,
        departmentId?: string,
        limit?: number,
    ): Promise<void> => {
        const params: Record<string, string> = buildTimeParams(startTimestamp, endTimestamp)
        if (departmentId) params.department_id = departmentId
        if (limit !== undefined) params.limit = String(limit)
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
            role: resp.user.role || 'viewer',
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

    // Tenant Whitelist Management
    getTenantWhitelist: (): Promise<{
        tenants: Array<{
            id: number
            tenant_id: string
            name: string
            added_by: string
            created_at: string
        }>
        config: {
            wildcard_mode: boolean
            env_override: boolean
            description: string
        }
    }> => {
        return get('/enterprise/tenant-whitelist')
    },

    addTenantToWhitelist: (tenant_id: string, name?: string): Promise<{ id: number; tenant_id: string; name: string }> => {
        return post('/enterprise/tenant-whitelist', { tenant_id, name })
    },

    updateTenantWhitelist: (id: number, name: string): Promise<{ id: number; tenant_id: string; name: string }> => {
        return put(`/enterprise/tenant-whitelist/${id}`, { name })
    },

    removeTenantFromWhitelist: (id: number): Promise<void> => {
        return del(`/enterprise/tenant-whitelist/${id}`)
    },

    updateWhitelistConfig: (config: {
        wildcard_mode: boolean
        env_override: boolean
        description?: string
    }): Promise<void> => {
        return put('/enterprise/tenant-whitelist/config', config)
    },

    // Feishu User Management APIs
    getFeishuUsers: (
        page?: number,
        per_page?: number,
        keyword?: string,
        sort_by?: string,
        order?: 'asc' | 'desc',
        level1_department?: string,
        level2_department?: string,
        role?: string
    ): Promise<FeishuUsersResponse> => {
        return get<FeishuUsersResponse>('/enterprise/feishu/users', {
            params: { page, per_page, keyword, sort_by, order, level1_department, level2_department, role }
        })
    },

    getFeishuDepartments: (page?: number, per_page?: number, keyword?: string): Promise<FeishuDepartmentsResponse> => {
        return get<FeishuDepartmentsResponse>('/enterprise/feishu/departments', {
            params: { page, per_page, keyword }
        })
    },

    getDepartmentLevels: (level1_id?: string): Promise<{
        level1_departments: FeishuDepartment[]
        level2_departments: FeishuDepartment[]
    }> => {
        return get('/enterprise/feishu/department-levels', {
            params: { level1_id }
        })
    },

    getFeishuSyncStatus: (): Promise<SyncStatus> => {
        return get<SyncStatus>('/enterprise/feishu/sync-status')
    },

    getFeishuSyncHistory: (page = 1, perPage = 10): Promise<FeishuSyncHistoryResponse> => {
        return get<FeishuSyncHistoryResponse>('/enterprise/feishu/sync-history', {
            params: { page, per_page: perPage }
        })
    },

    triggerFeishuSync: (): Promise<{ message: string }> => {
        return post('/enterprise/feishu/sync', {})
    },

    updateFeishuUserRole: (open_id: string, role: string): Promise<void> => {
        return put(`/enterprise/feishu/users/${open_id}/role`, { role })
    },

    // Department Quota APIs
    bindPolicyToDepartment: (department_id: string, quota_policy_id: number): Promise<void> => {
        return post('/enterprise/quota/bind-department', { department_id, quota_policy_id })
    },

    unbindPolicyFromDepartment: (department_id: string): Promise<void> => {
        return del(`/enterprise/quota/bind-department/${department_id}`)
    },

    // User Quota APIs
    bindPolicyToUser: (open_id: string, quota_policy_id: number): Promise<void> => {
        return post('/enterprise/quota/bind-user', { open_id, quota_policy_id })
    },

    unbindPolicyFromUser: (open_id: string): Promise<void> => {
        return del(`/enterprise/quota/bind-user/${open_id}`)
    },

    // Batch bind / list bindings APIs
    batchBindPolicyToDepartments: (department_ids: string[], quota_policy_id: number): Promise<BatchBindResponse> => {
        return post<BatchBindResponse>('/enterprise/quota/batch-bind-departments', { department_ids, quota_policy_id })
    },

    batchBindPolicyToUsers: (open_ids: string[], quota_policy_id: number): Promise<{ bindings: UserQuotaPolicy[]; errors: string[] }> => {
        return post('/enterprise/quota/batch-bind-users', { open_ids, quota_policy_id })
    },

    listDepartmentPolicyBindings: (policy_id?: number): Promise<DepartmentPolicyBindingsResponse> => {
        const params: Record<string, string> = {}
        if (policy_id) params.policy_id = String(policy_id)
        return get<DepartmentPolicyBindingsResponse>('/enterprise/quota/department-bindings', { params })
    },

    listUserPolicyBindings: (policy_id?: number): Promise<UserPolicyBindingsResponse> => {
        const params: Record<string, string> = {}
        if (policy_id) params.policy_id = String(policy_id)
        return get<UserPolicyBindingsResponse>('/enterprise/quota/user-bindings', { params })
    },

    // Tenant Summary API
    getTenantSummary: (): Promise<TenantSummaryResponse> => {
        return get<TenantSummaryResponse>('/enterprise/tenant-whitelist/summary')
    },

    // Rejected Tenant Login APIs
    getRejectedTenantLogins: (): Promise<RejectedTenantLoginsResponse> => {
        return get<RejectedTenantLoginsResponse>('/enterprise/tenant-whitelist/rejected')
    },

    dismissRejectedTenantLogin: (id: number): Promise<void> => {
        return del<void>(`/enterprise/tenant-whitelist/rejected/${id}`)
    },

    // Role Permission APIs
    getMyPermissions: (): Promise<{ role: string; permissions: string[] }> => {
        return get<{ role: string; permissions: string[] }>('/enterprise/role-permissions/my')
    },

    getAllPermissionKeys: (): Promise<{ modules: PermissionModuleInfo[] }> => {
        return get<{ modules: PermissionModuleInfo[] }>('/enterprise/role-permissions/all-keys')
    },

    getRolePermissions: (): Promise<{ roles: Record<string, string[]> }> => {
        return get<{ roles: Record<string, string[]> }>('/enterprise/role-permissions')
    },

    updateRolePermissions: (role: string, permissions: string[]): Promise<{ role: string; permissions: string[] }> => {
        return put<{ role: string; permissions: string[] }>(`/enterprise/role-permissions/${role}`, { permissions })
    },

    // Quota Notification Config APIs
    getNotifConfig: (): Promise<QuotaNotifConfigResponse> => {
        return get<QuotaNotifConfigResponse>('/enterprise/quota/notif-config')
    },

    updateNotifConfig: (cfg: QuotaNotifConfig): Promise<QuotaNotifConfig> => {
        return put<QuotaNotifConfig>('/enterprise/quota/notif-config', cfg)
    },

    getAlertHistory: (page = 1, perPage = 20, filters?: {
        open_id?: string; status?: string; tier?: number;
        keyword?: string; period_type?: string;
        start_time?: number; end_time?: number;
    }): Promise<QuotaAlertHistoryResponse> => {
        return get<QuotaAlertHistoryResponse>('/enterprise/quota/alert-history', {
            params: { page, per_page: perPage, ...filters }
        })
    },

    // My Access APIs
    getMyAccess: (): Promise<MyAccessResponse> => {
        return get<MyAccessResponse>('/enterprise/my-access')
    },

    getMyStats: (startTs: number, endTs: number): Promise<MyStatsResponse> => {
        return get<MyStatsResponse>('/enterprise/my-access/stats', {
            params: { start_timestamp: String(startTs), end_timestamp: String(endTs) },
        })
    },

    createMyToken: (name: string): Promise<MyTokenInfo> => {
        return post<MyTokenInfo>('/enterprise/my-access/tokens', { name })
    },

    disableMyToken: (id: number): Promise<void> => {
        return del<void>(`/enterprise/my-access/tokens/${id}`)
    },

    getMyTokenStats: (startTs: number, endTs: number): Promise<TokenPeriodStats[]> => {
        return get<TokenPeriodStats[]>('/enterprise/my-access/token-stats', {
            params: { start_timestamp: String(startTs), end_timestamp: String(endTs) },
        })
    },

    getMyLogs: (params: {
        start_timestamp?: number
        end_timestamp?: number
        model_name?: string
        code_type?: string
        after_id?: number
        limit?: number
    }): Promise<GetMyLogsResult> => {
        const query = buildTimeParams(params.start_timestamp, params.end_timestamp)
        if (params.model_name) query.model_name = params.model_name
        if (params.code_type) query.code_type = params.code_type
        if (params.after_id) query.after_id = String(params.after_id)
        if (params.limit) query.limit = String(params.limit)
        return get<GetMyLogsResult>('/enterprise/my-access/logs', { params: query })
    },

    getMyLogDetail: (logId: number): Promise<RequestDetail> => {
        return get<RequestDetail>(`/enterprise/my-access/logs/${logId}`)
    },
}
