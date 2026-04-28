// src/feature/channel/components/ChannelTestDialog.tsx
import { useLayoutEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { CheckCircle, XCircle, Loader2, X, ExternalLink, ChevronDown, ChevronRight } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { ChannelTestResult } from '@/api/channel'
import { JsonViewer } from '@/feature/log/components/JsonViewer'

interface ChannelTestDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    isTesting: boolean
    results: ChannelTestResult[]
    onCancel: () => void
    onChannelClick?: (channelId: number) => void
    showChannelInfo?: boolean
}

export function ChannelTestDialog({
    open,
    onOpenChange,
    isTesting,
    results,
    onCancel,
    onChannelClick,
    showChannelInfo = false,
}: ChannelTestDialogProps) {
    const { t } = useTranslation()
    const [activeTab, setActiveTab] = useState<'all' | 'success' | 'failed'>('all')
    const [expandedItems, setExpandedItems] = useState<Set<string>>(new Set())
    const resultScrollRef = useRef<HTMLDivElement | null>(null)
    const scrollStateRef = useRef({
        scrollTop: 0,
        scrollHeight: 0,
        shouldStickToTop: true,
        activeTab: 'all' as 'all' | 'success' | 'failed',
    })

    const successResults = results.filter(r => r.success && r.data?.success)
    const failedResults = results.filter(r => !(r.success && r.data?.success))

    const successCount = successResults.length
    const failCount = failedResults.length

    const getFilteredResults = () => {
        switch (activeTab) {
            case 'success':
                return successResults
            case 'failed':
                return failedResults
            default:
                return results
        }
    }

    const filteredResults = getFilteredResults()

    const isJsonString = (value?: string) => {
        if (!value) return false
        try {
            JSON.parse(value)
            return true
        } catch {
            return false
        }
    }

    const formatModeLabel = (mode?: string) => {
        if (!mode) return '-'
        return t(`modeType.${mode}`, { defaultValue: mode })
    }

    const getResultKey = (result: ChannelTestResult, index: number) =>
        `${result.data?.channel_id ?? 'na'}-${result.data?.model ?? result.data?.actual_model ?? 'unknown'}-${result.data?.test_at ?? index}`

    const toggleExpanded = (key: string) => {
        setExpandedItems(prev => {
            const next = new Set(prev)
            if (next.has(key)) {
                next.delete(key)
            } else {
                next.add(key)
            }
            return next
        })
    }

    const handleResultClick = (result: ChannelTestResult) => {
        if (showChannelInfo && result.data?.channel_id && onChannelClick) {
            onChannelClick(result.data.channel_id)
        }
    }

    useLayoutEffect(() => {
        const container = resultScrollRef.current
        if (!container) return

        const previous = scrollStateRef.current
        const activeTabChanged = previous.activeTab !== activeTab

        if (activeTabChanged) {
            container.scrollTop = 0
        } else if (previous.shouldStickToTop) {
            container.scrollTop = 0
        } else {
            const heightDelta = container.scrollHeight - previous.scrollHeight
            container.scrollTop = previous.scrollTop + Math.max(0, heightDelta)
        }

        scrollStateRef.current = {
            scrollTop: container.scrollTop,
            scrollHeight: container.scrollHeight,
            shouldStickToTop: container.scrollTop <= 8,
            activeTab,
        }
    }, [filteredResults.length, activeTab])

    const handleResultScroll = () => {
        const container = resultScrollRef.current
        if (!container) return

        scrollStateRef.current = {
            scrollTop: container.scrollTop,
            scrollHeight: container.scrollHeight,
            shouldStickToTop: container.scrollTop <= 8,
            activeTab,
        }
    }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as 'all' | 'success' | 'failed')}>
                <DialogContent className="top-[4vh] left-1/2 -translate-x-1/2 translate-y-0 flex flex-col w-[96vw] lg:max-w-[1100px] h-[min(88vh,820px)] p-0 overflow-hidden">
                    <DialogHeader className="shrink-0 border-b bg-background px-6 pt-6 pb-4">
                        <DialogTitle className="flex items-center gap-2">
                            {isTesting ? (
                                <>
                                    <Loader2 className="h-5 w-5 animate-spin text-primary" />
                                    {t("channel.testing")}
                                </>
                            ) : (
                                <>
                                    {failCount === 0 && successCount > 0 ? (
                                        <CheckCircle className="h-5 w-5 text-green-600" />
                                    ) : (
                                        <XCircle className="h-5 w-5 text-red-600" />
                                    )}
                                    {t("channel.testResults")}
                                </>
                            )}
                        </DialogTitle>

                        <div className="flex items-center justify-between pt-2 text-sm">
                            <div className="flex items-center gap-4">
                                <span className="text-muted-foreground">
                                    {t("channel.testProgress")}: {results.length} {t("channel.testModels")}
                                </span>
                                {successCount > 0 && (
                                    <span className="text-green-600 dark:text-green-500">
                                        {successCount} {t("channel.testSuccessCount")}
                                    </span>
                                )}
                                {failCount > 0 && (
                                    <span className="text-red-600 dark:text-red-500">
                                        {failCount} {t("channel.testFailedCount")}
                                    </span>
                                )}
                            </div>
                        </div>

                        {isTesting && (
                            <div className="mt-2 h-2 w-full overflow-hidden rounded-full bg-muted">
                                <div
                                    className="h-full bg-primary transition-all duration-300"
                                    style={{ width: `${Math.max(5, (successCount + failCount) / Math.max(1, results.length || 1) * 100)}%` }}
                                />
                            </div>
                        )}

                        <TabsList className="mt-3 grid w-full grid-cols-3">
                            <TabsTrigger value="all">
                                {t("channel.testTabAll")} ({results.length})
                            </TabsTrigger>
                            <TabsTrigger value="success" className="data-[state=active]:text-green-600">
                                <CheckCircle className="mr-1 h-3.5 w-3.5" />
                                {successCount}
                            </TabsTrigger>
                            <TabsTrigger value="failed" className="data-[state=active]:text-red-600">
                                <XCircle className="mr-1 h-3.5 w-3.5" />
                                {failCount}
                            </TabsTrigger>
                        </TabsList>
                    </DialogHeader>

                    <div className="flex min-h-0 flex-1 flex-col px-6 pb-6">
                        <TabsContent value={activeTab} className="mt-0 min-h-0 flex-1 overflow-hidden">
                            <div
                                ref={resultScrollRef}
                                onScroll={handleResultScroll}
                                className="h-full overflow-y-auto pr-2"
                                style={{ overflowAnchor: 'none' }}
                            >
                                {isTesting && (
                                    <div className="sticky top-0 z-20 bg-background py-1 text-sm text-muted-foreground">
                                        <div className="flex items-center justify-center">
                                            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                            <span>{t("channel.testingInProgress")}</span>
                                        </div>
                                    </div>
                                )}

                                <div className="space-y-2 pb-2">
                                    {filteredResults.map((result, index) => {
                                        const canClick = showChannelInfo && result.data?.channel_id && onChannelClick
                                        const resultKey = getResultKey(result, index)
                                        const isExpanded = expandedItems.has(resultKey)
                                        const isSuccess = result.success && result.data?.success
                                        const statusCode = result.data?.code

                                        return (
                                            <div
                                                key={resultKey}
                                                className={cn(
                                                    "overflow-hidden rounded-lg border text-sm",
                                                    isSuccess
                                                        ? "border-green-200 bg-green-50 dark:border-green-800 dark:bg-green-950/30"
                                                        : "border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950/30"
                                                )}
                                            >
                                                <div className="flex items-start gap-3 p-3">
                                                    <button
                                                        type="button"
                                                        onClick={() => toggleExpanded(resultKey)}
                                                        className="mt-0.5 rounded-sm p-0.5 text-muted-foreground transition-colors hover:bg-background/60 hover:text-foreground"
                                                        aria-label={isExpanded ? t("channel.testCollapseDetails") : t("channel.testExpandDetails")}
                                                    >
                                                        {isExpanded ? (
                                                            <ChevronDown className="h-4 w-4" />
                                                        ) : (
                                                            <ChevronRight className="h-4 w-4" />
                                                        )}
                                                    </button>

                                                    {isSuccess ? (
                                                        <CheckCircle className="mt-0.5 h-4 w-4 shrink-0 text-green-600 dark:text-green-500" />
                                                    ) : (
                                                        <XCircle className="mt-0.5 h-4 w-4 shrink-0 text-red-600 dark:text-red-500" />
                                                    )}

                                                    <div className="min-w-0 flex-1">
                                                        {showChannelInfo && result.data?.channel_name && (
                                                            <div className="mb-1 flex items-center gap-1.5">
                                                                <span className="text-xs font-medium text-primary dark:text-primary/80">
                                                                    {result.data.channel_name}
                                                                </span>
                                                                {result.data.channel_id && (
                                                                    <span className="text-xs text-muted-foreground">
                                                                        (ID: {result.data.channel_id})
                                                                    </span>
                                                                )}
                                                                {canClick && (
                                                                    <button
                                                                        type="button"
                                                                        onClick={() => handleResultClick(result)}
                                                                        className="inline-flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-primary"
                                                                    >
                                                                        <ExternalLink className="h-3 w-3" />
                                                                        {t("channel.viewChannel")}
                                                                    </button>
                                                                )}
                                                            </div>
                                                        )}

                                                        <div className="flex flex-wrap items-center gap-2">
                                                            <div className="break-all font-medium">
                                                                {result.data?.model || result.data?.actual_model || `Model ${index + 1}`}
                                                            </div>
                                                            <Badge
                                                                variant={isSuccess ? 'secondary' : 'destructive'}
                                                                className={isSuccess ? 'border-green-200 bg-green-100 text-green-800 dark:border-green-800 dark:bg-green-900/20 dark:text-green-400' : ''}
                                                            >
                                                                {isSuccess ? t("channel.testSuccess") : t("channel.testFailed")}
                                                            </Badge>
                                                            <Badge variant="outline" className="font-mono">
                                                                {t("channel.statusCode")}: {statusCode ?? '-'}
                                                            </Badge>
                                                            {result.data?.took !== undefined && (
                                                                <Badge variant="outline" className="font-mono">
                                                                    {t("channel.testTook")}: {result.data.took.toFixed(2)}s
                                                                </Badge>
                                                            )}
                                                        </div>

                                                        {result.message && (
                                                            <div className="mt-1 break-all text-xs text-muted-foreground">
                                                                {result.message}
                                                            </div>
                                                        )}

                                                        {result.data?.response && !isExpanded && (
                                                            <div
                                                                className={cn(
                                                                    "mt-2 line-clamp-2 break-all text-xs",
                                                                    isSuccess ? "text-muted-foreground" : "text-red-600 dark:text-red-400"
                                                                )}
                                                            >
                                                                {result.data.response}
                                                            </div>
                                                        )}
                                                    </div>
                                                </div>

                                                {isExpanded && (
                                                    <div className="space-y-3 border-t bg-background/60 px-4 py-3">
                                                        <div className="grid gap-2 text-xs text-muted-foreground md:grid-cols-2">
                                                            <div><span className="font-medium text-foreground">{t("channel.statusCode")}:</span> {statusCode ?? '-'}</div>
                                                            <div><span className="font-medium text-foreground">{t("channel.testMode")}:</span> {formatModeLabel(result.data?.mode)}</div>
                                                            <div><span className="font-medium text-foreground">{t("channel.testModel")}:</span> {result.data?.model || '-'}</div>
                                                            <div><span className="font-medium text-foreground">{t("channel.testActualModel")}:</span> {result.data?.actual_model || '-'}</div>
                                                            <div><span className="font-medium text-foreground">{t("channel.testAt")}:</span> {result.data?.test_at || '-'}</div>
                                                            <div><span className="font-medium text-foreground">{t("channel.testTook")}:</span> {result.data?.took !== undefined ? `${result.data.took.toFixed(2)}s` : '-'}</div>
                                                        </div>

                                                        {result.message && (
                                                            <div className="space-y-1">
                                                                <div className="text-xs font-medium">{t("channel.testMessage")}</div>
                                                                <pre className="whitespace-pre-wrap break-all rounded-md border bg-muted/60 p-3 text-xs">{result.message}</pre>
                                                            </div>
                                                        )}

                                                        <div className="space-y-1">
                                                            <div className="text-xs font-medium">{t("channel.testResponse")}</div>
                                                            {result.data?.response ? (
                                                                isJsonString(result.data.response) ? (
                                                                    <div className="max-h-64 overflow-auto rounded-md border bg-muted/60 p-2">
                                                                        <JsonViewer
                                                                            src={result.data.response}
                                                                            name={false}
                                                                            collapsed={2}
                                                                            collapseStringsAfterLength={200}
                                                                        />
                                                                    </div>
                                                                ) : (
                                                                    <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all rounded-md border bg-muted/60 p-3 text-xs">
                                                                        {result.data.response}
                                                                    </pre>
                                                                )
                                                            ) : (
                                                                <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all rounded-md border bg-muted/60 p-3 text-xs">-</pre>
                                                            )}
                                                        </div>
                                                    </div>
                                                )}
                                            </div>
                                        )
                                    })}

                                    {filteredResults.length === 0 && !isTesting && (
                                        <div className="flex items-center justify-center py-8 text-muted-foreground">
                                            <span className="text-sm">
                                                {activeTab === 'success' ? t("channel.testNoSuccess") : t("channel.testNoFailed")}
                                            </span>
                                        </div>
                                    )}
                                </div>
                            </div>
                        </TabsContent>

                        <div className="flex shrink-0 justify-end gap-2 pt-4">
                            {isTesting ? (
                                <Button variant="destructive" onClick={onCancel}>
                                    <X className="mr-2 h-4 w-4" />
                                    {t("channel.testCancel")}
                                </Button>
                            ) : (
                                <Button onClick={() => onOpenChange(false)}>
                                    {t("common.close")}
                                </Button>
                            )}
                        </div>
                    </div>
                </DialogContent>
            </Tabs>
        </Dialog>
    )
}
