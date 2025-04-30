// src/feature/channel/hooks.ts
import { useMutation, useQuery, useQueryClient, useInfiniteQuery } from '@tanstack/react-query'
import { channelApi } from '@/api/channel'
import { useState } from 'react'
import { ChannelCreateRequest, ChannelUpdateRequest, ChannelStatusRequest } from '@/types/channel'
import { toast } from 'sonner'
import { ConstantCategory, getConstant } from '@/constant'

// 获取渠道类型元数据
export const useChannelTypeMetas = () => {
    const query = useQuery({
        queryKey: ['channelTypeMetas'],
        queryFn: channelApi.getTypeMetas,
    })

    return {
        ...query,
    }
}

// 获取渠道列表（支持无限滚动）
export const useChannels = () => {
    const query = useInfiniteQuery({
        queryKey: ['channels'],
        queryFn: ({ pageParam }) => channelApi.getChannels(pageParam as number, getConstant(ConstantCategory.CONFIG, 'DEFAULT_PAGE_SIZE', 20)),
        initialPageParam: 1,
        getNextPageParam: (lastPage, allPages) => {
            if (!lastPage || typeof lastPage.total === 'undefined') {
                return undefined
            }

            // 检查allPages是否存在
            if (!allPages) {
                return undefined
            }

            // 计算已加载的项目总数
            const loadedItemsCount = allPages.reduce((count, page) => {
                return count + (page.channels?.length || 0)
            }, 0)

            // 如果服务器返回的总数大于已加载的数量，则还有下一页
            return lastPage.total > loadedItemsCount ? allPages.length + 1 : undefined
        },
        enabled: true,
    })

    return {
        ...query,
    }
}

// 创建渠道
export const useCreateChannel = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: (data: ChannelCreateRequest) => {
            return channelApi.createChannel(data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['channels'] })
            setError(null)
            toast.success('渠道创建成功')
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message || '创建渠道失败')
        },
    })

    return {
        createChannel: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}

// 更新渠道
export const useUpdateChannel = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: ({ id, data }: { id: number, data: ChannelUpdateRequest }) => {
            return channelApi.updateChannel(id, data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['channels'] })
            setError(null)
            toast.success('渠道更新成功')
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message || '更新渠道失败')
        },
    })

    return {
        updateChannel: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}

// 删除渠道
export const useDeleteChannel = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: (id: number) => {
            return channelApi.deleteChannel(id)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['channels'] })
            setError(null)
            toast.success('渠道删除成功')
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message || '删除渠道失败')
        },
    })

    return {
        deleteChannel: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}

// 更新渠道状态
export const useUpdateChannelStatus = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: ({ id, status }: { id: number, status: ChannelStatusRequest }) => {
            return channelApi.updateChannelStatus(id, status)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['channels'] })
            setError(null)
            toast.success('状态更新成功')
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message || '状态更新失败')
        },
    })

    return {
        updateStatus: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}