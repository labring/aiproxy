import React, { useState, useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import type { DateRange } from 'react-day-picker'
import { Search, RotateCcw } from 'lucide-react'

import { Button } from '@/components/ui/button'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { DateRangePicker } from '@/components/common/DateRangePicker'
import { DashboardFilters } from '@/types/dashboard'
import { Combobox } from '@/components/ui/combobox'
import { channelApi } from '@/api/channel'

interface MonitorFiltersProps {
    onFiltersChange: (filters: DashboardFilters) => void
    loading?: boolean
    availableModels?: string[]
    availableChannels?: number[]
    defaultChannel?: number
}

export function MonitorFilters({ onFiltersChange, loading = false, availableModels = [], availableChannels = [], defaultChannel }: MonitorFiltersProps) {
    const { t } = useTranslation()

    const getDefaultDateRange = (): DateRange => {
        const today = new Date()
        const sevenDaysAgo = new Date()
        sevenDaysAgo.setDate(today.getDate() - 7)
        return { from: sevenDaysAgo, to: today }
    }

    const [model, setModel] = useState('')
    const [channel, setChannel] = useState(defaultChannel ? String(defaultChannel) : '')
    const [dateRange, setDateRange] = useState<DateRange | undefined>(getDefaultDateRange())
    const [timespan, setTimespan] = useState<'day' | 'hour'>('day')

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
                // fallback: use IDs as names
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

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()

        const filters: DashboardFilters = {
            model: model || undefined,
            channel: channel ? Number(channel) : undefined,
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
        onFiltersChange(filters)
    }

    const handleReset = () => {
        setModel('')
        setChannel('')
        const defaultDateRange = getDefaultDateRange()
        setDateRange(defaultDateRange)
        setTimespan('day')

        const filters: DashboardFilters = {
            timespan: 'day',
            timezone: getClientTimezone(),
            start_timestamp: Math.floor(defaultDateRange.from!.getTime() / 1000),
            end_timestamp: Math.floor(defaultDateRange.to!.setHours(23, 59, 59, 999) / 1000)
        }
        onFiltersChange(filters)
    }

    const channelOptions = useMemo(() =>
        availableChannels.map(id => ({
            value: String(id),
            label: `${channelInfoMap[id]?.name || `#${id}`} (#${id})`,
        })),
        [availableChannels, channelInfoMap]
    )

    const modelOptions = useMemo(() =>
        availableModels.map(m => ({ value: m, label: m })),
        [availableModels]
    )

    return (
        <div className="bg-card border border-border rounded-lg p-4 shadow-none">
            <form onSubmit={handleSubmit}>
                <div className="flex items-center gap-4">
                    {/* Channel 选择器 */}
                    <div className="flex-1 min-w-0">
                        <Combobox
                            options={channelOptions}
                            value={channel}
                            onValueChange={setChannel}
                            placeholder={t('monitor.filters.channelPlaceholder')}
                            emptyText={t('common.noResult')}
                            disabled={loading}
                            className="h-10"
                        />
                    </div>

                    {/* Model 选择器 */}
                    <div className="flex-1 min-w-0">
                        <Combobox
                            options={modelOptions}
                            value={model}
                            onValueChange={setModel}
                            placeholder={t('monitor.filters.modelPlaceholder')}
                            emptyText={t('common.noResult')}
                            disabled={loading}
                            className="h-10"
                        />
                    </div>

                    {/* 日期范围 */}
                    <div className="min-w-48 max-w-72">
                        <DateRangePicker
                            value={dateRange}
                            onChange={setDateRange}
                            placeholder={t('monitor.filters.dateRangePlaceholder')}
                            disabled={loading}
                            className="h-10"
                        />
                    </div>

                    {/* 时间粒度 */}
                    <div className="w-24">
                        <Select
                            value={timespan}
                            onValueChange={(value: 'day' | 'hour') => setTimespan(value)}
                            disabled={loading}
                        >
                            <SelectTrigger className="h-10">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="hour">{t('monitor.filters.timespanHour')}</SelectItem>
                                <SelectItem value="day">{t('monitor.filters.timespanDay')}</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    {/* 操作按钮 */}
                    <div className="flex gap-2 flex-shrink-0">
                        <Button type="submit" disabled={loading} className="h-10 px-4">
                            <Search className="h-4 w-4 mr-2" />
                            {loading ? t('common.loading') : t('monitor.filters.search')}
                        </Button>
                        <Button
                            type="button"
                            variant="outline"
                            onClick={handleReset}
                            disabled={loading}
                            className="h-10 px-4"
                        >
                            <RotateCcw className="h-4 w-4 mr-2" />
                            {t('monitor.filters.reset')}
                        </Button>
                    </div>
                </div>
            </form>
        </div>
    )
}
