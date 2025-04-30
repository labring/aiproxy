// src/types/token.ts
export interface Token {
    key: string
    name: string
    group: string
    subnets: string[] | null
    models: string[] | null
    status: number
    id: number
    quota: number
    used_amount: number
    request_count: number
    created_at: number
    expired_at: number
    accessed_at: number
}

export interface TokensResponse {
    tokens: Token[]
    total: number
}

export interface TokenCreateRequest {
    name: string
}

export interface TokenStatusRequest {
    status: number
}