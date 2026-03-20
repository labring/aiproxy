import { useState, useMemo, useEffect, useRef } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { BarChart2, DollarSign, Hash, Building2, ArrowRight, TrendingUp, TrendingDown, Minus, ArrowUpDown, ArrowUp, ArrowDown, Settings2 } from "lucide-react"
import * as echarts from "echarts"
import { type DateRange } from "react-day-picker"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { DateRangePicker } from "@/components/common/DateRangePicker"
import { enterpriseApi, type DepartmentSummary, type ModelDistributionItem } from "@/api/enterprise"
import { ROUTES } from "@/routes/constants"
import { type TimeRange, getTimeRange, formatNumber, formatAmount, useDarkMode, getEChartsTheme, ALL_FILTER } from "@/lib/enterprise"
import { cn } from "@/lib/utils"

// Column configuration for department summary table
type DeptSortField = "department_name" | "member_count" | "active_users" | "request_count" | "used_amount" | "total_tokens" | "input_tokens" | "output_tokens" | "success_rate" | "avg_cost" | "unique_models"
type SortDirection = "asc" | "desc"

interface DeptColumnConfig {
    key: DeptSortField
    labelKey: string
    align: "left" | "right"
    defaultVisible: boolean
    format?: (value: number) => string
    renderCell?: (dept: DepartmentSummary) => React.ReactNode
}

const DEPT_COLUMNS: DeptColumnConfig[] = [
    { key: "department_name", labelKey: "enterprise.dashboard.department", align: "left", defaultVisible: true },
    { key: "request_count", labelKey: "enterprise.dashboard.requests", align: "right", defaultVisible: true, format: formatNumber },
    { key: "used_amount", labelKey: "enterprise.dashboard.amount", align: "right", defaultVisible: true, format: formatAmount },
    { key: "total_tokens", labelKey: "enterprise.dashboard.tokens", align: "right", defaultVisible: false, format: formatNumber },
    { key: "input_tokens", labelKey: "enterprise.dashboard.inputTokens", align: "right", defaultVisible: false, format: formatNumber },
    { key: "output_tokens", labelKey: "enterprise.dashboard.outputTokens", align: "right", defaultVisible: false, format: formatNumber },
    { key: "active_users", labelKey: "enterprise.dashboard.activeUsers", align: "right", defaultVisible: true },
    { key: "member_count", labelKey: "enterprise.dashboard.memberCount", align: "right", defaultVisible: false },
    { key: "success_rate", labelKey: "enterprise.dashboard.successRate", align: "right", defaultVisible: true,
        renderCell: (dept) => dept.success_rate > 0 ? `${dept.success_rate.toFixed(1)}%` : "-" },
    { key: "avg_cost", labelKey: "enterprise.dashboard.avgCost", align: "right", defaultVisible: false, format: formatAmount },
    { key: "unique_models", labelKey: "enterprise.dashboard.uniqueModels", align: "right", defaultVisible: false, format: formatNumber },
]

