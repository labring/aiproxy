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
import { DashboardFilters } from '@/types/dashboard'

interface GroupDashboardFiltersProps {
    onFiltersChange: (filters: DashboardFilters & { tokenName?: string }) => void
    loading?: boolean
    availableModels?: string[]
    availableTokenNames?: string[]
    defaultTokenName?: string
}

export function GroupDashboardFilters({
    onFiltersChange,
    loading = false,
    availableModels = [],
    availableTokenNames = [],
    defaultTokenName,
}: GroupDashboardFiltersProps) {
    const { t } = useTranslation()

    const getDefaultDateRange = (): DateRange => {
        const today = new Date()
        const oneDayAgo = new Date()
        oneDayAgo.setDate(today.getDate() - 1)
        return { from: oneDayAgo, to: today }
    }

    const [tokenName, setTokenName] = useState(defaultTokenName || '')
    const [model, setModel] = useState('')
    const [dateRange, setDateRange] = useState<DateRange | undefined>(getDefaultDateRange())
    const [timespan, setTimespan] = useState<'minute' | 'hour' | 'day' | 'month'>('hour')

    const getClientTimezone = () => Intl.DateTimeFormat().resolvedOptions().timeZone

    const buildFilters = useCallback((): DashboardFilters & { tokenName?: string } => {
        const effectiveTokenName = tokenName === '__all__' ? '' : tokenName
        const effectiveModel = model === '__all__' ? '' : model

        const filters: DashboardFilters & { tokenName?: string } = {
            tokenName: effectiveTokenName || undefined,
            model: effectiveModel || undefined,
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
    }, [tokenName, model, dateRange, timespan])

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
        setTokenName('')
        setModel('')
        setDateRange(getDefaultDateRange())
        setTimespan('hour')
    }

    return (
        <div className="bg-card border border-border rounded-lg p-3 shadow-none">
            <div className="flex items-center gap-2">
                {/* Token Name */}
                <div className="w-44 flex-shrink-0">
                    <Select value={tokenName} onValueChange={setTokenName} disabled={loading}>
                        <SelectTrigger className="h-9">
                            <SelectValue placeholder={t('group.dashboard.tokenNamePlaceholder')} />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="__all__">{t('log.filters.statusAll')}</SelectItem>
                            {availableTokenNames.map((name) => (
                                <SelectItem key={name} value={name}>{name}</SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>

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

                <div className="flex-1" />

                {/* Date Range */}
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
