import { useState, useEffect, useRef, useCallback, type KeyboardEvent } from "react"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"
import { useMutation, useQuery } from "@tanstack/react-query"
import * as echarts from "echarts"
import {
    FileBarChart,
    Table2,
    BarChart3,
    LineChart,
    PieChart,
    Download,
    ChevronDown,
    ChevronRight,
    Loader2,
    X,
    Grid3X3,
    Zap,
    Filter,
    Check,
    ChevronsUpDown,
} from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
    enterpriseApi,
    type CustomReportRequest,
    type CustomReportResponse,
} from "@/api/enterprise"
import { type TimeRange, getTimeRange } from "@/lib/enterprise"

// ─── Field catalog (client-side categories + labels) ─────────────────────────

interface FieldDef {
    key: string
    category: string
}

const DIMENSION_FIELDS: FieldDef[] = [
    { key: "user_name", category: "identity" },
    { key: "department", category: "identity" },
    { key: "model", category: "identity" },
    { key: "time_day", category: "time" },
    { key: "time_week", category: "time" },
    { key: "time_hour", category: "time" },
]

const MEASURE_FIELDS: FieldDef[] = [
    // requests
    { key: "request_count", category: "requests" },
    { key: "retry_count", category: "requests" },
    { key: "exception_count", category: "requests" },
    { key: "status_2xx", category: "requests" },
    { key: "status_4xx", category: "requests" },
    { key: "status_5xx", category: "requests" },
    { key: "status_429", category: "requests" },
    { key: "cache_hit_count", category: "requests" },
    // tokens
    { key: "input_tokens", category: "tokens" },
    { key: "output_tokens", category: "tokens" },
    { key: "total_tokens", category: "tokens" },
    { key: "cached_tokens", category: "tokens" },
    { key: "image_input_tokens", category: "tokens" },
    { key: "audio_input_tokens", category: "tokens" },
    { key: "web_search_count", category: "tokens" },
    // cost
    { key: "used_amount", category: "cost" },
    { key: "input_amount", category: "cost" },
    { key: "output_amount", category: "cost" },
    { key: "cached_amount", category: "cost" },
    // performance
    { key: "total_time_ms", category: "performance" },
    { key: "total_ttfb_ms", category: "performance" },
    // computed
    { key: "success_rate", category: "computed" },
    { key: "error_rate", category: "computed" },
    { key: "throttle_rate", category: "computed" },
    { key: "cache_hit_rate", category: "computed" },
    { key: "avg_tokens_per_req", category: "computed" },
    { key: "avg_cost_per_req", category: "computed" },
    { key: "avg_latency", category: "computed" },
    { key: "avg_ttfb", category: "computed" },
    { key: "output_input_ratio", category: "computed" },
    { key: "cost_per_1k_tokens", category: "computed" },
    { key: "retry_rate", category: "computed" },
    { key: "unique_models", category: "computed" },
    { key: "active_users", category: "computed" },
]

const FIELD_LABELS: Record<string, { zh: string; en: string }> = {
    // dimensions
    user_name: { zh: "用户名", en: "User" },
    department: { zh: "部门", en: "Department" },
    model: { zh: "模型", en: "Model" },
    time_hour: { zh: "小时", en: "Hour" },
    time_day: { zh: "天", en: "Day" },
    time_week: { zh: "周", en: "Week" },
    // requests
    request_count: { zh: "请求数", en: "Requests" },
    retry_count: { zh: "重试数", en: "Retries" },
    exception_count: { zh: "异常数", en: "Exceptions" },
    status_2xx: { zh: "成功数", en: "2xx" },
    status_4xx: { zh: "客户端错误", en: "4xx" },
    status_5xx: { zh: "服务端错误", en: "5xx" },
    status_429: { zh: "限流数", en: "429" },
    cache_hit_count: { zh: "缓存命中", en: "Cache Hits" },
    // tokens
    input_tokens: { zh: "输入 Token", en: "Input Tokens" },
    output_tokens: { zh: "输出 Token", en: "Output Tokens" },
    total_tokens: { zh: "总 Token", en: "Total Tokens" },
    cached_tokens: { zh: "缓存 Token", en: "Cached Tokens" },
    image_input_tokens: { zh: "图片 Token", en: "Image Tokens" },
    audio_input_tokens: { zh: "音频 Token", en: "Audio Tokens" },
    web_search_count: { zh: "联网搜索", en: "Web Searches" },
    // cost
    used_amount: { zh: "总费用", en: "Total Cost" },
    input_amount: { zh: "输入费用", en: "Input Cost" },
    output_amount: { zh: "输出费用", en: "Output Cost" },
    cached_amount: { zh: "缓存费用", en: "Cache Cost" },
    // performance
    total_time_ms: { zh: "总耗时(ms)", en: "Total Time (ms)" },
    total_ttfb_ms: { zh: "总TTFB(ms)", en: "Total TTFB (ms)" },
    // computed
    success_rate: { zh: "成功率 %", en: "Success Rate %" },
    error_rate: { zh: "错误率 %", en: "Error Rate %" },
    throttle_rate: { zh: "限流率 %", en: "Throttle Rate %" },
    cache_hit_rate: { zh: "缓存命中率 %", en: "Cache Hit Rate %" },
    avg_tokens_per_req: { zh: "平均Token/请求", en: "Avg Tokens/Req" },
    avg_cost_per_req: { zh: "平均费用/请求", en: "Avg Cost/Req" },
    avg_latency: { zh: "平均延迟(ms)", en: "Avg Latency (ms)" },
    avg_ttfb: { zh: "平均TTFB(ms)", en: "Avg TTFB (ms)" },
    output_input_ratio: { zh: "输出/输入比", en: "Output/Input Ratio" },
    cost_per_1k_tokens: { zh: "千Token成本", en: "Cost/1K Tokens" },
    retry_rate: { zh: "重试率 %", en: "Retry Rate %" },
    unique_models: { zh: "使用模型数", en: "Unique Models" },
    active_users: { zh: "活跃用户数", en: "Active Users" },
}

