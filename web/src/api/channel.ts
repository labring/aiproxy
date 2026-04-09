// src/api/channel.ts
import { get, post, put, del } from './index'
import { useAuthStore } from '@/store/auth'
import {
    ChannelTypeMetaMap,
    ChannelsResponse,
    ChannelCreateRequest,
    ChannelUpdateRequest,
    ChannelStatusRequest,
    Channel
} from '@/types/channel'

// 渠道测试结果类型
export interface ChannelTestResult {
    data?: {
        test_at: string
        model: string
        actual_model: string
        response: string
        channel_name: string
        channel_type: number
        channel_id: number
        took: number
        success: boolean
        mode: string
        code: number
    }
    message?: string
    success: boolean
}

export const channelApi = {
    getTypeMetas: async (): Promise<ChannelTypeMetaMap> => {
        const response = await get<ChannelTypeMetaMap>('channels/type_metas')
        return response
    },

    getChannels: async (
        page: number,
        perPage: number,
        keyword?: string,
        channelType?: number
    ): Promise<ChannelsResponse> => {
        const params: Record<string, string | number> = {
            p: page,
            per_page: perPage,
        }
        if (keyword) {
            params.keyword = keyword
        }
        if (channelType && channelType > 0) {
            params.channel_type = channelType
        }
        const response = await get<ChannelsResponse>('channels/search', { params })
        return response
    },

    getAllChannels: async (): Promise<Channel[]> => {
        const response = await get<Channel[]>('channels/all')
        return response
    },

    getChannel: async (id: number): Promise<Channel> => {
        const response = await get<Channel>(`channel/${id}`)
        return response
    },

    getChannelBatchInfo: async (ids: number[]): Promise<{ id: number; name: string; type: number }[]> => {
        const response = await post<{ id: number; name: string; type: number }[]>('channels/batch_info', ids)
        return response
    },

    createChannel: async (data: ChannelCreateRequest): Promise<void> => {
        await post('channel/', data)
        return
    },

    updateChannel: async (id: number, data: ChannelUpdateRequest): Promise<void> => {
        await put(`channel/${id}`, data)
        return
    },

    deleteChannel: async (id: number): Promise<void> => {
        await del(`channel/${id}`)
        return
    },

    updateChannelStatus: async (id: number, status: ChannelStatusRequest): Promise<void> => {
        await post(`channel/${id}/status`, status)
        return
    },

    // 测试渠道所有模型 (SSE 模式)
    testChannel: (
        id: number,
        onResult: (result: ChannelTestResult) => void,
        onComplete: () => void,
        onError: (error: Error) => void
    ): (() => void) => {
        // Get authorization token from auth store
        const token = useAuthStore.getState().token

        // Build URL with query parameters including token
        const params = new URLSearchParams({
            return_success: 'true',
            success_body: 'true',
            stream: 'true',
        })
        if (token) {
            params.set('key', token)
        }

        const eventSource = new EventSource(
            `/api/channel/${id}/test?${params.toString()}`,
            { withCredentials: true }
        )

        eventSource.onmessage = (event) => {
            try {
                const result = JSON.parse(event.data) as ChannelTestResult
                onResult(result)
            } catch (e) {
                console.error('Failed to parse test result:', e)
            }
        }

        eventSource.onerror = (error) => {
            console.error('SSE error:', error)
            onError(new Error('测试连接失败'))
            eventSource.close()
            onComplete()
        }

        eventSource.addEventListener('done', () => {
            eventSource.close()
            onComplete()
        })

        // 返回取消函数
        return () => {
            eventSource.close()
        }
    },

    // 测试单个模型
    testChannelModel: async (id: number, model: string): Promise<ChannelTestResult> => {
        const response = await get<ChannelTestResult>(`channel/${id}/${model}?success_body=true`)
        return response
    },

    // 测试未保存的渠道配置（单个模型）
    testChannelPreview: async (data: {
        type: number
        key: string
        base_url?: string
        proxy_url?: string
        name?: string
        model: string
        model_mapping?: Record<string, string>
        configs?: Record<string, unknown>
    }): Promise<ChannelTestResult> => {
        const response = await post<ChannelTestResult>('channel/test-preview?return_success=true&success_body=true', data)
        return response
    },

    // 测试未保存的渠道配置（所有模型，SSE 模式）
    testChannelPreviewAllStream: (
        data: {
            type: number
            key: string
            base_url?: string
            proxy_url?: string
            name?: string
            models: string[]
            model_mapping?: Record<string, string>
            configs?: Record<string, unknown>
        },
        onResult: (result: ChannelTestResult) => void,
        onComplete: () => void,
        onError: (error: Error) => void
    ): (() => void) => {
        // 使用 POST 请求但需要 SSE 响应
        // 先创建请求体
        const body = JSON.stringify(data)

        // 使用 fetch API 发送 POST 请求并处理 SSE 响应
        const controller = new AbortController()
        const signal = controller.signal

        // Get authorization token from auth store
        const token = useAuthStore.getState().token

        fetch('/api/channel/test-preview-all?stream=true&return_success=true&success_body=true', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                ...(token ? { 'Authorization': token } : {}),
            },
            body: body,
            signal: signal,
            credentials: 'include',
        }).then(response => {
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`)
            }

            const reader = response.body?.getReader()
            if (!reader) {
                throw new Error('No response body')
            }

            const decoder = new TextDecoder()

            const readChunk = () => {
                reader.read().then(({ done, value }) => {
                    if (done) {
                        onComplete()
                        return
                    }

                    const chunk = decoder.decode(value, { stream: true })
                    const lines = chunk.split('\n')

                    for (const line of lines) {
                        if (line.startsWith('data: ')) {
                            const data = line.slice(6)
                            if (data === '[DONE]') {
                                onComplete()
                                return
                            }
                            try {
                                const result = JSON.parse(data) as ChannelTestResult
                                onResult(result)
                            } catch (e) {
                                console.error('Failed to parse SSE data:', e)
                            }
                        }
                    }

                    readChunk()
                }).catch(err => {
                    if (err.name !== 'AbortError') {
                        onError(err)
                    }
                })
            }

            readChunk()
        }).catch(err => {
            if (err.name !== 'AbortError') {
                onError(err)
            }
        })

        return () => {
            controller.abort()
        }
    },

    // 测试所有已保存的渠道 (SSE 模式)
    testAllChannels: (
        onResult: (result: ChannelTestResult) => void,
        onComplete: () => void,
        onError: (error: Error) => void
    ): (() => void) => {
        // Get authorization token from auth store
        const token = useAuthStore.getState().token

        // Build URL with query parameters including token
        const params = new URLSearchParams({
            return_success: 'true',
            success_body: 'true',
            stream: 'true',
        })
        if (token) {
            params.set('key', token)
        }

        const eventSource = new EventSource(
            `/api/channels/test?${params.toString()}`,
            { withCredentials: true }
        )

        eventSource.onmessage = (event) => {
            try {
                const result = JSON.parse(event.data) as ChannelTestResult
                onResult(result)
            } catch (e) {
                console.error('Failed to parse test result:', e)
            }
        }

        eventSource.onerror = (error) => {
            console.error('SSE error:', error)
            onError(new Error('测试连接失败'))
            eventSource.close()
            onComplete()
        }

        eventSource.addEventListener('done', () => {
            eventSource.close()
            onComplete()
        })

        // 返回取消函数
        return () => {
            eventSource.close()
        }
    }
}
