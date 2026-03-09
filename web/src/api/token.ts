// src/api/token.ts
import { get, post, del, put } from './index'
import { TokensResponse, Token, TokenStatusRequest, TokenUpdateRequest, TokenCreateRequest } from '@/types/token'

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

    getToken: async (id: number): Promise<Token> => {
        const response = await get<Token>(`tokens/${id}`)
        return response
    },

    createToken: async (data: TokenCreateRequest): Promise<Token> => {
        // 重要：group的值与name保持一致，创建时使用auto_create_group=true
        const response = await post<Token>(`token/${data.name}?auto_create_group=true`, {
            name: data.name,
            quota: data.quota,
            period_quota: data.period_quota,
            period_type: data.period_type,
        })
        return response
    },

    deleteToken: async (id: number): Promise<void> => {
        await del(`tokens/${id}`)
        return
    },

    updateToken: async (id: number, data: TokenUpdateRequest): Promise<Token> => {
        const response = await put<Token>(`tokens/${id}`, data)
        return response
    },

    updateTokenStatus: async (id: number, status: TokenStatusRequest): Promise<void> => {
        await post(`tokens/${id}/status`, status)
        return
    },

    getGroupTokens: async (group: string, page: number, perPage: number): Promise<TokensResponse> => {
        const response = await get<TokensResponse>(`token/${group}`, {
            params: {
                p: page,
                per_page: perPage
            }
        })
        return response
    },

    createGroupToken: async (group: string, data: TokenCreateRequest): Promise<Token> => {
        const response = await post<Token>(`token/${group}`, {
            name: data.name,
            quota: data.quota,
            period_quota: data.period_quota,
            period_type: data.period_type,
        })
        return response
    }
}