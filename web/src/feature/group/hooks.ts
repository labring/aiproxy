// src/feature/group/hooks.ts
import { useMutation, useQueryClient, useQuery } from '@tanstack/react-query'
import { groupApi } from '@/api/group'
import { useState } from 'react'
import type {
    GroupCreateRequest,
    GroupUpdateRequest,
    GroupStatusRequest,
    GroupModelConfigSaveRequest
} from '@/types/group'
import type { ConsumptionRankingQuery } from '@/types/consumption-ranking'
import { toast } from 'sonner'

// Get groups list (paginated)
export const useGroups = (page: number, perPage: number, keyword?: string) => {
    const query = useQuery({
        queryKey: ['groups', page, perPage, keyword],
        queryFn: () => groupApi.getGroups(page, perPage, keyword),
    })

    return {
        ...query,
    }
}

// Search groups
export const useSearchGroups = (keyword: string, page: number, perPage: number, order?: string, status?: number) => {
    const query = useQuery({
        queryKey: ['groups', 'search', keyword, page, perPage, order, status],
        queryFn: () => groupApi.searchGroups(keyword, page, perPage, order, status),
        enabled: keyword.length > 0,
    })

    return {
        ...query,
    }
}

export const useConsumptionRanking = (query: ConsumptionRankingQuery) => {
    const rankingPage = query.page ?? 1
    const rankingPageSize = query.per_page ?? 20

    return useQuery({
        queryKey: [
            'consumption-ranking',
            query.type,
            rankingPage,
            rankingPageSize,
            query.start_timestamp,
            query.end_timestamp,
            query.timezone,
            query.order,
        ],
        queryFn: () => groupApi.getConsumptionRanking(query),
    })
}

// Get single group
export const useGroup = (groupId: string) => {
    const query = useQuery({
        queryKey: ['group', groupId],
        queryFn: () => groupApi.getGroup(groupId),
        enabled: !!groupId,
    })

    return {
        ...query,
    }
}

// Create group
export const useCreateGroup = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: ({ groupId, data }: { groupId: string, data: GroupCreateRequest }) => {
            return groupApi.createGroup(groupId, data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['groups'] })
            setError(null)
            toast.success('Group created successfully')
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message || 'Failed to create group')
        },
    })

    return {
        createGroup: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}

// Update group
export const useUpdateGroup = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: ({ groupId, data }: { groupId: string, data: GroupUpdateRequest }) => {
            return groupApi.updateGroup(groupId, data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['groups'] })
            setError(null)
            toast.success('Group updated successfully')
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message || 'Failed to update group')
        },
    })

    return {
        updateGroup: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}

// Delete group
export const useDeleteGroup = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: (groupId: string) => {
            return groupApi.deleteGroup(groupId)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['groups'] })
            setError(null)
            toast.success('Group deleted successfully')
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message || 'Failed to delete group')
        },
    })

    return {
        deleteGroup: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}

// Update group status
export const useUpdateGroupStatus = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: ({ groupId, status }: { groupId: string, status: GroupStatusRequest }) => {
            return groupApi.updateGroupStatus(groupId, status)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['groups'] })
            setError(null)
            toast.success('Status updated successfully')
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message || 'Failed to update status')
        },
    })

    return {
        updateStatus: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}

// Get group model configs
export const useGroupModelConfigs = (groupId: string) => {
    const query = useQuery({
        queryKey: ['groupModelConfigs', groupId],
        queryFn: () => groupApi.getGroupModelConfigs(groupId),
        enabled: !!groupId,
    })

    return {
        ...query,
    }
}

// Save group model configs
export const useSaveGroupModelConfigs = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: ({ groupId, configs }: { groupId: string, configs: GroupModelConfigSaveRequest[] }) => {
            return groupApi.saveGroupModelConfigs(groupId, configs)
        },
        onSuccess: (_, variables) => {
            queryClient.invalidateQueries({ queryKey: ['groupModelConfigs', variables.groupId] })
            setError(null)
            toast.success('Model configs saved successfully')
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message || 'Failed to save model configs')
        },
    })

    return {
        saveConfigs: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}
