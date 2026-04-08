
// ─── Chart type definitions ─────────────────────────────────────────────────

export type ChartType =
    | "auto"
    | "bar"
    | "stacked_bar"
    | "line"
    | "area"
    | "pie"
    | "heatmap"
    | "treemap"
    | "radar"

export type ViewMode = "table" | "chart" | "pivot" | "split"

// ─── Field catalog ──────────────────────────────────────────────────────────

export interface FieldDef {
    key: string
    category: string
}

export const DIMENSION_FIELDS: FieldDef[] = [
    { key: "user_name", category: "identity" },
    { key: "department", category: "identity" },
    { key: "level1_department", category: "identity" },
    { key: "level2_department", category: "identity" },
    { key: "model", category: "identity" },
    { key: "time_day", category: "time" },
    { key: "time_week", category: "time" },
    { key: "time_hour", category: "time" },
]

export const MEASURE_FIELDS: FieldDef[] = [
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
    { key: "reconciliation_tokens", category: "computed" },
]

export const CATEGORIES = ["requests", "tokens", "cost", "performance", "computed"] as const

// ─── Field labels ───────────────────────────────────────────────────────────

const FIELD_LABELS: Record<string, { zh: string; en: string }> = {
    // dimensions
    user_name: { zh: "用户名", en: "User" },
    department: { zh: "部门", en: "Department" },
    level1_department: { zh: "一级部门", en: "Level 1 Dept" },
    level2_department: { zh: "二级部门", en: "Level 2 Dept" },
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
    reconciliation_tokens: { zh: "对账 Token (不含缓存)", en: "Reconciliation Tokens" },
}

export function getLabel(key: string, lang: string): string {
    const entry = FIELD_LABELS[key]
    if (!entry) return key
    return lang.startsWith("zh") ? entry.zh : entry.en
}

// ─── Cell formatting ────────────────────────────────────────────────────────

