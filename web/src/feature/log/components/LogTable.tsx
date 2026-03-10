import React, { useMemo, useState, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import {
    createColumnHelper,
    flexRender,
    getCoreRowModel,
    useReactTable,
} from '@tanstack/react-table'
import { ChevronDown, ChevronRight, ChevronLeft, ChevronsLeft, ChevronsRight } from 'lucide-react'
import { format } from 'date-fns'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ExpandedLogContent } from './ExpandedLogContent'
import { toast } from 'sonner'
import type { LogRecord } from '@/types/log'

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
        navigator.clipboard.writeText(text).then(() => {
            toast.success(t('common.copied'))
        }).catch(() => {
            toast.error(t('common.copyFailed'))
        })
    }, [t])

    const clickableCell = 'cursor-pointer hover:text-primary hover:underline underline-offset-4 transition-colors'

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
            columnHelper.accessor('group', {
                header: t('log.group'),
                cell: (info) => {
                    const value = info.getValue()
                    if (!value) return <div className="text-sm text-muted-foreground">-</div>
                    return (
                        <div
                            className={`text-sm font-medium ${clickableCell}`}
                            onClick={() => onOpenGroupLog?.(value)}
                        >
                            {value}
                        </div>
                    )
                },
                size: 100,
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
                            onClick={() => copyToClipboard(value)}
                        >
                            {value}
                        </div>
                    )
                },
                size: 120,
            }),
            columnHelper.display({
                id: 'input_tokens',
                header: t('log.inputTokens'),
                cell: ({ row }) => (
                    <div className="text-right font-mono">
                        {row.original.usage?.input_tokens?.toLocaleString() || 0}
                    </div>
                ),
                size: 100,
            }),
            columnHelper.display({
                id: 'output_tokens',
                header: t('log.outputTokens'),
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
                        <Badge
                            variant={isSuccess ? 'secondary' : 'destructive'}
                            className={isSuccess ? 'bg-green-100 text-green-800 border-green-200 dark:bg-green-900/20 dark:text-green-400 dark:border-green-800' : ''}
                        >
                            {isSuccess ? t('log.success') : t('log.failed')}
                        </Badge>
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
        [t, expandedRows, onOpenGroupLog, copyToClipboard]
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
                <div className="rounded-lg border border-border bg-card shadow-none h-full overflow-hidden">
                    <div className="overflow-auto h-full">
                        <table className="w-full table-fixed">
                            <thead className="sticky top-0 bg-muted/50 backdrop-blur-sm">
                                <tr className="border-b border-border">
                                    {table.getHeaderGroups().map((headerGroup) =>
                                        headerGroup.headers.map((header, index) => (
                                            <th
                                                key={header.id}
                                                className={`px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider ${
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
                                                        className="px-4 py-3 text-sm"
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
                                                        <ExpandedLogContent log={row.original} />
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

            {/* 分页控制 - 固定在底部 */}
            <div className="flex-shrink-0 pt-4">
                <div className="flex items-center justify-between px-2">
                    <div className="flex-1 text-sm text-muted-foreground">
                        {t('table.pageInfo', {
                            current: page,
                            total: Math.ceil(total / pageSize) || 1
                        })}
                    </div>
                    <div className="flex items-center space-x-6 lg:space-x-8">
                        <div className="flex items-center space-x-2">
                            <p className="text-sm font-medium">{t('table.rowsPerPage')}</p>
                            <select
                                value={pageSize}
                                onChange={(e) => onPageSizeChange(Number(e.target.value))}
                                className="h-8 max-w-[80px] rounded border border-input bg-background px-2 text-sm"
                            >
                                {[10, 20, 30, 40, 50].map((size) => (
                                    <option key={size} value={size}>
                                        {size}
                                    </option>
                                ))}
                            </select>
                        </div>
                        <div className="flex items-center space-x-2">
                            <Button
                                variant="outline"
                                className="h-8 w-8 p-0"
                                onClick={() => onPageChange(1)}
                                disabled={page <= 1}
                            >
                                <ChevronsLeft className="h-4 w-4" />
                            </Button>
                            <Button
                                variant="outline"
                                className="h-8 w-8 p-0"
                                onClick={() => onPageChange(Math.max(1, page - 1))}
                                disabled={page <= 1}
                            >
                                <ChevronLeft className="h-4 w-4" />
                            </Button>
                            <Button
                                variant="outline"
                                className="h-8 w-8 p-0"
                                onClick={() => onPageChange(Math.min(Math.ceil(total / pageSize), page + 1))}
                                disabled={page >= Math.ceil(total / pageSize)}
                            >
                                <ChevronRight className="h-4 w-4" />
                            </Button>
                            <Button
                                variant="outline"
                                className="h-8 w-8 p-0"
                                onClick={() => onPageChange(Math.ceil(total / pageSize))}
                                disabled={page >= Math.ceil(total / pageSize)}
                            >
                                <ChevronsRight className="h-4 w-4" />
                            </Button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}
