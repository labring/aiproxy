// src/types/log.ts

// 价格信息
export interface LogPrice {
  cache_creation_price: number
  cache_creation_price_unit: number
  cached_price: number
  cached_price_unit: number
  image_input_price: number
  image_input_price_unit: number
  input_price: number
  input_price_unit: number
  output_price: number
  output_price_unit: number
  per_request_price: number
  thinking_mode_output_price: number
  thinking_mode_output_price_unit: number
  web_search_price: number
  web_search_price_unit: number
}

// 使用情况
export interface LogUsage {
  cache_creation_tokens: number
  cached_tokens: number
  image_input_tokens: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  total_tokens: number
  web_search_count: number
}

// 请求详情
export interface LogRequestDetail {
  id: number
  log_id: number
  request_body: string
  request_body_truncated: boolean
  response_body: string
  response_body_truncated: boolean
}

// 日志记录
export interface LogRecord {
  channel: number
  code: number
  content: string
  created_at: string
  endpoint: string
  group: string
  id: number
  ip: string
  metadata: Record<string, string>
  mode: number
  model: string
  price: LogPrice
  request_at: string
  request_detail: LogRequestDetail
  request_id: string
  retry_at: string
  retry_times: number
  token_id: number
  token_name: string
  ttfb_milliseconds: number
  usage: LogUsage
  used_amount: number
  user: string
}

// 日志响应数据
export interface LogResponse {
  channels: number[]
  logs: LogRecord[]
  models: string[]
  token_names: string[]
  total: number
}

// 日志过滤器
export interface LogFilters {
  keyName?: string // token name
  model?: string
  start_timestamp?: number
  end_timestamp?: number
  code_type?: 'all' | 'success' | 'error'
  page?: number
  per_page?: number
}

// 日志列表请求参数
export interface LogListParams extends LogFilters {
  group?: string // 当keyName有值时，group = keyName
} 