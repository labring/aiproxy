// src/feature/token/hooks.ts
import { useMutation, useQueryClient, useInfiniteQuery } from '@tanstack/react-query'
import { tokenApi } from '@/api/token'
import { useState } from 'react'
import { TokenCreateRequest, TokenStatusRequest } from '@/types/token'
import { toast } from 'sonner'
import { ConstantCategory, getConstant } from '@/constant'

// 获取Token列表（支持无限滚动）
export const useTokens = () => {
    const query = useInfiniteQuery({
        queryKey: ['tokens'],
        queryFn: ({ pageParam }) => tokenApi.getTokens(pageParam as number, getConstant(ConstantCategory.CONFIG, 'DEFAULT_PAGE_SIZE', 20)),
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
                return count + (page.tokens?.length || 0)
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

// 创建Token
export const useCreateToken = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: (data: TokenCreateRequest) => {
            return tokenApi.createToken(data.name)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['tokens'] })
            setError(null)
            toast.success('API Key创建成功')
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message || '创建API Key失败')
        },
    })

    return {
        createToken: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}

// 删除Token
export const useDeleteToken = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: (id: number) => {
            return tokenApi.deleteToken(id)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['tokens'] })
            setError(null)
            toast.success('API Key删除成功')
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message || '删除API Key失败')
        },
    })

    return {
        deleteToken: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}

// 更新Token状态
export const useUpdateTokenStatus = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: ({ id, status }: { id: number, status: TokenStatusRequest }) => {
            return tokenApi.updateTokenStatus(id, status)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['tokens'] })
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