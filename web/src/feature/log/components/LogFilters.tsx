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
import type { LogFilters } from '@/types/log'

interface LogFiltersProps {
    onFiltersChange: (filters: LogFilters) => void
    loading?: boolean
}

export function LogFilters({ onFiltersChange, loading = false }: LogFiltersProps) {
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

    const [keyName, setKeyName] = useState('')
    const [model, setModel] = useState('')
    const [dateRange, setDateRange] = useState<DateRange | undefined>(getDefaultDateRange())
    const [codeType, setCodeType] = useState<'all' | 'success' | 'error'>('all')

    // 处理表单提交
    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()

        const filters: LogFilters = {
            keyName: keyName.trim() || undefined,
            model: model.trim() || undefined,
            code_type: codeType,
            page: 1, // 重置到第一页
            per_page: 10
        }

        // 处理日期范围
        if (dateRange?.from) {
            filters.start_timestamp = dateRange.from.getTime()
        }
        if (dateRange?.to) {
            // 将结束时间设置为当天的 23:59:59
            const endDate = new Date(dateRange.to)
            endDate.setHours(23, 59, 59, 999)
            filters.end_timestamp = endDate.getTime()
        }

        onFiltersChange(filters)
    }

    // 重置过滤器
    const handleReset = () => {
        setKeyName('')
        setModel('')
        const defaultDateRange = getDefaultDateRange()
        setDateRange(defaultDateRange)
        setCodeType('all')

        const filters: LogFilters = {
            code_type: 'all',
            page: 1,
            per_page: 10,
            start_timestamp: defaultDateRange.from!.getTime(),
            end_timestamp: defaultDateRange.to!.setHours(23, 59, 59, 999)
        }
        onFiltersChange(filters)
    }

    return (
        <div className="bg-card border border-border rounded-lg p-4 shadow-none">
            <form onSubmit={handleSubmit}>
                <div className="flex items-center gap-4">
                    {/* Key Name 过滤器 */}
                    <TooltipProvider>
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <div className="flex-1 min-w-0">
                                    <Input
                                        placeholder={t('log.filters.keyPlaceholder')}
                                        value={keyName}
                                        onChange={(e) => setKeyName(e.target.value)}
                                        disabled={loading}
                                        className="h-10"
                                    />
                                </div>
                            </TooltipTrigger>
                            <TooltipContent>
                                <p>{t('log.filters.keyPlaceholder')}</p>
                            </TooltipContent>
                        </Tooltip>
                    </TooltipProvider>

                    {/* Model 过滤器 */}
                    <TooltipProvider>
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <div className="flex-1 min-w-0">
                                    <Input
                                        placeholder={t('log.filters.modelPlaceholder')}
                                        value={model}
                                        onChange={(e) => setModel(e.target.value)}
                                        disabled={loading}
                                        className="h-10"
                                    />
                                </div>
                            </TooltipTrigger>
                            <TooltipContent>
                                <p>{t('log.filters.modelPlaceholder')}</p>
                            </TooltipContent>
                        </Tooltip>
                    </TooltipProvider>

                    {/* 状态过滤器 */}
                    <div className="w-32">
                        <Select
                            value={codeType}
                            onValueChange={(value: 'all' | 'success' | 'error') => setCodeType(value)}
                            disabled={loading}
                        >
                            <SelectTrigger className="h-10">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="all">{t('log.filters.statusAll')}</SelectItem>
                                <SelectItem value="success">{t('log.filters.statusSuccess')}</SelectItem>
                                <SelectItem value="error">{t('log.filters.statusError')}</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    {/* 日期范围过滤器 */}
                    <div className="min-w-48 max-w-72">
                        <DateRangePicker
                            value={dateRange}
                            onChange={setDateRange}
                            placeholder={t('log.filters.dateRangePlaceholder')}
                            disabled={loading}
                            className="h-10"
                        />
                    </div>

                    {/* 操作按钮 */}
                    <div className="flex gap-2 flex-shrink-0">
                        <Button type="submit" disabled={loading} className="h-10 px-4">
                            <Search className="h-4 w-4 mr-2" />
                            {loading ? t('common.loading') : t('log.filters.search')}
                        </Button>
                        <Button
                            type="button"
                            variant="outline"
                            onClick={handleReset}
                            disabled={loading}
                            className="h-10 px-4"
                        >
                            <RotateCcw className="h-4 w-4 mr-2" />
                            {t('log.filters.reset')}
                        </Button>
                    </div>
                </div>
            </form>
        </div>
    )
} 