const CATEGORIES = ["requests", "tokens", "cost", "performance", "computed"] as const

function getLabel(key: string, lang: string): string {
    const entry = FIELD_LABELS[key]
    if (!entry) return key
    return lang.startsWith("zh") ? entry.zh : entry.en
}

function formatCellValue(key: string, value: unknown): string {
    if (value == null) return "-"
    const n = Number(value)
    if (Number.isNaN(n)) return String(value)

    // time dimensions
    if (key === "time_hour" || key === "time_day" || key === "time_week") {
        const d = new Date(n * 1000)
        if (key === "time_hour") return d.toLocaleString()
        return d.toLocaleDateString()
    }

    // percentages
    if (key.endsWith("_rate")) return `${n.toFixed(2)}%`

    // cost fields
    if (key.includes("amount") || key.includes("cost") || key === "avg_cost_per_req" || key === "cost_per_1k_tokens") {
        return `$${n.toFixed(4)}`
    }

    // latency
    if (key.includes("latency") || key.includes("ttfb") || key.includes("time_ms")) {
        return `${n.toFixed(1)} ms`
    }

    // ratios
    if (key === "output_input_ratio") return n.toFixed(2)

    // large numbers
    if (Number.isInteger(n) && n >= 1000) return n.toLocaleString()
    if (!Number.isInteger(n)) return n.toFixed(2)
    return String(n)
}

// ─── Report templates ────────────────────────────────────────────────────────

interface ReportTemplate {
    id: string
    labelKey: string
    dimensions: string[]
    measures: string[]
}

const REPORT_TEMPLATES: ReportTemplate[] = [
    {
        id: "dept_cost_top10",
        labelKey: "enterprise.customReport.templates.deptCostTop10",
        dimensions: ["department"],
        measures: ["used_amount", "request_count", "active_users"],
    },
    {
        id: "model_usage_trend",
        labelKey: "enterprise.customReport.templates.modelUsageTrend",
        dimensions: ["time_day", "model"],
        measures: ["request_count", "total_tokens", "used_amount"],
    },
    {
        id: "user_activity_rank",
        labelKey: "enterprise.customReport.templates.userActivityRank",
        dimensions: ["user_name"],
        measures: ["request_count", "used_amount", "unique_models", "success_rate"],
    },
    {
        id: "dept_model_pivot",
        labelKey: "enterprise.customReport.templates.deptModelPivot",
        dimensions: ["department", "model"],
        measures: ["used_amount", "request_count"],
    },
    {
        id: "daily_performance",
        labelKey: "enterprise.customReport.templates.dailyPerformance",
        dimensions: ["time_day"],
        measures: ["request_count", "avg_latency", "success_rate", "error_rate"],
    },
]

// ─── TagInput component ──────────────────────────────────────────────────────

function TagInput({
    values,
    onChange,
    placeholder,
}: {
    values: string[]
    onChange: (vals: string[]) => void
    placeholder: string
}) {
    const [input, setInput] = useState("")

    const addTag = () => {
        const trimmed = input.trim()
        if (trimmed && !values.includes(trimmed)) {
            onChange([...values, trimmed])
        }
        setInput("")
    }

    const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
        if (e.key === "Enter" || e.key === ",") {
            e.preventDefault()
            addTag()
        }
        if (e.key === "Backspace" && input === "" && values.length > 0) {
            onChange(values.slice(0, -1))
        }
    }

    return (
        <div className="flex flex-wrap items-center gap-1.5 p-1.5 border rounded-md min-h-[36px] bg-background">
            {values.map((v) => (
                <Badge key={v} variant="secondary" className="text-xs gap-1 px-2 py-0.5">
                    {v}
                    <X
                        className="w-3 h-3 cursor-pointer hover:text-destructive"
                        onClick={() => onChange(values.filter((x) => x !== v))}
                    />
                </Badge>
            ))}
            <Input
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                onBlur={addTag}
                placeholder={values.length === 0 ? placeholder : ""}
                className="border-0 shadow-none h-7 min-w-[120px] flex-1 focus-visible:ring-0 p-0 px-1"
            />
        </div>
    )
}

