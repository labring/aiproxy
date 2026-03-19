import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { RefreshCw, CheckCircle, AlertCircle, Info, ChevronDown, ChevronRight, Clock, History } from 'lucide-react'
import { ppioApi } from '../../api/ppio'
import type { DiagnosticResult, SyncHistory, SyncOptions, SyncProgressEvent, SyncResult } from '../../types/ppio'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'

export default function PPIOSyncPage() {
  const { t } = useTranslation()
  const [diagnostic, setDiagnostic] = useState<DiagnosticResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [progress, setProgress] = useState(0)
  const [progressMessage, setProgressMessage] = useState('')
  const [history, setHistory] = useState<SyncHistory[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({})

  const [syncOpts, setSyncOpts] = useState<SyncOptions>({
    sync_openai: true,
    sync_anthropic: true,
    auto_create_channels: true,
    changes_confirmed: false,
    delete_unmatched_model: false
  })

  const toggleSection = (key: string) => {
    setExpandedSections(prev => ({ ...prev, [key]: !prev[key] }))
  }

  const loadHistory = async () => {
    setHistoryLoading(true)
    try {
      const result = await ppioApi.history()
      setHistory(result || [])
    } catch {
      // Silently fail - history is non-critical
    } finally {
      setHistoryLoading(false)
    }
  }

  useEffect(() => {
    loadHistory()
  }, [])

  const loadDiagnostic = async () => {
    setLoading(true)
    try {
      const result = await ppioApi.diagnostic()
      setDiagnostic(result)
      toast.success(t('common.success'))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  const handleSync = () => {
    if (!diagnostic?.diff) {
      toast.error(t('enterprise.ppio.diagnosticHint'))
      return
    }

    setSyncing(true)
    setProgress(0)
    setProgressMessage('')

    ppioApi.execute(
      { ...syncOpts, changes_confirmed: true },
      (event: SyncProgressEvent) => {
        setProgress(event.progress || 0)
        setProgressMessage(event.message)
      },
      (_result: SyncResult) => {
        setSyncing(false)
        setProgress(100)
        toast.success(t('common.success'))
        loadDiagnostic()
        loadHistory()
      },
      (error: Error) => {
        setSyncing(false)
        toast.error(error.message)
      }
    )
  }

  const formatTime = (timeStr: string) => {
    try {
      return new Date(timeStr).toLocaleString()
    } catch {
      return timeStr
    }
  }

  const statusBadge = (status: string) => {
    const variants: Record<string, string> = {
      success: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
      partial: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400',
      failed: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
    }
    const labels: Record<string, string> = {
      success: t('enterprise.ppio.statusSuccess'),
      partial: t('enterprise.ppio.statusPartial'),
      failed: t('enterprise.ppio.statusFailed'),
    }
    return (
      <Badge className={variants[status] || 'bg-muted text-muted-foreground'}>
        {labels[status] || status}
      </Badge>
    )
  }

  const diff = diagnostic?.diff

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-foreground">{t('enterprise.ppio.title')}</h1>
        <p className="text-sm text-muted-foreground mt-1">{t('enterprise.ppio.description')}</p>
      </div>

      {/* Status Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardContent className="p-4">
            <div className="text-sm text-muted-foreground mb-1">{t('enterprise.ppio.localModels')}</div>
            <div className="text-2xl font-bold text-foreground">{diagnostic?.local_models ?? '-'}</div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="text-sm text-muted-foreground mb-1">{t('enterprise.ppio.remoteModels')}</div>
            <div className="text-2xl font-bold text-foreground">{diagnostic?.remote_models ?? '-'}</div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="text-sm text-muted-foreground mb-1">{t('enterprise.ppio.needSync')}</div>
            <div className="text-2xl font-bold text-orange-600 dark:text-orange-400">
              {diff ? (diff.summary.to_add + diff.summary.to_update) : '-'}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-1.5 text-sm text-muted-foreground mb-1">
              <Clock className="w-3.5 h-3.5" />
              {t('enterprise.ppio.lastSyncAt')}
            </div>
            <div className="text-sm font-medium text-foreground">
              {diagnostic?.last_sync_at ? formatTime(diagnostic.last_sync_at) : t('enterprise.ppio.never')}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Diagnostic Section */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle>{t('enterprise.ppio.diagnosticResult')}</CardTitle>
          <Button
            onClick={loadDiagnostic}
            disabled={loading}
            size="sm"
          >
            <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
            {loading ? t('enterprise.ppio.diagnosing') : t('enterprise.ppio.refreshDiagnostic')}
          </Button>
        </CardHeader>

        <CardContent>
          {loading ? (
            <div className="py-8 text-center text-muted-foreground">
              <RefreshCw className="w-8 h-8 animate-spin mx-auto mb-2" />
              <div>{t('enterprise.ppio.diagnosing')}</div>
            </div>
          ) : diff ? (
            <div className="space-y-4">
              <div className="grid grid-cols-3 gap-4">
                <div className="text-center p-3 bg-green-50 dark:bg-green-900/20 rounded-lg">
                  <div className="text-2xl font-bold text-green-600 dark:text-green-400">{diff.summary.to_add}</div>
                  <div className="text-sm text-muted-foreground">{t('enterprise.ppio.toAdd')}</div>
                </div>
                <div className="text-center p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
                  <div className="text-2xl font-bold text-blue-600 dark:text-blue-400">{diff.summary.to_update}</div>
                  <div className="text-sm text-muted-foreground">{t('enterprise.ppio.toUpdate')}</div>
                </div>
                <div className="text-center p-3 bg-red-50 dark:bg-red-900/20 rounded-lg">
                  <div className="text-2xl font-bold text-red-600 dark:text-red-400">{diff.summary.to_delete}</div>
                  <div className="text-sm text-muted-foreground">{t('enterprise.ppio.toDelete')}</div>
                </div>
              </div>

              {/* Change Details - Collapsible */}
              {(diff.changes.add.length > 0 || diff.changes.update.length > 0 || diff.changes.delete.length > 0) && (
                <div className="space-y-2">
                  <h3 className="text-sm font-medium text-foreground mb-2">{t('enterprise.ppio.changeDetails')}</h3>

                  {diff.changes.add.length > 0 && (
                    <div className="border rounded-lg">
                      <button
                        onClick={() => toggleSection('add')}
                        className="w-full flex items-center gap-2 p-2 text-sm text-left hover:bg-muted/50 rounded-lg transition-colors"
                      >
                        {expandedSections.add ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                        <span className="text-green-600 dark:text-green-400 font-medium">{t('enterprise.ppio.modelsToAdd')} ({diff.changes.add.length})</span>
                      </button>
                      {expandedSections.add && (
                        <div className="px-4 pb-2 space-y-1">
                          {diff.changes.add.map(d => (
                            <div key={d.model_id} className="text-xs text-muted-foreground font-mono">{d.model_id}</div>
                          ))}
                        </div>
                      )}
                    </div>
                  )}

                  {diff.changes.update.length > 0 && (
                    <div className="border rounded-lg">
                      <button
                        onClick={() => toggleSection('update')}
                        className="w-full flex items-center gap-2 p-2 text-sm text-left hover:bg-muted/50 rounded-lg transition-colors"
                      >
                        {expandedSections.update ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                        <span className="text-blue-600 dark:text-blue-400 font-medium">{t('enterprise.ppio.modelsToUpdate')} ({diff.changes.update.length})</span>
                      </button>
                      {expandedSections.update && (
                        <div className="px-4 pb-2 space-y-1">
                          {diff.changes.update.map(d => (
                            <div key={d.model_id} className="text-xs text-muted-foreground">
                              <span className="font-mono">{d.model_id}</span>
                              {d.changes && d.changes.length > 0 && (
                                <span className="ml-2 text-muted-foreground/60">({d.changes.join(', ')})</span>
                              )}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  )}

                  {diff.changes.delete.length > 0 && (
                    <div className="border rounded-lg">
                      <button
                        onClick={() => toggleSection('delete')}
                        className="w-full flex items-center gap-2 p-2 text-sm text-left hover:bg-muted/50 rounded-lg transition-colors"
                      >
                        {expandedSections.delete ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                        <span className="text-red-600 dark:text-red-400 font-medium">{t('enterprise.ppio.modelsToDelete')} ({diff.changes.delete.length})</span>
                      </button>
                      {expandedSections.delete && (
                        <div className="px-4 pb-2 space-y-1">
                          {diff.changes.delete.map(d => (
                            <div key={d.model_id} className="text-xs text-muted-foreground font-mono">{d.model_id}</div>
                          ))}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )}

              {/* Channel Status */}
              <div className="p-3 bg-muted/50 rounded-lg">
                <div className="text-sm font-medium mb-2">{t('enterprise.ppio.channelStatus')}</div>
                <div className="space-y-1 text-sm">
                  <div className="flex items-center gap-2">
                    {diff.channels.openai.exists ? (
                      <CheckCircle className="w-4 h-4 text-green-600 dark:text-green-400" />
                    ) : (
                      <AlertCircle className="w-4 h-4 text-orange-600 dark:text-orange-400" />
                    )}
                    <span className="text-foreground">OpenAI Channel: </span>
                    <span className={diff.channels.openai.exists ? 'text-green-600 dark:text-green-400' : 'text-orange-600 dark:text-orange-400'}>
                      {diff.channels.openai.exists
                        ? t('enterprise.ppio.channelExists')
                        : (diff.channels.openai.will_create
                          ? t('enterprise.ppio.channelWillCreate')
                          : t('enterprise.ppio.channelNotExists'))}
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    {diff.channels.anthropic.exists ? (
                      <CheckCircle className="w-4 h-4 text-green-600 dark:text-green-400" />
                    ) : (
                      <AlertCircle className="w-4 h-4 text-orange-600 dark:text-orange-400" />
                    )}
                    <span className="text-foreground">Anthropic Channel: </span>
                    <span className={diff.channels.anthropic.exists ? 'text-green-600 dark:text-green-400' : 'text-orange-600 dark:text-orange-400'}>
                      {diff.channels.anthropic.exists
                        ? t('enterprise.ppio.channelExists')
                        : (diff.channels.anthropic.will_create
                          ? t('enterprise.ppio.channelWillCreate')
                          : t('enterprise.ppio.channelNotExists'))}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          ) : (
            <div className="py-8 text-center text-muted-foreground">
              <Info className="w-8 h-8 mx-auto mb-2" />
              <div>{t('enterprise.ppio.diagnosticHint')}</div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Sync Config Panel */}
      <Card>
        <CardHeader>
          <CardTitle>{t('enterprise.ppio.syncConfig')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <Label htmlFor="sync-openai">{t('enterprise.ppio.syncOpenAI')}</Label>
            <Switch
              id="sync-openai"
              checked={syncOpts.sync_openai}
              onCheckedChange={(checked) => setSyncOpts({ ...syncOpts, sync_openai: checked })}
            />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="sync-anthropic">{t('enterprise.ppio.syncAnthropic')}</Label>
            <Switch
              id="sync-anthropic"
              checked={syncOpts.sync_anthropic}
              onCheckedChange={(checked) => setSyncOpts({ ...syncOpts, sync_anthropic: checked })}
            />
          </div>
          <div className="flex items-center justify-between">
            <Label htmlFor="auto-channels">{t('enterprise.ppio.autoCreateChannels')}</Label>
            <Switch
              id="auto-channels"
              checked={syncOpts.auto_create_channels}
              onCheckedChange={(checked) => setSyncOpts({ ...syncOpts, auto_create_channels: checked })}
            />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <Label htmlFor="delete-unmatched">{t('enterprise.ppio.deleteUnmatched')}</Label>
              <p className="text-xs text-muted-foreground mt-0.5">{t('enterprise.ppio.deleteUnmatchedHint')}</p>
            </div>
            <Switch
              id="delete-unmatched"
              checked={syncOpts.delete_unmatched_model || false}
              onCheckedChange={(checked) => setSyncOpts({ ...syncOpts, delete_unmatched_model: checked })}
            />
          </div>
        </CardContent>
      </Card>

      {/* Sync Button */}
      <Card>
        <CardContent className="p-4 space-y-4">
          <Button
            onClick={handleSync}
            disabled={syncing || !diff || (diff.summary.to_add + diff.summary.to_update + (syncOpts.delete_unmatched_model ? diff.summary.to_delete : 0) === 0)}
            className="w-full"
            size="lg"
          >
            {syncing ? `${t('enterprise.ppio.syncing')} (${progress}%)` : t('enterprise.ppio.executeSync')}
          </Button>

          {syncing && (
            <div className="space-y-2">
              <div className="flex justify-between text-sm text-muted-foreground">
                <span>{progressMessage}</span>
                <span>{progress}%</span>
              </div>
              <div className="w-full bg-muted rounded-full h-2">
                <div
                  className="bg-primary h-2 rounded-full transition-all duration-300"
                  style={{ width: `${progress}%` }}
                />
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Sync History */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <History className="w-5 h-5 text-muted-foreground" />
            <CardTitle>{t('enterprise.ppio.syncHistory')}</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          {historyLoading ? (
            <div className="py-8 text-center text-muted-foreground">
              <RefreshCw className="w-6 h-6 animate-spin mx-auto mb-2" />
            </div>
          ) : history.length === 0 ? (
            <div className="py-8 text-center text-muted-foreground text-sm">
              {t('enterprise.ppio.noHistory')}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b bg-muted/50">
                    <th className="text-left p-3 font-medium text-muted-foreground">{t('enterprise.ppio.historyTime')}</th>
                    <th className="text-left p-3 font-medium text-muted-foreground">{t('enterprise.ppio.historyStatus')}</th>
                    <th className="text-center p-3 font-medium text-muted-foreground">{t('enterprise.ppio.historyAdded')}</th>
                    <th className="text-center p-3 font-medium text-muted-foreground">{t('enterprise.ppio.historyUpdated')}</th>
                    <th className="text-center p-3 font-medium text-muted-foreground">{t('enterprise.ppio.historyDeleted')}</th>
                  </tr>
                </thead>
                <tbody>
                  {history.map(h => (
                    <tr key={h.id} className="border-b last:border-b-0 hover:bg-muted/50 transition-colors">
                      <td className="p-3 text-foreground">{formatTime(h.synced_at)}</td>
                      <td className="p-3">{statusBadge(h.status)}</td>
                      <td className="p-3 text-center text-green-600 dark:text-green-400">{h.result_parsed?.summary?.to_add ?? '-'}</td>
                      <td className="p-3 text-center text-blue-600 dark:text-blue-400">{h.result_parsed?.summary?.to_update ?? '-'}</td>
                      <td className="p-3 text-center text-red-600 dark:text-red-400">
                        {h.result_parsed?.details?.models_deleted?.length ?? 0}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
