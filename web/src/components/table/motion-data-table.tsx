import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { flexRender, Table as TableType, ColumnDef } from "@tanstack/react-table"
import { Loader2 } from "lucide-react"
import { useTranslation } from "react-i18next"
import { cn } from "@/lib/utils"
import { TableScrollContainer } from "@/components/ui/animation/components/table-scroll"
import { useEffect, useRef, useState, useMemo } from "react"

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

// 加载状态骨架屏组件
const TableSkeleton = <TData, TValue>({
    columns,
    rows = 5
}: {
    columns: ColumnDef<TData, TValue>[],
    rows?: number
}) => (
    <>
        {Array.from({ length: rows }).map((_, index) => (
            <TableRow key={`skeleton-row-${index}`} className="animate-pulse">
                {Array.from({ length: columns.length }).map((_, cellIndex) => (
                    <TableCell key={`skeleton-cell-${index}-${cellIndex}`}>
                        <div className="h-4 bg-gray-200 rounded w-3/4 dark:bg-gray-700"></div>
                    </TableCell>
                ))}
            </TableRow>
        ))}
    </>
)

// 中心加载动画组件
const CenteredLoader = <TData, TValue>({
    columns
}: {
    columns: ColumnDef<TData, TValue>[]
}) => {
    const { t } = useTranslation()
    return (
        <TableRow>
            <TableCell colSpan={columns.length} className="h-24">
                <div className="flex items-center justify-center space-x-2">
                    <Loader2 className="h-6 w-6 animate-spin text-primary" />
                    <span className="text-sm text-muted-foreground">{t("common.loading")}</span>
                </div>
            </TableCell>
        </TableRow>
    )
}

// 无数据状态组件
const NoResults = <TData, TValue>({
    columns
}: {
    columns: ColumnDef<TData, TValue>[]
}) => {
    const { t } = useTranslation()
    return (
        <TableRow>
            <TableCell colSpan={columns.length} className="h-24 text-center">
                {t('common.noResult')}
            </TableCell>
        </TableRow>
    )
}

