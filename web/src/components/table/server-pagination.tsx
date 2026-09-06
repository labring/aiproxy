import {
    ChevronLeft,
    ChevronRight,
    ChevronsLeft,
    ChevronsRight,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { useTranslation } from "react-i18next"

interface ServerPaginationProps {
    page: number
    pageSize: number
    total: number
    pageSizes?: number[]
    onPageChange: (page: number) => void
    onPageSizeChange: (pageSize: number) => void
}

export function ServerPagination({
    page,
    pageSize,
    total,
    pageSizes = [10, 20, 30, 50],
    onPageChange,
    onPageSizeChange,
}: ServerPaginationProps) {
    const { t } = useTranslation()
    const totalPages = Math.ceil(total / pageSize) || 1

    return (
        <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 py-3 text-xs">
            <div className="text-xs tabular-nums text-muted-foreground">
                {t('table.pageInfo', {
                    current: page,
                    total: totalPages
                })}
            </div>
            <div className="flex items-center gap-3">
                <div className="flex items-center space-x-2">
                    <p className="hidden text-xs text-muted-foreground sm:block">{t('table.rowsPerPage')}</p>
                    <select
                        aria-label={t('table.rowsPerPage')}
                        value={pageSize}
                        onChange={(e) => onPageSizeChange(Number(e.target.value))}
                        className="h-8 max-w-[80px] rounded border border-input bg-background px-2 text-sm"
                    >
                        {pageSizes.map((size) => (
                            <option key={size} value={size}>
                                {size}
                            </option>
                        ))}
                    </select>
                </div>
                <div className="flex items-center space-x-2">
                    <Button
                        variant="outline"
                        className="hidden h-8 w-8 p-0 lg:flex"
                        aria-label={t('ui.firstPage')}
                        onClick={() => onPageChange(1)}
                        disabled={page <= 1}
                    >
                        <ChevronsLeft className="h-4 w-4" />
                    </Button>
                    <Button
                        variant="outline"
                        className="h-8 w-8 p-0"
                        aria-label={t('ui.previousPage')}
                        onClick={() => onPageChange(Math.max(1, page - 1))}
                        disabled={page <= 1}
                    >
                        <ChevronLeft className="h-4 w-4" />
                    </Button>
                    <Button
                        variant="outline"
                        className="h-8 w-8 p-0"
                        aria-label={t('ui.nextPage')}
                        onClick={() => onPageChange(Math.min(totalPages, page + 1))}
                        disabled={page >= totalPages}
                    >
                        <ChevronRight className="h-4 w-4" />
                    </Button>
                    <Button
                        variant="outline"
                        className="hidden h-8 w-8 p-0 lg:flex"
                        aria-label={t('ui.lastPage')}
                        onClick={() => onPageChange(totalPages)}
                        disabled={page >= totalPages}
                    >
                        <ChevronsRight className="h-4 w-4" />
                    </Button>
                </div>
            </div>
        </div>
    )
}
