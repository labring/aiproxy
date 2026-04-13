export interface NovitaModel {
  id: string
  display_name: string
  description: string
  context_size: number
  max_output_tokens: number
  input_token_price_per_m: number
  output_token_price_per_m: number
  endpoints: string[]
  features: string[]
  input_modalities: string[]
  output_modalities: string[]
  model_type: string
  tags: (string | number)[]
  status: number
}

export interface ModelDiff {
  model_id: string
  action: 'add' | 'update' | 'delete' | 'shared'
  old_config?: Record<string, unknown>
  new_config?: Record<string, unknown>
  changes?: string[]
}

export interface SyncSummary {
  total_models: number
  to_add: number
  to_update: number
  to_delete: number
  cross_owner?: number
}

export interface ChannelInfo {
  exists: boolean
  id?: number
  will_create?: boolean
}

export interface ChannelsInfo {
  novita: ChannelInfo
}

export interface SyncDiff {
  summary: SyncSummary
  changes: {
    add: ModelDiff[] | null
    update: ModelDiff[] | null
    delete: ModelDiff[] | null
    shared?: ModelDiff[] | null
  }
  channels: ChannelsInfo
}

export interface SyncOptions {
  auto_create_channels: boolean
  changes_confirmed: boolean
  dry_run?: boolean
  delete_unmatched_model?: boolean
  anthropic_pure_passthrough?: boolean
  allow_passthrough_unknown?: boolean
}

export interface SyncResult {
  success: boolean
  summary: SyncSummary
  duration_ms: number
  errors?: string[]
  details?: {
    models_added?: string[]
    models_updated?: string[]
    models_deleted?: string[]
  }
  channels?: ChannelsInfo
}

export interface SyncProgressEvent {
  type: 'progress' | 'success' | 'error'
  step: string
  message: string
  progress?: number
  data?: unknown
}

export interface DiagnosticResult {
  last_sync_at?: string
  local_models: number
  remote_models: number
  diff?: SyncDiff
  channels: ChannelsInfo
}

export interface SyncHistory {
  id: number
  synced_at: string
  operator?: string
  sync_options: string
  result: string
  status: 'success' | 'partial' | 'failed'
  created_at: string
  result_parsed?: SyncResult
}

export interface NovitaConfig {
  channel_id: number
  api_key: string
  api_base: string
  configured: boolean
  mgmt_token_configured: boolean
  exchange_rate: number
  auto_sync_enabled: boolean
  auto_sync_force_disabled: boolean
}

export interface NovitaChannelItem {
  id: number
  name: string
  base_url: string
  key: string
}

export interface ModelCoverageItem {
  model: string
  endpoints?: string[]
  model_type?: string
}

export interface ModelCoverageResult {
  total: number
  covered: number
  uncovered: ModelCoverageItem[]
}
