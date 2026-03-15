import { useState, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Download, Check, ChevronsUpDown, ArrowUpDown, ArrowUp, ArrowDown, Settings2 } from "lucide-react"
import { DateRange } from "react-day-picker"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { DateRangePicker } from "@/components/common/DateRangePicker"
import { enterpriseApi } from "@/api/enterprise"
import { toast } from "sonner"
import { type TimeRange, getTimeRange, formatNumber, formatAmount } from "@/lib/enterprise"
import { cn } from "@/lib/utils"

type SortField = "rank" | "user_name" | "department_name" | "request_count" | "used_amount" | "total_tokens" | "input_tokens" | "output_tokens" | "unique_models"
type SortDirection = "asc" | "desc"

interface ColumnConfig {
    key: SortField
    align: "left" | "right"
    defaultVisible: boolean
    sortable: boolean
    format?: (value: number) => string
}

const COLUMNS: ColumnConfig[] = [
    { key: "rank", align: "left", defaultVisible: true, sortable: true },
    { key: "user_name", align: "left", defaultVisible: true, sortable: true },
    { key: "department_name", align: "left", defaultVisible: true, sortable: true },
    { key: "request_count", align: "right", defaultVisible: true, sortable: true, format: formatNumber },
    { key: "used_amount", align: "right", defaultVisible: true, sortable: true, format: formatAmount },
    { key: "total_tokens", align: "right", defaultVisible: true, sortable: true, format: formatNumber },
    { key: "input_tokens", align: "right", defaultVisible: false, sortable: true, format: formatNumber },
    { key: "output_tokens", align: "right", defaultVisible: false, sortable: true, format: formatNumber },
    { key: "unique_models", align: "right", defaultVisible: true, sortable: true, format: formatNumber },
]

// Translation keys for column labels
const COLUMN_LABELS = {
    rank: "enterprise.ranking.rank",
    user_name: "enterprise.ranking.userName",
    department_name: "enterprise.ranking.department",
    request_count: "enterprise.ranking.requests",
    used_amount: "enterprise.ranking.amount",
    total_tokens: "enterprise.ranking.tokens",
    input_tokens: "enterprise.ranking.inputTokens",
    output_tokens: "enterprise.ranking.outputTokens",
    unique_models: "enterprise.ranking.models",
} as const

