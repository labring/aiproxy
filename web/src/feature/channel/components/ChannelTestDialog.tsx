// src/feature/channel/components/ChannelTestDialog.tsx
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { CheckCircle, XCircle, Loader2, X, ExternalLink } from 'lucide-react'
import { cn } from '@/lib/utils'
import { ChannelTestResult } from '@/api/channel'

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

    const handleResultClick = (result: ChannelTestResult) => {
        if (showChannelInfo && result.data?.channel_id && onChannelClick) {
            onChannelClick(result.data.channel_id)
            // 不关闭测试对话框，保持测试结果
        }
    }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-[500px]">
                <DialogHeader>
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
                </DialogHeader>

                <div className="space-y-4">
                    {/* Progress summary */}
                    <div className="flex items-center justify-between text-sm">
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

                    {/* Progress bar */}
                    {isTesting && (
                        <div className="w-full bg-muted rounded-full h-2 overflow-hidden">
                            <div
                                className="h-full bg-primary transition-all duration-300"
                                style={{ width: `${Math.max(5, (successCount + failCount) / Math.max(1, results.length || 1) * 100)}%` }}
                            />
                        </div>
                    )}

                    {/* Tabs for filtering results */}
                    <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as 'all' | 'success' | 'failed')}>
                        <TabsList className="grid w-full grid-cols-3">
                            <TabsTrigger value="all">
                                {t("channel.testTabAll")} ({results.length})
                            </TabsTrigger>
                            <TabsTrigger value="success" className="data-[state=active]:text-green-600">
                                <CheckCircle className="h-3.5 w-3.5 mr-1" />
                                {successCount}
                            </TabsTrigger>
                            <TabsTrigger value="failed" className="data-[state=active]:text-red-600">
                                <XCircle className="h-3.5 w-3.5 mr-1" />
                                {failCount}
                            </TabsTrigger>
                        </TabsList>

                        <TabsContent value={activeTab} className="mt-2">
                            <div className="h-[280px] overflow-y-auto pr-2">
                                <div className="space-y-2">
                                    {[...filteredResults].reverse().map((result, index) => {
                                        const canClick = showChannelInfo && result.data?.channel_id && onChannelClick

                                        return (
                                            <div
                                                key={index}
                                                onClick={() => handleResultClick(result)}
                                                className={cn(
                                                    "flex items-start gap-2 p-3 rounded-lg text-sm border",
                                                    result.success && result.data?.success
                                                        ? "bg-green-50 dark:bg-green-950/30 border-green-200 dark:border-green-800"
                                                        : "bg-red-50 dark:bg-red-950/30 border-red-200 dark:border-red-800",
                                                    canClick && "cursor-pointer hover:ring-2 hover:ring-primary/50 transition-all"
                                                )}
                                            >
                                                {result.success && result.data?.success ? (
                                                    <CheckCircle className="h-4 w-4 text-green-600 dark:text-green-500 mt-0.5 shrink-0" />
                                                ) : (
                                                    <XCircle className="h-4 w-4 text-red-600 dark:text-red-500 mt-0.5 shrink-0" />
                                                )}
                                                <div className="flex-1 min-w-0">
                                                    {/* 渠道信息 */}
                                                    {showChannelInfo && result.data?.channel_name && (
                                                        <div className="flex items-center gap-1.5 mb-1">
                                                            <span className="text-xs font-medium text-primary dark:text-primary/80">
                                                                {result.data.channel_name}
                                                            </span>
                                                            {result.data.channel_id && (
                                                                <span className="text-xs text-muted-foreground">
                                                                    (ID: {result.data.channel_id})
                                                                </span>
                                                            )}
                                                            {canClick && (
                                                                <ExternalLink className="h-3 w-3 text-muted-foreground" />
                                                            )}
                                                        </div>
                                                    )}
                                                    {/* 模型信息 */}
                                                    <div className="font-medium">
                                                        {result.data?.model || result.data?.actual_model || `Model ${index + 1}`}
                                                    </div>
                                                    {result.message && (
                                                        <div className="text-xs text-muted-foreground mt-1 break-all">
                                                            {result.message}
                                                        </div>
                                                    )}
                                                    {result.data && !result.data.success && result.data.response && (
                                                        <div className="text-xs text-red-600 dark:text-red-400 mt-1 break-all">
                                                            {result.data.response.substring(0, 200)}
                                                            {result.data.response.length > 200 && '...'}
                                                        </div>
                                                    )}
                                                    {result.data?.took && (
                                                        <div className="text-xs text-muted-foreground mt-1">
                                                            {t("channel.testTook")}: {result.data.took.toFixed(2)}s
                                                        </div>
                                                    )}
                                                </div>
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

                                    {/* Loading indicator for pending tests */}
                                    {isTesting && (
                                        <div className="flex items-center justify-center py-4 text-muted-foreground">
                                            <Loader2 className="h-4 w-4 animate-spin mr-2" />
                                            <span className="text-sm">{t("channel.testingInProgress")}</span>
                                        </div>
                                    )}
                                </div>
                            </div>
                        </TabsContent>
                    </Tabs>

                    {/* Actions */}
                    <div className="flex justify-end gap-2">
                        {isTesting ? (
                            <Button variant="destructive" onClick={onCancel}>
                                <X className="h-4 w-4 mr-2" />
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
        </Dialog>
    )
}
