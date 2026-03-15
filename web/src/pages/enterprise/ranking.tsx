import { useState, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Download, Check, ChevronsUpDown } from "lucide-react"
import { DateRange } from "react-day-picker"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { DateRangePicker } from "@/components/common/DateRangePicker"
import { enterpriseApi } from "@/api/enterprise"
import { toast } from "sonner"
import { type TimeRange, getTimeRange, formatNumber, formatAmount } from "@/lib/enterprise"
import { cn } from "@/lib/utils"

export default function EnterpriseRanking() {
    const { t } = useTranslation()
    const [timeRange, setTimeRange] = useState<TimeRange>("7d")
    const [customDateRange, setCustomDateRange] = useState<DateRange | undefined>()
    const [selectedDepartments, setSelectedDepartments] = useState<string[]>([])
    const [limitType, setLimitType] = useState<"preset" | "custom" | "all">("preset")
    const [presetLimit, setPresetLimit] = useState<number>(50)
    const [customLimit, setCustomLimit] = useState<string>("100")
    const [deptPopoverOpen, setDeptPopoverOpen] = useState(false)

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

                    <Button variant="outline" onClick={handleExport}>
                        <Download className="w-4 h-4 mr-2" />
                        {t("enterprise.ranking.export")}
                    </Button>
                </div>
            </div>

            {/* Ranking table */}
            <Card>
                <CardHeader>
                    <CardTitle className="text-lg">
                        {t("enterprise.ranking.title")} ({ranking.length})
                    </CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="overflow-x-auto">
                        <table className="w-full text-sm">
                            <thead>
                                <tr className="border-b text-muted-foreground">
                                    <th className="text-left py-3 px-2 font-medium w-12">#</th>
                                    <th className="text-left py-3 px-2 font-medium">
                                        {t("enterprise.ranking.userName")}
                                    </th>
                                    <th className="text-left py-3 px-2 font-medium">
                                        {t("enterprise.ranking.department")}
                                    </th>
                                    <th className="text-right py-3 px-2 font-medium">
                                        {t("enterprise.ranking.requests")}
                                    </th>
                                    <th className="text-right py-3 px-2 font-medium">
                                        {t("enterprise.ranking.amount")}
                                    </th>
                                    <th className="text-right py-3 px-2 font-medium">
                                        {t("enterprise.ranking.tokens")}
                                    </th>
                                    <th className="text-right py-3 px-2 font-medium">
                                        {t("enterprise.ranking.inputTokens")}
                                    </th>
                                    <th className="text-right py-3 px-2 font-medium">
                                        {t("enterprise.ranking.outputTokens")}
                                    </th>
                                    <th className="text-right py-3 px-2 font-medium">
                                        {t("enterprise.ranking.models")}
                                    </th>
                                </tr>
                            </thead>
                            <tbody>
                                {isLoading ? (
                                    <tr>
                                        <td colSpan={9} className="text-center py-8 text-muted-foreground">
                                            {t("common.loading")}
                                        </td>
                                    </tr>
                                ) : ranking.length === 0 ? (
                                    <tr>
                                        <td colSpan={9} className="text-center py-8 text-muted-foreground">
                                            {t("common.noResult")}
                                        </td>
                                    </tr>
                                ) : (
                                    ranking.map((user) => (
                                        <tr
                                            key={user.group_id}
                                            className="border-b last:border-0 hover:bg-muted/50 transition-colors"
                                        >
                                            <td className="py-3 px-2">
                                                <span
                                                    className={
                                                        user.rank <= 3
                                                            ? "inline-flex items-center justify-center w-6 h-6 rounded-full text-xs font-bold text-white " +
                                                              (user.rank === 1
                                                                  ? "bg-yellow-500"
                                                                  : user.rank === 2
                                                                    ? "bg-gray-400"
                                                                    : "bg-amber-600")
                                                            : "text-muted-foreground"
                                                    }
                                                >
                                                    {user.rank}
                                                </span>
                                            </td>
                                            <td className="py-3 px-2 font-medium">{user.user_name}</td>
                                            <td className="py-3 px-2 text-muted-foreground">
                                                {user.department_name || user.department_id}
                                            </td>
                                            <td className="py-3 px-2 text-right">
                                                {formatNumber(user.request_count)}
                                            </td>
                                            <td className="py-3 px-2 text-right font-medium">
                                                {formatAmount(user.used_amount)}
                                            </td>
                                            <td className="py-3 px-2 text-right">
                                                {formatNumber(user.total_tokens)}
                                            </td>
                                            <td className="py-3 px-2 text-right text-muted-foreground">
                                                {formatNumber(user.input_tokens)}
                                            </td>
                                            <td className="py-3 px-2 text-right text-muted-foreground">
                                                {formatNumber(user.output_tokens)}
                                            </td>
                                            <td className="py-3 px-2 text-right">
                                                {user.unique_models}
                                            </td>
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
