import { useState, useEffect, useCallback, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import type { DateRange } from 'react-day-picker'
import { RotateCcw } from 'lucide-react'

import { Button } from '@/components/ui/button'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { DateRangePicker } from '@/components/common/DateRangePicker'
import { ChannelLabel } from '@/components/common/ChannelLabel'
import { DashboardFilters } from '@/types/dashboard'
import { channelApi } from '@/api/channel'
import { useChannelTypeMetas } from '@/feature/channel/hooks'

export type DataSourceMode = 'total' | 'serviceTierFlex' | 'serviceTierPriority' | 'claudeLongContext'

interface MonitorFiltersProps {
    onFiltersChange: (filters: DashboardFilters) => void
    onDataSourceChange?: (dataSource: DataSourceMode) => void
    loading?: boolean
    availableModels?: string[]
    availableChannels?: number[]
    defaultChannel?: number
    showDataSourceSelector?: boolean
    hasServiceTierData?: boolean
    hasLongContextData?: boolean
    dataSource?: DataSourceMode
}

export function MonitorFilters({
    onFiltersChange,
    onDataSourceChange,
    loading = false,
    availableModels = [],
    availableChannels = [],
    defaultChannel,
    showDataSourceSelector = false,
    hasServiceTierData = false,
    hasLongContextData = false,
    dataSource = 'total'
}: MonitorFiltersProps) {
    const { t } = useTranslation()
    const { data: typeMetas } = useChannelTypeMetas()

    const getDefaultDateRange = (): DateRange => {
        const today = new Date()
        const oneDayAgo = new Date()
        oneDayAgo.setDate(today.getDate() - 1)
        return { from: oneDayAgo, to: today }
    }

    const [model, setModel] = useState('')
    const [channel, setChannel] = useState(defaultChannel ? String(defaultChannel) : '')
    const [dateRange, setDateRange] = useState<DateRange | undefined>(getDefaultDateRange())
    const [timespan, setTimespan] = useState<'minute' | 'hour' | 'day' | 'month'>('hour')

    // Batch fetch channel names
    const [channelInfoMap, setChannelInfoMap] = useState<Record<number, { name: string; type: number }>>({})

    useEffect(() => {
        if (availableChannels.length === 0) return
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

    const getClientTimezone = () => Intl.DateTimeFormat().resolvedOptions().timeZone

    const buildFilters = useCallback((): DashboardFilters => {
        const effectiveModel = model === '__all__' ? '' : model
        const effectiveChannel = channel === '__all__' ? '' : channel

        const filters: DashboardFilters = {
            model: effectiveModel || undefined,
            channel: effectiveChannel ? Number(effectiveChannel) : undefined,
            timespan,
            timezone: getClientTimezone(),
        }
        if (dateRange?.from) {
            filters.start_timestamp = Math.floor(dateRange.from.getTime() / 1000)
        }
        if (dateRange?.to) {
            const endDate = new Date(dateRange.to)
            endDate.setHours(23, 59, 59, 999)
            filters.end_timestamp = Math.floor(endDate.getTime() / 1000)
        }
        return filters
    }, [model, channel, dateRange, timespan])

    // Auto-refresh on filter change (skip initial mount - page provides initial filters)
    const isFirstRender = useRef(true)
    useEffect(() => {
        if (isFirstRender.current) {
            isFirstRender.current = false
            return
        }
        onFiltersChange(buildFilters())
    }, [buildFilters]) // eslint-disable-line react-hooks/exhaustive-deps

    const handleReset = () => {
        setModel('')
        setChannel('')
        setDateRange(getDefaultDateRange())
        setTimespan('hour')
    }

    const getTypeName = (type: number) => typeMetas?.[type]?.name || ''

    return (
        <div className="bg-card border border-border rounded-lg p-3 shadow-none">
            <div className="flex items-center gap-2">
                {/* Channel */}
                {availableChannels.length > 0 && (
                    <div className="w-56 flex-shrink-0">
                        <Select value={channel} onValueChange={setChannel} disabled={loading}>
                            <SelectTrigger className="h-9">
                                <SelectValue placeholder={t('monitor.filters.channelPlaceholder')} />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="__all__">{t('log.filters.statusAll')}</SelectItem>
                                {availableChannels.map((id) => (
                                    <SelectItem key={id} value={String(id)}>
                                        <ChannelLabel id={id} info={channelInfoMap[id]} typeName={getTypeName(channelInfoMap[id]?.type)} compact />
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                )}

                {/* Model */}
                {availableModels.length > 0 && (
                    <div className="w-44 flex-shrink-0">
                        <Select value={model} onValueChange={setModel} disabled={loading}>
                            <SelectTrigger className="h-9">
                                <SelectValue placeholder={t('monitor.filters.modelPlaceholder')} />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="__all__">{t('log.filters.statusAll')}</SelectItem>
                                {availableModels.map((m) => (
                                    <SelectItem key={m} value={m}>{m}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                )}

                {/* Data Source Selector */}
                {showDataSourceSelector && (
                    <div className="w-36 flex-shrink-0">
                        <Select value={dataSource} onValueChange={(value) => onDataSourceChange?.(value as DataSourceMode)} disabled={loading}>
                            <SelectTrigger className="h-9">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="total">{t('monitor.filters.dataSourceTotal')}</SelectItem>
                                {hasServiceTierData && (
                                    <>
                                        <SelectItem value="serviceTierFlex">{t('monitor.filters.dataSourceFlex')}</SelectItem>
                                        <SelectItem value="serviceTierPriority">{t('monitor.filters.dataSourcePriority')}</SelectItem>
                                    </>
                                )}
                                {hasLongContextData && (
                                    <SelectItem value="claudeLongContext">{t('monitor.filters.dataSourceLongContext')}</SelectItem>
                                )}
                            </SelectContent>
                        </Select>
                    </div>
                )}

                <div className="flex-1" />

                {/* Date range */}
                <div className="w-56 flex-shrink-0">
                    <DateRangePicker
                        value={dateRange}
                        onChange={setDateRange}
                        placeholder={t('monitor.filters.dateRangePlaceholder')}
                        disabled={loading}
                        className="h-9"
                    />
                </div>

                {/* Timespan */}
                <div className="w-22 flex-shrink-0">
                    <Select
                        value={timespan}
                        onValueChange={(value: 'minute' | 'hour' | 'day' | 'month') => setTimespan(value)}
                        disabled={loading}
                    >
                        <SelectTrigger className="h-9">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="minute">{t('monitor.filters.timespanMinute')}</SelectItem>
                            <SelectItem value="hour">{t('monitor.filters.timespanHour')}</SelectItem>
                            <SelectItem value="day">{t('monitor.filters.timespanDay')}</SelectItem>
                            <SelectItem value="month">{t('monitor.filters.timespanMonth')}</SelectItem>
                        </SelectContent>
                    </Select>
                </div>

                {/* Reset */}
                <Button
                    type="button"
                    variant="outline"
                    onClick={handleReset}
                    disabled={loading}
                    className="h-9 px-3 flex-shrink-0"
                >
                    <RotateCcw className="h-4 w-4 mr-1.5" />
                    {t('monitor.filters.reset')}
                </Button>
            </div>
        </div>
    )
}
