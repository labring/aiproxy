// src/feature/group/hooks.ts
import { useMutation, useQueryClient, useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { groupApi } from '@/api/group'
import { useState } from 'react'
import type {
    GroupCreateRequest,
    GroupUpdateRequest,
    GroupStatusRequest,
    GroupModelConfigSaveRequest
} from '@/types/group'
import { toast } from 'sonner'
import { ConstantCategory, getConstant } from '@/constant'

// Get groups list (with infinite scroll support)
export const useGroups = () => {
    const query = useInfiniteQuery({
        queryKey: ['groups'],
        queryFn: ({ pageParam }) => groupApi.getGroups(pageParam as number, getConstant(ConstantCategory.CONFIG, 'DEFAULT_PAGE_SIZE', 20)),
        initialPageParam: 1,
        getNextPageParam: (lastPage, allPages) => {
            if (!lastPage || typeof lastPage.total === 'undefined') {
                return undefined
            }

            if (!allPages) {
                return undefined
            }

            const loadedItemsCount = allPages.reduce((count, page) => {
                return count + (page.groups?.length || 0)
            }, 0)

            return lastPage.total > loadedItemsCount ? allPages.length + 1 : undefined
        },
        enabled: true,
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
