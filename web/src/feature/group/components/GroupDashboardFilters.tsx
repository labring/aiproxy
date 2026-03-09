import React, { useState } from 'react'
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
        const sevenDaysAgo = new Date()
        sevenDaysAgo.setDate(today.getDate() - 7)
        return { from: sevenDaysAgo, to: today }
    }

    const [tokenName, setTokenName] = useState(defaultTokenName || '')
    const [model, setModel] = useState('')
    const [dateRange, setDateRange] = useState<DateRange | undefined>(getDefaultDateRange())
    const [timespan, setTimespan] = useState<'day' | 'hour'>('day')

    const getClientTimezone = () => Intl.DateTimeFormat().resolvedOptions().timeZone

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()

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
        onFiltersChange(filters)
    }

    const handleReset = () => {
        setTokenName('')
        setModel('')
        const defaultDateRange = getDefaultDateRange()
        setDateRange(defaultDateRange)
        setTimespan('day')

        const filters: DashboardFilters & { tokenName?: string } = {
            timespan: 'day',
            timezone: getClientTimezone(),
            start_timestamp: Math.floor(defaultDateRange.from!.getTime() / 1000),
            end_timestamp: Math.floor(defaultDateRange.to!.setHours(23, 59, 59, 999) / 1000)
        }
        onFiltersChange(filters)
    }

    return (
        <div className="bg-card border border-border rounded-lg p-4 shadow-none">
            <form onSubmit={handleSubmit}>
                <div className="flex items-center gap-4">
                    {/* Token Name */}
                    {availableTokenNames.length > 0 && (
                        <div className="w-44">
                            <Select value={tokenName} onValueChange={setTokenName} disabled={loading}>
                                <SelectTrigger className="h-10">
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
                    )}

                    {/* Model */}
                    {availableModels.length > 0 && (
                        <div className="w-44">
                            <Select value={model} onValueChange={setModel} disabled={loading}>
                                <SelectTrigger className="h-10">
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

                    {/* Date Range */}
                    <div className="min-w-48 max-w-72">
                        <DateRangePicker
                            value={dateRange}
                            onChange={setDateRange}
                            placeholder={t('monitor.filters.dateRangePlaceholder')}
                            disabled={loading}
                            className="h-10"
                        />
                    </div>

                    {/* Timespan */}
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

                    {/* Buttons */}
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
