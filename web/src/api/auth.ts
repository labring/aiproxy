import { get } from './index'
import { AxiosRequestConfig } from 'axios'
import { ChannelTypeMeta } from '@/types/channel'

// Auth API endpoints
export const authApi = {

    // Get channel type metas
    getChannelTypeMetas: (token?: string): Promise<ChannelTypeMeta[]> => {
        const config: AxiosRequestConfig = {}

        if (token) {
            config.headers = {
                Authorization: `${token}`
            }
        }

        return get<ChannelTypeMeta[]>('/channels/type_metas', config)
    },

} 