// ─── Chip selector component ─────────────────────────────────────────────────

function ChipSelector({
    fields,
    selected,
    onChange,
    lang,
}: {
    fields: FieldDef[]
    selected: string[]
    onChange: (keys: string[]) => void
    lang: string
}) {
    return (
        <div className="flex flex-wrap gap-1.5">
            {fields.map((f) => {
                const active = selected.includes(f.key)
                return (
                    <Badge
                        key={f.key}
                        variant={active ? "default" : "outline"}
                        className={`cursor-pointer select-none transition-all text-xs px-2.5 py-1 ${
                            active
                                ? "bg-[#6A6DE6] hover:bg-[#5A5DD6] text-white border-transparent"
                                : "hover:bg-[#6A6DE6]/10 hover:border-[#6A6DE6]/30"
                        }`}
                        onClick={() => {
                            onChange(
                                active
                                    ? selected.filter((k) => k !== f.key)
                                    : [...selected, f.key],
                            )
                        }}
                    >
                        {getLabel(f.key, lang)}
                        {active && <X className="w-3 h-3 ml-1" />}
                    </Badge>
                )
            })}
        </div>
    )
}

// ─── Report chart ────────────────────────────────────────────────────────────

const CHART_COLORS = [
    "#6A6DE6", "#8A8DF7", "#4ECDC4", "#FF6B6B", "#FFD93D",
    "#6BCB77", "#FF8E53", "#A78BFA", "#F472B6", "#38BDF8",
]

function ReportChart({
    data,
    dimensions,
    measures,
    chartType,
    lang,
}: {
    data: CustomReportResponse
    dimensions: string[]
    measures: string[]
    chartType: "bar" | "line" | "pie"
    lang: string
}) {
    const chartRef = useRef<HTMLDivElement>(null)
    const instance = useRef<echarts.ECharts | null>(null)

    useEffect(() => {
        if (!chartRef.current || data.rows.length === 0) return

        if (!instance.current) {
            instance.current = echarts.init(chartRef.current)
        }

        // Build category labels from dimension values
        const labels = data.rows.map((row) =>
            dimensions.map((d) => String(row[d] ?? "")).join(" / "),
        )

        // Only chart numeric measures
        const numericMeasures = measures.filter((m) => {
            const first = data.rows[0]?.[m]
            return first !== undefined && !Number.isNaN(Number(first))
        })

        if (chartType === "pie") {
            // Pie uses the first numeric measure
            const measure = numericMeasures[0]
            if (!measure) return
            instance.current.setOption({
                tooltip: { trigger: "item", formatter: "{b}: {c} ({d}%)" },
                series: [{
                    type: "pie",
                    radius: ["40%", "70%"],
                    data: data.rows.slice(0, 15).map((row, i) => ({
                        name: labels[i],
                        value: Number(row[measure] ?? 0),
                        itemStyle: { color: CHART_COLORS[i % CHART_COLORS.length] },
                    })),
                    label: { show: true, formatter: "{b}\n{d}%" },
                }],
            }, true)
        } else {
            // Detect scale difference for dual Y-axis
            const PERCENTAGE_FIELDS = new Set([
                "success_rate", "error_rate", "throttle_rate", "cache_hit_rate", "retry_rate",
            ])
            const hasPercentage = numericMeasures.some((m) => PERCENTAGE_FIELDS.has(m))
            const hasAbsolute = numericMeasures.some((m) => !PERCENTAGE_FIELDS.has(m))
            const needDualAxis = hasPercentage && hasAbsolute && numericMeasures.length > 1

            instance.current.setOption({
                tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
                legend: { data: numericMeasures.map((m) => getLabel(m, lang)) },
                grid: { left: "3%", right: needDualAxis ? "8%" : "4%", bottom: "3%", containLabel: true },
                xAxis: {
                    type: "category",
                    data: labels.slice(0, 50),
                    axisLabel: { rotate: labels[0]?.length > 8 ? 30 : 0, fontSize: 11 },
                },
                yAxis: needDualAxis
                    ? [
                        { type: "value", name: lang.startsWith("zh") ? "数值" : "Value" },
                        { type: "value", name: "%", max: 100, min: 0 },
                    ]
                    : { type: "value" },
                series: numericMeasures.map((m, i) => ({
                    name: getLabel(m, lang),
                    type: chartType,
                    yAxisIndex: needDualAxis && PERCENTAGE_FIELDS.has(m) ? 1 : 0,
                    data: data.rows.slice(0, 50).map((row) => Number(row[m] ?? 0)),
                    itemStyle: { color: CHART_COLORS[i % CHART_COLORS.length] },
                    smooth: chartType === "line",
                })),
            }, true)
        }

        const handleResize = () => instance.current?.resize()
        window.addEventListener("resize", handleResize)
        return () => {
            window.removeEventListener("resize", handleResize)
        }
    }, [data, dimensions, measures, chartType, lang])

    // Clean up chart on unmount
    useEffect(() => {
        return () => {
            instance.current?.dispose()
            instance.current = null
        }
    }, [])

    return <div ref={chartRef} className="w-full h-[400px]" />
}

