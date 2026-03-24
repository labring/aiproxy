// src/feature/channel/hooks.ts
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { channelApi, ChannelTestResult } from '@/api/channel'
import { modelApi } from '@/api/model'
import { useState, useCallback } from 'react'
import { ChannelCreateRequest, ChannelUpdateRequest, ChannelStatusRequest } from '@/types/channel'
import { toast } from 'sonner'

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

// 获取渠道类型默认模型
export const useChannelDefaultModels = (type: number) => {
    return useQuery({
        queryKey: ['channelDefaultModels', type],
        queryFn: () => modelApi.getDefaultModelsByType(type),
        enabled: type > 0,
    })
}

// 获取所有渠道类型默认模型
export const useAllChannelDefaultModels = () => {
    return useQuery({
        queryKey: ['allChannelDefaultModels'],
        queryFn: () => modelApi.getAllDefaultModels(),
    })
}

// 获取渠道列表（分页）
export const useChannels = (
    page: number,
    perPage: number,
    keyword?: string,
    channelType?: number
) => {
    const query = useQuery({
        queryKey: ['channels', page, perPage, keyword, channelType],
        queryFn: () => channelApi.getChannels(page, perPage, keyword, channelType),
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

// 测试渠道 (SSE 模式)
export const useTestChannel = () => {
    const [isTesting, setIsTesting] = useState(false)
    const [results, setResults] = useState<ChannelTestResult[]>([])
    const [error, setError] = useState<string | null>(null)
    const [cancelRef, setCancelRef] = useState<(() => void) | null>(null)

    const testChannel = useCallback((id: number) => {
        setIsTesting(true)
        setResults([])
        setError(null)

        const cancel = channelApi.testChannel(
            id,
            (result) => {
                setResults(prev => [...prev, result])
            },
            () => {
                setIsTesting(false)
                // 使用 setResults 的回调来获取最新的 results
                setResults(prev => {
                    const failedTests = prev.filter(r => !r.success || (r.data && !r.data.success))
                    if (failedTests.length === 0 && prev.length > 0) {
                        toast.success('渠道测试全部通过')
                    } else if (failedTests.length > 0) {
                        toast.warning(`部分模型测试失败 (${failedTests.length}/${prev.length})`)
                    }
                    return prev
                })
            },
            (err) => {
                setIsTesting(false)
                setError(err.message)
                toast.error(err.message)
            }
        )

        setCancelRef(() => cancel)
    }, [])

    const cancelTest = useCallback(() => {
        if (cancelRef) {
            cancelRef()
            setIsTesting(false)
            setCancelRef(null)
        }
    }, [cancelRef])

    return {
        testChannel,
        cancelTest,
        isTesting,
        results,
        error,
        clearError: () => setError(null),
        clearResults: () => setResults([]),
    }
}

// 测试未保存的渠道配置（单个模型）
export const useTestChannelPreview = () => {
    const [isTesting, setIsTesting] = useState(false)
    const [error, setError] = useState<string | null>(null)

    const testChannelPreview = async (data: {
        type: number
        key: string
        base_url?: string
        proxy_url?: string
        name?: string
        model: string
        model_mapping?: Record<string, string>
        configs?: Record<string, unknown>
    }) => {
        setIsTesting(true)
        setError(null)

        try {
            const result = await channelApi.testChannelPreview(data)
            if (result.success) {
                toast.success('渠道测试成功')
            } else {
                const message = result.message || (result.data?.response?.substring(0, 200)) || '测试失败'
                toast.error(`测试失败: ${message}`)
            }
            return result
        } catch (err) {
            const errorMessage = err instanceof Error ? err.message : '测试请求失败'
            setError(errorMessage)
            toast.error(errorMessage)
            return null
        } finally {
            setIsTesting(false)
        }
    }

    return {
        testChannelPreview,
        isTesting,
        error,
        clearError: () => setError(null),
    }
}

// 测试未保存的渠道配置（所有模型，SSE 模式）
export const useTestChannelPreviewAll = () => {
    const [isTesting, setIsTesting] = useState(false)
    const [results, setResults] = useState<ChannelTestResult[]>([])
    const [error, setError] = useState<string | null>(null)
    const [cancelRef, setCancelRef] = useState<(() => void) | null>(null)

    const testChannelPreviewAll = useCallback((data: {
        type: number
        key: string
        base_url?: string
        proxy_url?: string
        name?: string
        models: string[]
        model_mapping?: Record<string, string>
        configs?: Record<string, unknown>
    }) => {
        setIsTesting(true)
        setResults([])
        setError(null)

        const cancel = channelApi.testChannelPreviewAllStream(
            data,
            (result) => {
                setResults(prev => [...prev, result])
            },
            () => {
                setIsTesting(false)
                // 检查结果
                setResults(prev => {
                    const failedTests = prev.filter(r => !r.success || (r.data && !r.data.success))
                    if (failedTests.length === 0 && prev.length > 0) {
                        toast.success('渠道测试全部通过')
                    } else if (failedTests.length > 0) {
                        toast.warning(`部分模型测试失败 (${failedTests.length}/${prev.length})`)
                    }
                    return prev
                })
            },
            (err) => {
                setIsTesting(false)
                setError(err.message)
                toast.error(err.message)
            }
        )

        setCancelRef(() => cancel)
    }, [])

    const cancelTest = useCallback(() => {
        if (cancelRef) {
            cancelRef()
            setIsTesting(false)
            setCancelRef(null)
        }
    }, [cancelRef])

    return {
        testChannelPreviewAll,
        cancelTest,
        isTesting,
        results,
        error,
        clearError: () => setError(null),
        clearResults: () => setResults([]),
    }
}

// 测试所有已保存的渠道 (SSE 模式)
export const useTestAllChannels = () => {
    const [isTesting, setIsTesting] = useState(false)
    const [results, setResults] = useState<ChannelTestResult[]>([])
    const [error, setError] = useState<string | null>(null)
    const [cancelRef, setCancelRef] = useState<(() => void) | null>(null)

    const testAllChannels = useCallback(() => {
        setIsTesting(true)
        setResults([])
        setError(null)

        const cancel = channelApi.testAllChannels(
            (result) => {
                setResults(prev => [...prev, result])
            },
            () => {
                setIsTesting(false)
                // 检查结果
                setResults(prev => {
                    const failedTests = prev.filter(r => !r.success || (r.data && !r.data.success))
                    if (failedTests.length === 0 && prev.length > 0) {
                        toast.success('所有渠道测试全部通过')
                    } else if (failedTests.length > 0) {
                        toast.warning(`部分渠道测试失败 (${failedTests.length}/${prev.length})`)
                    }
                    return prev
                })
            },
            (err) => {
                setIsTesting(false)
                setError(err.message)
                toast.error(err.message)
            }
        )

        setCancelRef(() => cancel)
    }, [])

    const cancelTest = useCallback(() => {
        if (cancelRef) {
            cancelRef()
            setIsTesting(false)
            setCancelRef(null)
        }
    }, [cancelRef])

    return {
        testAllChannels,
        cancelTest,
        isTesting,
        results,
        error,
        clearError: () => setError(null),
        clearResults: () => setResults([]),
    }
}
