import { useState, useEffect, useCallback, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import type { DateRange } from 'react-day-picker'
import { RotateCcw } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { DateRangePicker } from '@/components/common/DateRangePicker'
import { ChannelLabel } from '@/components/common/ChannelLabel'
import type { LogFilters as LogFiltersType } from '@/types/log'
import { channelApi } from '@/api/channel'
import { useChannelTypeMetas } from '@/feature/channel/hooks'

interface LogFiltersProps {
    onFiltersChange: (filters: LogFiltersType) => void
    loading?: boolean
    availableModels?: string[]
    availableTokenNames?: string[]
    availableChannels?: number[]
    tokenNameFirst?: boolean
    defaultTokenName?: string
}

export function LogFilters({
    onFiltersChange,
    loading = false,
    availableModels,
    availableTokenNames,
    availableChannels,
    tokenNameFirst = false,
    defaultTokenName = '',
}: LogFiltersProps) {
    const { t } = useTranslation()
    const { data: typeMetas } = useChannelTypeMetas()

    // Batch fetch channel names
    const [channelInfoMap, setChannelInfoMap] = useState<Record<number, { name: string; type: number }>>({})

    useEffect(() => {
        if (!availableChannels || availableChannels.length === 0) return
        const missing = availableChannels.filter(id => !(id in channelInfoMap))
        if (missing.length === 0) return

        channelApi.getChannelBatchInfo(missing)
            .then(infos => {
                setChannelInfoMap(prev => {
                    const next = { ...prev }
                    for (const info of infos) {
                        next[info.id] = { name: info.name, type: info.type }
                    }
                    return next
                })
            })
            .catch(() => {
                setChannelInfoMap(prev => {
                    const next = { ...prev }
                    for (const id of missing) {
                        if (!(id in next)) next[id] = { name: `#${id}`, type: 0 }
                    }
                    return next
                })
            })
    }, [availableChannels]) // eslint-disable-line react-hooks/exhaustive-deps

    const getDefaultDateRange = (): DateRange => {
        const today = new Date()
        const sevenDaysAgo = new Date()
        sevenDaysAgo.setDate(today.getDate() - 7)
        return { from: sevenDaysAgo, to: today }
    }

    const [model, setModel] = useState('')
    const [tokenName, setTokenName] = useState(defaultTokenName)
    const [channel, setChannel] = useState('')
    const [keyword, setKeyword] = useState('')
    const [dateRange, setDateRange] = useState<DateRange | undefined>(getDefaultDateRange())
    const [codeType, setCodeType] = useState<'all' | 'success' | 'error'>('all')

    const buildFilters = useCallback((): LogFiltersType => {
        const effectiveModel = model === '__all__' ? '' : model
        const effectiveTokenName = tokenName === '__all__' ? '' : tokenName
        const effectiveChannel = channel === '__all__' ? '' : channel

        const filters: LogFiltersType = {
            model: effectiveModel.trim() || undefined,
            token_name: effectiveTokenName.trim() || undefined,
            channel: effectiveChannel ? parseInt(effectiveChannel) : undefined,
            keyword: keyword.trim() || undefined,
            code_type: codeType,
            page: 1,
            per_page: 10
        }

        if (dateRange?.from) {
            filters.start_timestamp = dateRange.from.getTime()
        }
        if (dateRange?.to) {
            const endDate = new Date(dateRange.to)
            endDate.setHours(23, 59, 59, 999)
            filters.end_timestamp = endDate.getTime()
        }

        return filters
    }, [model, tokenName, channel, keyword, dateRange, codeType])

    // Auto-refresh on filter change (skip initial mount), debounce keyword input
    const debounceRef = useRef<ReturnType<typeof setTimeout>>()
    const prevKeywordRef = useRef(keyword)
    const isFirstRender = useRef(true)

    useEffect(() => {
        if (isFirstRender.current) {
            isFirstRender.current = false
            return
        }
        // If only keyword changed, debounce
        if (prevKeywordRef.current !== keyword) {
            prevKeywordRef.current = keyword
            clearTimeout(debounceRef.current)
            debounceRef.current = setTimeout(() => {
                onFiltersChange(buildFilters())
            }, 500)
            return () => clearTimeout(debounceRef.current)
        }
        // Otherwise fire immediately
        onFiltersChange(buildFilters())
    }, [buildFilters]) // eslint-disable-line react-hooks/exhaustive-deps

    const handleReset = () => {
        setModel('')
        setTokenName('')
        setChannel('')
        setKeyword('')
        setDateRange(getDefaultDateRange())
        setCodeType('all')
    }

    const showChannel = !!availableChannels && availableChannels.length > 0
    const showTokenName = !!availableTokenNames && availableTokenNames.length > 0

    const getTypeName = (type: number) => typeMetas?.[type]?.name || ''

    // Channel filter
    const channelFilter = showChannel && (
        <div className="w-56 flex-shrink-0">
            <Select value={channel} onValueChange={setChannel} disabled={loading}>
                <SelectTrigger className="h-9">
                    <SelectValue placeholder={t('log.filters.channelPlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="__all__">{t('log.filters.statusAll')}</SelectItem>
                    {availableChannels!.map((ch) => (
                        <SelectItem key={ch} value={String(ch)}>
                            <ChannelLabel id={ch} info={channelInfoMap[ch]} typeName={getTypeName(channelInfoMap[ch]?.type)} compact />
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
        </div>
    )

    // Model filter
    const modelFilter = (
        <div className="w-44 flex-shrink-0">
            <Select value={model} onValueChange={setModel} disabled={loading}>
                <SelectTrigger className="h-9">
                    <SelectValue placeholder={t('log.filters.modelPlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="__all__">{t('log.filters.statusAll')}</SelectItem>
                    {(availableModels || []).map((m) => (
                        <SelectItem key={m} value={m}>{m}</SelectItem>
                    ))}
                </SelectContent>
            </Select>
        </div>
    )

    // Token name filter
    const tokenNameFilter = showTokenName && (
        <div className="w-44 flex-shrink-0">
            <Select value={tokenName} onValueChange={setTokenName} disabled={loading}>
                <SelectTrigger className="h-9">
                    <SelectValue placeholder={t('log.filters.tokenNamePlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="__all__">{t('log.filters.statusAll')}</SelectItem>
                    {availableTokenNames!.map((name) => (
                        <SelectItem key={name} value={name}>{name}</SelectItem>
                    ))}
                </SelectContent>
            </Select>
        </div>
    )

    return (
        <div className="bg-card border border-border rounded-lg p-3 shadow-none">
            <div className="flex items-center gap-2">
                {/* 根据 tokenNameFirst 控制顺序 */}
                {tokenNameFirst ? (
                    <>{tokenNameFilter}{channelFilter}{modelFilter}</>
                ) : (
                    <>{channelFilter}{modelFilter}{tokenNameFilter}</>
                )}

                {/* Status filter */}
                <div className="w-28 flex-shrink-0">
                    <Select
                        value={codeType}
                        onValueChange={(value: 'all' | 'success' | 'error') => setCodeType(value)}
                        disabled={loading}
                    >
                        <SelectTrigger className="h-9">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="all">{t('log.filters.statusAll')}</SelectItem>
                            <SelectItem value="success">{t('log.filters.statusSuccess')}</SelectItem>
                            <SelectItem value="error">{t('log.filters.statusError')}</SelectItem>
                        </SelectContent>
                    </Select>
                </div>

                <div className="flex-1" />

                {/* Date range */}
                <div className="w-56 flex-shrink-0">
                    <DateRangePicker
                        value={dateRange}
                        onChange={setDateRange}
                        placeholder={t('log.filters.dateRangePlaceholder')}
                        disabled={loading}
                        className="h-9"
                    />
                </div>

                {/* Keyword search */}
                <div className="w-40 flex-shrink-0">
                    <Input
                        placeholder={t('common.search')}
                        value={keyword}
                        onChange={(e) => setKeyword(e.target.value)}
                        disabled={loading}
                        className="h-9"
                    />
                </div>

                {/* Reset */}
                <Button
                    type="button"
                    variant="outline"
                    onClick={handleReset}
                    disabled={loading}
                    className="h-9 px-3 flex-shrink-0"
                    size="sm"
                >
                    <RotateCcw className="h-3.5 w-3.5 mr-1.5" />
                    {t('log.filters.reset')}
                </Button>
            </div>
        </div>
    )
}
