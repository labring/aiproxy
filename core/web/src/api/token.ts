// src/api/token.ts
import { get, post, del } from './index'
import { TokensResponse, Token,  TokenStatusRequest } from '@/types/token'

export const tokenApi = {
    getTokens: async (page: number, perPage: number): Promise<TokensResponse> => {
        const response = await get<TokensResponse>('tokens/search', {
            params: {
                p: page,
                per_page: perPage
            }
        })
        return response
    },

    createToken: async (name: string): Promise<Token> => {
        // 重要：group的值与name保持一致，创建时使用auto_create_group=true
        const response = await post<Token>(`token/${name}?auto_create_group=true`, {
            name
        })
        return response
    },

    deleteToken: async (id: number): Promise<void> => {
        await del(`tokens/${id}`)
        return
    },

    updateTokenStatus: async (id: number, status: TokenStatusRequest): Promise<void> => {
        await post(`tokens/${id}/status`, status)
        return
    }
}