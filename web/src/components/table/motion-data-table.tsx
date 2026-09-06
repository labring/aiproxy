import { useState } from 'react'
import {
    flexRender,
    type Table as TableType,
    type ColumnDef,
} from '@tanstack/react-table'
import { Columns3, Rows3, List, SearchX, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import {
    DropdownMenu,
    DropdownMenuCheckboxItem,
    DropdownMenuContent,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

interface DataTableProps<TData, TValue> {
    table: TableType<TData>
    columns: ColumnDef<TData, TValue>[]
    style?: 'default' | 'border' | 'simple'
    isLoading?: boolean
    loadingRows?: number
    loadingStyle?: 'centered' | 'skeleton'
    fixedHeader?: boolean
    animatedRows?: boolean
    showScrollShadows?: boolean
    onRowClick?: (row: TData) => void
}

export function DataTable<TData, TValue>({
    table,
    style = 'default',
    isLoading = false,
    loadingRows = 5,
    loadingStyle = 'centered',
    fixedHeader = false,
    onRowClick,
}: DataTableProps<TData, TValue>) {
    const { t } = useTranslation()
    const [compact, setCompact] = useState(false)
    const rows = table.getRowModel().rows
    const visibleCount = table.getVisibleLeafColumns().length
    const initialLoading = isLoading && rows.length === 0

    return (
        <div
            className={cn(
                'flex h-full min-h-0 min-w-0 flex-col',
                style === 'border' && 'rounded-lg border',
            )}
        >
            <div className="flex shrink-0 items-center justify-between gap-2 py-2">
                <span
                    className="text-xs tabular-nums text-muted-foreground"
                    role="status"
                >
                    {t('ui.visibleRows', { count: rows.length })}
                    {isLoading && (
                        <Loader2 className="ml-2 inline size-3 animate-spin" />
                    )}
                </span>
                <div className="flex items-center gap-1">
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="size-8"
                                aria-label={t('ui.compactRows')}
                                aria-pressed={compact}
                                onClick={() => setCompact(!compact)}
                            >
                                {compact ? <List /> : <Rows3 />}
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t('ui.compactRows')}</TooltipContent>
                    </Tooltip>
                    <DropdownMenu>
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <DropdownMenuTrigger asChild>
                                    <Button
                                        type="button"
                                        variant="ghost"
                                        size="icon"
                                        className="size-8"
                                        aria-label={t('ui.columns')}
                                    >
                                        <Columns3 />
                                    </Button>
                                </DropdownMenuTrigger>
                            </TooltipTrigger>
                            <TooltipContent>{t('ui.columns')}</TooltipContent>
                        </Tooltip>
                        <DropdownMenuContent
                            align="end"
                            className="max-h-80 overflow-y-auto"
                        >
                            <DropdownMenuLabel>
                                {t('ui.columns')}
                            </DropdownMenuLabel>
                            <DropdownMenuSeparator />
                            {table
                                .getAllLeafColumns()
                                .filter(
                                    (column) =>
                                        column.getCanHide() &&
                                        column.columnDef.header,
                                )
                                .map((column) => {
                                    const header = table
                                        .getFlatHeaders()
                                        .find(
                                            (item) =>
                                                item.column.id === column.id,
                                        )
                                    return (
                                        <DropdownMenuCheckboxItem
                                            key={column.id}
                                            checked={column.getIsVisible()}
                                            disabled={
                                                column.getIsVisible() &&
                                                visibleCount <= 2
                                            }
                                            onSelect={(event) =>
                                                event.preventDefault()
                                            }
                                            onCheckedChange={(value) =>
                                                column.toggleVisibility(value)
                                            }
                                        >
                                            <span className="[&_*]:!py-0 [&_button]:hidden">
                                                {header
                                                    ? flexRender(
                                                          column.columnDef
                                                              .header,
                                                          header.getContext(),
                                                      )
                                                    : column.id}
                                            </span>
                                        </DropdownMenuCheckboxItem>
                                    )
                                })}
                        </DropdownMenuContent>
                    </DropdownMenu>
                </div>
            </div>
            <div
                className="min-h-0 flex-1 overflow-auto border-y"
                aria-busy={initialLoading}
            >
                <table
                    className={cn(
                        'w-full caption-bottom text-sm tabular-nums',
                        compact ? '[&_td]:py-1.5' : '[&_td]:py-3',
                    )}
                >
                    <TableHeader
                        className={cn(
                            'bg-muted',
                            fixedHeader && 'sticky top-0 z-10',
                        )}
                    >
                        {table.getHeaderGroups().map((group) => (
                            <TableRow key={group.id}>
                                {group.headers.map((header) => (
                                    <TableHead
                                        key={header.id}
                                        scope="col"
                                        className="h-10 px-3 text-xs text-muted-foreground [&_div]:py-0"
                                        style={
                                            header.column.columnDef.maxSize
                                                ? {
                                                      width: header.getSize(),
                                                      maxWidth:
                                                          header.column
                                                              .columnDef
                                                              .maxSize,
                                                  }
                                                : undefined
                                        }
                                    >
                                        {header.isPlaceholder
                                            ? null
                                            : flexRender(
                                                  header.column.columnDef
                                                      .header,
                                                  header.getContext(),
                                              )}
                                    </TableHead>
                                ))}
                            </TableRow>
                        ))}
                    </TableHeader>
                    <TableBody>
                        {initialLoading ? (
                            loadingStyle === 'skeleton' ? (
                                Array.from(
                                    { length: loadingRows },
                                    (_, index) => (
                                        <TableRow key={index}>
                                            {table
                                                .getVisibleLeafColumns()
                                                .map((column) => (
                                                    <TableCell
                                                        key={column.id}
                                                        className="px-3"
                                                    >
                                                        <div className="h-4 w-3/4 animate-pulse rounded-sm bg-muted" />
                                                    </TableCell>
                                                ))}
                                        </TableRow>
                                    ),
                                )
                            ) : (
                                <TableRow>
                                    <TableCell
                                        colSpan={visibleCount}
                                        className="h-28 text-center"
                                    >
                                        <Loader2 className="mr-2 inline size-4 animate-spin" />
                                        {t('common.loading')}
                                    </TableCell>
                                </TableRow>
                            )
                        ) : rows.length ? (
                            rows.map((row) => (
                                <TableRow
                                    key={row.id}
                                    data-state={
                                        row.getIsSelected()
                                            ? 'selected'
                                            : undefined
                                    }
                                    className={cn(
                                        'hover:bg-accent/50 focus-visible:bg-accent outline-none',
                                        onRowClick && 'cursor-pointer',
                                    )}
                                    tabIndex={onRowClick ? 0 : undefined}
                                    onClick={
                                        onRowClick
                                            ? () => onRowClick(row.original)
                                            : undefined
                                    }
                                    onKeyDown={
                                        onRowClick
                                            ? (event) => {
                                                  if (
                                                      event.target ===
                                                          event.currentTarget &&
                                                      event.key === 'Enter'
                                                  )
                                                      onRowClick(row.original)
                                              }
                                            : undefined
                                    }
                                >
                                    {row.getVisibleCells().map((cell) => (
                                        <TableCell
                                            key={cell.id}
                                            className="px-3"
                                            style={
                                                cell.column.columnDef.maxSize
                                                    ? {
                                                          width: cell.column.getSize(),
                                                          maxWidth:
                                                              cell.column
                                                                  .columnDef
                                                                  .maxSize,
                                                          overflow: 'hidden',
                                                      }
                                                    : undefined
                                            }
                                        >
                                            {flexRender(
                                                cell.column.columnDef.cell,
                                                cell.getContext(),
                                            )}
                                        </TableCell>
                                    ))}
                                </TableRow>
                            ))
                        ) : (
                            <TableRow>
                                <TableCell
                                    colSpan={visibleCount}
                                    className="h-44 text-center"
                                >
                                    <SearchX className="mx-auto mb-3 size-6 text-muted-foreground/60" />
                                    <span className="text-sm text-muted-foreground">
                                        {t('common.noResult')}
                                    </span>
                                </TableCell>
                            </TableRow>
                        )}
                    </TableBody>
                </table>
            </div>
        </div>
    )
}
