import { get, post, put } from './index'
import { useAuthStore } from '@/store/auth'
import type {
  DiagnosticResult,
  ModelCoverageResult,
  PPIOChannelItem,
  PPIOConfig,
  SyncDiff,
  SyncHistory,
  SyncOptions,
  SyncProgressEvent,
  SyncResult
} from '../types/ppio'

export const ppioApi = {
  /**
   * 获取可选的 PPIO Channel 列表
   */
  listChannels: async (): Promise<PPIOChannelItem[]> => {
    return get<PPIOChannelItem[]>('/enterprise/ppio/channels')
  },

  /**
   * 获取当前 PPIO 配置
   */
  getConfig: async (): Promise<PPIOConfig> => {
    return get<PPIOConfig>('/enterprise/ppio/config')
  },

  /**
   * 选择一个 Channel 作为 PPIO 配置
   */
  updateConfig: async (channelId: number): Promise<void> => {
    return put('/enterprise/ppio/config', { channel_id: channelId })
  },

  /**
   * 诊断：对比远程和本地模型差异
   */
  diagnostic: async (): Promise<DiagnosticResult> => {
    return get<DiagnosticResult>('/enterprise/ppio/sync/diagnostic')
  },

  /**
   * 预览：显示将要执行的变更
   */
  preview: async (opts: SyncOptions): Promise<SyncDiff> => {
    return post<SyncDiff>('/enterprise/ppio/sync/preview', opts)
  },

  /**
   * 执行同步（SSE 流式）
   */
  execute: (
    opts: SyncOptions,
    onProgress: (event: SyncProgressEvent) => void,
    onComplete: (result: SyncResult) => void,
    onError: (error: Error) => void
  ): (() => void) => {
    const controller = new AbortController()

    const token = useAuthStore.getState().token

    fetch('/api/enterprise/ppio/sync/execute', {
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

    // Return cancel function
    return () => controller.abort()
  },

  /**
   * 保存管理后台 Token（用于获取闭源模型）
   */
  updateMgmtToken: async (token: string): Promise<void> => {
    return put('/enterprise/ppio/mgmt-token', { token })
  },

  /**
   * 获取同步历史
   */
  history: async (): Promise<SyncHistory[]> => {
    return get<SyncHistory[]>('/enterprise/ppio/sync/history')
  },

  /**
   * 检查模型 Channel 覆盖率（有 ModelConfig 但未分配到任何 Channel 的模型）
   */
  modelCoverage: async (): Promise<ModelCoverageResult> => {
    return get<ModelCoverageResult>('/enterprise/ppio/model-coverage')
  }
}