function MetricCard({
    title,
    value,
    icon: Icon,
    changePct,
}: {
    title: string
    value: string | number
    icon: React.ComponentType<{ className?: string }>
    changePct?: number
}) {
    return (
        <Card className="border border-gray-100 dark:border-gray-800">
            <CardContent className="p-6">
                <div className="flex items-center justify-between">
                    <div>
                        <p className="text-sm text-muted-foreground">{title}</p>
                        <p className="text-2xl font-bold mt-1">{value}</p>
                        {changePct !== undefined && (
                            <div className="flex items-center gap-1 mt-1">
                                {changePct > 0 ? (
                                    <TrendingUp className="w-3.5 h-3.5 text-green-500" />
                                ) : changePct < 0 ? (
                                    <TrendingDown className="w-3.5 h-3.5 text-red-500" />
                                ) : (
                                    <Minus className="w-3.5 h-3.5 text-muted-foreground" />
                                )}
                                <span
                                    className={`text-xs font-medium ${
                                        changePct > 0
                                            ? "text-green-500"
                                            : changePct < 0
                                              ? "text-red-500"
                                              : "text-muted-foreground"
                                    }`}
                                >
                                    {changePct > 0 ? "+" : ""}
                                    {changePct.toFixed(1)}%
                                </span>
                            </div>
                        )}
                    </div>
                    <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-[#6A6DE6]/10 to-[#8A8DF7]/10 flex items-center justify-center">
                        <Icon className="w-5 h-5 text-[#6A6DE6]" />
                    </div>
                </div>
            </CardContent>
        </Card>
    )
}

function DepartmentPieChart({ departments }: { departments: DepartmentSummary[] }) {
    const chartRef = useRef<HTMLDivElement>(null)
    const chartInstance = useRef<echarts.ECharts | null>(null)
    const isDark = useDarkMode()

    useEffect(() => {
        if (!chartRef.current) return

        if (!chartInstance.current) {
            chartInstance.current = echarts.init(chartRef.current)
        }

        const theme = getEChartsTheme(isDark)
        const data = departments
            .filter((d) => d.used_amount > 0)
            .sort((a, b) => b.used_amount - a.used_amount)
            .slice(0, 10)
            .map((d) => ({
                name: d.department_name || d.department_id,
                value: Math.round(d.used_amount * 100) / 100,
            }))

        chartInstance.current.setOption({
            tooltip: {
                trigger: "item",
                formatter: "{b}: ${c} ({d}%)",
            },
            series: [
                {
                    type: "pie",
                    radius: ["40%", "70%"],
                    avoidLabelOverlap: true,
                    itemStyle: {
                        borderRadius: 6,
                        borderColor: theme.borderColor,
                        borderWidth: 2,
                    },
                    label: {
                        show: true,
                        formatter: "{b}",
                        color: theme.textColor,
                    },
                    data,
                },
            ],
        })

        const handleResize = () => chartInstance.current?.resize()
        window.addEventListener("resize", handleResize)

        return () => {
            window.removeEventListener("resize", handleResize)
            chartInstance.current?.dispose()
            chartInstance.current = null
        }
    }, [departments, isDark])

    return <div ref={chartRef} className="w-full h-80" />
}

function ModelDistributionChart({ models }: { models: ModelDistributionItem[] }) {
    const chartRef = useRef<HTMLDivElement>(null)
    const chartInstance = useRef<echarts.ECharts | null>(null)
    const isDark = useDarkMode()
    const { t } = useTranslation()

    useEffect(() => {
        if (!chartRef.current || models.length === 0) return

        if (!chartInstance.current) {
            chartInstance.current = echarts.init(chartRef.current)
        }

        const theme = getEChartsTheme(isDark)
        const top10 = models.slice(0, 10)
        chartInstance.current.setOption({
            tooltip: {
                trigger: "axis",
                axisPointer: { type: "shadow" },
            },
            grid: {
                left: "3%",
                right: "4%",
                bottom: "3%",
                top: "8%",
                containLabel: true,
            },
            xAxis: {
                type: "category",
                data: top10.map((m) => {
                    const parts = m.model.split("/")
                    return parts[parts.length - 1]
                }),
                axisLabel: { rotate: 30, fontSize: 10, color: theme.subTextColor },
            },
            yAxis: {
                type: "value",
                name: t("enterprise.department.chartAmount"),
                nameTextStyle: { color: theme.subTextColor },
                axisLabel: { color: theme.subTextColor },
                splitLine: { lineStyle: { color: theme.splitLineColor } },
            },
            series: [
                {
                    type: "bar",
                    data: top10.map((m) => ({
                        value: Math.round(m.used_amount * 100) / 100,
                        itemStyle: { borderRadius: [4, 4, 0, 0] },
                    })),
                    itemStyle: {
                        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                            { offset: 0, color: "#6A6DE6" },
                            { offset: 1, color: "#8A8DF7" },
                        ]),
                    },
                },
            ],
        })

        const handleResize = () => chartInstance.current?.resize()
        window.addEventListener("resize", handleResize)

        return () => {
            window.removeEventListener("resize", handleResize)
            chartInstance.current?.dispose()
            chartInstance.current = null
        }
    }, [models, isDark, t])

    return <div ref={chartRef} className="w-full h-80" />
}

