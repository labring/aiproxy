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
import { cn } from "@/lib/utils" // 确保导入了工具函数，如果没有可以手动添加

interface DataTableProps<TData, TValue> {
    table: TableType<TData>
    columns: ColumnDef<TData, TValue>[]
    style?: 'default' | 'border' | 'simple'
    isLoading?: boolean
    loadingRows?: number
    loadingStyle?: 'centered' | 'skeleton'
    fixedHeader?: boolean
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
}) => (
    <TableRow>
        <TableCell colSpan={columns.length} className="h-24">
            <div className="flex items-center justify-center space-x-2">
                <Loader2 className="h-6 w-6 animate-spin text-primary" />
                <span className="text-sm text-muted-foreground">加载中...</span>
            </div>
        </TableCell>
    </TableRow>
)

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
}: DataTableProps<TData, TValue>) {
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

        return table.getRowModel().rows.map((row) => (
            <TableRow
                key={row.id}
                data-state={row.getIsSelected() && "selected"}
            >
                {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                ))}
            </TableRow>
        ))
    }

    // 表头渲染函数
    const renderTableHeader = () => (
        <TableHeader className={fixedHeader ? "sticky top-0 z-10 bg-background border-b" : ""}>
            {table.getHeaderGroups().map((headerGroup) => (
                <TableRow key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
                        <TableHead key={header.id}>
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

    // 根据样式选择和固定表头选项构建表格
    if (fixedHeader) {
        // 使用固定表头的布局结构
        return (
            <div className={cn(
                "w-full h-full relative",
                style === 'border' && "rounded-md border"
            )}>
                <div className="overflow-auto h-full">
                    <table className="w-full caption-bottom text-sm">
                        {renderTableHeader()}
                        <tbody className={cn(
                            // 只有当isLoading为true且没有行数据时才移除最后一行的边框
                            (isLoading || !table.getRowModel().rows?.length) ? "[&_tr:last-child]:border-0" : ""
                        )}>
                            {renderTableBody()}
                        </tbody>
                    </table>
                </div>
            </div>
        )
    }

    // 原始表格布局（无固定表头）
    switch (style) {
        case 'simple':
            return (
                <div className="w-full h-full">
                    <Table>
                        {renderTableHeader()}
                        <TableBody>
                            {renderTableBody()}
                        </TableBody>
                    </Table>
                </div>
            )

        case 'border':
            return (
                <div className="rounded-md border h-full w-full">
                    <Table>
                        {renderTableHeader()}
                        <TableBody>
                            {renderTableBody()}
                        </TableBody>
                    </Table>
                </div>
            )

        default:
            return (
                <div className="w-full h-full">
                    <Table>
                        {renderTableHeader()}
                        <TableBody className={isLoading || !table.getRowModel().rows?.length ? "[&_tr:last-child]:!border-b-0" : ""}>
                            {renderTableBody()}
                        </TableBody>
                    </Table>
                </div>
            )
    }
}