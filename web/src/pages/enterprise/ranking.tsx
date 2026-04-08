import { useState, useMemo, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import {
    Download, ArrowUpDown, ArrowUp, ArrowDown, Settings2, Trophy,
} from "lucide-react"
import { useHasPermission } from "@/lib/permissions"
import { DateRange } from "react-day-picker"
import { Card, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import {
    Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
    DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent,
    DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
    Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
    Tooltip, TooltipContent, TooltipTrigger,
} from "@/components/ui/tooltip"
import { DateRangePicker } from "@/components/common/DateRangePicker"
import { enterpriseApi, type UserRankingItem } from "@/api/enterprise"
import { toast } from "sonner"
import {
    type TimeRange, getTimeRange, formatNumber, formatAmount, formatRate, ALL_FILTER,
} from "@/lib/enterprise"
import { cn } from "@/lib/utils"

type SortField =
    | "rank" | "user_name" | "department_name"
    | "total_tokens" | "request_count" | "unique_models" | "success_rate"
    | "cost_per_1k_tokens"
    | "used_amount" | "input_tokens" | "output_tokens" | "reconciliation_tokens"
    | "avg_tokens_per_req" | "avg_cost_per_req" | "output_input_ratio"

type SortDirection = "asc" | "desc"

/** Extended row with client-computed derived metrics. */
interface RankingRow extends UserRankingItem {
    cost_per_1k_tokens: number
    avg_tokens_per_req: number
    avg_cost_per_req: number
    output_input_ratio: number
    reconciliation_tokens: number
}

interface ColumnDef {
    key: SortField
    labelKey: string
    align: "left" | "right"
    defaultVisible: boolean
    /** Cannot be hidden via column picker. */
    pinned?: boolean
    sortable: boolean
    /** Appears under "optional" group in column picker. */
    optional?: boolean
    format?: (v: number) => string
    renderCell?: (row: RankingRow) => React.ReactNode
}

function formatCost(v: number): string {
    if (v <= 0) return "-"
    if (v < 0.0001) return `¥${v.toExponential(2)}`
    if (v < 0.01) return `¥${v.toFixed(4)}`
    return `¥${v.toFixed(4)}`
}

function formatRatio(v: number): string {
    if (!isFinite(v) || v <= 0) return "-"
    return v.toFixed(2)
}

const MEDAL_COLORS = [
    "bg-yellow-500",   // gold
    "bg-gray-400",     // silver
    "bg-amber-600",    // bronze
] as const

function RankBadge({ rank }: { rank: number }) {
    if (rank <= 3) {
        return (
            <span className={cn(
                "inline-flex items-center justify-center w-6 h-6 rounded-full text-xs font-bold text-white",
                MEDAL_COLORS[rank - 1],
            )}>
                {rank}
            </span>
        )
    }
    return <span className="text-muted-foreground tabular-nums">{rank}</span>
}

const COLUMNS: ColumnDef[] = [
    {
        key: "rank", labelKey: "enterprise.ranking.rank",
        align: "left", defaultVisible: true, pinned: true, sortable: true,
        renderCell: (row) => <RankBadge rank={row.rank} />,
    },
    {
        key: "user_name", labelKey: "enterprise.ranking.userName",
        align: "left", defaultVisible: true, pinned: true, sortable: true,
        renderCell: (row) => <span className="font-medium">{row.user_name}</span>,
    },
    {
        key: "department_name", labelKey: "enterprise.ranking.department",
        align: "left", defaultVisible: true, sortable: true,
        renderCell: (row) => (
            <span className="text-muted-foreground">{row.department_name || row.department_id}</span>
        ),
    },
    {
        key: "total_tokens", labelKey: "enterprise.ranking.tokens",
        align: "right", defaultVisible: true, sortable: true, format: formatNumber,
    },
    {
        key: "request_count", labelKey: "enterprise.ranking.requests",
        align: "right", defaultVisible: true, sortable: true, format: formatNumber,
    },
    {
        key: "unique_models", labelKey: "enterprise.ranking.models",
        align: "right", defaultVisible: true, sortable: true, format: formatNumber,
    },
    {
        key: "success_rate", labelKey: "enterprise.ranking.successRate",
        align: "right", defaultVisible: true, sortable: true,
        renderCell: (row) => row.success_rate > 0 ? (
            <Tooltip>
                <TooltipTrigger asChild>
                    <span className={cn(
                        row.success_rate >= 99 && "text-green-600 dark:text-green-400",
                        row.success_rate < 90 && "text-red-600 dark:text-red-400",
                    )}>
                        {formatRate(row.success_rate)}
                    </span>
                </TooltipTrigger>
                <TooltipContent>{row.success_rate.toFixed(2)}%</TooltipContent>
            </Tooltip>
        ) : <span className="text-muted-foreground">-</span>,
    },
    {
        key: "cost_per_1k_tokens", labelKey: "enterprise.ranking.costPer1kTokens",
        align: "right", defaultVisible: true, sortable: true, format: formatCost,
    },
    // --- Optional columns ---
    {
        key: "avg_tokens_per_req", labelKey: "enterprise.ranking.avgTokensPerReq",
        align: "right", defaultVisible: false, sortable: true, optional: true,
        format: formatNumber,
    },
    {
        key: "avg_cost_per_req", labelKey: "enterprise.ranking.avgCostPerReq",
        align: "right", defaultVisible: false, sortable: true, optional: true,
        format: formatCost,
    },
    {
        key: "output_input_ratio", labelKey: "enterprise.ranking.outputInputRatio",
        align: "right", defaultVisible: false, sortable: true, optional: true,
        format: formatRatio,
    },
    {
        key: "used_amount", labelKey: "enterprise.ranking.amount",
        align: "right", defaultVisible: false, sortable: true, optional: true,
        format: formatAmount,
    },
    {
        key: "input_tokens", labelKey: "enterprise.ranking.inputTokens",
        align: "right", defaultVisible: false, sortable: true, optional: true,
        format: formatNumber,
    },
    {
        key: "output_tokens", labelKey: "enterprise.ranking.outputTokens",
        align: "right", defaultVisible: false, sortable: true, optional: true,
        format: formatNumber,
    },
    {
        key: "reconciliation_tokens", labelKey: "enterprise.ranking.reconciliationTokens",
        align: "right", defaultVisible: false, sortable: true, optional: true,
        format: formatNumber,
    },
]

const DEFAULT_VISIBLE = new Set(
    COLUMNS.filter(c => c.defaultVisible).map(c => c.key),
)

const DEFAULT_COLS = COLUMNS.filter(c => !c.optional && !c.pinned)
const OPTIONAL_COLS = COLUMNS.filter(c => c.optional)

function deriveRow(item: UserRankingItem): RankingRow {
    const totalTokens = item.total_tokens || 0
    const requestCount = item.request_count || 0
    const usedAmount = item.used_amount || 0
    const inputTokens = item.input_tokens || 0
    const outputTokens = item.output_tokens || 0

    const cachedTokens = item.cached_tokens || 0
    const cacheCreationTokens = item.cache_creation_tokens || 0

    return {
        ...item,
        cost_per_1k_tokens: totalTokens > 0 ? (usedAmount / totalTokens) * 1000 : 0,
        avg_tokens_per_req: requestCount > 0 ? totalTokens / requestCount : 0,
        avg_cost_per_req: requestCount > 0 ? usedAmount / requestCount : 0,
        output_input_ratio: inputTokens > 0 ? outputTokens / inputTokens : 0,
        reconciliation_tokens: Math.max(0, inputTokens - cachedTokens - cacheCreationTokens) + outputTokens,
    }
}

export default function EnterpriseRanking() {
    const { t } = useTranslation()

    const [timeRange, setTimeRange] = useState<TimeRange>("7d")
    const [customDateRange, setCustomDateRange] = useState<DateRange | undefined>()
    const [limitType, setLimitType] = useState<"preset" | "custom" | "all">("preset")
    const [presetLimit, setPresetLimit] = useState<number>(50)
    const [customLimit, setCustomLimit] = useState<string>("100")
    const [selectedLevel1, setSelectedLevel1] = useState<string>("")
    const [selectedLevel2, setSelectedLevel2] = useState<string>("")
    const [visibleColumns, setVisibleColumns] = useState<Set<SortField>>(
        () => new Set(DEFAULT_VISIBLE),
    )
    const [sortField, setSortField] = useState<SortField>("total_tokens")
    const [sortDirection, setSortDirection] = useState<SortDirection>("desc")

    const limit = useMemo(() => {
        if (limitType === "all") return 0
        if (limitType === "custom") return parseInt(customLimit) || 50
        return presetLimit
    }, [limitType, presetLimit, customLimit])

    const { start, end } = useMemo(() => {
        if (timeRange === "custom" && customDateRange?.from) {
            const startTs = Math.floor(customDateRange.from.getTime() / 1000)
            const endTs = customDateRange.to
                ? Math.floor(customDateRange.to.getTime() / 1000) + 86399
                : Math.floor(Date.now() / 1000)
            return getTimeRange("custom", startTs, endTs)
        }
        return getTimeRange(timeRange)
    }, [timeRange, customDateRange])

    const departmentFilter = useMemo(() => {
        if (selectedLevel2) return selectedLevel2
        if (selectedLevel1) return selectedLevel1
        return undefined
    }, [selectedLevel1, selectedLevel2])

    const { data: deptLevels } = useQuery({
        queryKey: ["enterprise", "department-levels", selectedLevel1],
        queryFn: () => enterpriseApi.getDepartmentLevels(selectedLevel1 || undefined),
    })

    const level1Departments = useMemo(() => deptLevels?.level1_departments ?? [], [deptLevels])
    const level2Departments = useMemo(() => deptLevels?.level2_departments ?? [], [deptLevels])

    const { data: rankingData, isLoading } = useQuery({
        queryKey: ["enterprise", "ranking", start, end, departmentFilter, limit],
        queryFn: () => enterpriseApi.getUserRanking(departmentFilter, limit, start, end),
    })

    const rows: RankingRow[] = useMemo(
        () => (rankingData?.ranking || []).map(deriveRow),
        [rankingData],
    )

    const sortedRows = useMemo(() => {
        if (!rows.length) return rows

        const sorted = [...rows].sort((a, b) => {
            const field = sortField
            if (field === "user_name" || field === "department_name") {
                const av = field === "department_name"
                    ? (a.department_name || a.department_id || "")
                    : (a.user_name || "")
                const bv = field === "department_name"
                    ? (b.department_name || b.department_id || "")
                    : (b.user_name || "")
                const cmp = av.localeCompare(bv, "zh-CN")
                return sortDirection === "asc" ? cmp : -cmp
            }

            const aNum = Number(a[field as keyof RankingRow]) || 0
            const bNum = Number(b[field as keyof RankingRow]) || 0
            return sortDirection === "asc" ? aNum - bNum : bNum - aNum
        })

        // Re-assign rank numbers to match current display order so that
        // the gold/silver/bronze badges always correspond to the visible
        // top-3, regardless of which column the user sorts by.
        for (let i = 0; i < sorted.length; i++) {
            sorted[i] = { ...sorted[i], rank: i + 1 }
        }
        return sorted
    }, [rows, sortField, sortDirection])


    const handleSort = useCallback((field: SortField) => {
        setSortField(prev => {
            if (prev === field) {
                setSortDirection(d => d === "asc" ? "desc" : "asc")
                return prev
            }
            // Text fields default ascending, numeric fields default descending
            const isText = field === "rank" || field === "user_name" || field === "department_name"
            setSortDirection(isText ? "asc" : "desc")
            return field
        })
    }, [])

    const toggleColumn = useCallback((key: SortField) => {
        setVisibleColumns(prev => {
            const next = new Set(prev)
            if (next.has(key)) {
                next.delete(key)
                if (sortField === key) {
                    setSortField("total_tokens")
                    setSortDirection("desc")
                }
            } else {
                next.add(key)
            }
            return next
        })
    }, [sortField])

    const canExport = useHasPermission('export_manage')

    const handleExport = async () => {
        try {
            await enterpriseApi.exportReport(start, end, departmentFilter, limit)
            toast.success(t("common.success"))
        } catch {
            toast.error(t("error.unknown"))
        }
    }

    const visibleCols = useMemo(
        () => COLUMNS.filter(c => visibleColumns.has(c.key)),
        [visibleColumns],
    )


    const renderSortIcon = (field: SortField) => {
        if (sortField !== field) {
            return <ArrowUpDown className="ml-1 h-3 w-3 opacity-40" />
        }
        return sortDirection === "asc"
            ? <ArrowUp className="ml-1 h-3 w-3 text-primary" />
            : <ArrowDown className="ml-1 h-3 w-3 text-primary" />
    }

    const getCellValue = (row: RankingRow, col: ColumnDef) => {
        if (col.renderCell) return col.renderCell(row)
        const value = row[col.key as keyof RankingRow]
        if (col.format && typeof value === "number") return col.format(value)
        return value
    }


    return (
        <div className="p-6 space-y-4">
            {/* Header */}
            <div className="flex items-center justify-between flex-wrap gap-3">
                <div className="flex items-center gap-2">
                    <Trophy className="h-5 w-5 text-yellow-500" />
                    <h1 className="text-2xl font-bold">{t("enterprise.ranking.title")}</h1>
                    <Badge variant="secondary" className="ml-1 tabular-nums">
                        {isLoading ? "..." : t("enterprise.ranking.totalCount", { count: rows.length })}
                    </Badge>
                </div>

                {canExport && (
                    <Button variant="outline" size="sm" onClick={handleExport}>
                        <Download className="w-4 h-4 mr-1.5" />
                        {t("enterprise.ranking.export")}
                    </Button>
                )}
            </div>

            {/* Toolbar */}
            <div className="flex items-center gap-2 flex-wrap">
                {/* Department filters */}
                <Select
                    value={selectedLevel1}
                    onValueChange={(v) => {
                        setSelectedLevel1(v === ALL_FILTER ? "" : v)
                        setSelectedLevel2("")
                    }}
                >
                    <SelectTrigger className="w-40 h-8 text-xs">
                        <SelectValue placeholder={t("enterprise.dashboard.level1Dept")} />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value={ALL_FILTER}>{t("enterprise.dashboard.allLevel1Depts")}</SelectItem>
                        {level1Departments.map((dept) => (
                            <SelectItem key={dept.department_id} value={dept.department_id}>
                                {dept.name || dept.department_id}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>

                <Select
                    value={selectedLevel2}
                    onValueChange={(v) => setSelectedLevel2(v === ALL_FILTER ? "" : v)}
                    disabled={!selectedLevel1}
                >
                    <SelectTrigger className="w-40 h-8 text-xs">
                        <SelectValue placeholder={t("enterprise.dashboard.level2Dept")} />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value={ALL_FILTER}>{t("enterprise.dashboard.allLevel2Depts")}</SelectItem>
                        {level2Departments.map((dept) => (
                            <SelectItem key={dept.department_id} value={dept.department_id}>
                                {dept.name || dept.department_id}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>

                <div className="w-px h-5 bg-border mx-1" />

                {/* Time range */}
                <Select value={timeRange} onValueChange={(v) => {
                    setTimeRange(v as TimeRange)
                    if (v !== "custom") setCustomDateRange(undefined)
                }}>
                    <SelectTrigger className="w-32 h-8 text-xs">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="7d">{t("enterprise.dashboard.last7Days")}</SelectItem>
                        <SelectItem value="30d">{t("enterprise.dashboard.last30Days")}</SelectItem>
                        <SelectItem value="month">{t("enterprise.dashboard.thisMonth")}</SelectItem>
                        <SelectItem value="last_week">{t("enterprise.dashboard.lastWeek")}</SelectItem>
                        <SelectItem value="last_month">{t("enterprise.dashboard.lastMonth")}</SelectItem>
                        <SelectItem value="custom">{t("enterprise.ranking.customRange")}</SelectItem>
                    </SelectContent>
                </Select>
                {timeRange === "custom" && (
                    <DateRangePicker
                        value={customDateRange}
                        onChange={setCustomDateRange}
                        className="w-56"
                    />
                )}

                <div className="w-px h-5 bg-border mx-1" />

                {/* Limit */}
                <Select
                    value={limitType === "preset" ? String(presetLimit) : limitType}
                    onValueChange={(v) => {
                        if (v === "custom") {
                            setLimitType("custom")
                        } else if (v === "all") {
                            setLimitType("all")
                        } else {
                            setLimitType("preset")
                            setPresetLimit(Number(v))
                        }
                    }}
                >
                    <SelectTrigger className="w-28 h-8 text-xs">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="20">{t("enterprise.ranking.top", { count: 20 })}</SelectItem>
                        <SelectItem value="50">{t("enterprise.ranking.top", { count: 50 })}</SelectItem>
                        <SelectItem value="100">{t("enterprise.ranking.top", { count: 100 })}</SelectItem>
                        <SelectItem value="custom">{t("enterprise.ranking.customLimit")}</SelectItem>
                        <SelectItem value="all">{t("enterprise.ranking.showAll")}</SelectItem>
                    </SelectContent>
                </Select>
                {limitType === "custom" && (
                    <Input
                        type="number"
                        value={customLimit}
                        onChange={(e) => setCustomLimit(e.target.value)}
                        className="w-20 h-8 text-xs"
                        min={1}
                        max={10000}
                    />
                )}

                <div className="flex-1" />

                {/* Column picker */}
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button variant="outline" size="sm" className="h-8 gap-1.5">
                            <Settings2 className="h-3.5 w-3.5" />
                            {t("enterprise.ranking.columns")}
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-52">
                        <DropdownMenuLabel>{t("enterprise.ranking.defaultColumns")}</DropdownMenuLabel>
                        {DEFAULT_COLS.map((col) => (
                            <DropdownMenuCheckboxItem
                                key={col.key}
                                checked={visibleColumns.has(col.key)}
                                onCheckedChange={() => toggleColumn(col.key)}
                            >
                                {t(col.labelKey as never)}
                            </DropdownMenuCheckboxItem>
                        ))}
                        <DropdownMenuSeparator />
                        <DropdownMenuLabel>{t("enterprise.ranking.optionalColumns")}</DropdownMenuLabel>
                        {OPTIONAL_COLS.map((col) => (
                            <DropdownMenuCheckboxItem
                                key={col.key}
                                checked={visibleColumns.has(col.key)}
                                onCheckedChange={() => toggleColumn(col.key)}
                            >
                                {t(col.labelKey as never)}
                            </DropdownMenuCheckboxItem>
                        ))}
                    </DropdownMenuContent>
                </DropdownMenu>
            </div>

            {/* Data Table */}
            <Card className="border">
                <CardContent className="p-0">
                    <Table>
                        <TableHeader>
                            <TableRow className="hover:bg-transparent">
                                {visibleCols.map((col) => (
                                    <TableHead
                                        key={col.key}
                                        className={cn(
                                            col.align === "right" && "text-right",
                                            col.key === "rank" && "w-14",
                                            col.sortable && "cursor-pointer select-none hover:text-primary transition-colors",
                                        )}
                                        onClick={() => col.sortable && handleSort(col.key)}
                                    >
                                        <span className={cn(
                                            "inline-flex items-center gap-0.5",
                                            col.align === "right" && "justify-end w-full",
                                        )}>
                                            {col.key === "rank" ? "#" : t(col.labelKey as never)}
                                            {col.sortable && renderSortIcon(col.key)}
                                        </span>
                                    </TableHead>
                                ))}
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {isLoading ? (
                                Array.from({ length: 8 }).map((_, i) => (
                                    <TableRow key={i}>
                                        {visibleCols.map((col) => (
                                            <TableCell key={col.key}>
                                                <Skeleton className={cn(
                                                    "h-4 rounded",
                                                    col.align === "right" ? "w-16 ml-auto" : "w-20",
                                                )} />
                                            </TableCell>
                                        ))}
                                    </TableRow>
                                ))
                            ) : sortedRows.length === 0 ? (
                                <TableRow>
                                    <TableCell
                                        colSpan={visibleCols.length}
                                        className="text-center py-12 text-muted-foreground"
                                    >
                                        {t("common.noResult")}
                                    </TableCell>
                                </TableRow>
                            ) : (
                                sortedRows.map((row) => (
                                    <TableRow key={row.group_id}>
                                        {visibleCols.map((col) => (
                                            <TableCell
                                                key={col.key}
                                                className={cn(
                                                    col.align === "right" && "text-right tabular-nums",
                                                )}
                                            >
                                                {getCellValue(row, col)}
                                            </TableCell>
                                        ))}
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                </CardContent>
            </Card>
        </div>
    )
}
