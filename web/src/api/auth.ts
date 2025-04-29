import { get } from './index'
import { AxiosRequestConfig } from 'axios'
import { ChannelTypeMap } from '@/types/channel'

// Auth API endpoints
export const authApi = {

    // Get channel type metas
    getChannelTypeMetas: (token?: string): Promise<ChannelTypeMap> => {
        const config: AxiosRequestConfig = {}

        if (token) {
            config.headers = {
                Authorization: `${token}`
            }
        }

        return get<ChannelTypeMap>('/channels/type_metas', config)
    },

} 