import { get, post, put } from './index'
import { useAuthStore } from '@/store/auth'
import type {
  DiagnosticResult,
  ModelCoverageResult,
  NovitaChannelItem,
  NovitaConfig,
  SyncDiff,
  SyncHistory,
  SyncOptions,
  SyncProgressEvent,
  SyncResult
} from '../types/novita'

export const novitaApi = {
  listChannels: async (): Promise<NovitaChannelItem[]> => {
    return get<NovitaChannelItem[]>('/enterprise/novita/channels')
  },

  getConfig: async (): Promise<NovitaConfig> => {
    return get<NovitaConfig>('/enterprise/novita/config')
  },

  updateConfig: async (channelId: number): Promise<void> => {
    return put('/enterprise/novita/config', { channel_id: channelId })
  },

  diagnostic: async (): Promise<DiagnosticResult> => {
    return get<DiagnosticResult>('/enterprise/novita/sync/diagnostic')
  },

  preview: async (opts: SyncOptions): Promise<SyncDiff> => {
    return post<SyncDiff>('/enterprise/novita/sync/preview', opts)
  },

  execute: (
    opts: SyncOptions,
    onProgress: (event: SyncProgressEvent) => void,
    onComplete: (result: SyncResult) => void,
    onError: (error: Error) => void
  ): (() => void) => {
    const controller = new AbortController()

    const token = useAuthStore.getState().token

    fetch('/api/enterprise/novita/sync/execute', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
        ...(token ? { 'Authorization': token } : {}),
      },
      body: JSON.stringify(opts),
      signal: controller.signal
    })
      .then(async response => {
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`)
        }

        const reader = response.body?.getReader()
        if (!reader) {
          throw new Error('Response body is null')
        }

        const decoder = new TextDecoder()
        let buffer = ''

        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          buffer += decoder.decode(value, { stream: true })
          const lines = buffer.split('\n')
          buffer = lines.pop() || ''

          for (const line of lines) {
            if (line.startsWith('data: ')) {
              const data = line.slice(6)
              try {
                const event: SyncProgressEvent = JSON.parse(data)
                if (event.type === 'progress') {
                  onProgress(event)
                } else if (event.type === 'success') {
                  onProgress(event)
                  onComplete(event.data as SyncResult)
                } else if (event.type === 'error') {
                  onError(new Error(event.message))
                }
              } catch (err) {
                console.error('Failed to parse SSE event:', err)
              }
            }
          }
        }
      })
      .catch(err => {
        if (err.name !== 'AbortError') {
          onError(err)
        }
      })

    return () => controller.abort()
  },

  updateMgmtToken: async (token: string): Promise<void> => {
    return put('/enterprise/novita/mgmt-token', { token })
  },

  history: async (): Promise<SyncHistory[]> => {
    return get<SyncHistory[]>('/enterprise/novita/sync/history')
  },

  modelCoverage: async (): Promise<ModelCoverageResult> => {
    return get<ModelCoverageResult>('/enterprise/novita/model-coverage')
  }
}
