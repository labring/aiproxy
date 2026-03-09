import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { format } from 'date-fns'
import { Separator } from '@/components/ui/separator'
import { JsonViewer } from './JsonViewer'
import { useLogDetail } from '@/feature/log/hooks'
import type { LogRecord, LogRequestDetail } from '@/types/log'

// Format price with unit
const formatPrice = (price: number, unit: number): string => {
    if (!price) return '-'
    if (unit > 0) return `${price}/${unit}`
    return price.toString()
}

export const ExpandedLogContent = ({ log }: { log: LogRecord }) => {
    const { t } = useTranslation()
    const needsDetail = !!log.request_detail
    const [requestDetail, setRequestDetail] = useState<LogRequestDetail | null>(null)

    const {
        data: logDetail,
        isLoading: isLoadingDetail,
        error: logDetailError
    } = useLogDetail(needsDetail ? log.id : null)

    useEffect(() => {
        if (logDetail) {
            setRequestDetail(logDetail)
        }
    }, [logDetail])

    const requestBody = needsDetail && requestDetail ? requestDetail.request_body : null
    const responseBody = needsDetail && requestDetail ? requestDetail.response_body : null
    const requestTruncated = needsDetail && requestDetail ? requestDetail.request_body_truncated : false
    const responseTruncated = needsDetail && requestDetail ? requestDetail.response_body_truncated : false
    const isLoadingData = needsDetail && isLoadingDetail
    const hasError = needsDetail && logDetailError

    const calculateDuration = () => {
        if (!log.request_at || !log.created_at) return '-'
        const requestAt = new Date(log.request_at)
        const createdAt = new Date(log.created_at)
        const duration = (createdAt.getTime() - requestAt.getTime()) / 1000
        return `${duration.toFixed(2)}s`
    }

    return (
        <div className="p-4 space-y-4 bg-muted/50 border-t">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                {/* Basic info */}
                <div className="space-y-2">
                    <h4 className="font-semibold text-sm">{t('log.basicInfo')}</h4>
                    <div className="space-y-1 text-sm">
                        <div><span className="font-medium">{t('log.id')}:</span> {log.id}</div>
                        <div><span className="font-medium">{t('log.requestId')}:</span> {log.request_id || '-'}</div>
                        <div><span className="font-medium">{t('log.upstreamId')}:</span> {log.upstream_id || '-'}</div>
                        <div><span className="font-medium">{t('log.group')}:</span> {log.group || '-'}</div>
                        <div><span className="font-medium">{t('log.keyName')}:</span> {log.token_name || '-'}</div>
                        <div><span className="font-medium">{t('log.model')}:</span> {log.model || '-'}</div>
                        <div><span className="font-medium">{t('log.channel')}:</span> {log.channel}</div>
                        <div><span className="font-medium">{t('log.mode')}:</span> {t(`modeType.${log.mode}`, { defaultValue: log.mode?.toString() || '-' })}</div>
                        <div><span className="font-medium">{t('log.user')}:</span> {log.user || '-'}</div>
                        <div><span className="font-medium">{t('log.ip')}:</span> {log.ip || '-'}</div>
                        <div><span className="font-medium">{t('log.endpoint')}:</span> {log.endpoint || '-'}</div>
                        <div><span className="font-medium">{t('log.usedAmount')}:</span> {log.used_amount ? `$${log.used_amount.toFixed(6)}` : '-'}</div>
                        {log.content && <div><span className="font-medium">{t('log.content')}:</span> {log.content}</div>}
                    </div>
                </div>

                {/* Token usage info */}
                <div className="space-y-2">
                    <h4 className="font-semibold text-sm">{t('log.tokenInfo')}</h4>
                    <div className="space-y-1 text-sm">
                        <div><span className="font-medium">{t('log.inputTokens')}:</span> {log.usage?.input_tokens?.toLocaleString() || 0}</div>
                        <div><span className="font-medium">{t('log.outputTokens')}:</span> {log.usage?.output_tokens?.toLocaleString() || 0}</div>
                        <div><span className="font-medium">{t('log.total')}:</span> {log.usage?.total_tokens?.toLocaleString() || 0}</div>
                        <div><span className="font-medium">{t('log.cacheCreation')}:</span> {log.usage?.cache_creation_tokens?.toLocaleString() || 0}</div>
                        <div><span className="font-medium">{t('log.cached')}:</span> {log.usage?.cached_tokens?.toLocaleString() || 0}</div>
                        <div><span className="font-medium">{t('log.imageInput')}:</span> {log.usage?.image_input_tokens?.toLocaleString() || 0}</div>
                        <div><span className="font-medium">{t('log.reasoning')}:</span> {log.usage?.reasoning_tokens?.toLocaleString() || 0}</div>
                        <div><span className="font-medium">{t('log.webSearchCount')}:</span> {log.usage?.web_search_count || 0}</div>
                    </div>
                </div>

                {/* Time info */}
                <div className="space-y-2">
                    <h4 className="font-semibold text-sm">{t('log.timeInfo')}</h4>
                    <div className="space-y-1 text-sm">
                        <div><span className="font-medium">{t('log.created')}:</span> {log.created_at ? format(new Date(log.created_at), 'yyyy-MM-dd HH:mm:ss') : '-'}</div>
                        <div><span className="font-medium">{t('log.request')}:</span> {log.request_at ? format(new Date(log.request_at), 'yyyy-MM-dd HH:mm:ss') : '-'}</div>
                        <div><span className="font-medium">{t('log.duration')}:</span> {calculateDuration()}</div>
                        {log.retry_at && <div><span className="font-medium">{t('log.retry')}:</span> {format(new Date(log.retry_at), 'yyyy-MM-dd HH:mm:ss')}</div>}
                        <div><span className="font-medium">{t('log.retryTimes')}:</span> {log.retry_times || 0}</div>
                        <div><span className="font-medium">{t('log.ttfb')}:</span> {log.ttfb_milliseconds || 0}ms</div>
                    </div>
                </div>

                {/* Price info */}
                <div className="space-y-2">
                    <h4 className="font-semibold text-sm">{t('log.priceInfo')}</h4>
                    <div className="space-y-1 text-sm">
                        <div><span className="font-medium">{t('log.inputPrice')}:</span> {formatPrice(log.price?.input_price, log.price?.input_price_unit)}</div>
                        <div><span className="font-medium">{t('log.outputPrice')}:</span> {formatPrice(log.price?.output_price, log.price?.output_price_unit)}</div>
                        <div><span className="font-medium">{t('log.cacheCreationPrice')}:</span> {formatPrice(log.price?.cache_creation_price, log.price?.cache_creation_price_unit)}</div>
                        <div><span className="font-medium">{t('log.cachedPrice')}:</span> {formatPrice(log.price?.cached_price, log.price?.cached_price_unit)}</div>
                        <div><span className="font-medium">{t('log.imageInputPrice')}:</span> {formatPrice(log.price?.image_input_price, log.price?.image_input_price_unit)}</div>
                        <div><span className="font-medium">{t('log.perRequestPrice')}:</span> {log.price?.per_request_price || '-'}</div>
                        <div><span className="font-medium">{t('log.thinkingPrice')}:</span> {formatPrice(log.price?.thinking_mode_output_price, log.price?.thinking_mode_output_price_unit)}</div>
                        <div><span className="font-medium">{t('log.webSearchPrice')}:</span> {formatPrice(log.price?.web_search_price, log.price?.web_search_price_unit)}</div>
                    </div>
                </div>
            </div>

            {/* Metadata */}
            {log.metadata && Object.keys(log.metadata).length > 0 && (
                <>
                    <Separator />
                    <div className="space-y-2">
                        <h4 className="font-semibold text-sm">{t('log.metadata')}</h4>
                        <div className="flex flex-wrap gap-2">
                            {Object.entries(log.metadata).map(([key, value]) => (
                                <span key={key} className="inline-flex items-center px-2 py-1 rounded-md bg-muted text-xs">
                                    <span className="font-medium">{key}:</span>&nbsp;{value}
                                </span>
                            ))}
                        </div>
                    </div>
                </>
            )}

            <Separator />

            {/* Request and response body */}
            {needsDetail && (
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                    <div>
                        <h4 className="font-semibold text-sm mb-2">{t('log.requestBody')}</h4>
                        {isLoadingData ? (
                            <div className="flex items-center justify-center p-4 border rounded">
                                <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary mr-2"></div>
                                <span className="text-sm">{t('common.loading')}</span>
                            </div>
                        ) : hasError ? (
                            <div className="text-sm text-red-500 p-2 border rounded">
                                {t('log.failed')}
                            </div>
                        ) : requestBody ? (
                            <>
                                <JsonViewer
                                    src={requestBody}
                                    collapsed={1}
                                    name="request"
                                />
                                {requestTruncated && (
                                    <div className="text-xs text-amber-600 mt-1">⚠️ {t('log.contentTruncated')}</div>
                                )}
                            </>
                        ) : (
                            <div className="text-sm text-muted-foreground p-2 border rounded">
                                {t('log.noRequestBody')}
                            </div>
                        )}
                    </div>
                    <div>
                        <h4 className="font-semibold text-sm mb-2">{t('log.responseBody')}</h4>
                        {isLoadingData ? (
                            <div className="flex items-center justify-center p-4 border rounded">
                                <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary mr-2"></div>
                                <span className="text-sm">{t('common.loading')}</span>
                            </div>
                        ) : hasError ? (
                            <div className="text-sm text-red-500 p-2 border rounded">
                                {t('log.failed')}
                            </div>
                        ) : responseBody ? (
                            <>
                                <JsonViewer
                                    src={responseBody}
                                    collapsed={1}
                                    name="response"
                                />
                                {responseTruncated && (
                                    <div className="text-xs text-amber-600 mt-1">⚠️ {t('log.contentTruncated')}</div>
                                )}
                            </>
                        ) : (
                            <div className="text-sm text-muted-foreground p-2 border rounded">
                                {t('log.noResponseBody')}
                            </div>
                        )}
                    </div>
                </div>
            )}
        </div>
    )
}