export function formatCellValue(key: string, value: unknown): string {
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
        return `¥${n.toFixed(4)}`
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

// ─── Dimension value formatting (for chart labels) ─────────────────────────

export const TIME_DIMENSIONS = new Set(["time_hour", "time_day", "time_week"])

export function formatDimValue(dimKey: string, value: unknown): string {
    if (value == null) return "-"
    if (TIME_DIMENSIONS.has(dimKey)) {
        const n = Number(value)
        if (Number.isNaN(n) || n === 0) return String(value)
        const d = new Date(n * 1000)
        if (dimKey === "time_hour") {
            return d.toLocaleString(undefined, { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" })
        }
        if (dimKey === "time_week") {
            const end = new Date(d.getTime() + 6 * 86400000)
            return `${d.toLocaleDateString(undefined, { month: "numeric", day: "numeric" })}~${end.toLocaleDateString(undefined, { month: "numeric", day: "numeric" })}`
        }
        return d.toLocaleDateString(undefined, { year: "numeric", month: "numeric", day: "numeric" })
    }
    return String(value)
}

// ─── Time-aware sorting ─────────────────────────────────────────────────────

/** Sort rows by time dimension (ascending) if present, otherwise return as-is */
export function sortRowsByTime(
    rows: Record<string, unknown>[],
    dimensions: string[],
): Record<string, unknown>[] {
    const timeDim = dimensions.find((d) => TIME_DIMENSIONS.has(d))
    if (!timeDim) return rows
    return [...rows].sort((a, b) => Number(a[timeDim] ?? 0) - Number(b[timeDim] ?? 0))
}

/** Sort string keys that may represent time dimension values */
export function sortDimKeys(keys: string[], dimKey: string): string[] {
    if (!TIME_DIMENSIONS.has(dimKey)) return keys
    return [...keys].sort((a, b) => Number(a) - Number(b))
}

// ─── Percentage fields set ──────────────────────────────────────────────────

export const PERCENTAGE_FIELDS = new Set([
    "success_rate", "error_rate", "throttle_rate", "cache_hit_rate", "retry_rate",
])

export const COST_FIELDS = new Set([
    "used_amount", "input_amount", "output_amount", "cached_amount",
    "avg_cost_per_req", "cost_per_1k_tokens",
])

// ─── Report templates ───────────────────────────────────────────────────────

export interface ReportTemplate {
    id: string
    labelKey: string
    dimensions: string[]
    measures: string[]
}

export const REPORT_TEMPLATES: ReportTemplate[] = [
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

// ─── Chart colors ───────────────────────────────────────────────────────────

export const CHART_COLORS = [
    "#6A6DE6", "#8A8DF7", "#4ECDC4", "#FF6B6B", "#FFD93D",
    "#6BCB77", "#FF8E53", "#A78BFA", "#F472B6", "#38BDF8",
]

// ─── Smart chart recommendation ─────────────────────────────────────────────

export function recommendChartType(dimensions: string[], measures: string[]): ChartType {
    const hasTimeDim = dimensions.some((d) => d.startsWith("time_"))
    const categoryDims = dimensions.filter((d) => !d.startsWith("time_"))

    // time + another dimension → stacked bar
    if (hasTimeDim && categoryDims.length >= 1) return "stacked_bar"
    // time only → line
    if (hasTimeDim) return "line"
    // 2 category dims + 1 measure → heatmap
    if (categoryDims.length === 2 && measures.length === 1) return "heatmap"
    // 1 dim + ≥3 measures → radar
    if (dimensions.length === 1 && measures.length >= 3) return "radar"
    // 1 dim + 1 measure + few categories → pie
    if (dimensions.length === 1 && measures.length === 1) return "pie"
    // default
    return "bar"
}

// ─── Chart type metadata for picker ─────────────────────────────────────────

export interface ChartTypeInfo {
    type: ChartType
    labelKey: string
    icon: string // emoji as simple icon
}

export const CHART_TYPE_OPTIONS: ChartTypeInfo[] = [
    { type: "auto", labelKey: "enterprise.customReport.autoChart", icon: "✨" },
    { type: "bar", labelKey: "enterprise.customReport.barChart", icon: "📊" },
    { type: "stacked_bar", labelKey: "enterprise.customReport.stackedBarChart", icon: "📊" },
    { type: "line", labelKey: "enterprise.customReport.lineChart", icon: "📈" },
    { type: "area", labelKey: "enterprise.customReport.areaChart", icon: "📉" },
    { type: "pie", labelKey: "enterprise.customReport.pieChart", icon: "🥧" },
    { type: "heatmap", labelKey: "enterprise.customReport.heatmapChart", icon: "🟧" },
    { type: "treemap", labelKey: "enterprise.customReport.treemapChart", icon: "🌳" },
    { type: "radar", labelKey: "enterprise.customReport.radarChart", icon: "🕸️" },
]

// ─── KPI helpers ────────────────────────────────────────────────────────────

export interface KpiItem {
    key: string
    label: string
    value: string
    rawValue: number
}

const KPI_PRIORITY = [
    "used_amount", "request_count", "total_tokens",
    "active_users", "unique_models", "success_rate",
    "input_tokens", "output_tokens", "avg_latency",
]

export function computeKpis(
    rows: Record<string, unknown>[],
    measures: string[],
    lang: string,
): KpiItem[] {
    if (rows.length === 0) return []

    // Pick up to 4 measures in priority order
    const ordered = KPI_PRIORITY.filter((k) => measures.includes(k))
    const remaining = measures.filter((k) => !ordered.includes(k))
    const picked = [...ordered, ...remaining].slice(0, 4)

    return picked.map((key) => {
        const values = rows.map((r) => Number(r[key] ?? 0)).filter((n) => !Number.isNaN(n))
        const sum = values.reduce((a, b) => a + b, 0)
        const isRate = PERCENTAGE_FIELDS.has(key)
        const rawValue = isRate ? (values.length > 0 ? sum / values.length : 0) : sum

        return {
            key,
            label: getLabel(key, lang),
            value: formatCellValue(key, rawValue),
            rawValue,
        }
    })
}
