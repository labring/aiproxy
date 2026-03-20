import { useState, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Download, ArrowUpDown, ArrowUp, ArrowDown, Settings2 } from "lucide-react"
import { DateRange } from "react-day-picker"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { DateRangePicker } from "@/components/common/DateRangePicker"
import { enterpriseApi, type UserRankingItem } from "@/api/enterprise"
import { toast } from "sonner"
import { type TimeRange, getTimeRange, formatNumber, formatAmount, ALL_FILTER } from "@/lib/enterprise"
import { cn } from "@/lib/utils"

type SortField = "rank" | "user_name" | "department_name" | "request_count" | "used_amount" | "total_tokens" | "input_tokens" | "output_tokens" | "success_rate" | "unique_models"
type SortDirection = "asc" | "desc"

interface ColumnConfig {
    key: SortField
    labelKey: string
    align: "left" | "right"
    defaultVisible: boolean
    sortable: boolean
    format?: (value: number) => string
    renderCell?: (user: UserRankingItem) => React.ReactNode
}

const COLUMNS: ColumnConfig[] = [
    { key: "rank", labelKey: "enterprise.ranking.rank", align: "left", defaultVisible: true, sortable: true,
        renderCell: (user) => {
            const rank = user.rank
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
        },
    },
    { key: "user_name", labelKey: "enterprise.ranking.userName", align: "left", defaultVisible: true, sortable: true },
    { key: "department_name", labelKey: "enterprise.ranking.department", align: "left", defaultVisible: true, sortable: true },
    { key: "request_count", labelKey: "enterprise.ranking.requests", align: "right", defaultVisible: true, sortable: true, format: formatNumber },
    { key: "used_amount", labelKey: "enterprise.ranking.amount", align: "right", defaultVisible: true, sortable: true, format: formatAmount },
    { key: "total_tokens", labelKey: "enterprise.ranking.tokens", align: "right", defaultVisible: true, sortable: true, format: formatNumber },
    { key: "input_tokens", labelKey: "enterprise.ranking.inputTokens", align: "right", defaultVisible: false, sortable: true, format: formatNumber },
    { key: "output_tokens", labelKey: "enterprise.ranking.outputTokens", align: "right", defaultVisible: false, sortable: true, format: formatNumber },
    { key: "success_rate", labelKey: "enterprise.ranking.successRate", align: "right", defaultVisible: false, sortable: true,
        renderCell: (user) => user.success_rate > 0 ? `${user.success_rate.toFixed(1)}%` : "-" },
    { key: "unique_models", labelKey: "enterprise.ranking.models", align: "right", defaultVisible: true, sortable: true, format: formatNumber },
]