export default function EnterpriseRanking() {
    const { t } = useTranslation()
    const [timeRange, setTimeRange] = useState<TimeRange>("7d")
    const [customDateRange, setCustomDateRange] = useState<DateRange | undefined>()
    const [selectedDepartments, setSelectedDepartments] = useState<string[]>([])
    const [limitType, setLimitType] = useState<"preset" | "custom" | "all">("preset")
    const [presetLimit, setPresetLimit] = useState<number>(50)
    const [customLimit, setCustomLimit] = useState<string>("100")
    const [deptPopoverOpen, setDeptPopoverOpen] = useState(false)

    // Column visibility state
    const [visibleColumns, setVisibleColumns] = useState<Set<SortField>>(() => {
        return new Set(COLUMNS.filter(c => c.defaultVisible).map(c => c.key))
    })

    // Sorting state
    const [sortField, setSortField] = useState<SortField>("rank")
    const [sortDirection, setSortDirection] = useState<SortDirection>("asc")

    // Calculate actual limit
    const limit = useMemo(() => {
        if (limitType === "all") return 0 // 0 means no limit
        if (limitType === "custom") return parseInt(customLimit) || 50
        return presetLimit
    }, [limitType, presetLimit, customLimit])

    // Calculate time range
    const { start, end } = useMemo(() => {
        if (timeRange === "custom" && customDateRange?.from) {
            const startTs = Math.floor(customDateRange.from.getTime() / 1000)
            const endTs = customDateRange.to
                ? Math.floor(customDateRange.to.getTime() / 1000) + 86399 // End of day
                : Math.floor(Date.now() / 1000)
            return getTimeRange("custom", startTs, endTs)
        }
        return getTimeRange(timeRange)
    }, [timeRange, customDateRange])

    // Fetch department list first
    const { data: deptData } = useQuery({
        queryKey: ["enterprise", "departments-for-filter", start, end],
        queryFn: () => enterpriseApi.getDepartmentSummary(start, end),
    })

    const departments = deptData?.departments || []

    // Build department filter string for API (comma-separated)
    const departmentFilter = useMemo(() => {
        if (selectedDepartments.length === 0) return undefined
        return selectedDepartments.join(",")
    }, [selectedDepartments])

    const { data: rankingData, isLoading } = useQuery({
        queryKey: ["enterprise", "ranking", start, end, departmentFilter, limit],
        queryFn: () =>
            enterpriseApi.getUserRanking(
                departmentFilter,
                limit || undefined, // 0 means no limit, pass undefined
                start,
                end,
            ),
    })

    const ranking = rankingData?.ranking || []

    // Sort data
    const sortedRanking = useMemo(() => {
        if (!ranking.length) return ranking

        return [...ranking].sort((a, b) => {
            let aVal: string | number
            let bVal: string | number

            // Special handling for department_name: use department_id as fallback (consistent with display)
            if (sortField === "department_name") {
                aVal = a.department_name || a.department_id || ""
                bVal = b.department_name || b.department_id || ""
            } else {
                aVal = a[sortField] ?? ""
                bVal = b[sortField] ?? ""
            }

            // Handle string comparison
            if (typeof aVal === "string" && typeof bVal === "string") {
                const cmp = aVal.localeCompare(bVal, "zh-CN")
                return sortDirection === "asc" ? cmp : -cmp
            }

            // Handle number comparison
            const aNum = Number(aVal) || 0
            const bNum = Number(bVal) || 0
            return sortDirection === "asc" ? aNum - bNum : bNum - aNum
        })
    }, [ranking, sortField, sortDirection])

    const handleSort = (field: SortField) => {
        if (sortField === field) {
            setSortDirection(prev => prev === "asc" ? "desc" : "asc")
        } else {
            setSortField(field)
            setSortDirection("asc")
        }
    }

    const toggleColumn = (key: SortField) => {
        setVisibleColumns(prev => {
            const next = new Set(prev)
            if (next.has(key)) {
                // Don't allow hiding all columns - keep at least rank and user_name
                if (key !== "rank" && key !== "user_name") {
                    next.delete(key)
                    // Reset sort if hiding the currently sorted column
                    if (sortField === key) {
                        setSortField("rank")
                        setSortDirection("asc")
                    }
                }
            } else {
                next.add(key)
            }
            return next
        })
    }

    const handleExport = async () => {
        try {
            await enterpriseApi.exportReport(start, end)
            toast.success(t("common.success"))
        } catch {
            toast.error(t("error.unknown"))
        }
    }

    const toggleDepartment = (deptId: string) => {
        setSelectedDepartments((prev) =>
            prev.includes(deptId)
                ? prev.filter((d) => d !== deptId)
                : [...prev, deptId]
        )
    }

    const selectAllDepartments = () => {
        if (selectedDepartments.length === departments.length) {
            setSelectedDepartments([])
        } else {
            setSelectedDepartments(departments.map((d) => d.department_id))
        }
    }

    // Display text for department filter
    const departmentDisplayText = useMemo(() => {
        if (selectedDepartments.length === 0) {
            return t("enterprise.ranking.allDepartments")
        }
        if (selectedDepartments.length === 1) {
            const dept = departments.find((d) => d.department_id === selectedDepartments[0])
            return dept?.department_name || selectedDepartments[0]
        }
        return t("enterprise.ranking.departmentsSelected", { count: selectedDepartments.length })
    }, [selectedDepartments, departments, t])

    // Get visible columns config
    const visibleColumnConfigs = COLUMNS.filter(c => visibleColumns.has(c.key))

    // Render sort icon
    const renderSortIcon = (field: SortField) => {
        if (sortField !== field) {
            return <ArrowUpDown className="ml-1 h-3 w-3 opacity-50" />
        }
        return sortDirection === "asc"
            ? <ArrowUp className="ml-1 h-3 w-3" />
            : <ArrowDown className="ml-1 h-3 w-3" />
    }

    // Get cell value
    const getCellValue = (user: (typeof ranking)[0], col: ColumnConfig) => {
        const value = user[col.key as keyof typeof user]
        if (col.key === "rank") {
            const rank = value as number
            return (
                <span
                    className={
                        rank <= 3
                            ? "inline-flex items-center justify-center w-6 h-6 rounded-full text-xs font-bold text-white " +
                              (rank === 1 ? "bg-yellow-500" : rank === 2 ? "bg-gray-400" : "bg-amber-600")
                            : "text-muted-foreground"
                    }
                >
                    {rank}
                </span>
            )
        }
        if (col.key === "user_name") {
            return <span className="font-medium">{value as string}</span>
        }
        if (col.key === "department_name") {
            return <span className="text-muted-foreground">{(value as string) || user.department_id}</span>
        }
        if (col.format && typeof value === "number") {
            return col.format(value)
        }
        return value
    }

    return (
        <div className="p-6 space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between flex-wrap gap-4">
                <h1 className="text-2xl font-bold">{t("enterprise.ranking.title")}</h1>
                <div className="flex items-center gap-3 flex-wrap">
                    {/* Time Range Filter */}
                    <div className="flex items-center gap-2">
                        <Select value={timeRange} onValueChange={(v) => setTimeRange(v as TimeRange)}>
                            <SelectTrigger className="w-36">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="7d">{t("enterprise.dashboard.last7Days")}</SelectItem>
                                <SelectItem value="30d">{t("enterprise.dashboard.last30Days")}</SelectItem>
                                <SelectItem value="month">{t("enterprise.dashboard.thisMonth")}</SelectItem>
                                <SelectItem value="custom">{t("enterprise.ranking.customRange")}</SelectItem>
                            </SelectContent>
                        </Select>
                        {timeRange === "custom" && (
                            <DateRangePicker
                                value={customDateRange}
                                onChange={setCustomDateRange}
                                className="w-64"
                            />
                        )}
                    </div>

                    {/* Department Multi-Select */}
                    <Popover open={deptPopoverOpen} onOpenChange={setDeptPopoverOpen}>
                        <PopoverTrigger asChild>
                            <Button
                                variant="outline"
                                role="combobox"
                                aria-expanded={deptPopoverOpen}
                                className="w-48 justify-between"
                            >
                                <span className="truncate">{departmentDisplayText}</span>
                                <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                            </Button>
                        </PopoverTrigger>
                        <PopoverContent className="w-56 p-0" align="start">
                            <div className="p-2 border-b">
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="w-full justify-start"
                                    onClick={selectAllDepartments}
                                >
                                    <Check
                                        className={cn(
                                            "mr-2 h-4 w-4",
                                            selectedDepartments.length === departments.length
                                                ? "opacity-100"
                                                : "opacity-0"
                                        )}
                                    />
                                    {selectedDepartments.length === departments.length
                                        ? t("enterprise.ranking.deselectAll")
                                        : t("enterprise.ranking.selectAll")}
                                </Button>
                            </div>
                            <div className="max-h-60 overflow-y-auto p-1">
                                {departments.map((dept) => (
                                    <div
                                        key={dept.department_id}
                                        className={cn(
                                            "flex items-center px-2 py-1.5 cursor-pointer rounded hover:bg-accent",
                                            selectedDepartments.includes(dept.department_id) && "bg-accent/50"
                                        )}
                                        onClick={() => toggleDepartment(dept.department_id)}
                                    >
                                        <Check
                                            className={cn(
                                                "mr-2 h-4 w-4",
                                                selectedDepartments.includes(dept.department_id)
                                                    ? "opacity-100"
                                                    : "opacity-0"
                                            )}
                                        />
                                        <span className="text-sm truncate">
                                            {dept.department_name || dept.department_id}
                                        </span>
                                    </div>
                                ))}
                            </div>
                            {selectedDepartments.length > 0 && (
                                <div className="p-2 border-t">
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        className="w-full text-muted-foreground"
                                        onClick={() => setSelectedDepartments([])}
                                    >
                                        {t("enterprise.ranking.clearSelection")}
                                    </Button>
                                </div>
                            )}
                        </PopoverContent>
                    </Popover>

                    {/* Limit Filter */}
                    <div className="flex items-center gap-2">
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
                            <SelectTrigger className="w-32">
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
                                className="w-20"
                                min={1}
                                max={10000}
                            />
                        )}
                    </div>

                    {/* Column Visibility */}
                    <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                            <Button variant="outline" size="icon">
                                <Settings2 className="h-4 w-4" />
                            </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-48">
                            <DropdownMenuLabel>{t("enterprise.ranking.columns")}</DropdownMenuLabel>
                            <DropdownMenuSeparator />
                            {COLUMNS.map((col) => (
                                <DropdownMenuCheckboxItem
                                    key={col.key}
                                    checked={visibleColumns.has(col.key)}
                                    onCheckedChange={() => toggleColumn(col.key)}
                                    disabled={col.key === "rank" || col.key === "user_name"}
                                >
                                    {t(COLUMN_LABELS[col.key])}
                                </DropdownMenuCheckboxItem>
                            ))}
                        </DropdownMenuContent>
                    </DropdownMenu>

                    <Button variant="outline" onClick={handleExport}>
                        <Download className="w-4 h-4 mr-2" />
                        {t("enterprise.ranking.export")}
                    </Button>
                </div>
            </div>

            {/* Ranking table */}
            <Card>
                <CardHeader className="pb-3">
                    <CardTitle className="text-base font-medium text-muted-foreground">
                        {t("enterprise.ranking.totalCount", { count: ranking.length })}
                    </CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="overflow-x-auto">
                        <table className="w-full text-sm">
                            <thead>
                                <tr className="border-b text-muted-foreground">
                                    {visibleColumnConfigs.map((col) => (
                                        <th
                                            key={col.key}
                                            className={cn(
                                                "py-3 px-2 font-medium",
                                                col.align === "right" ? "text-right" : "text-left",
                                                col.key === "rank" && "w-12",
                                                col.sortable && "cursor-pointer select-none hover:text-foreground transition-colors"
                                            )}
                                            onClick={() => col.sortable && handleSort(col.key)}
                                        >
                                            <span className="inline-flex items-center">
                                                {col.key === "rank" ? "#" : t(COLUMN_LABELS[col.key])}
                                                {col.sortable && renderSortIcon(col.key)}
                                            </span>
                                        </th>
                                    ))}
                                </tr>
                            </thead>
                            <tbody>
                                {isLoading ? (
                                    <tr>
                                        <td colSpan={visibleColumnConfigs.length} className="text-center py-8 text-muted-foreground">
                                            {t("common.loading")}
                                        </td>
                                    </tr>
                                ) : sortedRanking.length === 0 ? (
                                    <tr>
                                        <td colSpan={visibleColumnConfigs.length} className="text-center py-8 text-muted-foreground">
                                            {t("common.noResult")}
                                        </td>
                                    </tr>
                                ) : (
                                    sortedRanking.map((user) => (
                                        <tr
                                            key={user.group_id}
                                            className="border-b last:border-0 hover:bg-muted/50 transition-colors"
                                        >
                                            {visibleColumnConfigs.map((col) => (
                                                <td
                                                    key={col.key}
                                                    className={cn(
                                                        "py-3 px-2",
                                                        col.align === "right" ? "text-right" : "text-left"
                                                    )}
                                                >
                                                    {getCellValue(user, col)}
                                                </td>
                                            ))}
                                        </tr>
                                    ))
                                )}
                            </tbody>
                        </table>
                    </div>
                </CardContent>
            </Card>
        </div>
    )
}
