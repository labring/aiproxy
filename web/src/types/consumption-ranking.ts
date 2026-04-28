export type ConsumptionRankingType = 'group' | 'channel' | 'model'

export interface ConsumptionRankingQuery {
    type: ConsumptionRankingType
    start_timestamp?: number
    end_timestamp?: number
    timezone?: string
    page?: number
    per_page?: number
    order?: string
}

export interface ConsumptionRankingItem {
    rank: number
    group_id?: string
    channel_id?: number
    model?: string
    request_count: number
    used_amount: number
    input_tokens: number
    output_tokens: number
    total_tokens: number
}

export interface ConsumptionRankingResponse {
    items: ConsumptionRankingItem[]
    total: number
    type: ConsumptionRankingType
}