export function DataTable<TData, TValue>({
    table,
    columns,
    style = 'default',
    isLoading = false,
    loadingRows = 5,
    loadingStyle = 'centered',
    fixedHeader = false,
    animatedRows = false,
    showScrollShadows = true,
    onRowClick,
}: DataTableProps<TData, TValue>) {
    // 用于跟踪已渲染行的ref
    const rowsRef = useRef<HTMLElement[]>([])
    const [inViewRows, setInViewRows] = useState<Set<string>>(new Set())
    const observerRef = useRef<IntersectionObserver | null>(null)

    // 提取复杂表达式为变量
    const tableRows = table.getRowModel().rows
    const tableRowCount = tableRows.length

    // 用数据指纹检测数据变化（分页切换、排序等），而不仅仅是行数变化
    const dataKey = useMemo(() => {
        if (tableRowCount === 0) return ''
        return `${tableRows[0].id}-${tableRows[tableRowCount - 1].id}-${tableRowCount}`
    }, [tableRows, tableRowCount])

    // 数据变化时重置已显示行集合并重建 IntersectionObserver
    // 必须在同一个 effect 中完成，否则 observer 不会重新触发已在视口中的元素
    useEffect(() => {
        if (!animatedRows) return

        setInViewRows(new Set())

        observerRef.current = new IntersectionObserver(
            (entries) => {
                const newIds: string[] = []
                entries.forEach(entry => {
                    if (entry.isIntersecting) {
                        const rowId = entry.target.getAttribute('data-row-id')
                        if (rowId) newIds.push(rowId)
                    }
                })
                if (newIds.length > 0) {
                    setInViewRows(prev => {
                        const toAdd = newIds.filter(id => !prev.has(id))
                        if (toAdd.length === 0) return prev
                        const updated = new Set(prev)
                        toAdd.forEach(id => updated.add(id))
                        return updated
                    })
                }
            },
            { threshold: 0.1 }
        )

        return () => {
            observerRef.current?.disconnect()
            observerRef.current = null
        }
    }, [dataKey, animatedRows])

    // 当行 DOM 元素挂载时，通过 ref callback 注册观察
    const observeRow = (el: HTMLElement | null, rowIndex: number) => {
        if (!el || !animatedRows) return
        rowsRef.current[rowIndex] = el
        observerRef.current?.observe(el)
    }

    // 渲染表格主体内容
    const renderTableBody = () => {
        if (isLoading) {
            // 根据 loadingStyle 选项决定使用哪种加载动画
            return loadingStyle === 'centered'
                ? <CenteredLoader<TData, TValue> columns={columns} />
                : <TableSkeleton<TData, TValue> columns={columns} rows={loadingRows} />
        }

        if (!table.getRowModel().rows?.length) {
            return <NoResults<TData, TValue> columns={columns} />
        }

        return table.getRowModel().rows.map((row, rowIndex) => {
            const isInView = inViewRows.has(row.id) || !animatedRows

            // 初始加载时前 15 行有交错延迟，滚动进入视口的行立即显示
            const staggerDelay = !isInView
                ? `${Math.min(rowIndex, 15) * 30}ms`
                : '0ms'

            return (
                <TableRow
                    key={row.id}
                    data-row-id={row.id}
                    data-state={row.getIsSelected() && "selected"}
                    ref={el => observeRow(el, rowIndex)}
                    className={cn(
                        animatedRows && "transition-opacity duration-300",
                        animatedRows && !isInView ? "opacity-0" : "opacity-100",
                        onRowClick && "cursor-pointer"
                    )}
                    style={{
                        transitionDelay: animatedRows ? staggerDelay : '0ms'
                    }}
                    onClick={onRowClick ? () => onRowClick(row.original) : undefined}
                >
                    {row.getVisibleCells().map((cell) => (
                        <TableCell
                            key={cell.id}
                            style={cell.column.columnDef.maxSize ? { width: cell.column.getSize(), maxWidth: cell.column.columnDef.maxSize, overflow: 'hidden' } : undefined}
                        >
                            {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </TableCell>
                    ))}
                </TableRow>
            )
        })
    }

    // 表头渲染函数
    const renderTableHeader = () => (
        <TableHeader className={fixedHeader ? "sticky top-0 z-10 bg-background border-b" : ""}>
            {table.getHeaderGroups().map((headerGroup) => (
                <TableRow key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
                        <TableHead
                            key={header.id}
                            style={header.column.columnDef.maxSize ? { width: header.getSize(), maxWidth: header.column.columnDef.maxSize } : undefined}
                        >
                            {header.isPlaceholder
                                ? null
                                : flexRender(
                                    header.column.columnDef.header,
                                    header.getContext()
                                )}
                        </TableHead>
                    ))}
                </TableRow>
            ))}
        </TableHeader>
    )

    // 使用滚动容器
    const renderScrollableTable = () => (
        <TableScrollContainer showShadows={showScrollShadows}>
            <table className="w-full caption-bottom text-sm">
                {renderTableHeader()}
                <tbody className={cn(
                    // 只有当isLoading为true且没有行数据时才移除最后一行的边框
                    (isLoading || !table.getRowModel().rows?.length) ? "[&_tr:last-child]:border-0" : ""
                )}>
                    {renderTableBody()}
                </tbody>
            </table>
        </TableScrollContainer>
    )

    // 根据样式选择和固定表头选项构建表格
    if (fixedHeader) {
        // 使用固定表头的布局结构
        return (
            <div className={cn(
                "w-full h-full relative",
                style === 'border' && "rounded-md border"
            )}>
                {renderScrollableTable()}
            </div>
        )
    }

    // 原始表格布局（无固定表头）
    switch (style) {
        case 'simple':
            return (
                <div className="w-full h-full">
                    <TableScrollContainer showShadows={showScrollShadows}>
                        <Table>
                            {renderTableHeader()}
                            <TableBody>
                                {renderTableBody()}
                            </TableBody>
                        </Table>
                    </TableScrollContainer>
                </div>
            )

        case 'border':
            return (
                <div className="rounded-md border h-full w-full">
                    <TableScrollContainer showShadows={showScrollShadows}>
                        <Table>
                            {renderTableHeader()}
                            <TableBody>
                                {renderTableBody()}
                            </TableBody>
                        </Table>
                    </TableScrollContainer>
                </div>
            )

        default:
            return (
                <div className="w-full h-full">
                    <TableScrollContainer showShadows={showScrollShadows}>
                        <Table>
                            {renderTableHeader()}
                            <TableBody className={isLoading || !table.getRowModel().rows?.length ? "[&_tr:last-child]:!border-b-0" : ""}>
                                {renderTableBody()}
                            </TableBody>
                        </Table>
                    </TableScrollContainer>
                </div>
            )
    }
}