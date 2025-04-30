// src/types/model.ts
export interface ModelConfigDetail {
    max_input_tokens?: number
    max_output_tokens?: number
    max_context_tokens?: number
    vision?: boolean
    tool_choice?: boolean
    support_formats?: string[]
    support_voices?: string[]
}

export interface ModelPrice {
    input_price: number
    output_price: number
    per_request_price: number
    cache_creation_price?: number
    cache_creation_price_unit?: number
    cached_price?: number
    cached_price_unit?: number
    image_input_price?: number
    image_input_price_unit?: number
    image_output_price?: number
    image_output_price_unit?: number
    web_search_price?: number
    web_search_price_unit?: number
}

export interface ModelConfig {
    config?: ModelConfigDetail
    created_at: number
    updated_at: number
    image_prices: number[] | null
    model: string
    owner: string
    image_batch_size?: number
    type: number
    price: ModelPrice
    rpm: number
    tpm?: number
}

export interface ModelCreateRequest {
    model: string
    type: number
}