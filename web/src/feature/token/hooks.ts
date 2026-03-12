// src/feature/token/hooks.ts
import { useMutation, useQueryClient, useQuery } from '@tanstack/react-query'
import { tokenApi } from '@/api/token'
import { useState } from 'react'
import { TokenCreateRequest, TokenStatusRequest, TokenUpdateRequest } from '@/types/token'
import { toast } from 'sonner'

// 获取Token列表（分页）
export const useTokens = (page: number, perPage: number, keyword?: string) => {
    const query = useQuery({
        queryKey: ['tokens', page, perPage, keyword],
        queryFn: () => tokenApi.getTokens(page, perPage, keyword),
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
            return tokenApi.createToken(data)
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

// 更新Token（包括限额配置）
export const useUpdateToken = () => {
    const queryClient = useQueryClient()
    const [error, setError] = useState<ApiError | null>(null)

    const mutation = useMutation({
        mutationFn: ({ id, data }: { id: number, data: TokenUpdateRequest }) => {
            return tokenApi.updateToken(id, data)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['tokens'] })
            queryClient.invalidateQueries({ queryKey: ['groupTokens'] })
            queryClient.invalidateQueries({ queryKey: ['groups'] })
            setError(null)
            toast.success('Token更新成功')
        },
        onError: (err: ApiError) => {
            setError(err)
            toast.error(err.message || 'Token更新失败')
        },
    })

    return {
        updateToken: mutation.mutate,
        isLoading: mutation.isPending,
        error,
        clearError: () => setError(null),
    }
}