// ─── Pivot table component ───────────────────────────────────────────────────

function PivotTable({
    data,
    dim1,
    dim2,
    measures,
    selectedMeasure,
    onMeasureChange,
    lang,
    t,
}: {
    data: CustomReportResponse
    dim1: string
    dim2: string
    measures: string[]
    selectedMeasure: string
    onMeasureChange: (m: string) => void
    lang: string
    t: TFunction
}) {
    // Build pivot map: dim1Value -> dim2Value -> measure value
    const pivotMap = new Map<string, Map<string, unknown>>()
    const dim2Values = new Set<string>()

    for (const row of data.rows) {
        const d1 = String(row[dim1] ?? "")
        const d2 = String(row[dim2] ?? "")
        dim2Values.add(d2)
        if (!pivotMap.has(d1)) pivotMap.set(d1, new Map())
        pivotMap.get(d1)!.set(d2, row[selectedMeasure])
    }

    const dim1Keys = Array.from(pivotMap.keys())
    const dim2Keys = Array.from(dim2Values)

    return (
        <div>
            {measures.length > 1 && (
                <div className="flex items-center gap-2 p-4 pb-2">
                    <span className="text-sm text-muted-foreground">{t("enterprise.customReport.pivotMeasure")}:</span>
                    <Select value={selectedMeasure} onValueChange={onMeasureChange}>
                        <SelectTrigger className="w-[200px] h-8">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {measures.map((m) => (
                                <SelectItem key={m} value={m}>{getLabel(m, lang)}</SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
            )}
            <div className="overflow-x-auto">
                <table className="w-full text-sm">
                    <thead>
                        <tr className="border-b bg-muted/40">
                            <th className="px-4 py-3 text-left font-medium text-muted-foreground whitespace-nowrap sticky left-0 bg-muted/40 z-10">
                                {getLabel(dim1, lang)} \ {getLabel(dim2, lang)}
                            </th>
                            {dim2Keys.map((d2) => (
                                <th key={d2} className="px-4 py-3 text-right font-medium text-muted-foreground whitespace-nowrap">
                                    {formatCellValue(dim2, d2)}
                                </th>
                            ))}
                        </tr>
                    </thead>
                    <tbody>
                        {dim1Keys.map((d1) => (
                            <tr key={d1} className="border-b last:border-0 hover:bg-muted/20 transition-colors">
                                <td className="px-4 py-2.5 font-medium whitespace-nowrap sticky left-0 bg-background z-10">
                                    {formatCellValue(dim1, d1)}
                                </td>
                                {dim2Keys.map((d2) => (
                                    <td key={d2} className="px-4 py-2.5 text-right whitespace-nowrap">
                                        {formatCellValue(selectedMeasure, pivotMap.get(d1)?.get(d2))}
                                    </td>
                                ))}
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    )
}

// ─── Department filter popover ───────────────────────────────────────────────

function DepartmentFilter({
    selected,
    onChange,
    lang,
    t,
    timeRange,
}: {
    selected: string[]
    onChange: (ids: string[]) => void
    lang: string
    t: TFunction
    timeRange: TimeRange
}) {
    const [open, setOpen] = useState(false)
    const { start, end } = getTimeRange(timeRange)

    const { data: deptData } = useQuery({
        queryKey: ["enterprise", "departments", start, end],
        queryFn: () => enterpriseApi.getDepartmentSummary(start, end),
    })

    const departments = deptData?.departments ?? []

    const toggleDept = (id: string) => {
        onChange(
            selected.includes(id)
                ? selected.filter((d) => d !== id)
                : [...selected, id],
        )
    }

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button variant="outline" size="sm" className="gap-1.5">
                    <Filter className="w-3.5 h-3.5" />
                    {t("enterprise.customReport.filterDepartments")}
                    {selected.length > 0 && (
                        <Badge variant="secondary" className="ml-1 px-1.5 py-0 text-xs">
                            {selected.length}
                        </Badge>
                    )}
                    <ChevronsUpDown className="w-3.5 h-3.5 opacity-50" />
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[280px] p-0" align="start">
                <div className="flex items-center justify-between px-3 py-2 border-b">
                    <span className="text-xs text-muted-foreground">
                        {selected.length > 0
                            ? `${selected.length} ${lang.startsWith("zh") ? "个已选" : "selected"}`
                            : t("enterprise.customReport.allDepartments")}
                    </span>
                    <div className="flex gap-1">
                        {selected.length > 0 ? (
                            <Button variant="ghost" size="sm" className="h-6 text-xs px-2" onClick={() => onChange([])}>
                                {t("enterprise.customReport.clearSelection")}
                            </Button>
                        ) : (
                            <Button
                                variant="ghost"
                                size="sm"
                                className="h-6 text-xs px-2"
                                onClick={() => onChange(departments.map((d) => d.department_id))}
                            >
                                {t("enterprise.customReport.selectAll")}
                            </Button>
                        )}
                    </div>
                </div>
                <div className="max-h-[240px] overflow-y-auto py-1">
                    {departments.map((dept) => {
                        const isSelected = selected.includes(dept.department_id)
                        return (
                            <button
                                key={dept.department_id}
                                type="button"
                                className="w-full flex items-center gap-2 px-3 py-1.5 text-sm hover:bg-muted/50 transition-colors text-left"
                                onClick={() => toggleDept(dept.department_id)}
                            >
                                <div className={`w-4 h-4 rounded border flex items-center justify-center ${
                                    isSelected ? "bg-[#6A6DE6] border-[#6A6DE6]" : "border-muted-foreground/30"
                                }`}>
                                    {isSelected && <Check className="w-3 h-3 text-white" />}
                                </div>
                                <span className="truncate">{dept.department_name}</span>
                                <span className="text-xs text-muted-foreground ml-auto">{dept.member_count}</span>
                            </button>
                        )
                    })}
                    {departments.length === 0 && (
                        <div className="text-center text-muted-foreground text-sm py-4">
                            {t("enterprise.customReport.allDepartments")}
                        </div>
                    )}
                </div>
            </PopoverContent>
        </Popover>
    )
}

// ─── Main page component ─────────────────────────────────────────────────────

export default function EnterpriseCustomReport() {
    const { t, i18n } = useTranslation()
    const lang = i18n.language

    // State
    const [selectedDimensions, setSelectedDimensions] = useState<string[]>(["department"])
    const [selectedMeasures, setSelectedMeasures] = useState<string[]>(["request_count", "used_amount"])
    const [timeRange, setTimeRange] = useState<TimeRange>("7d")
    const [viewMode, setViewMode] = useState<"table" | "chart" | "pivot">("table")
    const [chartType, setChartType] = useState<"bar" | "line" | "pie">("bar")
    const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set(["requests", "computed"]))
    const [reportData, setReportData] = useState<CustomReportResponse | null>(null)
    const [sortBy, setSortBy] = useState<string | undefined>()
    const [sortOrder, setSortOrder] = useState<"asc" | "desc">("desc")
    const [pivotMeasure, setPivotMeasure] = useState<string>("")

    // Filter state
    const [filterDepts, setFilterDepts] = useState<string[]>([])
    const [filterModels, setFilterModels] = useState<string[]>([])
    const [filterUsers, setFilterUsers] = useState<string[]>([])

    // Ref to track pending template apply for auto-generate
    const pendingGenerate = useRef(false)

    // Generate report mutation
    const mutation = useMutation({
        mutationFn: (req: CustomReportRequest) => enterpriseApi.generateCustomReport(req),
        onSuccess: (data) => setReportData(data),
    })
    // Stable ref for mutate to avoid re-creating handleGenerate on mutation state changes
    const mutateRef = useRef(mutation.mutate)
    mutateRef.current = mutation.mutate

    const handleGenerate = useCallback((
        dims?: string[],
        meas?: string[],
        fDepts?: string[],
        fModels?: string[],
        fUsers?: string[],
    ) => {
        const d = dims ?? selectedDimensions
        const m = meas ?? selectedMeasures
        if (d.length === 0 || m.length === 0) return

        // Auto-switch to line chart for time dimensions
        const hasTimeDim = d.some((dim) => dim.startsWith("time_"))
        if (hasTimeDim) {
            setChartType("line")
            setViewMode("chart")
        }

        const { start, end } = getTimeRange(timeRange)
        const filters: CustomReportRequest["filters"] = {}
        const fd = fDepts ?? filterDepts
        const fm = fModels ?? filterModels
        const fu = fUsers ?? filterUsers
        if (fd.length > 0) filters.department_ids = fd
        if (fm.length > 0) filters.models = fm
        if (fu.length > 0) filters.user_names = fu

        const req: CustomReportRequest = {
            dimensions: d,
            measures: m,
            filters,
            time_range: { start_timestamp: start, end_timestamp: end },
            sort_by: sortBy,
            sort_order: sortOrder,
            limit: 200,
        }
        mutateRef.current(req)
    }, [selectedDimensions, selectedMeasures, timeRange, filterDepts, filterModels, filterUsers, sortBy, sortOrder])

    // Handle template click
    const applyTemplate = useCallback((template: ReportTemplate) => {
        setSelectedDimensions(template.dimensions)
        setSelectedMeasures(template.measures)
        setPivotMeasure("")
        pendingGenerate.current = true
    }, [])

    // Effect: trigger generate after template state is applied
    useEffect(() => {
        if (pendingGenerate.current) {
            pendingGenerate.current = false
            handleGenerate(selectedDimensions, selectedMeasures, filterDepts, filterModels, filterUsers)
        }
    }, [selectedDimensions, selectedMeasures, handleGenerate, filterDepts, filterModels, filterUsers])

    const toggleCategory = (cat: string) => {
        setExpandedCategories((prev) => {
            const next = new Set(prev)
            if (next.has(cat)) next.delete(cat)
            else next.add(cat)
            return next
        })
    }

    // CSV export
    const handleExportCsv = () => {
        if (!reportData || reportData.rows.length === 0) return
        const cols = reportData.columns
        const header = cols.map((c) => c.label).join(",")
        const rows = reportData.rows.map((row) =>
            cols.map((c) => {
                const v = row[c.key]
                const s = String(v ?? "")
                // Escape quotes and wrap if contains comma, quote, or newline
                if (s.includes(",") || s.includes('"') || s.includes("\n")) {
                    return `"${s.replace(/"/g, '""')}"`
                }
                return s
            }).join(","),
        )
        const bom = "\uFEFF"
        const csv = bom + header + "\n" + rows.join("\n")
        const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" })
        const url = URL.createObjectURL(blob)
        const a = document.createElement("a")
        a.href = url
        a.download = `custom_report_${new Date().toISOString().slice(0, 10)}.csv`
        a.click()
        URL.revokeObjectURL(url)
    }

    // Group measures by category
    const measuresByCategory = CATEGORIES.map((cat) => ({
        category: cat,
        fields: MEASURE_FIELDS.filter((f) => f.category === cat),
    }))

    const canGenerate = selectedDimensions.length > 0 && selectedMeasures.length > 0
    const canPivot = selectedDimensions.length === 2

    // Auto-reset viewMode when pivot is no longer available
    useEffect(() => {
        if (!canPivot && viewMode === "pivot") {
            setViewMode("table")
        }
    }, [canPivot, viewMode])

    // Determine active pivot measure
    const activePivotMeasure = pivotMeasure && selectedMeasures.includes(pivotMeasure)
        ? pivotMeasure
        : selectedMeasures[0] ?? ""

    return (
        <div className="p-6 space-y-6">
            {/* Header */}
            <div>
                <h1 className="text-2xl font-bold flex items-center gap-2">
                    <FileBarChart className="w-6 h-6 text-[#6A6DE6]" />
                    {t("enterprise.customReport.title")}
                </h1>
                <p className="text-muted-foreground mt-1">
                    {t("enterprise.customReport.description")}
                </p>
            </div>

            {/* Quick Start Templates */}
            <Card>
                <CardHeader className="pb-3">
                    <CardTitle className="text-base flex items-center gap-2">
                        <Zap className="w-4 h-4 text-amber-500" />
                        {t("enterprise.customReport.templates.title")}
                    </CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="flex flex-wrap gap-2">
                        {REPORT_TEMPLATES.map((tpl) => (
                            <Button
                                key={tpl.id}
                                variant="outline"
                                size="sm"
                                className="text-xs"
                                onClick={() => applyTemplate(tpl)}
                                disabled={mutation.isPending}
                            >
                                {t(tpl.labelKey as never)}
                            </Button>
                        ))}
                    </div>
                </CardContent>
            </Card>

            {/* Config panel */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                {/* Dimensions */}
                <Card>
                    <CardHeader className="pb-3">
                        <CardTitle className="text-base">{t("enterprise.customReport.dimensions")}</CardTitle>
                        <CardDescription>{t("enterprise.customReport.dimensionsDesc")}</CardDescription>
                    </CardHeader>
                    <CardContent>
                        <ChipSelector
                            fields={DIMENSION_FIELDS}
                            selected={selectedDimensions}
                            onChange={setSelectedDimensions}
                            lang={lang}
                        />
                    </CardContent>
                </Card>

                {/* Measures — collapsible by category */}
                <Card>
                    <CardHeader className="pb-3">
                        <CardTitle className="text-base">{t("enterprise.customReport.measures")}</CardTitle>
                        <CardDescription>{t("enterprise.customReport.measuresDesc")}</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-2">
                        {measuresByCategory.map(({ category, fields }) => (
                            <div key={category}>
                                <button
                                    type="button"
                                    className="flex items-center gap-1.5 text-sm font-medium text-muted-foreground hover:text-foreground transition-colors w-full text-left py-1"
                                    onClick={() => toggleCategory(category)}
                                >
                                    {expandedCategories.has(category) ? (
                                        <ChevronDown className="w-3.5 h-3.5" />
                                    ) : (
                                        <ChevronRight className="w-3.5 h-3.5" />
                                    )}
                                    {t(`enterprise.customReport.categories.${category}`)}
                                    <span className="text-xs text-muted-foreground/60 ml-1">
                                        ({fields.filter((f) => selectedMeasures.includes(f.key)).length}/{fields.length})
                                    </span>
                                </button>
                                {expandedCategories.has(category) && (
                                    <div className="ml-5 mt-1">
                                        <ChipSelector
                                            fields={fields}
                                            selected={selectedMeasures}
                                            onChange={setSelectedMeasures}
                                            lang={lang}
                                        />
                                    </div>
                                )}
                            </div>
                        ))}
                    </CardContent>
                </Card>
            </div>

            {/* Actions row + Filters */}
            <div className="flex flex-wrap items-center gap-3">
                <Select value={timeRange} onValueChange={(v) => setTimeRange(v as TimeRange)}>
                    <SelectTrigger className="w-[140px]">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="7d">{lang.startsWith("zh") ? "最近 7 天" : "Last 7 Days"}</SelectItem>
                        <SelectItem value="30d">{lang.startsWith("zh") ? "最近 30 天" : "Last 30 Days"}</SelectItem>
                        <SelectItem value="month">{lang.startsWith("zh") ? "本月" : "This Month"}</SelectItem>
                    </SelectContent>
                </Select>

                {/* Filter controls */}
                <DepartmentFilter
                    selected={filterDepts}
                    onChange={setFilterDepts}
                    lang={lang}
                    t={t}
                    timeRange={timeRange}
                />

                <Popover>
                    <PopoverTrigger asChild>
                        <Button variant="outline" size="sm" className="gap-1.5">
                            <Filter className="w-3.5 h-3.5" />
                            {t("enterprise.customReport.filterModels")}
                            {filterModels.length > 0 && (
                                <Badge variant="secondary" className="ml-1 px-1.5 py-0 text-xs">
                                    {filterModels.length}
                                </Badge>
                            )}
                        </Button>
                    </PopoverTrigger>
                    <PopoverContent className="w-[300px] p-3" align="start">
                        <div className="text-sm font-medium mb-2">{t("enterprise.customReport.filterModels")}</div>
                        <TagInput
                            values={filterModels}
                            onChange={setFilterModels}
                            placeholder={t("enterprise.customReport.addFilter")}
                        />
                    </PopoverContent>
                </Popover>

                <Popover>
                    <PopoverTrigger asChild>
                        <Button variant="outline" size="sm" className="gap-1.5">
                            <Filter className="w-3.5 h-3.5" />
                            {t("enterprise.customReport.filterUsers")}
                            {filterUsers.length > 0 && (
                                <Badge variant="secondary" className="ml-1 px-1.5 py-0 text-xs">
                                    {filterUsers.length}
                                </Badge>
                            )}
                        </Button>
                    </PopoverTrigger>
                    <PopoverContent className="w-[300px] p-3" align="start">
                        <div className="text-sm font-medium mb-2">{t("enterprise.customReport.filterUsers")}</div>
                        <TagInput
                            values={filterUsers}
                            onChange={setFilterUsers}
                            placeholder={t("enterprise.customReport.addFilter")}
                        />
                    </PopoverContent>
                </Popover>

                <Button
                    onClick={() => handleGenerate()}
                    disabled={!canGenerate || mutation.isPending}
                    className="bg-[#6A6DE6] hover:bg-[#5A5DD6] text-white"
                >
                    {mutation.isPending ? (
                        <>
                            <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                            {t("enterprise.customReport.generating")}
                        </>
                    ) : (
                        t("enterprise.customReport.generate")
                    )}
                </Button>

                {reportData && reportData.rows.length > 0 && (
                    <>
                        <div className="flex items-center border rounded-md overflow-hidden ml-auto">
                            <TooltipProvider>
                                <Tooltip>
                                    <TooltipTrigger asChild>
                                        <Button
                                            variant={viewMode === "table" ? "default" : "ghost"}
                                            size="sm"
                                            onClick={() => setViewMode("table")}
                                            className={viewMode === "table" ? "bg-[#6A6DE6] text-white rounded-none" : "rounded-none"}
                                        >
                                            <Table2 className="w-4 h-4" />
                                        </Button>
                                    </TooltipTrigger>
                                    <TooltipContent>{t("enterprise.customReport.tableView")}</TooltipContent>
                                </Tooltip>
                                <Tooltip>
                                    <TooltipTrigger asChild>
                                        <Button
                                            variant={viewMode === "chart" ? "default" : "ghost"}
                                            size="sm"
                                            onClick={() => setViewMode("chart")}
                                            className={viewMode === "chart" ? "bg-[#6A6DE6] text-white rounded-none" : "rounded-none"}
                                        >
                                            <BarChart3 className="w-4 h-4" />
                                        </Button>
                                    </TooltipTrigger>
                                    <TooltipContent>{t("enterprise.customReport.chartView")}</TooltipContent>
                                </Tooltip>
                                {canPivot && (
                                    <Tooltip>
                                        <TooltipTrigger asChild>
                                            <Button
                                                variant={viewMode === "pivot" ? "default" : "ghost"}
                                                size="sm"
                                                onClick={() => setViewMode("pivot")}
                                                className={viewMode === "pivot" ? "bg-[#6A6DE6] text-white rounded-none" : "rounded-none"}
                                            >
                                                <Grid3X3 className="w-4 h-4" />
                                            </Button>
                                        </TooltipTrigger>
                                        <TooltipContent>{t("enterprise.customReport.pivotView")}</TooltipContent>
                                    </Tooltip>
                                )}
                            </TooltipProvider>
                        </div>

                        {viewMode === "chart" && (
                            <div className="flex items-center gap-1 border rounded-md overflow-hidden">
                                {([
                                    ["bar", BarChart3],
                                    ["line", LineChart],
                                    ["pie", PieChart],
                                ] as const).map(([type, Icon]) => (
                                    <Button
                                        key={type}
                                        variant={chartType === type ? "default" : "ghost"}
                                        size="sm"
                                        onClick={() => setChartType(type)}
                                        className={chartType === type ? "bg-[#6A6DE6] text-white rounded-none" : "rounded-none"}
                                    >
                                        <Icon className="w-4 h-4" />
                                    </Button>
                                ))}
                            </div>
                        )}

                        <Button variant="outline" size="sm" onClick={handleExportCsv}>
                            <Download className="w-4 h-4 mr-1.5" />
                            {t("enterprise.customReport.exportCsv")}
                        </Button>
                    </>
                )}
            </div>

            {/* Error state */}
            {mutation.isError && (
                <Card className="border-destructive">
                    <CardContent className="py-4 text-center text-destructive">
                        {mutation.error instanceof Error ? mutation.error.message : String(mutation.error)}
                    </CardContent>
                </Card>
            )}

            {/* Results */}
            {reportData && reportData.rows.length > 0 && (
                <Card>
                    <CardHeader className="pb-2 pt-4 px-4">
                        <p className="text-xs text-muted-foreground">
                            {reportData.total} {lang.startsWith("zh") ? "条结果" : "results"}
                        </p>
                    </CardHeader>
                    <CardContent className="p-0">
                        {viewMode === "table" ? (
                            <div className="overflow-x-auto">
                                <table className="w-full text-sm">
                                    <thead>
                                        <tr className="border-b bg-muted/40">
                                            {reportData.columns.map((col) => (
                                                <th
                                                    key={col.key}
                                                    className="px-4 py-3 text-left font-medium text-muted-foreground cursor-pointer hover:text-foreground transition-colors whitespace-nowrap"
                                                    onClick={() => {
                                                        // Sort client-side for instant feedback
                                                        const newOrder = sortBy === col.key && sortOrder === "desc" ? "asc" : "desc"
                                                        const newSortBy = col.key
                                                        setSortBy(newSortBy)
                                                        setSortOrder(newOrder)

                                                        if (reportData) {
                                                            const sorted = [...reportData.rows].sort((a, b) => {
                                                                const va = Number(a[newSortBy]) || 0
                                                                const vb = Number(b[newSortBy]) || 0
                                                                if (va !== vb) return newOrder === "desc" ? vb - va : va - vb
                                                                return String(a[newSortBy] ?? "").localeCompare(String(b[newSortBy] ?? ""))
                                                            })
                                                            setReportData({ ...reportData, rows: sorted })
                                                        }
                                                    }}
                                                >
                                                    {getLabel(col.key, lang)}
                                                    {sortBy === col.key && (
                                                        <span className="ml-1 text-[#6A6DE6]">
                                                            {sortOrder === "asc" ? "↑" : "↓"}
                                                        </span>
                                                    )}
                                                </th>
                                            ))}
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {reportData.rows.map((row, i) => (
                                            <tr
                                                key={i}
                                                className="border-b last:border-0 hover:bg-muted/20 transition-colors"
                                            >
                                                {reportData.columns.map((col) => (
                                                    <td key={col.key} className="px-4 py-2.5 whitespace-nowrap">
                                                        {formatCellValue(col.key, row[col.key])}
                                                    </td>
                                                ))}
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        ) : viewMode === "pivot" && canPivot ? (
                            <PivotTable
                                data={reportData}
                                dim1={selectedDimensions[0]}
                                dim2={selectedDimensions[1]}
                                measures={selectedMeasures}
                                selectedMeasure={activePivotMeasure}
                                onMeasureChange={setPivotMeasure}
                                lang={lang}
                                t={t}
                            />
                        ) : (
                            <div className="p-4">
                                <ReportChart
                                    data={reportData}
                                    dimensions={selectedDimensions}
                                    measures={selectedMeasures}
                                    chartType={chartType}
                                    lang={lang}
                                />
                            </div>
                        )}
                    </CardContent>
                </Card>
            )}

            {/* Empty state */}
            {reportData && reportData.rows.length === 0 && (
                <Card>
                    <CardContent className="py-12 text-center text-muted-foreground">
                        {t("enterprise.customReport.noData")}
                    </CardContent>
                </Card>
            )}

            {/* Validation hints */}
            {!reportData && !mutation.isPending && (
                <Card className="border-dashed">
                    <CardContent className="py-12 text-center text-muted-foreground">
                        <FileBarChart className="w-10 h-10 mx-auto mb-3 opacity-40" />
                        <p>
                            {selectedDimensions.length === 0
                                ? t("enterprise.customReport.selectDimension")
                                : selectedMeasures.length === 0
                                  ? t("enterprise.customReport.selectMeasure")
                                  : t("enterprise.customReport.generate")}
                        </p>
                    </CardContent>
                </Card>
            )}
        </div>
    )
}
