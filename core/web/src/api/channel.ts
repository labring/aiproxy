// src/api/channel.ts
import { get, post, put, del } from './index'
import {
    ChannelTypeMetaMap,
    ChannelsResponse,
    ChannelCreateRequest,
    ChannelUpdateRequest,
    ChannelStatusRequest
} from '@/types/channel'

export const channelApi = {
    getTypeMetas: async (): Promise<ChannelTypeMetaMap> => {
        const response = await get<ChannelTypeMetaMap>('channels/type_metas')
        return response
    },

    getChannels: async (page: number, perPage: number): Promise<ChannelsResponse> => {
        const response = await get<ChannelsResponse>('channels/search', {
            params: {
                p: page,
                per_page: perPage
            }
        })
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
    }
}