import { useMemo, useState } from 'react'
import type { DateRange } from 'react-day-picker'
import { useTranslation } from 'react-i18next'
import {
    BarChart3,
    ChevronDown,
    ChevronUp,
    Eye,
    RotateCcw
} from 'lucide-react'

import { useGroupConsumptionRanking } from '../hooks'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
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
import { DEFAULT_TIMEZONE, formatRangeTimestamp, zonedBoundaryToUnix } from '@/utils/timezone'
import { useGroupSummaryMetrics } from '@/feature/monitor/runtime-hooks'

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

const formatTokens = (tokens: number): string => {
    return tokens.toLocaleString()
}


interface GroupRankingPanelProps {
    onViewGroup: (groupId: string) => void
}

export function GroupRankingPanel({ onViewGroup }: GroupRankingPanelProps) {
    const { t } = useTranslation()
    const [open, setOpen] = useState(true)
    const [dateRange, setDateRange] = useState<DateRange | undefined>(getDefaultDateRange())
    const [timezone, setTimezone] = useState(DEFAULT_TIMEZONE)
    const [page, setPage] = useState(1)
    const [pageSize, setPageSize] = useState(10)

    const query = useMemo(() => {
        const nextQuery = {
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
    }, [dateRange, page, pageSize, timezone])

    const { data, isLoading, isFetching } = useGroupConsumptionRanking(query)
    const currentPageGroupIds = useMemo(
        () => (data?.items || []).map((item) => item.group_id).filter(Boolean),
        [data?.items],
    )
    const { data: runtimeMetrics } = useGroupSummaryMetrics(
        {
            groups: currentPageGroupIds,
        },
        currentPageGroupIds.length > 0,
    )
    const effectiveTimezone = query.timezone || DEFAULT_TIMEZONE

    const handleDateRangeChange = (nextRange: DateRange | undefined) => {
        setDateRange(nextRange)
        setPage(1)
    }

    const handleTimezoneChange = (value: string) => {
        setTimezone(value)
        setPage(1)
    }

    const handleReset = () => {
        setDateRange(getDefaultDateRange())
        setTimezone(DEFAULT_TIMEZONE)
        setPage(1)
        setPageSize(10)
    }

    return (
        <Card className="mb-6 gap-0">
            <Collapsible open={open} onOpenChange={setOpen}>
                <CardHeader className="gap-4">
                    <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                        <div className="space-y-1.5">
                            <div className="flex items-center gap-2">
                                <BarChart3 className="h-5 w-5 text-primary dark:text-[#6A6DE6]" />
                                <CardTitle>{t('group.ranking.title')}</CardTitle>
                            </div>
                            <CardDescription>{t('group.ranking.description')}</CardDescription>
                            {query.start_timestamp && query.end_timestamp && (
                                <div className="text-xs text-muted-foreground">
                                    {t('group.ranking.range', {
                                        start: formatRangeTimestamp(query.start_timestamp, effectiveTimezone),
                                        end: formatRangeTimestamp(query.end_timestamp, effectiveTimezone),
                                        timezone: effectiveTimezone,
                                    })}
                                </div>
                            )}
                        </div>

                        <div className="flex flex-wrap items-center gap-2">
                            <div className="w-full sm:w-64">
                                <DateRangePicker
                                    value={dateRange}
                                    onChange={handleDateRangeChange}
                                    placeholder={t('group.ranking.dateRangePlaceholder')}
                                    className="h-9"
                                />
                            </div>
                            <TimezoneInput
                                value={timezone}
                                onChange={handleTimezoneChange}
                            />
                            <Button
                                type="button"
                                variant="outline"
                                onClick={handleReset}
                                className="h-9 px-3"
                            >
                                <RotateCcw className="mr-1.5 h-4 w-4" />
                                {t('monitor.filters.reset')}
                            </Button>
                            <CollapsibleTrigger asChild>
                                <Button type="button" variant="ghost" size="sm" className="h-9 px-3">
                                    {open ? (
                                        <ChevronUp className="mr-1.5 h-4 w-4" />
                                    ) : (
                                        <ChevronDown className="mr-1.5 h-4 w-4" />
                                    )}
                                    {open ? t('group.ranking.collapse') : t('group.ranking.expand')}
                                </Button>
                            </CollapsibleTrigger>
                        </div>
                    </div>
                </CardHeader>

                <CollapsibleContent>
                    <CardContent className="space-y-4">
                        <div className="rounded-lg border">
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead className="w-16">#</TableHead>
                                        <TableHead>{t('group.ranking.group')}</TableHead>
                                        <TableHead>{t('group.ranking.requestCount')}</TableHead>
                                        <TableHead>{t('common.runtime')}</TableHead>
                                        <TableHead>{t('group.ranking.totalTokens')}</TableHead>
                                        <TableHead className="text-right">{t('group.ranking.usedAmount')}</TableHead>
                                        <TableHead className="w-28 text-right">{t('group.ranking.actions')}</TableHead>
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
                                                <TableCell className="text-right"><Skeleton className="ml-auto h-4 w-20" /></TableCell>
                                                <TableCell className="text-right"><Skeleton className="ml-auto h-8 w-20" /></TableCell>
                                            </TableRow>
                                        ))
                                    ) : data?.items?.length ? (
                                        data.items.map(item => (
                                            <TableRow key={item.group_id}>
                                                <TableCell className="font-medium">{item.rank}</TableCell>
                                                <TableCell>
                                                    <button
                                                        type="button"
                                                        className="font-medium text-left text-primary hover:underline dark:text-[#8B8DFF]"
                                                        onClick={() => onViewGroup(item.group_id)}
                                                    >
                                                        {item.group_id}
                                                    </button>
                                                </TableCell>
                                                <TableCell className="font-mono">{item.request_count.toLocaleString()}</TableCell>
                                                <TableCell>
                                                    {runtimeMetrics?.groups?.[item.group_id] ? (
                                                        <div className="flex flex-wrap gap-1">
                                                            <Badge variant="outline" className="text-xs">
                                                                RPM {runtimeMetrics.groups[item.group_id].rpm.toLocaleString()}
                                                            </Badge>
                                                            <Badge variant="outline" className="text-xs">
                                                                TPM {runtimeMetrics.groups[item.group_id].tpm.toLocaleString()}
                                                            </Badge>
                                                        </div>
                                                    ) : (
                                                        <div className="text-muted-foreground text-sm">-</div>
                                                    )}
                                                </TableCell>
                                                <TableCell className="font-mono">{formatTokens(item.total_tokens)}</TableCell>
                                                <TableCell className="text-right font-mono">{formatAmount(item.used_amount)}</TableCell>
                                                <TableCell className="text-right">
                                                    <Button
                                                        type="button"
                                                        variant="ghost"
                                                        size="sm"
                                                        onClick={() => onViewGroup(item.group_id)}
                                                        className="h-8 px-2"
                                                    >
                                                        <Eye className="mr-1.5 h-4 w-4" />
                                                        {t('group.ranking.view')}
                                                    </Button>
                                                </TableCell>
                                            </TableRow>
                                        ))
                                    ) : (
                                        <TableRow>
                                            <TableCell colSpan={7} className="py-10 text-center text-muted-foreground">
                                                {t('common.noResult')}
                                            </TableCell>
                                        </TableRow>
                                    )}
                                </TableBody>
                            </Table>
                        </div>

                        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                            <div className="text-sm text-muted-foreground">
                                {isFetching && !isLoading ? t('common.loading') : t('group.ranking.summary', {
                                    total: data?.total ?? 0,
                                })}
                            </div>
                            <div className="w-full lg:w-auto lg:min-w-[420px]">
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
                        </div>
                    </CardContent>
                </CollapsibleContent>
            </Collapsible>
        </Card>
    )
}