export default function EnterpriseDashboard() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [timeRange, setTimeRange] = useState<TimeRange>("7d")
    const [customDateRange, setCustomDateRange] = useState<DateRange | undefined>()

    // Hierarchical department filters
    const [selectedLevel1, setSelectedLevel1] = useState<string>("")
    const [selectedLevel2, setSelectedLevel2] = useState<string>("")

    // Column visibility
    const [visibleColumns, setVisibleColumns] = useState<Set<DeptSortField>>(() => {
        return new Set(DEPT_COLUMNS.filter(c => c.defaultVisible).map(c => c.key))
    })

    // Sorting
    const [sortField, setSortField] = useState<DeptSortField>("used_amount")
    const [sortDirection, setSortDirection] = useState<SortDirection>("desc")

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

    const { data, isLoading } = useQuery({
        queryKey: ["enterprise", "department-summary", start, end, departmentFilter],
        queryFn: () => enterpriseApi.getDepartmentSummary(start, end),
    })

    const { data: comparisonData } = useQuery({
        queryKey: ["enterprise", "comparison", timeRange, departmentFilter],
        queryFn: () => {
            const period = timeRange === "7d" || timeRange === "last_week" ? "weekly" : "monthly"
            return enterpriseApi.getComparison(period, departmentFilter)
        },
    })

    const { data: modelData } = useQuery({
        queryKey: ["enterprise", "model-distribution", start, end, departmentFilter],
        queryFn: () => enterpriseApi.getModelDistribution(departmentFilter, start, end),
    })

    // Filter departments by selected level1/level2
    const departments = useMemo(() => {
        const allDepts = data?.departments || []
        if (selectedLevel2) {
            return allDepts.filter(d => d.department_id === selectedLevel2)
        }
        if (selectedLevel1) {
            // Filter by level1: include the level1 department and all its level2 children
            const level2Ids = new Set(level2Departments.map(d => d.department_id))
            return allDepts.filter(d =>
                d.department_id === selectedLevel1 || level2Ids.has(d.department_id)
            )
        }
        return allDepts
    }, [data?.departments, selectedLevel1, selectedLevel2, level2Departments])

    const models = modelData?.distribution || []
    const changes = comparisonData?.changes

    // Sort departments
    const sortedDepartments = useMemo(() => {
        if (!departments.length) return departments
        return [...departments].sort((a, b) => {
            if (sortField === "department_name") {
                const aVal = a.department_name || a.department_id || ""
                const bVal = b.department_name || b.department_id || ""
                const cmp = aVal.localeCompare(bVal, "zh-CN")
                return sortDirection === "asc" ? cmp : -cmp
            }
            const aNum = Number(a[sortField]) || 0
            const bNum = Number(b[sortField]) || 0
            return sortDirection === "asc" ? aNum - bNum : bNum - aNum
        })
    }, [departments, sortField, sortDirection])

    const totals = useMemo(() => {
        return departments.reduce(
            (acc, d) => ({
                requests: acc.requests + (d.request_count || 0),
                amount: acc.amount + (d.used_amount || 0),
                tokens: acc.tokens + (d.total_tokens || 0),
                activeDepts: acc.activeDepts + 1,
            }),
            { requests: 0, amount: 0, tokens: 0, activeDepts: 0 },
        )
    }, [departments])

    const handleSort = (field: DeptSortField) => {
        if (sortField === field) {
            setSortDirection(prev => prev === "asc" ? "desc" : "asc")
        } else {
            setSortField(field)
            setSortDirection(field === "department_name" ? "asc" : "desc")
        }
    }

    const toggleColumn = (key: DeptSortField) => {
        setVisibleColumns(prev => {
            const next = new Set(prev)
            if (next.has(key)) {
                if (key !== "department_name") {
                    next.delete(key)
                    if (sortField === key) {
                        setSortField("used_amount")
                        setSortDirection("desc")
                    }
                }
            } else {
                next.add(key)
            }
            return next
        })
    }

    const renderSortIcon = (field: DeptSortField) => {
        if (sortField !== field) {
            return <ArrowUpDown className="ml-1 h-3 w-3 opacity-50" />
        }
        return sortDirection === "asc"
            ? <ArrowUp className="ml-1 h-3 w-3" />
            : <ArrowDown className="ml-1 h-3 w-3" />
    }

    const visibleColumnConfigs = DEPT_COLUMNS.filter(c => visibleColumns.has(c.key))

    const getCellValue = (dept: DepartmentSummary, col: DeptColumnConfig) => {
        if (col.key === "department_name") {
            return <span className="font-medium">{dept.department_name || dept.department_id}</span>
        }
        if (col.renderCell) {
            return col.renderCell(dept)
        }
        const value = dept[col.key as keyof DepartmentSummary]
        if (col.format && typeof value === "number") {
            return col.format(value)
        }
        return value
    }

    return (
        <div className="p-6 space-y-6">
            {/* Header with filters */}
            <div className="flex items-center justify-between flex-wrap gap-4">
                <h1 className="text-2xl font-bold">{t("enterprise.dashboard.title")}</h1>

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

                    {/* Time Range Selector */}
                    <div className="flex items-center gap-2">
                        <Select value={timeRange} onValueChange={(v) => {
                            setTimeRange(v as TimeRange)
                            if (v !== "custom") {
                                setCustomDateRange(undefined)
                            }
                        }}>
                            <SelectTrigger className="w-40">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="7d">{t("enterprise.dashboard.last7Days")}</SelectItem>
                                <SelectItem value="30d">{t("enterprise.dashboard.last30Days")}</SelectItem>
                                <SelectItem value="month">{t("enterprise.dashboard.thisMonth")}</SelectItem>
                                <SelectItem value="last_week">{t("enterprise.dashboard.lastWeek")}</SelectItem>
                                <SelectItem value="last_month">{t("enterprise.dashboard.lastMonth")}</SelectItem>
                                <SelectItem value="custom">{t("enterprise.dashboard.customRange")}</SelectItem>
                            </SelectContent>
                        </Select>

                        {/* Custom Date Range Picker */}
                        {timeRange === "custom" && (
                            <DateRangePicker
                                value={customDateRange}
                                onChange={setCustomDateRange}
                                placeholder={t("enterprise.dashboard.selectDateRange")}
                                className="w-64"
                            />
                        )}
                    </div>
                </div>
            </div>

            {/* Metric cards */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                <MetricCard
                    title={t("enterprise.dashboard.totalRequests")}
                    value={isLoading ? "..." : formatNumber(totals.requests)}
                    icon={BarChart2}
                    changePct={changes?.request_count_pct}
                />
                <MetricCard
                    title={t("enterprise.dashboard.totalAmount")}
                    value={isLoading ? "..." : formatAmount(totals.amount)}
                    icon={DollarSign}
                    changePct={changes?.used_amount_pct}
                />
                <MetricCard
                    title={t("enterprise.dashboard.totalTokens")}
                    value={isLoading ? "..." : formatNumber(totals.tokens)}
                    icon={Hash}
                    changePct={changes?.total_tokens_pct}
                />
                <MetricCard
                    title={t("enterprise.dashboard.activeDepartments")}
                    value={isLoading ? "..." : totals.activeDepts}
                    icon={Building2}
                />
            </div>

            {/* Main content: table + chart */}
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                {/* Department table */}
                <Card className="lg:col-span-2">
                    <CardHeader>
                        <div className="flex items-center justify-between">
                            <CardTitle className="text-lg">{t("enterprise.dashboard.departmentSummary")}</CardTitle>
                            <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                    <Button variant="outline" size="icon" className="h-8 w-8">
                                        <Settings2 className="h-4 w-4" />
                                    </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="end" className="w-48">
                                    <DropdownMenuLabel>{t("enterprise.dashboard.columns")}</DropdownMenuLabel>
                                    <DropdownMenuSeparator />
                                    {DEPT_COLUMNS.map((col) => (
                                        <DropdownMenuCheckboxItem
                                            key={col.key}
                                            checked={visibleColumns.has(col.key)}
                                            onCheckedChange={() => toggleColumn(col.key)}
                                            disabled={col.key === "department_name"}
                                        >
                                            {t(col.labelKey as never)}
                                        </DropdownMenuCheckboxItem>
                                    ))}
                                </DropdownMenuContent>
                            </DropdownMenu>
                        </div>
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
                                                    "py-3 px-2 font-medium cursor-pointer select-none hover:text-foreground transition-colors",
                                                    col.align === "right" ? "text-right" : "text-left",
                                                )}
                                                onClick={() => handleSort(col.key)}
                                            >
                                                <span className={cn(
                                                    "inline-flex items-center",
                                                    col.align === "right" && "justify-end"
                                                )}>
                                                    {t(col.labelKey as never)}
                                                    {renderSortIcon(col.key)}
                                                </span>
                                            </th>
                                        ))}
                                        <th className="text-right py-3 px-2 font-medium" />
                                    </tr>
                                </thead>
                                <tbody>
                                    {isLoading ? (
                                        <tr>
                                            <td colSpan={visibleColumnConfigs.length + 1} className="text-center py-8 text-muted-foreground">
                                                {t("common.loading")}
                                            </td>
                                        </tr>
                                    ) : sortedDepartments.length === 0 ? (
                                        <tr>
                                            <td colSpan={visibleColumnConfigs.length + 1} className="text-center py-8 text-muted-foreground">
                                                {t("common.noResult")}
                                            </td>
                                        </tr>
                                    ) : (
                                        sortedDepartments.map((dept) => (
                                            <tr
                                                key={dept.department_id}
                                                className="border-b last:border-0 hover:bg-muted/50 cursor-pointer transition-colors"
                                                onClick={() =>
                                                    navigate(`${ROUTES.ENTERPRISE_DEPARTMENT}/${dept.department_id}`)
                                                }
                                            >
                                                {visibleColumnConfigs.map((col) => (
                                                    <td
                                                        key={col.key}
                                                        className={cn(
                                                            "py-3 px-2",
                                                            col.align === "right" ? "text-right" : "text-left",
                                                        )}
                                                    >
                                                        {getCellValue(dept, col)}
                                                    </td>
                                                ))}
                                                <td className="py-3 px-2 text-right">
                                                    <Button variant="ghost" size="sm">
                                                        <ArrowRight className="w-4 h-4" />
                                                    </Button>
                                                </td>
                                            </tr>
                                        ))
                                    )}
                                </tbody>
                            </table>
                        </div>
                    </CardContent>
                </Card>

                {/* Pie chart */}
                <Card>
                    <CardHeader>
                        <CardTitle className="text-lg">{t("enterprise.dashboard.departmentChart")}</CardTitle>
                    </CardHeader>
                    <CardContent>
                        {departments.length > 0 ? (
                            <DepartmentPieChart departments={departments} />
                        ) : (
                            <div className="h-80 flex items-center justify-center text-muted-foreground">
                                {isLoading ? t("common.loading") : t("common.noResult")}
                            </div>
                        )}
                    </CardContent>
                </Card>
            </div>

            {/* Model Distribution */}
            <Card>
                <CardHeader>
                    <CardTitle className="text-lg">{t("enterprise.dashboard.modelDistribution")}</CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                        <div>
                            {models.length > 0 ? (
                                <ModelDistributionChart models={models} />
                            ) : (
                                <div className="h-80 flex items-center justify-center text-muted-foreground">
                                    {isLoading ? t("common.loading") : t("common.noResult")}
                                </div>
                            )}
                        </div>
                        <div className="overflow-x-auto">
                            <table className="w-full text-sm">
                                <thead>
                                    <tr className="border-b text-muted-foreground">
                                        <th className="text-left py-2 px-2 font-medium">{t("enterprise.dashboard.model")}</th>
                                        <th className="text-right py-2 px-2 font-medium">{t("enterprise.dashboard.requests")}</th>
                                        <th className="text-right py-2 px-2 font-medium">{t("enterprise.dashboard.amount")}</th>
                                        <th className="text-right py-2 px-2 font-medium">{t("enterprise.dashboard.share")}</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {models.slice(0, 10).map((m) => (
                                        <tr key={m.model} className="border-b last:border-0">
                                            <td className="py-2 px-2 font-medium text-xs truncate max-w-[180px]">
                                                {m.model}
                                            </td>
                                            <td className="py-2 px-2 text-right">{formatNumber(m.request_count)}</td>
                                            <td className="py-2 px-2 text-right">{formatAmount(m.used_amount)}</td>
                                            <td className="py-2 px-2 text-right">{m.percentage.toFixed(1)}%</td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    </div>
                </CardContent>
            </Card>
        </div>
    )
}
