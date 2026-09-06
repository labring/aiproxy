import React, { useMemo, useState, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import {
    createColumnHelper,
    flexRender,
    getCoreRowModel,
    useReactTable,
} from '@tanstack/react-table'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { format } from 'date-fns'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ServerPagination } from '@/components/table/server-pagination'
import { ExpandedLogContent } from './ExpandedLogContent'
import { toast } from 'sonner'
import type { LogRecord } from '@/types/log'
import { writeTextToClipboard } from '@/lib/clipboard'
import { ChannelLabel } from '@/components/common/ChannelLabel'
import { useChannelInfoMap, useChannelTypeMetas } from '@/feature/channel/hooks'

const columnHelper = createColumnHelper<LogRecord>()

// 点击 group/token_name 时不展开行的列 ID
const NON_EXPAND_COLUMNS = new Set(['details', 'group', 'token_name', 'model'])

interface LogTableProps {
    data: LogRecord[]
    total: number
    loading?: boolean
    page: number
    pageSize: number
    onPageChange: (page: number) => void
    onPageSizeChange: (pageSize: number) => void
    onOpenGroupLog?: (group: string, tokenName?: string) => void
}

// 使用一个单独的组件来处理每行的展开内容，这样每一行都有自己的state
export function LogTable({
    data,
    total,
    loading = false,
    page,
    pageSize,
    onPageChange,
    onPageSizeChange,
    onOpenGroupLog,
}: LogTableProps) {
    const { t } = useTranslation()
    const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set())
    const channelIds = useMemo(
        () => [...new Set(data.map(log => log.channel).filter(id => id > 0))].sort((a, b) => a - b),
        [data],
    )
    const { data: channelInfoMap } = useChannelInfoMap(channelIds)
    const { data: typeMetas } = useChannelTypeMetas()

    const toggleRowExpansion = (rowId: number) => {
        const newExpanded = new Set(expandedRows)
        if (newExpanded.has(rowId)) {
            newExpanded.delete(rowId)
        } else {
            newExpanded.add(rowId)
        }
        setExpandedRows(newExpanded)
    }

    const copyToClipboard = useCallback((text: string) => {
        writeTextToClipboard(text).then(() => {
            toast.success(t('common.copied'))
        }).catch(() => {
            toast.error(t('common.copyFailed'))
        })
    }, [t])

    const clickableCell = 'block max-w-48 truncate text-left cursor-pointer hover:text-primary hover:underline underline-offset-4 transition-colors'

    const columns = useMemo(
        () => [
            columnHelper.display({
                id: 'details',
                header: '',
                cell: ({ row }) => (
                    <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => toggleRowExpansion(row.original.id)}
                        className="h-8 w-8 p-0"
                        aria-label={t('log.basicInfo')}
                        aria-expanded={expandedRows.has(row.original.id)}
                    >
                        {expandedRows.has(row.original.id) ? (
                            <ChevronDown className="h-4 w-4" />
                        ) : (
                            <ChevronRight className="h-4 w-4" />
                        )}
                    </Button>
                ),
                size: 40,
            }),
            columnHelper.accessor('channel', {
                header: t('log.channel'),
                cell: (info) => {
                    const id = info.getValue()
                    if (!id) return <span className="text-muted-foreground">-</span>
                    const channelInfo = channelInfoMap?.[id]
                    return (
                        <ChannelLabel
                            id={id}
                            info={channelInfo}
                            typeName={channelInfo ? typeMetas?.[channelInfo.type]?.name : undefined}
                            compact
                            className="max-w-full flex-wrap"
                        />
                    )
                },
                size: 240,
            }),
            columnHelper.accessor('group', {
                header: t('log.group'),
                cell: (info) => {
                    const value = info.getValue()
                    if (!value) return <div className="text-sm text-muted-foreground">-</div>
                    return (
                        <div
                            className={`text-sm font-medium ${clickableCell}`}
                            title={value}
                            onClick={() => onOpenGroupLog?.(value)}
                        >
                            {value}
                        </div>
                    )
                },
                size: 160,
            }),
            columnHelper.accessor('token_name', {
                header: t('log.keyName'),
                cell: (info) => {
                    const value = info.getValue()
                    const group = info.row.original.group
                    if (!value) return <div className="font-medium text-muted-foreground">-</div>
                    return (
                        <div
                            className={`font-medium ${clickableCell}`}
                            title={value}
                            onClick={() => group && onOpenGroupLog?.(group, value)}
                        >
                            {value}
                        </div>
                    )
                },
                size: 150,
            }),
            columnHelper.accessor('model', {
                header: t('log.model'),
                cell: (info) => {
                    const value = info.getValue()
                    if (!value) return <div className="font-mono text-sm text-muted-foreground">-</div>
                    return (
                        <div
                            className={`font-mono text-sm ${clickableCell}`}
                            title={value}
                            onClick={() => copyToClipboard(value)}
                        >
                            {value}
                        </div>
                    )
                },
                size: 180,
            }),
            columnHelper.display({
                id: 'input_tokens',
                header: () => <span title={t('log.inputTokens')}>{t('ui.inputTokens')}</span>,
                cell: ({ row }) => (
                    <div className="text-right font-mono">
                        {row.original.usage?.input_tokens?.toLocaleString() || 0}
                    </div>
                ),
                size: 100,
            }),
            columnHelper.display({
                id: 'output_tokens',
                header: () => <span title={t('log.outputTokens')}>{t('ui.outputTokens')}</span>,
                cell: ({ row }) => (
                    <div className="text-right font-mono">
                        {row.original.usage?.output_tokens?.toLocaleString() || 0}
                    </div>
                ),
                size: 100,
            }),
            columnHelper.display({
                id: 'duration',
                header: t('log.duration'),
                cell: ({ row }) => {
                    if (!row.original.request_at || !row.original.created_at) {
                        return (
                            <div className="text-right font-mono">
                                -
                            </div>
                        )
                    }
                    const requestAt = new Date(row.original.request_at)
                    const createdAt = new Date(row.original.created_at)
                    const duration = (createdAt.getTime() - requestAt.getTime()) / 1000
                    return (
                        <div className="text-right font-mono">
                            {duration.toFixed(2)}s
                        </div>
                    )
                },
                size: 80,
            }),
            columnHelper.display({
                id: 'used_amount',
                header: t('log.usedAmount'),
                cell: ({ row }) => (
                    <div className="text-right font-mono">
                        ${Number(row.original.amount?.used_amount ?? row.original.used_amount ?? 0).toFixed(4)}
                    </div>
                ),
                size: 100,
            }),
            columnHelper.accessor('code', {
                header: t('log.state'),
                cell: (info) => {
                    const code = info.getValue()
                    const isSuccess = code === 200
                    return (
                        <div className="flex items-center gap-2">
                            <Badge
                                variant={isSuccess ? 'secondary' : 'destructive'}
                                className={isSuccess ? 'bg-green-100 text-green-800 border-green-200 dark:bg-green-900/20 dark:text-green-400 dark:border-green-800' : ''}
                            >
                                {isSuccess ? t('log.success') : t('log.failed')}
                            </Badge>
                            <span className="font-mono text-xs text-muted-foreground">
                                {code || '-'}
                            </span>
                        </div>
                    )
                },
                size: 80,
            }),
            columnHelper.accessor('created_at', {
                header: t('log.time'),
                cell: (info) => (
                    <div className="text-sm text-muted-foreground">
                        {info.getValue() ? format(new Date(info.getValue()), 'yyyy-MM-dd HH:mm:ss') : '-'}
                    </div>
                ),
                size: 140,
            }),
        ],
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [t, expandedRows, onOpenGroupLog, copyToClipboard, channelInfoMap, typeMetas]
    )

    const table = useReactTable({
        data: data || [],
        columns,
        getCoreRowModel: getCoreRowModel(),
        manualPagination: true,
        pageCount: Math.ceil(total / pageSize),
    })

    return (
        <div className="h-full flex flex-col">
            <div className="flex-1 min-h-0">
                <div className="h-full overflow-hidden border-y bg-card">
                    <div className="overflow-auto h-full">
                        <table className="w-full min-w-[1240px] table-fixed tabular-nums">
                            <thead className="sticky top-0 z-10 bg-muted">
                                <tr className="border-b border-border">
                                    {table.getHeaderGroups().map((headerGroup) =>
                                        headerGroup.headers.map((header, index) => (
                                            <th
                                                key={header.id}
                                                className={`px-4 py-2 text-left text-xs font-medium text-muted-foreground ${
                                                    index === 0 ? 'rounded-tl-lg' : ''
                                                } ${
                                                    index === headerGroup.headers.length - 1 ? 'rounded-tr-lg' : ''
                                                }`}
                                                style={{ width: header.getSize() }}
                                            >
                                                {header.isPlaceholder
                                                    ? null
                                                    : flexRender(
                                                        header.column.columnDef.header,
                                                        header.getContext()
                                                    )}
                                            </th>
                                        ))
                                    )}
                                </tr>
                            </thead>
                            <tbody>
                                {loading ? (
                                    <tr>
                                        <td colSpan={columns.length} className="px-4 py-8 text-center">
                                            <div className="flex items-center justify-center">
                                                <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary"></div>
                                                <span className="ml-2">{t('common.loading')}</span>
                                            </div>
                                        </td>
                                    </tr>
                                ) : data.length === 0 ? (
                                    <tr>
                                        <td colSpan={columns.length} className="px-4 py-8 text-center text-muted-foreground">
                                            {t('common.noResult')}
                                        </td>
                                    </tr>
                                ) : (
                                    table.getRowModel().rows.map((row) => (
                                        <React.Fragment key={row.original.id}>
                                            <tr
                                                className="border-b border-border hover:bg-muted/50 transition-colors cursor-pointer"
                                                onClick={(e) => {
                                                    // 找到点击所在的 td，获取对应的列 ID
                                                    const td = (e.target as HTMLElement).closest('td')
                                                    if (!td) return
                                                    const cellIndex = Array.from(td.parentElement!.children).indexOf(td)
                                                    const columnId = row.getVisibleCells()[cellIndex]?.column.id
                                                    // 非特殊列点击展开行
                                                    if (!NON_EXPAND_COLUMNS.has(columnId)) {
                                                        toggleRowExpansion(row.original.id)
                                                    }
                                                }}
                                            >
                                                {row.getVisibleCells().map((cell) => (
                                                    <td
                                                        key={cell.id}
                                                        className="px-4 py-2.5 text-sm"
                                                        style={{ width: cell.column.getSize() }}
                                                    >
                                                        {flexRender(
                                                            cell.column.columnDef.cell,
                                                            cell.getContext()
                                                        )}
                                                    </td>
                                                ))}
                                            </tr>
                                            {expandedRows.has(row.original.id) && (
                                                <tr>
                                                    <td colSpan={columns.length} className="p-0">
                                                        <ExpandedLogContent
                                                            log={row.original}
                                                            channelInfo={channelInfoMap?.[row.original.channel]}
                                                            typeMetas={typeMetas}
                                                        />
                                                    </td>
                                                </tr>
                                            )}
                                        </React.Fragment>
                                    ))
                                )}
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>

            <ServerPagination page={page} pageSize={pageSize} total={total} onPageChange={onPageChange} onPageSizeChange={onPageSizeChange} pageSizes={[10, 20, 30, 40, 50]} />
        </div>
    )
}
