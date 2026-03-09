import React, { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { DateRange } from 'react-day-picker'
import { Search, RotateCcw } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import {
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from '@/components/ui/tooltip'
import { DateRangePicker } from '@/components/common/DateRangePicker'
import type { LogFilters as LogFiltersType } from '@/types/log'

interface LogFiltersProps {
    onFiltersChange: (filters: LogFiltersType) => void
    loading?: boolean
    availableModels?: string[]
    availableTokenNames?: string[]
    availableChannels?: number[]
}

export function LogFilters({
    onFiltersChange,
    loading = false,
    availableModels,
    availableTokenNames,
    availableChannels,
}: LogFiltersProps) {
    const { t } = useTranslation()

    const getDefaultDateRange = (): DateRange => {
        const today = new Date()
        const sevenDaysAgo = new Date()
        sevenDaysAgo.setDate(today.getDate() - 7)
        return { from: sevenDaysAgo, to: today }
    }

    const [model, setModel] = useState('')
    const [tokenName, setTokenName] = useState('')
    const [channel, setChannel] = useState('')
    const [dateRange, setDateRange] = useState<DateRange | undefined>(getDefaultDateRange())
    const [codeType, setCodeType] = useState<'all' | 'success' | 'error'>('all')

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()

        const effectiveModel = model === '__all__' ? '' : model
        const effectiveTokenName = tokenName === '__all__' ? '' : tokenName
        const effectiveChannel = channel === '__all__' ? '' : channel

        const filters: LogFiltersType = {
            model: effectiveModel.trim() || undefined,
            token_name: effectiveTokenName.trim() || undefined,
            channel: effectiveChannel ? parseInt(effectiveChannel) : undefined,
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

        onFiltersChange(filters)
    }

    const handleReset = () => {
        setModel('')
        setTokenName('')
        setChannel('')
        const defaultDateRange = getDefaultDateRange()
        setDateRange(defaultDateRange)
        setCodeType('all')

        const filters: LogFiltersType = {
            code_type: 'all',
            page: 1,
            per_page: 10,
            start_timestamp: defaultDateRange.from!.getTime(),
            end_timestamp: defaultDateRange.to!.setHours(23, 59, 59, 999)
        }
        onFiltersChange(filters)
    }

    const showTokenName = !!availableTokenNames
    const showChannel = !!availableChannels

    return (
        <div className="bg-card border border-border rounded-lg p-4 shadow-none">
            <form onSubmit={handleSubmit}>
                <div className="flex items-center gap-3 flex-wrap">
                    {/* Model filter */}
                    {availableModels && availableModels.length > 0 ? (
                        <div className="w-44">
                            <Select value={model} onValueChange={setModel} disabled={loading}>
                                <SelectTrigger className="h-9">
                                    <SelectValue placeholder={t('log.filters.modelPlaceholder')} />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="__all__">{t('log.filters.statusAll')}</SelectItem>
                                    {availableModels.map((m) => (
                                        <SelectItem key={m} value={m}>{m}</SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                    ) : (
                        <TooltipProvider>
                            <Tooltip>
                                <TooltipTrigger asChild>
                                    <div className="flex-1 min-w-0">
                                        <Input
                                            placeholder={t('log.filters.modelPlaceholder')}
                                            value={model}
                                            onChange={(e) => setModel(e.target.value)}
                                            disabled={loading}
                                            className="h-9"
                                        />
                                    </div>
                                </TooltipTrigger>
                                <TooltipContent>
                                    <p>{t('log.filters.modelPlaceholder')}</p>
                                </TooltipContent>
                            </Tooltip>
                        </TooltipProvider>
                    )}

                    {/* Token name filter */}
                    {showTokenName && (
                        <div className="w-44">
                            {availableTokenNames!.length > 0 ? (
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
                            ) : (
                                <Input
                                    placeholder={t('log.filters.tokenNamePlaceholder')}
                                    value={tokenName}
                                    onChange={(e) => setTokenName(e.target.value)}
                                    disabled={loading}
                                    className="h-9"
                                />
                            )}
                        </div>
                    )}

                    {/* Channel filter */}
                    {showChannel && availableChannels!.length > 0 && (
                        <div className="w-36">
                            <Select value={channel} onValueChange={setChannel} disabled={loading}>
                                <SelectTrigger className="h-9">
                                    <SelectValue placeholder={t('log.filters.channelPlaceholder')} />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="__all__">{t('log.filters.statusAll')}</SelectItem>
                                    {availableChannels!.map((ch) => (
                                        <SelectItem key={ch} value={String(ch)}>
                                            {t('log.channel')} {ch}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                    )}

                    {/* Status filter */}
                    <div className="w-28">
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

                    {/* Date range */}
                    <div className="min-w-44 max-w-64">
                        <DateRangePicker
                            value={dateRange}
                            onChange={setDateRange}
                            placeholder={t('log.filters.dateRangePlaceholder')}
                            disabled={loading}
                            className="h-9"
                        />
                    </div>

                    {/* Action buttons */}
                    <div className="flex gap-2 flex-shrink-0">
                        <Button type="submit" disabled={loading} className="h-9 px-3" size="sm">
                            <Search className="h-3.5 w-3.5 mr-1.5" />
                            {loading ? t('common.loading') : t('log.filters.search')}
                        </Button>
                        <Button
                            type="button"
                            variant="outline"
                            onClick={handleReset}
                            disabled={loading}
                            className="h-9 px-3"
                            size="sm"
                        >
                            <RotateCcw className="h-3.5 w-3.5 mr-1.5" />
                            {t('log.filters.reset')}
                        </Button>
                    </div>
                </div>
            </form>
        </div>
    )
}
