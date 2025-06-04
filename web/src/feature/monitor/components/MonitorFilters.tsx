import React, { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { DateRange } from 'react-day-picker'

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
import { DashboardFilters } from '@/types/dashboard'

interface MonitorFiltersProps {
    onFiltersChange: (filters: DashboardFilters) => void
    loading?: boolean
}

export function MonitorFilters({ onFiltersChange, loading = false }: MonitorFiltersProps) {
    const { t } = useTranslation()
    
    // 计算默认日期范围（当前时间往前7天）
    const getDefaultDateRange = (): DateRange => {
        const today = new Date()
        const sevenDaysAgo = new Date()
        sevenDaysAgo.setDate(today.getDate() - 7)
        
        return {
            from: sevenDaysAgo,
            to: today
        }
    }
    
    const [key, setKey] = useState('')
    const [model, setModel] = useState('')
    const [dateRange, setDateRange] = useState<DateRange | undefined>(getDefaultDateRange())
    const [timespan, setTimespan] = useState<'day' | 'hour'>('day')

    // 获取客户端时区
    const getClientTimezone = () => {
        return Intl.DateTimeFormat().resolvedOptions().timeZone
    }

    // 处理表单提交
    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()
        
        const filters: DashboardFilters = {
            key: key.trim() || undefined,
            model: model.trim() || undefined,
            timespan,
            timezone: getClientTimezone(),
        }

        // 处理日期范围
        if (dateRange?.from) {
            filters.start_timestamp = Math.floor(dateRange.from.getTime() / 1000)
        }
        if (dateRange?.to) {
            // 将结束时间设置为当天的 23:59:59
            const endDate = new Date(dateRange.to)
            endDate.setHours(23, 59, 59, 999)
            filters.end_timestamp = Math.floor(endDate.getTime() / 1000)
        }

        onFiltersChange(filters)
    }

    // 重置过滤器
    const handleReset = () => {
        setKey('')
        setModel('')
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

    return (
        <div className="bg-card border border-border rounded-lg p-4 shadow-none">
            <form onSubmit={handleSubmit}>
                <div className="flex items-center gap-4">
                    {/* Key 过滤器 */}
                    <TooltipProvider>
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <div className="flex-1 min-w-0">
                                    <Input
                                        placeholder={t('monitor.filters.keyPlaceholder')}
                                        value={key}
                                        onChange={(e) => setKey(e.target.value)}
                                        disabled={loading}
                                        className="h-10"
                                    />
                                </div>
                            </TooltipTrigger>
                            <TooltipContent>
                                <p>{t('monitor.filters.keyPlaceholder')}</p>
                            </TooltipContent>
                        </Tooltip>
                    </TooltipProvider>

                    {/* Model 过滤器 */}
                    <TooltipProvider>
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <div className="flex-1 min-w-0">
                                    <Input
                                        placeholder={t('monitor.filters.modelPlaceholder')}
                                        value={model}
                                        onChange={(e) => setModel(e.target.value)}
                                        disabled={loading}
                                        className="h-10"
                                    />
                                </div>
                            </TooltipTrigger>
                            <TooltipContent>
                                <p>{t('monitor.filters.modelPlaceholder')}</p>
                            </TooltipContent>
                        </Tooltip>
                    </TooltipProvider>

                    {/* 日期范围过滤器 */}
                    <div className="min-w-48  max-w-72">
                        <DateRangePicker
                            value={dateRange}
                            onChange={setDateRange}
                            placeholder={t('monitor.filters.dateRangePlaceholder')}
                            disabled={loading}
                            className="h-10"
                        />
                    </div>

                    {/* 时间粒度过滤器 */}
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
                        <Button type="submit" disabled={loading} className="h-10 px-6">
                            {loading ? t('common.loading') : t('monitor.filters.apply')}
                        </Button>
                        <Button 
                            type="button" 
                            variant="outline" 
                            onClick={handleReset}
                            disabled={loading}
                            className="h-10 px-4"
                        >
                            {t('monitor.filters.reset')}
                        </Button>
                    </div>
                </div>
            </form>
        </div>
    )
} 