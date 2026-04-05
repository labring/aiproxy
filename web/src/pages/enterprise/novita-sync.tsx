import { useState, useEffect, useRef, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { RefreshCw, CheckCircle, AlertCircle, Info, ChevronDown, ChevronRight, Clock, History, Save, Key, DollarSign } from 'lucide-react'
import { novitaApi } from '../../api/novita'
import type { DiagnosticResult, ModelCoverageResult, ModelDiff, NovitaChannelItem, NovitaConfig, SyncHistory, SyncOptions, SyncProgressEvent } from '../../types/novita'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { useHasPermission } from '@/lib/permissions'

const ENDPOINT_LABELS: Record<string, { label: string; color: string }> = {
  'chat/completions': { label: 'Chat', color: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' },
  'anthropic': { label: 'Anthropic', color: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400' },
  'responses': { label: 'Responses', color: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400' },
  'embeddings': { label: 'Embeddings', color: 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-400' },
  'completions': { label: 'Completions', color: 'bg-gray-100 text-gray-700 dark:bg-gray-800/50 dark:text-gray-400' },
  'gemini': { label: 'Gemini', color: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
  'batch-api': { label: 'Batch', color: 'bg-gray-100 text-gray-600 dark:bg-gray-800/50 dark:text-gray-400' },
}

function EndpointBadges({ endpoints }: { endpoints?: string[] }) {
  if (!endpoints || endpoints.length === 0) return null
  return (
    <span className="inline-flex flex-wrap gap-1 ml-2">
      {endpoints.map(ep => {
        const cfg = ENDPOINT_LABELS[ep] || { label: ep, color: 'bg-gray-100 text-gray-600 dark:bg-gray-800/50 dark:text-gray-400' }
        return (
          <Badge key={ep} variant="secondary" className={`text-[10px] px-1.5 py-0 h-4 font-normal ${cfg.color}`}>
            {cfg.label}
          </Badge>
        )
      })}
    </span>
  )
}

function ModelRow({ d }: { d: ModelDiff }) {
  const config = d.new_config || d.old_config
  const endpoints = config?.endpoints as string[] | undefined
  const modelType = config?.model_type as string | undefined
  return (
    <div className="flex items-center flex-wrap gap-1 py-0.5">
      <span className="font-mono text-xs text-muted-foreground">{d.model_id}</span>
      <EndpointBadges endpoints={endpoints} />
      {modelType && modelType !== 'chat' && (
        <Badge variant="outline" className="text-[10px] px-1.5 py-0 h-4 font-normal">
          {modelType}
        </Badge>
      )}
      {d.changes && d.changes.length > 0 && (
        <span className="text-[10px] text-muted-foreground/60 ml-1">({d.changes.join(', ')})</span>
      )}
    </div>
  )
}

export default function NovitaSyncPage() {
  const { t } = useTranslation()
  const canManage = useHasPermission('access_control_manage')
  const cancelSyncRef = useRef<(() => void) | null>(null)
  const [diagnostic, setDiagnostic] = useState<DiagnosticResult | null>(null)
  const [coverage, setCoverage] = useState<ModelCoverageResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [progress, setProgress] = useState(0)
  const [progressMessage, setProgressMessage] = useState('')
  const [history, setHistory] = useState<SyncHistory[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({})

  const [channels, setChannels] = useState<NovitaChannelItem[]>([])
  const [config, setConfig] = useState<NovitaConfig | null>(null)
  const [configLoading, setConfigLoading] = useState(false)
  const [configSaving, setConfigSaving] = useState(false)
  const [selectedBaseURL, setSelectedBaseURL] = useState<string>('')
  const [selectedChannelId, setSelectedChannelId] = useState<string>('')

  const [directApiKey, setDirectApiKey] = useState<string>('')
  const [directApiKeySaving, setDirectApiKeySaving] = useState(false)

  const [mgmtToken, setMgmtToken] = useState<string>('')
  const [mgmtTokenSaving, setMgmtTokenSaving] = useState(false)
  const [exchangeRate, setExchangeRate] = useState<string>('')
  const [exchangeRateSaving, setExchangeRateSaving] = useState(false)

  const [syncOpts, setSyncOpts] = useState<SyncOptions>({
    auto_create_channels: true,
    changes_confirmed: false,
    delete_unmatched_model: false,
    anthropic_pure_passthrough: true
  })

  const baseURLGroups = useMemo(() => {
    const groups = new Map<string, NovitaChannelItem[]>()
    for (const ch of channels) {
      const existing = groups.get(ch.base_url) || []
      existing.push(ch)
      groups.set(ch.base_url, existing)
    }
    return groups
  }, [channels])

  const uniqueBaseURLs = useMemo(() => Array.from(baseURLGroups.keys()), [baseURLGroups])

  const filteredChannels = useMemo(() => {
    if (!selectedBaseURL) return []
    return baseURLGroups.get(selectedBaseURL) || []
  }, [selectedBaseURL, baseURLGroups])

  const toggleSection = (key: string) => {
    setExpandedSections(prev => ({ ...prev, [key]: !prev[key] }))
  }

  const loadConfig = async () => {
    setConfigLoading(true)
    try {
      const [channelList, currentConfig] = await Promise.all([
        novitaApi.listChannels(),
        novitaApi.getConfig()
      ])
      setChannels(channelList || [])
      setConfig(currentConfig)

      if (currentConfig.channel_id && channelList) {
        const activeChannel = channelList.find(ch => ch.id === currentConfig.channel_id)
        if (activeChannel) {
          setSelectedBaseURL(activeChannel.base_url)
          setSelectedChannelId(String(activeChannel.id))
        }
      }
    } catch {
      // Non-critical
    } finally {
      setConfigLoading(false)
    }
  }

  const saveConfig = async () => {
    const channelId = Number(selectedChannelId)
    if (!channelId) {
      toast.error(t('enterprise.novita.configSelectChannel'))
      return
    }
    setConfigSaving(true)
    try {
      await novitaApi.updateConfig(channelId)
      toast.success(t('enterprise.novita.configSaved'))
      await loadConfig()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setConfigSaving(false)
    }
  }

  const saveMgmtToken = async () => {
    if (!mgmtToken.trim()) {
      toast.error(t('enterprise.novita.mgmtTokenRequired'))
      return
    }
    setMgmtTokenSaving(true)
    try {
      await novitaApi.updateMgmtToken(mgmtToken.trim())
      toast.success(t('enterprise.novita.mgmtTokenSaved'))
      setMgmtToken('')
      await loadConfig()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setMgmtTokenSaving(false)
    }
  }

  const saveExchangeRate = async () => {
    const rate = parseFloat(exchangeRate)
    if (!rate || rate <= 0) {
      toast.error(t('enterprise.novita.exchangeRateInvalid'))
      return
    }
    setExchangeRateSaving(true)
    try {
      await novitaApi.updateExchangeRate(rate)
      toast.success(t('enterprise.novita.exchangeRateSaved'))
      setExchangeRate('')
      await loadConfig()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setExchangeRateSaving(false)
    }
  }

  const saveDirectApiKey = async () => {
    if (!directApiKey.trim()) {
      toast.error(t('enterprise.novita.apiKeyRequired'))
      return
    }
    setDirectApiKeySaving(true)
    try {
      await novitaApi.updateAPIKey(directApiKey.trim())
      toast.success(t('enterprise.novita.apiKeySaved'))
      setDirectApiKey('')
      await loadConfig()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setDirectApiKeySaving(false)
    }
  }

  useEffect(() => {
    if (filteredChannels.length === 1) {
      setSelectedChannelId(String(filteredChannels[0].id))
    } else if (!filteredChannels.find(ch => String(ch.id) === selectedChannelId)) {
      setSelectedChannelId('')
    }
  }, [selectedBaseURL, filteredChannels, selectedChannelId])

  const loadHistory = async () => {
    setHistoryLoading(true)
    try {
      const result = await novitaApi.history()
      setHistory(result || [])
    } catch {
      // Non-critical
    } finally {
      setHistoryLoading(false)
    }
  }

  useEffect(() => {
    loadConfig()
    loadHistory()
    return () => { cancelSyncRef.current?.() }
  }, [])

  const loadCoverage = async () => {
    try {
      const result = await novitaApi.modelCoverage()
      setCoverage(result)
    } catch {
      // Non-critical
    }
  }

  const loadDiagnostic = async () => {
    setLoading(true)
    try {
      const [diagResult] = await Promise.all([novitaApi.diagnostic(), loadCoverage()])
      setDiagnostic(diagResult)
      toast.success(t('common.success'))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  const handleSync = () => {
    if (!diagnostic?.diff) {
      toast.error(t('enterprise.novita.diagnosticHint'))
      return
    }

    setSyncing(true)
    setProgress(0)
    setProgressMessage('')

    cancelSyncRef.current = novitaApi.execute(
      { ...syncOpts, changes_confirmed: true },
      (event: SyncProgressEvent) => {
        setProgress(event.progress || 0)
        setProgressMessage(event.message)
      },
      () => {
        cancelSyncRef.current = null
        setSyncing(false)
        setProgress(100)
        toast.success(t('common.success'))
        loadDiagnostic()
        loadHistory()
      },
      (error: Error) => {
        cancelSyncRef.current = null
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
      success: t('enterprise.novita.statusSuccess'),
      partial: t('enterprise.novita.statusPartial'),
      failed: t('enterprise.novita.statusFailed'),
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
        <h1 className="text-2xl font-bold text-foreground">{t('enterprise.novita.title')}</h1>
        <p className="text-sm text-muted-foreground mt-1">{t('enterprise.novita.description')}</p>
      </div>

      {/* API Config Card */}
      <Card>
        <CardHeader>
          <CardTitle>{t('enterprise.novita.configTitle')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {configLoading ? (
            <div className="py-4 text-center text-muted-foreground">
              <RefreshCw className="w-5 h-5 animate-spin mx-auto" />
            </div>
          ) : channels.length === 0 && !config?.configured ? (
            <div className="space-y-3">
              <div className="p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg text-sm flex items-center gap-2">
                <Info className="w-4 h-4 text-blue-600 dark:text-blue-400" />
                <span className="text-blue-700 dark:text-blue-400">
                  {t('enterprise.novita.directApiKeyHint')}
                </span>
              </div>
              <div className="space-y-2">
                <Label>{t('enterprise.novita.apiKeyLabel')}</Label>
                <Input
                  type="password"
                  placeholder={t('enterprise.novita.apiKeyPlaceholder')}
                  value={directApiKey}
                  onChange={(e) => setDirectApiKey(e.target.value)}
                />
              </div>
              {canManage && (
                <Button
                  onClick={saveDirectApiKey}
                  disabled={directApiKeySaving || !directApiKey.trim()}
                  size="sm"
                >
                  <Save className="w-4 h-4 mr-2" />
                  {directApiKeySaving ? t('enterprise.novita.apiKeySaving') : t('enterprise.novita.apiKeySave')}
                </Button>
              )}
            </div>
          ) : channels.length === 0 && config?.configured ? (
            <div className="p-3 bg-green-50 dark:bg-green-900/20 rounded-lg text-sm flex items-center gap-2">
              <CheckCircle className="w-4 h-4 text-green-600 dark:text-green-400" />
              <span className="text-green-700 dark:text-green-400">
                {t('enterprise.novita.configConfigured')}: {config.api_base} ({config.api_key})
              </span>
            </div>
          ) : (
            <>
              {config?.configured && (
                <div className="flex items-center gap-2 p-3 bg-green-50 dark:bg-green-900/20 rounded-lg text-sm">
                  <CheckCircle className="w-4 h-4 text-green-600 dark:text-green-400" />
                  <span className="text-green-700 dark:text-green-400">
                    {t('enterprise.novita.configConfigured')}: {config.api_base} ({config.api_key})
                  </span>
                </div>
              )}

              <div className="space-y-2">
                <Label>{t('enterprise.novita.configApiBase')}</Label>
                <Select value={selectedBaseURL} onValueChange={(v) => { setSelectedBaseURL(v); setSelectedChannelId('') }}>
                  <SelectTrigger>
                    <SelectValue placeholder={t('enterprise.novita.configSelectBaseURL')} />
                  </SelectTrigger>
                  <SelectContent>
                    {uniqueBaseURLs.map(url => (
                      <SelectItem key={url} value={url}>
                        {url}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {selectedBaseURL && filteredChannels.length > 0 && (
                <div className="space-y-2">
                  <Label>{t('enterprise.novita.configApiKey')}</Label>
                  <Select value={selectedChannelId} onValueChange={setSelectedChannelId}>
                    <SelectTrigger>
                      <SelectValue placeholder={t('enterprise.novita.configSelectKey')} />
                    </SelectTrigger>
                    <SelectContent>
                      {filteredChannels.map(ch => (
                        <SelectItem key={ch.id} value={String(ch.id)}>
                          {ch.name} ({ch.key})
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}

              {canManage && (
                <Button
                  onClick={saveConfig}
                  disabled={configSaving || !selectedChannelId}
                  size="sm"
                >
                  <Save className="w-4 h-4 mr-2" />
                  {configSaving ? t('enterprise.novita.configSaving') : t('enterprise.novita.configSave')}
                </Button>
              )}
            </>
          )}
        </CardContent>
      </Card>

      {/* Mgmt Token Card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Key className="w-4 h-4" />
            {t('enterprise.novita.mgmtTokenTitle')}
          </CardTitle>
          <p className="text-sm text-muted-foreground">{t('enterprise.novita.mgmtTokenDescription')}</p>
        </CardHeader>
        <CardContent className="space-y-4">
          {config?.mgmt_token_configured && (
            <div className="flex items-center gap-2 p-3 bg-green-50 dark:bg-green-900/20 rounded-lg text-sm">
              <CheckCircle className="w-4 h-4 text-green-600 dark:text-green-400" />
              <span className="text-green-700 dark:text-green-400">
                {t('enterprise.novita.mgmtTokenConfigured')}
              </span>
            </div>
          )}
          <div className="space-y-2">
            <Label>{t('enterprise.novita.mgmtTokenTitle')}</Label>
            <Input
              type="password"
              value={mgmtToken}
              onChange={(e) => setMgmtToken(e.target.value)}
              placeholder={t('enterprise.novita.mgmtTokenPlaceholder')}
            />
            <p className="text-xs text-muted-foreground">{t('enterprise.novita.mgmtTokenHint')}</p>
          </div>
          {canManage && (
            <Button
              onClick={saveMgmtToken}
              disabled={mgmtTokenSaving || !mgmtToken.trim()}
              size="sm"
            >
              <Save className="w-4 h-4 mr-2" />
              {mgmtTokenSaving ? t('common.saving') : t('enterprise.novita.mgmtTokenSave')}
            </Button>
          )}
        </CardContent>
      </Card>

      {/* Exchange Rate Card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <DollarSign className="w-4 h-4" />
            {t('enterprise.novita.exchangeRateTitle')}
          </CardTitle>
          <p className="text-sm text-muted-foreground">{t('enterprise.novita.exchangeRateDescription')}</p>
        </CardHeader>
        <CardContent className="space-y-4">
          {config && (
            <div className="flex items-center gap-2 p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg text-sm">
              <Info className="w-4 h-4 text-blue-600 dark:text-blue-400" />
              <span className="text-blue-700 dark:text-blue-400">
                {t('enterprise.novita.exchangeRateConfigured')}: {config.exchange_rate} USD/CNY
              </span>
            </div>
          )}
          <div className="space-y-2">
            <Label>{t('enterprise.novita.exchangeRateLabel')}</Label>
            <Input
              type="number"
              step="0.01"
              min="0"
              value={exchangeRate}
              onChange={(e) => setExchangeRate(e.target.value)}
              placeholder={t('enterprise.novita.exchangeRatePlaceholder')}
            />
          </div>
          {canManage && (
            <Button
              onClick={saveExchangeRate}
              disabled={exchangeRateSaving || !exchangeRate.trim()}
              size="sm"
            >
              <Save className="w-4 h-4 mr-2" />
              {exchangeRateSaving ? t('common.saving') : t('enterprise.novita.exchangeRateSave')}
            </Button>
          )}
        </CardContent>
      </Card>

      {/* Status Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardContent className="p-4">
            <div className="text-sm text-muted-foreground mb-1">{t('enterprise.novita.localModels')}</div>
            <div className="text-2xl font-bold text-foreground">{diagnostic?.local_models ?? '-'}</div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="text-sm text-muted-foreground mb-1">{t('enterprise.novita.remoteModels')}</div>
            <div className="text-2xl font-bold text-foreground">{diagnostic?.remote_models ?? '-'}</div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="text-sm text-muted-foreground mb-1">{t('enterprise.novita.needSync')}</div>
            <div className="text-2xl font-bold text-orange-600 dark:text-orange-400">
              {diff ? (diff.summary.to_add + diff.summary.to_update) : '-'}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-1.5 text-sm text-muted-foreground mb-1">
              <Clock className="w-3.5 h-3.5" />
              {t('enterprise.novita.lastSyncAt')}
            </div>
            <div className="text-sm font-medium text-foreground">
              {diagnostic?.last_sync_at ? formatTime(diagnostic.last_sync_at) : t('enterprise.novita.never')}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Model Coverage Card */}
      {coverage && (
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <CardTitle className="text-base">{t('enterprise.novita.modelCoverage')}</CardTitle>
              <Badge
                className={
                  coverage.uncovered.length === 0
                    ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                    : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
                }
              >
                {coverage.covered} / {coverage.total}
              </Badge>
            </div>
          </CardHeader>
          {coverage.uncovered.length > 0 && (
            <CardContent>
              <p className="text-xs text-muted-foreground mb-2">{t('enterprise.novita.uncoveredHint')}</p>
              <div className="space-y-1 max-h-48 overflow-y-auto">
                {coverage.uncovered.map(item => (
                  <div key={item.model} className="flex items-center flex-wrap gap-1 text-xs py-0.5">
                    <span className="font-mono text-muted-foreground">{item.model}</span>
                    <EndpointBadges endpoints={item.endpoints} />
                    {item.model_type && item.model_type !== 'chat' && (
                      <Badge variant="outline" className="text-[10px] px-1.5 py-0 h-4 font-normal">
                        {item.model_type}
                      </Badge>
                    )}
                  </div>
                ))}
              </div>
            </CardContent>
          )}
        </Card>
      )}

      {/* Diagnostic Section */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle>{t('enterprise.novita.diagnosticResult')}</CardTitle>
          <Button
            onClick={loadDiagnostic}
            disabled={loading}
            size="sm"
          >
            <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
            {loading ? t('enterprise.novita.diagnosing') : t('enterprise.novita.refreshDiagnostic')}
          </Button>
        </CardHeader>

        <CardContent>
          {loading ? (
            <div className="py-8 text-center text-muted-foreground">
              <RefreshCw className="w-8 h-8 animate-spin mx-auto mb-2" />
              <div>{t('enterprise.novita.diagnosing')}</div>
            </div>
          ) : diff ? (
            <div className="space-y-4">
              <div className="grid grid-cols-3 gap-4">
                <div className="text-center p-3 bg-green-50 dark:bg-green-900/20 rounded-lg">
                  <div className="text-2xl font-bold text-green-600 dark:text-green-400">{diff.summary.to_add}</div>
                  <div className="text-sm text-muted-foreground">{t('enterprise.novita.toAdd')}</div>
                </div>
                <div className="text-center p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
                  <div className="text-2xl font-bold text-blue-600 dark:text-blue-400">{diff.summary.to_update}</div>
                  <div className="text-sm text-muted-foreground">{t('enterprise.novita.toUpdate')}</div>
                </div>
                <div className="text-center p-3 bg-red-50 dark:bg-red-900/20 rounded-lg">
                  <div className="text-2xl font-bold text-red-600 dark:text-red-400">{diff.summary.to_delete}</div>
                  <div className="text-sm text-muted-foreground">{t('enterprise.novita.toDelete')}</div>
                </div>
              </div>

              {((diff.changes.add?.length ?? 0) > 0 || (diff.changes.update?.length ?? 0) > 0 || (diff.changes.delete?.length ?? 0) > 0) && (
                <div className="space-y-2">
                  <h3 className="text-sm font-medium text-foreground mb-2">{t('enterprise.novita.changeDetails')}</h3>

                  {(diff.changes.add?.length ?? 0) > 0 && (
                    <div className="border rounded-lg">
                      <button
                        onClick={() => toggleSection('add')}
                        className="w-full flex items-center gap-2 p-2 text-sm text-left hover:bg-muted/50 rounded-lg transition-colors"
                      >
                        {expandedSections.add ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                        <span className="text-green-600 dark:text-green-400 font-medium">{t('enterprise.novita.modelsToAdd')} ({diff.changes.add!.length})</span>
                      </button>
                      {expandedSections.add && (
                        <div className="px-4 pb-2 space-y-0.5">
                          {diff.changes.add!.map(d => (
                            <ModelRow key={d.model_id} d={d} />
                          ))}
                        </div>
                      )}
                    </div>
                  )}

                  {(diff.changes.update?.length ?? 0) > 0 && (
                    <div className="border rounded-lg">
                      <button
                        onClick={() => toggleSection('update')}
                        className="w-full flex items-center gap-2 p-2 text-sm text-left hover:bg-muted/50 rounded-lg transition-colors"
                      >
                        {expandedSections.update ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                        <span className="text-blue-600 dark:text-blue-400 font-medium">{t('enterprise.novita.modelsToUpdate')} ({diff.changes.update!.length})</span>
                      </button>
                      {expandedSections.update && (
                        <div className="px-4 pb-2 space-y-0.5">
                          {diff.changes.update!.map(d => (
                            <ModelRow key={d.model_id} d={d} />
                          ))}
                        </div>
                      )}
                    </div>
                  )}

                  {(diff.changes.delete?.length ?? 0) > 0 && (
                    <div className="border rounded-lg">
                      <button
                        onClick={() => toggleSection('delete')}
                        className="w-full flex items-center gap-2 p-2 text-sm text-left hover:bg-muted/50 rounded-lg transition-colors"
                      >
                        {expandedSections.delete ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                        <span className="text-red-600 dark:text-red-400 font-medium">{t('enterprise.novita.modelsToDelete')} ({diff.changes.delete!.length})</span>
                      </button>
                      {expandedSections.delete && (
                        <div className="px-4 pb-2 space-y-0.5">
                          {diff.changes.delete!.map(d => (
                            <ModelRow key={d.model_id} d={d} />
                          ))}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )}

              {/* Endpoint Legend */}
              <div className="p-3 bg-muted/50 rounded-lg">
                <div className="text-sm font-medium mb-2">{t('enterprise.novita.endpointLegend')}</div>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-1 text-xs text-muted-foreground">
                  {([
                    ['chat/completions', t('enterprise.novita.endpointChat')],
                    ['anthropic',        t('enterprise.novita.endpointAnthropic')],
                    ['responses',        t('enterprise.novita.endpointResponses')],
                    ['embeddings',       t('enterprise.novita.endpointEmbeddings')],
                  ] as [string, string][]).map(([key, desc]) => (
                    <div key={key} className="flex items-center gap-2">
                      <Badge variant="secondary" className={`text-[10px] px-1.5 py-0 h-4 font-normal ${ENDPOINT_LABELS[key].color}`}>
                        {ENDPOINT_LABELS[key].label}
                      </Badge>
                      <span>{desc}</span>
                    </div>
                  ))}
                </div>
              </div>

              {/* Channel Status */}
              <div className="p-3 bg-muted/50 rounded-lg">
                <div className="text-sm font-medium mb-2">{t('enterprise.novita.channelStatus')}</div>
                <div className="space-y-1 text-sm">
                  <div className="flex items-center gap-2">
                    {diff.channels.novita.exists ? (
                      <CheckCircle className="w-4 h-4 text-green-600 dark:text-green-400" />
                    ) : (
                      <AlertCircle className="w-4 h-4 text-orange-600 dark:text-orange-400" />
                    )}
                    <span className="text-foreground">海外 Channel: </span>
                    <span className={diff.channels.novita.exists ? 'text-green-600 dark:text-green-400' : 'text-orange-600 dark:text-orange-400'}>
                      {diff.channels.novita.exists
                        ? t('enterprise.novita.channelExists')
                        : (diff.channels.novita.will_create
                          ? t('enterprise.novita.channelWillCreate')
                          : t('enterprise.novita.channelNotExists'))}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          ) : (
            <div className="py-8 text-center text-muted-foreground">
              <Info className="w-8 h-8 mx-auto mb-2" />
              <div>{t('enterprise.novita.diagnosticHint')}</div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Sync Config Panel */}
      <Card>
        <CardHeader>
          <CardTitle>{t('enterprise.novita.syncConfig')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <Label htmlFor="auto-channels">{t('enterprise.novita.autoCreateChannels')}</Label>
            <Switch
              id="auto-channels"
              checked={syncOpts.auto_create_channels}
              onCheckedChange={(checked) => setSyncOpts({ ...syncOpts, auto_create_channels: checked })}
              disabled={!canManage}
            />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <Label htmlFor="delete-unmatched">{t('enterprise.novita.deleteUnmatched')}</Label>
              <p className="text-xs text-muted-foreground mt-0.5">{t('enterprise.novita.deleteUnmatchedHint')}</p>
            </div>
            <Switch
              id="delete-unmatched"
              checked={syncOpts.delete_unmatched_model || false}
              onCheckedChange={(checked) => setSyncOpts({ ...syncOpts, delete_unmatched_model: checked })}
              disabled={!canManage}
            />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <Label htmlFor="anthropic-pure-passthrough">{t('enterprise.novita.anthropicPurePassthrough')}</Label>
              <p className="text-xs text-muted-foreground mt-0.5">{t('enterprise.novita.anthropicPurePassthroughHint')}</p>
            </div>
            <Switch
              id="anthropic-pure-passthrough"
              checked={syncOpts.anthropic_pure_passthrough ?? true}
              onCheckedChange={(checked) => setSyncOpts({ ...syncOpts, anthropic_pure_passthrough: checked })}
              disabled={!canManage}
            />
          </div>
        </CardContent>
      </Card>

      {/* Sync Button */}
      <Card>
        <CardContent className="p-4 space-y-4">
          {canManage && (
            <Button
              onClick={handleSync}
              disabled={syncing || !diff || (diff.summary.to_add + diff.summary.to_update + (syncOpts.delete_unmatched_model ? diff.summary.to_delete : 0) === 0)}
              className="w-full"
              size="lg"
            >
              {syncing ? `${t('enterprise.novita.syncing')} (${progress}%)` : t('enterprise.novita.executeSync')}
            </Button>
          )}

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
            <CardTitle>{t('enterprise.novita.syncHistory')}</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          {historyLoading ? (
            <div className="py-8 text-center text-muted-foreground">
              <RefreshCw className="w-6 h-6 animate-spin mx-auto mb-2" />
            </div>
          ) : history.length === 0 ? (
            <div className="py-8 text-center text-muted-foreground text-sm">
              {t('enterprise.novita.noHistory')}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b bg-muted/50">
                    <th className="text-left p-3 font-medium text-muted-foreground">{t('enterprise.novita.historyTime')}</th>
                    <th className="text-left p-3 font-medium text-muted-foreground">{t('enterprise.novita.historyStatus')}</th>
                    <th className="text-center p-3 font-medium text-muted-foreground">{t('enterprise.novita.historyAdded')}</th>
                    <th className="text-center p-3 font-medium text-muted-foreground">{t('enterprise.novita.historyUpdated')}</th>
                    <th className="text-center p-3 font-medium text-muted-foreground">{t('enterprise.novita.historyDeleted')}</th>
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
