import { useMemo, useState } from 'react'
import type { DateRange } from 'react-day-picker'
import { useTranslation } from 'react-i18next'
import {
    BarChart3,
    Eye,
    RotateCcw,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { TimezoneInput } from '@/components/common/TimezoneInput'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'
import { DateRangePicker } from '@/components/common/DateRangePicker'
import { ServerPagination } from '@/components/table/server-pagination'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { DEFAULT_TIMEZONE, formatRangeTimestamp, zonedBoundaryToUnix } from '@/utils/timezone'
import { useConsumptionRanking } from '../hooks'
import type { ConsumptionRankingType } from '@/types/consumption-ranking'
import { useGroupSummaryMetrics, useRuntimeMetrics } from '@/feature/monitor/runtime-hooks'
import type { RuntimeRateMetric } from '@/types/runtime-metrics'
import { useAllChannels } from '@/feature/channel/hooks'

const getDefaultDateRange = (): DateRange => {
    const today = new Date()
    const sevenDaysAgo = new Date()
    sevenDaysAgo.setDate(today.getDate() - 6)

    return {
        from: sevenDaysAgo,
        to: today,
    }
}

const formatAmount = (amount: number): string => {
    return `$${amount.toLocaleString(undefined, {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
    })}`
}

const formatTokens = (tokens: number): string => tokens.toLocaleString()

interface ConsumptionRankingPanelProps {
    onViewGroup: (groupId: string) => void
    onViewChannel: (channelId: number) => void
    onViewModel: (modelName: string) => void
}

export function ConsumptionRankingPanel({
    onViewGroup,
    onViewChannel,
    onViewModel,
}: ConsumptionRankingPanelProps) {
    const { t } = useTranslation()
    const [rankingType, setRankingType] = useState<ConsumptionRankingType>('group')
    const [dateRange, setDateRange] = useState<DateRange | undefined>(getDefaultDateRange())
    const [timezone, setTimezone] = useState(DEFAULT_TIMEZONE)
    const [page, setPage] = useState(1)
    const [pageSize, setPageSize] = useState(10)

    const query = useMemo(() => {
        const nextQuery = {
            type: rankingType,
            page,
            per_page: pageSize,
            timezone: timezone || DEFAULT_TIMEZONE,
            order: 'used_amount_desc',
        } as const

        return {
            ...nextQuery,
            start_timestamp: dateRange?.from
                ? zonedBoundaryToUnix(dateRange.from, nextQuery.timezone, false)
                : undefined,
            end_timestamp: dateRange?.to
                ? zonedBoundaryToUnix(dateRange.to, nextQuery.timezone, true)
                : undefined,
        }
    }, [dateRange, page, pageSize, rankingType, timezone])

    const { data, isLoading, isFetching } = useConsumptionRanking(query)
    const currentPageGroupIds = useMemo(
        () => (rankingType === 'group'
            ? (data?.items || []).map((item) => item.group_id).filter(Boolean) as string[]
            : []),
        [data?.items, rankingType],
    )
    const { data: groupRuntimeMetrics } = useGroupSummaryMetrics(
        {
            groups: currentPageGroupIds,
        },
        rankingType === 'group' && currentPageGroupIds.length > 0,
    )
    const { data: runtimeMetrics } = useRuntimeMetrics()
    const { data: allChannels } = useAllChannels(rankingType === 'channel')
    const effectiveTimezone = query.timezone || DEFAULT_TIMEZONE
    const channelNameMap = useMemo(
        () => Object.fromEntries((allChannels || []).map((channel) => [channel.id, channel.name])),
        [allChannels],
    )

    const handleDateRangeChange = (nextRange: DateRange | undefined) => {
        setDateRange(nextRange)
        setPage(1)
    }

    const handleTimezoneChange = (value: string) => {
        setTimezone(value)
        setPage(1)
    }

    const handleTypeChange = (value: string) => {
        setRankingType(value as ConsumptionRankingType)
        setPage(1)
    }

    const handleReset = () => {
        setRankingType('group')
        setDateRange(getDefaultDateRange())
        setTimezone(DEFAULT_TIMEZONE)
        setPage(1)
        setPageSize(10)
    }

    const nameHeader = (() => {
        switch (rankingType) {
            case 'channel':
                return t('consumptionRanking.channel')
            case 'model':
                return t('consumptionRanking.model')
            default:
                return t('consumptionRanking.group')
        }
    })()

    const renderNameCell = (item: NonNullable<typeof data>['items'][number]) => {
        switch (rankingType) {
            case 'channel':
                return item.channel_id !== undefined
                    ? channelNameMap[item.channel_id] || `#${item.channel_id}`
                    : '-'
            case 'model':
                return item.model || '-'
            default:
                return item.group_id || '-'
        }
    }

    const getRuntimeMetric = (item: NonNullable<typeof data>['items'][number]): RuntimeRateMetric | undefined => {
        switch (rankingType) {
            case 'channel':
                return item.channel_id !== undefined
                    ? runtimeMetrics?.channels?.[String(item.channel_id)]
                    : undefined
            case 'model':
                return item.model
                    ? runtimeMetrics?.models?.[item.model]
                    : undefined
            default:
                return item.group_id
                    ? groupRuntimeMetrics?.groups?.[item.group_id]
                    : undefined
        }
    }

    const renderRuntimeCell = (item: NonNullable<typeof data>['items'][number]) => {
        const metric = getRuntimeMetric(item)
        if (!metric) {
            return <div className="text-sm text-muted-foreground">-</div>
        }

        return (
            <div className="flex flex-wrap gap-1">
                <Badge variant="outline" className="text-xs">
                    RPM {metric.rpm.toLocaleString()}
                </Badge>
                <Badge variant="outline" className="text-xs">
                    TPM {metric.tpm.toLocaleString()}
                </Badge>
            </div>
        )
    }

    return (
        <Card className="mb-6 gap-0">
            <CardHeader className="gap-4">
                <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                    <div className="space-y-1.5">
                        <div className="flex items-center gap-2">
                            <BarChart3 className="h-5 w-5 text-primary dark:text-[#6A6DE6]" />
                            <CardTitle>{t('consumptionRanking.title')}</CardTitle>
                        </div>
                        <CardDescription>{t('consumptionRanking.description')}</CardDescription>
                        {query.start_timestamp && query.end_timestamp && (
                            <div className="text-xs text-muted-foreground">
                                {t('consumptionRanking.range', {
                                    start: formatRangeTimestamp(query.start_timestamp, effectiveTimezone),
                                    end: formatRangeTimestamp(query.end_timestamp, effectiveTimezone),
                                    timezone: effectiveTimezone,
                                })}
                            </div>
                        )}
                    </div>

                    <div className="flex flex-wrap items-center gap-2">
                        <Tabs value={rankingType} onValueChange={handleTypeChange}>
                            <TabsList>
                                <TabsTrigger value="channel">{t('consumptionRanking.channelTab')}</TabsTrigger>
                                <TabsTrigger value="model">{t('consumptionRanking.modelTab')}</TabsTrigger>
                                <TabsTrigger value="group">{t('consumptionRanking.groupTab')}</TabsTrigger>
                            </TabsList>
                        </Tabs>
                        <div className="w-full sm:w-64">
                            <DateRangePicker
                                value={dateRange}
                                onChange={handleDateRangeChange}
                                placeholder={t('consumptionRanking.dateRangePlaceholder')}
                                className="h-9"
                            />
                        </div>
                        <TimezoneInput value={timezone} onChange={handleTimezoneChange} />
                        <Button type="button" variant="outline" onClick={handleReset} className="h-9 px-3">
                            <RotateCcw className="mr-1.5 h-4 w-4" />
                            {t('monitor.filters.reset')}
                        </Button>
                    </div>
                </div>
            </CardHeader>

            <CardContent className="space-y-4">
                <div className="rounded-lg border">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead className="w-16">#</TableHead>
                                <TableHead>{nameHeader}</TableHead>
                                <TableHead>{t('consumptionRanking.requestCount')}</TableHead>
                                <TableHead>{t('common.runtime')}</TableHead>
                                <TableHead>{t('consumptionRanking.inputTokens')}</TableHead>
                                <TableHead>{t('consumptionRanking.outputTokens')}</TableHead>
                                <TableHead>{t('consumptionRanking.totalTokens')}</TableHead>
                                <TableHead className="text-right">{t('consumptionRanking.usedAmount')}</TableHead>
                                <TableHead className="w-28 text-right">{t('consumptionRanking.actions')}</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {isLoading ? (
                                Array.from({ length: 5 }).map((_, index) => (
                                    <TableRow key={`ranking-loading-${index}`}>
                                        <TableCell><Skeleton className="h-4 w-8" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-28" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                                        <TableCell className="text-right"><Skeleton className="ml-auto h-4 w-20" /></TableCell>
                                        <TableCell className="text-right"><Skeleton className="ml-auto h-8 w-20" /></TableCell>
                                    </TableRow>
                                ))
                            ) : data?.items?.length ? (
                                data.items.map((item) => (
                                    <TableRow key={`${rankingType}-${item.group_id || item.channel_id || item.model}`}>
                                        <TableCell className="font-medium">{item.rank}</TableCell>
                                        <TableCell>
                                            {rankingType === 'group' && item.group_id ? (
                                                <button
                                                    type="button"
                                                    className="font-medium text-left text-primary hover:underline dark:text-[#8B8DFF]"
                                                    onClick={() => onViewGroup(item.group_id!)}
                                                >
                                                    {renderNameCell(item)}
                                                </button>
                                            ) : rankingType === 'channel' && item.channel_id !== undefined ? (
                                                <button
                                                    type="button"
                                                    className="font-medium text-left text-primary hover:underline dark:text-[#8B8DFF]"
                                                    onClick={() => onViewChannel(item.channel_id!)}
                                                >
                                                    {renderNameCell(item)}
                                                </button>
                                            ) : rankingType === 'model' && item.model ? (
                                                <button
                                                    type="button"
                                                    className="font-medium text-left text-primary hover:underline dark:text-[#8B8DFF]"
                                                    onClick={() => onViewModel(item.model!)}
                                                >
                                                    {renderNameCell(item)}
                                                </button>
                                            ) : (
                                                <span className="font-medium">{renderNameCell(item)}</span>
                                            )}
                                        </TableCell>
                                        <TableCell className="font-mono">{item.request_count.toLocaleString()}</TableCell>
                                        <TableCell>{renderRuntimeCell(item)}</TableCell>
                                        <TableCell className="font-mono">{formatTokens(item.input_tokens)}</TableCell>
                                        <TableCell className="font-mono">{formatTokens(item.output_tokens)}</TableCell>
                                        <TableCell className="font-mono">{formatTokens(item.total_tokens)}</TableCell>
                                        <TableCell className="text-right font-mono">{formatAmount(item.used_amount)}</TableCell>
                                        <TableCell className="text-right">
                                            {rankingType === 'group' && item.group_id ? (
                                                <Button
                                                    type="button"
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={() => onViewGroup(item.group_id!)}
                                                    className="h-8 px-2"
                                                >
                                                    <Eye className="mr-1.5 h-4 w-4" />
                                                    {t('consumptionRanking.view')}
                                                </Button>
                                            ) : (
                                                <span className="text-muted-foreground">-</span>
                                            )}
                                        </TableCell>
                                    </TableRow>
                                ))
                            ) : (
                                <TableRow>
                                    <TableCell colSpan={9} className="py-10 text-center text-muted-foreground">
                                        {t('common.noResult')}
                                    </TableCell>
                                </TableRow>
                            )}
                        </TableBody>
                    </Table>
                </div>

                <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                    <div className="text-sm text-muted-foreground">
                        {isFetching && !isLoading ? t('common.loading') : t('consumptionRanking.summary', {
                            total: data?.total ?? 0,
                        })}
                    </div>
                    <ServerPagination
                        page={page}
                        pageSize={pageSize}
                        total={data?.total ?? 0}
                        onPageChange={setPage}
                        onPageSizeChange={(size) => {
                            setPageSize(size)
                            setPage(1)
                        }}
                    />
                </div>
            </CardContent>
        </Card>
    )
}
