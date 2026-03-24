// src/api/group.ts
import { get, post, put, del } from './index'
import {
    Group,
    GroupsResponse,
    GroupConsumptionRankingQuery,
    GroupConsumptionRankingResponse,
    GroupCreateRequest,
    GroupUpdateRequest,
    GroupStatusRequest,
    GroupModelConfig,
    GroupModelConfigSaveRequest
} from '@/types/group'

export const groupApi = {
    // Get all groups with pagination
    getGroups: async (page: number, perPage: number, keyword?: string, order?: string): Promise<GroupsResponse> => {
        const params = new URLSearchParams()
        params.append('p', page.toString())
        params.append('per_page', perPage.toString())
        if (order) {
            params.append('order', order)
        }
        if (keyword) {
            params.append('keyword', keyword)
            const response = await get<GroupsResponse>(`groups/search?${params.toString()}`)
            return response
        }
        const response = await get<GroupsResponse>(`groups/?${params.toString()}`)
        return response
    },

    // Search groups with keyword
    searchGroups: async (keyword: string, page: number, perPage: number, order?: string, status?: number): Promise<GroupsResponse> => {
        const params = new URLSearchParams()
        params.append('keyword', keyword)
        params.append('p', page.toString())
        params.append('per_page', perPage.toString())
        if (order) {
            params.append('order', order)
        }
        if (status !== undefined) {
            params.append('status', status.toString())
        }
        const response = await get<GroupsResponse>(`groups/search?${params.toString()}`)
        return response
    },

    getGroupConsumptionRanking: async (query: GroupConsumptionRankingQuery): Promise<GroupConsumptionRankingResponse> => {
        const params = new URLSearchParams()
        if (query.page !== undefined) {
            params.append('page', query.page.toString())
        }
        if (query.per_page !== undefined) {
            params.append('per_page', query.per_page.toString())
        }
        if (query.start_timestamp !== undefined) {
            params.append('start_timestamp', query.start_timestamp.toString())
        }
        if (query.end_timestamp !== undefined) {
            params.append('end_timestamp', query.end_timestamp.toString())
        }
        if (query.timezone) {
            params.append('timezone', query.timezone)
        }
        if (query.order) {
            params.append('order', query.order)
        }

        const response = await get<GroupConsumptionRankingResponse>(`groups/ranking?${params.toString()}`)
        return response
    },

    // Get a single group by ID
    getGroup: async (groupId: string): Promise<Group> => {
        const response = await get<Group>(`group/${groupId}`)
        return response
    },

    // Create a new group
    createGroup: async (groupId: string, data: GroupCreateRequest): Promise<Group> => {
        const response = await post<Group>(`group/${groupId}`, data)
        return response
    },

    // Update a group
    updateGroup: async (groupId: string, data: GroupUpdateRequest): Promise<Group> => {
        const response = await put<Group>(`group/${groupId}`, data)
        return response
    },

    // Delete a group
    deleteGroup: async (groupId: string): Promise<void> => {
        await del(`group/${groupId}`)
    },

    // Update group status
    updateGroupStatus: async (groupId: string, status: GroupStatusRequest): Promise<void> => {
        await post(`group/${groupId}/status`, status)
    },

    // Update group RPM ratio
    updateGroupRPMRatio: async (groupId: string, rpmRatio: number): Promise<void> => {
        await post(`group/${groupId}/rpm_ratio`, { rpm_ratio: rpmRatio })
    },

    // Update group TPM ratio
    updateGroupTPMRatio: async (groupId: string, tpmRatio: number): Promise<void> => {
        await post(`group/${groupId}/tpm_ratio`, { tpm_ratio: tpmRatio })
    },

    // Get group model configs
    getGroupModelConfigs: async (groupId: string): Promise<GroupModelConfig[]> => {
        const response = await get<GroupModelConfig[]>(`group/${groupId}/model_configs/`)
        return response
    },

    // Save group model configs (batch)
    saveGroupModelConfigs: async (groupId: string, configs: GroupModelConfigSaveRequest[]): Promise<void> => {
        await post(`group/${groupId}/model_configs/`, configs)
    },

    // Get single model config
    getGroupModelConfig: async (groupId: string, model: string): Promise<GroupModelConfig> => {
        const response = await get<GroupModelConfig>(`group/${groupId}/model_config/${model}`)
        return response
    },

    // Save single model config
    saveGroupModelConfig: async (groupId: string, model: string, config: GroupModelConfigSaveRequest): Promise<void> => {
        await post(`group/${groupId}/model_config/${model}`, config)
    },

    // Delete model config
    deleteGroupModelConfig: async (groupId: string, model: string): Promise<void> => {
        await del(`group/${groupId}/model_config/${model}`)
    },

    // Delete multiple model configs
    deleteGroupModelConfigs: async (groupId: string, models: string[]): Promise<void> => {
        await del(`group/${groupId}/model_configs/`, { data: models })
    },
}
