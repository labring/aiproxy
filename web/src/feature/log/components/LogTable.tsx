import React, { useMemo, useState } from 'react'
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
import { Separator } from '@/components/ui/separator'
import { JsonViewer } from './JsonViewer'
import type { LogRecord } from '@/types/log'

const columnHelper = createColumnHelper<LogRecord>()

interface LogTableProps {
    data: LogRecord[]
    total: number
    loading?: boolean
    page: number
    pageSize: number
    onPageChange: (page: number) => void
    onPageSizeChange: (pageSize: number) => void
}

export function LogTable({
    data,
    total,
    loading = false,
    page,
    pageSize,
    onPageChange,
    onPageSizeChange,
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
            columnHelper.accessor('token_name', {
                header: t('log.keyName'),
                cell: (info) => (
                    <div className="font-medium text-foreground">
                        {info.getValue() || '-'}
                    </div>
                ),
                size: 150,
            }),
            columnHelper.accessor('model', {
                header: t('log.model'),
                cell: (info) => (
                    <div className="font-mono text-sm">
                        {info.getValue() || '-'}
                    </div>
                ),
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
        [t, expandedRows]
    )

    const table = useReactTable({
        data: data || [],
        columns,
        getCoreRowModel: getCoreRowModel(),
        manualPagination: true,
        pageCount: Math.ceil(total / pageSize),
    })

    const renderExpandedContent = (log: LogRecord) => {
        return (
            <div className="p-4 space-y-4 bg-muted/50 border-t">
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {/* 基本信息 */}
                    <div className="space-y-2">
                        <h4 className="font-semibold text-sm">{t('log.basicInfo')}</h4>
                        <div className="space-y-1 text-sm">
                            <div><span className="font-medium">{t('log.id')}:</span> {log.id}</div>
                            <div><span className="font-medium">{t('log.requestId')}:</span> {log.request_id}</div>
                            <div><span className="font-medium">{t('log.channel')}:</span> {log.channel}</div>
                            <div><span className="font-medium">{t('log.user')}:</span> {log.user || '-'}</div>
                            <div><span className="font-medium">{t('log.ip')}:</span> {log.ip}</div>
                            <div><span className="font-medium">{t('log.endpoint')}:</span> {log.endpoint}</div>
                        </div>
                    </div>

                    {/* Token信息 */}
                    <div className="space-y-2">
                        <h4 className="font-semibold text-sm">{t('log.tokenInfo')}</h4>
                        <div className="space-y-1 text-sm">
                            <div><span className="font-medium">{t('log.cacheCreation')}:</span> {log.usage?.cache_creation_tokens || 0}</div>
                            <div><span className="font-medium">{t('log.cached')}:</span> {log.usage?.cached_tokens || 0}</div>
                            <div><span className="font-medium">{t('log.imageInput')}:</span> {log.usage?.image_input_tokens || 0}</div>
                            <div><span className="font-medium">{t('log.reasoning')}:</span> {log.usage?.reasoning_tokens || 0}</div>
                            <div><span className="font-medium">{t('log.total')}:</span> {log.usage?.total_tokens || 0}</div>
                            <div><span className="font-medium">{t('log.webSearchCount')}:</span> {log.usage?.web_search_count || 0}</div>
                        </div>
                    </div>

                    {/* 时间信息 */}
                    <div className="space-y-2">
                        <h4 className="font-semibold text-sm">{t('log.timeInfo')}</h4>
                        <div className="space-y-1 text-sm">
                            <div><span className="font-medium">{t('log.created')}:</span> {log.created_at ? format(new Date(log.created_at), 'yyyy-MM-dd HH:mm:ss') : '-'}</div>
                            <div><span className="font-medium">{t('log.request')}:</span> {log.request_at ? format(new Date(log.request_at), 'yyyy-MM-dd HH:mm:ss') : '-'}</div>
                            {log.retry_at && <div><span className="font-medium">{t('log.retry')}:</span> {format(new Date(log.retry_at), 'yyyy-MM-dd HH:mm:ss')}</div>}
                            <div><span className="font-medium">{t('log.retryTimes')}:</span> {log.retry_times || 0}</div>
                            <div><span className="font-medium">{t('log.ttfb')}:</span> {log.ttfb_milliseconds || 0}ms</div>
                        </div>
                    </div>
                </div>

                <Separator />

                {/* 请求和响应内容 */}
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                    <div>
                        <h4 className="font-semibold text-sm mb-2">{t('log.requestBody')}</h4>
                        {log.request_detail?.request_body ? (
                            <JsonViewer
                                src={log.request_detail.request_body}
                                collapsed={1}
                                name="request"
                            />
                        ) : (
                            <div className="text-sm text-muted-foreground p-2 border rounded">
                                {t('log.noRequestBody')}
                            </div>
                        )}
                        {log.request_detail?.request_body_truncated && (
                            <div className="text-xs text-amber-600 mt-1">⚠️ {t('log.contentTruncated')}</div>
                        )}
                    </div>
                    <div>
                        <h4 className="font-semibold text-sm mb-2">{t('log.responseBody')}</h4>
                        {log.request_detail?.response_body ? (
                            <JsonViewer
                                src={log.request_detail.response_body}
                                collapsed={1}
                                name="response"
                            />
                        ) : (
                            <div className="text-sm text-muted-foreground p-2 border rounded">
                                {t('log.noResponseBody')}
                            </div>
                        )}
                        {log.request_detail?.response_body_truncated && (
                            <div className="text-xs text-amber-600 mt-1">⚠️ {t('log.contentTruncated')}</div>
                        )}
                    </div>
                </div>
            </div>
        )
    }

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
                                            <tr className="border-b border-border hover:bg-muted/50 transition-colors">
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
                                                        {renderExpandedContent(row.original)}
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