export default function EnterpriseRanking() {
    const { t } = useTranslation()
    const [timeRange, setTimeRange] = useState<TimeRange>("7d")
    const [customDateRange, setCustomDateRange] = useState<DateRange | undefined>()
    const [limitType, setLimitType] = useState<"preset" | "custom" | "all">("preset")
    const [presetLimit, setPresetLimit] = useState<number>(50)
    const [customLimit, setCustomLimit] = useState<string>("100")

    // Hierarchical department filters
    const [selectedLevel1, setSelectedLevel1] = useState<string>("")
    const [selectedLevel2, setSelectedLevel2] = useState<string>("")

    // Column visibility state
    const [visibleColumns, setVisibleColumns] = useState<Set<SortField>>(() => {
        return new Set(COLUMNS.filter(c => c.defaultVisible).map(c => c.key))
    })

    // Sorting state
    const [sortField, setSortField] = useState<SortField>("rank")
    const [sortDirection, setSortDirection] = useState<SortDirection>("asc")

    // Calculate actual limit
    const limit = useMemo(() => {
        if (limitType === "all") return 0
        if (limitType === "custom") return parseInt(customLimit) || 50
        return presetLimit
    }, [limitType, presetLimit, customLimit])

    // Calculate time range
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

    // Fetch department hierarchy for level1/level2 filters
    const { data: deptLevels } = useQuery({
        queryKey: ["enterprise", "department-levels", selectedLevel1],
        queryFn: () => enterpriseApi.getDepartmentLevels(selectedLevel1 || undefined),
    })

    const level1Departments = useMemo(() => deptLevels?.level1_departments ?? [], [deptLevels])
    const level2Departments = useMemo(() => deptLevels?.level2_departments ?? [], [deptLevels])

    // Build department filter for API
    const departmentFilter = useMemo(() => {
        if (selectedLevel2) return selectedLevel2
        if (selectedLevel1) return selectedLevel1
        return undefined
    }, [selectedLevel1, selectedLevel2])

    const { data: rankingData, isLoading } = useQuery({
        queryKey: ["enterprise", "ranking", start, end, departmentFilter, limit],
        queryFn: () =>
            enterpriseApi.getUserRanking(
                departmentFilter,
                limit || undefined,
                start,
                end,
            ),
    })

    const ranking = useMemo(() => rankingData?.ranking || [], [rankingData])

    // Sort data
    const sortedRanking = useMemo(() => {
        if (!ranking.length) return ranking

        return [...ranking].sort((a, b) => {
            let aVal: string | number
            let bVal: string | number

            if (sortField === "department_name") {
                aVal = a.department_name || a.department_id || ""
                bVal = b.department_name || b.department_id || ""
            } else {
                aVal = a[sortField] ?? ""
                bVal = b[sortField] ?? ""
            }

            if (typeof aVal === "string" && typeof bVal === "string") {
                const cmp = aVal.localeCompare(bVal, "zh-CN")
                return sortDirection === "asc" ? cmp : -cmp
            }

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
            setSortDirection(field === "rank" || field === "user_name" || field === "department_name" ? "asc" : "desc")
        }
    }

    const toggleColumn = (key: SortField) => {
        setVisibleColumns(prev => {
            const next = new Set(prev)
            if (next.has(key)) {
                if (key !== "rank" && key !== "user_name") {
                    next.delete(key)
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

    const visibleColumnConfigs = COLUMNS.filter(c => visibleColumns.has(c.key))

    const renderSortIcon = (field: SortField) => {
        if (sortField !== field) {
            return <ArrowUpDown className="ml-1 h-3 w-3 opacity-50" />
        }
        return sortDirection === "asc"
            ? <ArrowUp className="ml-1 h-3 w-3" />
            : <ArrowDown className="ml-1 h-3 w-3" />
    }

    const getCellValue = (user: UserRankingItem, col: ColumnConfig) => {
        if (col.renderCell) {
            return col.renderCell(user)
        }
        const value = user[col.key as keyof UserRankingItem]
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
                    {/* Level 1 Department Filter */}
                    <Select
                        value={selectedLevel1}
                        onValueChange={(v) => {
                            setSelectedLevel1(v === ALL_FILTER ? "" : v)
                            setSelectedLevel2("")
                        }}
                    >
                        <SelectTrigger className="w-44">
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

                    {/* Level 2 Department Filter */}
                    <Select
                        value={selectedLevel2}
                        onValueChange={(v) => setSelectedLevel2(v === ALL_FILTER ? "" : v)}
                        disabled={!selectedLevel1}
                    >
                        <SelectTrigger className="w-44">
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

                    {/* Time Range Filter */}
                    <div className="flex items-center gap-2">
                        <Select value={timeRange} onValueChange={(v) => {
                            setTimeRange(v as TimeRange)
                            if (v !== "custom") {
                                setCustomDateRange(undefined)
                            }
                        }}>
                            <SelectTrigger className="w-36">
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
                                className="w-64"
                            />
                        )}
                    </div>

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
                                    {t(col.labelKey as never)}
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
                                                {col.key === "rank" ? "#" : t(col.labelKey as never)}
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
