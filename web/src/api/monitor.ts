import { get, post } from './index'
import {
    BatchGroupTokenMetricsRequestItem,
    BatchGroupTokenMetricsResponse,
    GroupModelMetricsResponse,
    GroupSummaryMetricsQuery,
    GroupSummaryMetricsResponse,
    GroupTokenMetricsResponse,
    GroupTokennameModelMetricsResponse,
    RuntimeMetricsResponse
} from '@/types/runtime-metrics'

export const monitorApi = {
    getRuntimeMetrics: async (): Promise<RuntimeMetricsResponse> => {
        const response = await get<RuntimeMetricsResponse>('monitor/runtime_metrics')
        return response
    },

    getGroupSummaryMetrics: async (query?: GroupSummaryMetricsQuery): Promise<GroupSummaryMetricsResponse> => {
        const params = new URLSearchParams()
        if (query?.groups?.length) {
            params.append('groups', query.groups.join(','))
        }

        const suffix = params.toString()
        const response = await get<GroupSummaryMetricsResponse>(`monitor/group_summary_metrics${suffix ? `?${suffix}` : ''}`)
        return response
    },

    getGroupTokenMetrics: async (groupId: string): Promise<GroupTokenMetricsResponse> => {
        const response = await get<GroupTokenMetricsResponse>(`monitor/group_token_metrics/${groupId}`)
        return response
    },

    getGroupModelMetrics: async (groupId: string): Promise<GroupModelMetricsResponse> => {
        const response = await get<GroupModelMetricsResponse>(`monitor/group_model_metrics/${groupId}`)
        return response
    },

    getGroupTokennameModelMetrics: async (groupId: string): Promise<GroupTokennameModelMetricsResponse> => {
        const response = await get<GroupTokennameModelMetricsResponse>(`monitor/group_tokenname_model_metrics/${groupId}`)
        return response
    },

    batchGetGroupTokenMetrics: async (items: BatchGroupTokenMetricsRequestItem[]): Promise<BatchGroupTokenMetricsResponse> => {
        const response = await post<BatchGroupTokenMetricsResponse>('monitor/batch_group_token_metrics', { items })
        return response
    },
}
