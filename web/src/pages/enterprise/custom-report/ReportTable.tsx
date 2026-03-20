import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Settings2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
    DropdownMenu,
    DropdownMenuCheckboxItem,
    DropdownMenuContent,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { CustomReportResponse } from "@/api/enterprise"
import { getLabel, formatCellValue, PERCENTAGE_FIELDS, COST_FIELDS, sortRowsByTime } from "./types"

// ─── Conditional coloring helpers ───────────────────────────────────────────

function getColumnRange(rows: Record<string, unknown>[], key: string): { min: number; max: number } | null {
    const values = rows.map((r) => Number(r[key])).filter((n) => !Number.isNaN(n))
    if (values.length === 0) return null
    const min = Math.min(...values)
    const max = Math.max(...values)
    if (min === max) return null
    return { min, max }
}

function getHeatColor(value: number, min: number, max: number, key: string): string | undefined {
    const ratio = (value - min) / (max - min)

    if (PERCENTAGE_FIELDS.has(key)) {
        // For rates: success_rate green→red inverted, error/throttle green→red normal
        if (key === "success_rate" || key === "cache_hit_rate") {
            // High is good: green
            const r = Math.round(255 - ratio * 200)
            const g = Math.round(55 + ratio * 200)
            return `rgba(${r}, ${g}, 100, 0.12)`
        }
        // High is bad: red
        const r = Math.round(55 + ratio * 200)
        const g = Math.round(255 - ratio * 200)
        return `rgba(${r}, ${g}, 100, 0.12)`
    }

    if (COST_FIELDS.has(key)) {
        // Purple gradient for cost
        return `rgba(106, 109, 230, ${0.05 + ratio * 0.15})`
    }

    // Default: neutral blue gradient
    return `rgba(59, 130, 246, ${0.04 + ratio * 0.12})`
}

// ─── ReportTable ────────────────────────────────────────────────────────────

export function ReportTable({
    data,
    dimensions,
    sortBy,
    sortOrder,
    onSort,
}: {
    data: CustomReportResponse
    dimensions: string[]
    sortBy: string | undefined
    sortOrder: "asc" | "desc"
    onSort: (key: string, order: "asc" | "desc") => void
}) {
    const { i18n, t } = useTranslation()
    const lang = i18n.language

    // Sort rows by time dimension if present (when no explicit sort applied)
    const rows = sortBy ? data.rows : sortRowsByTime(data.rows, dimensions)

    // Column visibility: dimension columns always visible
    const [hiddenColumns, setHiddenColumns] = useState<Set<string>>(new Set())

    const dimensionSet = new Set(dimensions)
    const visibleColumns = data.columns.filter((col) => !hiddenColumns.has(col.key))

    // Precompute column ranges for heat coloring
    const columnRanges = new Map<string, { min: number; max: number }>()
    for (const col of data.columns) {
        if (dimensionSet.has(col.key)) continue
        const range = getColumnRange(rows, col.key)
        if (range) columnRanges.set(col.key, range)
    }

    const toggleColumn = (key: string) => {
        setHiddenColumns((prev) => {
            const next = new Set(prev)
            if (next.has(key)) next.delete(key)
            else next.add(key)
            return next
        })
    }

    const handleSort = (key: string) => {
        const newOrder = sortBy === key && sortOrder === "desc" ? "asc" : "desc"
        onSort(key, newOrder)
    }

    return (
        <div>
            {/* Column visibility control */}
            <div className="flex justify-end px-4 py-2 border-b">
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button variant="outline" size="sm" className="h-7 text-xs gap-1.5">
                            <Settings2 className="w-3.5 h-3.5" />
                            {t("enterprise.customReport.columnVisibility")}
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-[200px]">
                        <DropdownMenuLabel className="text-xs">
                            {t("enterprise.customReport.columnVisibility")}
                        </DropdownMenuLabel>
                        <DropdownMenuSeparator />
                        {data.columns.map((col) => {
                            const isDimension = dimensionSet.has(col.key)
                            return (
                                <DropdownMenuCheckboxItem
                                    key={col.key}
                                    checked={!hiddenColumns.has(col.key)}
                                    onCheckedChange={() => !isDimension && toggleColumn(col.key)}
                                    disabled={isDimension}
                                    className="text-xs"
                                >
                                    {getLabel(col.key, lang)}
                                </DropdownMenuCheckboxItem>
                            )
                        })}
                    </DropdownMenuContent>
                </DropdownMenu>
            </div>

            {/* Table */}
            <div className="overflow-x-auto">
                <table className="w-full text-sm">
                    <thead>
                        <tr className="border-b bg-muted/40">
                            {/* Row number */}
                            <th className="px-3 py-3 text-center font-medium text-muted-foreground w-10 whitespace-nowrap sticky left-0 bg-muted/40 z-10">
                                #
                            </th>
                            {visibleColumns.map((col, colIdx) => {
                                const isDimension = dimensionSet.has(col.key)
                                // First dimension column is sticky
                                const isFirstDim = isDimension && colIdx === 0
                                return (
                                    <th
                                        key={col.key}
                                        className={`px-4 py-3 text-left font-medium text-muted-foreground cursor-pointer hover:text-foreground transition-colors whitespace-nowrap ${
                                            isFirstDim ? "sticky left-10 bg-muted/40 z-10" : ""
                                        }`}
                                        onClick={() => handleSort(col.key)}
                                    >
                                        {getLabel(col.key, lang)}
                                        {sortBy === col.key && (
                                            <span className="ml-1 text-[#6A6DE6]">
                                                {sortOrder === "asc" ? "↑" : "↓"}
                                            </span>
                                        )}
                                    </th>
                                )
                            })}
                        </tr>
                    </thead>
                    <tbody>
                        {rows.map((row, i) => (
                            <tr
                                key={i}
                                className="border-b last:border-0 hover:bg-muted/20 transition-colors"
                            >
                                {/* Row number */}
                                <td className="px-3 py-2.5 text-center text-xs text-muted-foreground sticky left-0 bg-background z-10">
                                    {i + 1}
                                </td>
                                {visibleColumns.map((col, colIdx) => {
                                    const isDimension = dimensionSet.has(col.key)
                                    const isFirstDim = isDimension && colIdx === 0
                                    const range = columnRanges.get(col.key)
                                    const numVal = Number(row[col.key])
                                    const bgColor = range && !Number.isNaN(numVal)
                                        ? getHeatColor(numVal, range.min, range.max, col.key)
                                        : undefined

                                    return (
                                        <td
                                            key={col.key}
                                            className={`px-4 py-2.5 whitespace-nowrap ${
                                                isFirstDim ? "sticky left-10 bg-background z-10 font-medium" : ""
                                            }`}
                                            style={bgColor ? { backgroundColor: bgColor } : undefined}
                                        >
                                            {formatCellValue(col.key, row[col.key])}
                                        </td>
                                    )
                                })}
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    )
}
