
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

export type ViewMode = "table" | "chart" | "pivot" | "split" | "dashboard"

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
    { key: "cache_creation_count", category: "requests" },
    // tokens
    { key: "input_tokens", category: "tokens" },
    { key: "output_tokens", category: "tokens" },
    { key: "total_tokens", category: "tokens" },
    { key: "cached_tokens", category: "tokens" },
    { key: "reasoning_tokens", category: "tokens" },
    { key: "image_input_tokens", category: "tokens" },
    { key: "audio_input_tokens", category: "tokens" },
    { key: "web_search_count", category: "tokens" },
    // cost
    { key: "used_amount", category: "cost" },
    { key: "input_amount", category: "cost" },
    { key: "output_amount", category: "cost" },
    { key: "cached_amount", category: "cost" },
    { key: "image_input_amount", category: "cost" },
    { key: "audio_input_amount", category: "cost" },
    { key: "image_output_amount", category: "cost" },
    { key: "reasoning_amount", category: "cost" },
    { key: "cache_creation_amount", category: "cost" },
    { key: "web_search_amount", category: "cost" },
    // performance
    { key: "total_time_ms", category: "performance" },
    { key: "total_ttfb_ms", category: "performance" },
    // efficiency (per-request)
    { key: "avg_tokens_per_req", category: "efficiency" },
    { key: "avg_cost_per_req", category: "efficiency" },
    { key: "avg_input_per_req", category: "efficiency" },
    { key: "avg_output_per_req", category: "efficiency" },
    { key: "avg_cached_per_req", category: "efficiency" },
    { key: "avg_reasoning_per_req", category: "efficiency" },
    { key: "avg_latency", category: "efficiency" },
    { key: "avg_ttfb", category: "efficiency" },
    { key: "tokens_per_second", category: "efficiency" },
    { key: "output_speed", category: "efficiency" },
    // per-user
    { key: "avg_tokens_per_user", category: "per_user" },
    { key: "avg_cost_per_user", category: "per_user" },
    { key: "avg_requests_per_user", category: "per_user" },
    // rates
    { key: "success_rate", category: "rates" },
    { key: "error_rate", category: "rates" },
    { key: "exception_rate", category: "rates" },
    { key: "throttle_rate", category: "rates" },
    { key: "cache_hit_rate", category: "rates" },
    { key: "retry_rate", category: "rates" },
    { key: "output_input_ratio", category: "rates" },
    // cost structure
    { key: "input_cost_pct", category: "cost_structure" },
    { key: "output_cost_pct", category: "cost_structure" },
    { key: "cache_savings_pct", category: "cost_structure" },
    { key: "cost_per_1k_tokens", category: "cost_structure" },
    { key: "cost_per_input_1k", category: "cost_structure" },
    { key: "cost_per_output_1k", category: "cost_structure" },
    // statistics
    { key: "unique_models", category: "statistics" },
    { key: "active_users", category: "statistics" },
    { key: "reconciliation_tokens", category: "statistics" },
]

export const CATEGORIES = [
    "requests", "tokens", "cost", "performance",
    "efficiency", "per_user", "rates", "cost_structure", "statistics",
] as const

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
    cache_creation_count: { zh: "缓存创建次数", en: "Cache Creates" },
    // tokens
    input_tokens: { zh: "输入 Token", en: "Input Tokens" },
    output_tokens: { zh: "输出 Token", en: "Output Tokens" },
    total_tokens: { zh: "总 Token", en: "Total Tokens" },
    cached_tokens: { zh: "缓存 Token", en: "Cached Tokens" },
    reasoning_tokens: { zh: "推理 Token", en: "Reasoning Tokens" },
    image_input_tokens: { zh: "图片 Token", en: "Image Tokens" },
    audio_input_tokens: { zh: "音频 Token", en: "Audio Tokens" },
    web_search_count: { zh: "联网搜索", en: "Web Searches" },
    // cost
    used_amount: { zh: "总费用", en: "Total Cost" },
    input_amount: { zh: "输入费用", en: "Input Cost" },
    output_amount: { zh: "输出费用", en: "Output Cost" },
    cached_amount: { zh: "缓存费用", en: "Cache Cost" },
    image_input_amount: { zh: "图片输入费用", en: "Image Input Cost" },
    audio_input_amount: { zh: "音频输入费用", en: "Audio Input Cost" },
    image_output_amount: { zh: "图片输出费用", en: "Image Output Cost" },
    reasoning_amount: { zh: "推理费用", en: "Reasoning Cost" },
    cache_creation_amount: { zh: "缓存创建费用", en: "Cache Creation Cost" },
    web_search_amount: { zh: "联网搜索费用", en: "Web Search Cost" },
    // performance
    total_time_ms: { zh: "总耗时(ms)", en: "Total Time (ms)" },
    total_ttfb_ms: { zh: "总TTFB(ms)", en: "Total TTFB (ms)" },
    // efficiency (per-request)
    avg_tokens_per_req: { zh: "平均Token/请求", en: "Avg Tokens/Req" },
    avg_cost_per_req: { zh: "平均费用/请求", en: "Avg Cost/Req" },
    avg_input_per_req: { zh: "平均输入Token/请求", en: "Avg Input/Req" },
    avg_output_per_req: { zh: "平均输出Token/请求", en: "Avg Output/Req" },
    avg_cached_per_req: { zh: "平均缓存Token/请求", en: "Avg Cached/Req" },
    avg_reasoning_per_req: { zh: "平均推理Token/请求", en: "Avg Reasoning/Req" },
    avg_latency: { zh: "平均延迟(ms)", en: "Avg Latency (ms)" },
    avg_ttfb: { zh: "平均TTFB(ms)", en: "Avg TTFB (ms)" },
    tokens_per_second: { zh: "Token吞吐量(/s)", en: "Tokens/Second" },
    output_speed: { zh: "输出速度(token/s)", en: "Output Speed (t/s)" },
    // per-user
    avg_tokens_per_user: { zh: "人均Token", en: "Avg Tokens/User" },
    avg_cost_per_user: { zh: "人均费用", en: "Avg Cost/User" },
    avg_requests_per_user: { zh: "人均请求数", en: "Avg Requests/User" },
    // rates
    success_rate: { zh: "成功率 %", en: "Success Rate %" },
    error_rate: { zh: "错误率 %", en: "Error Rate %" },
    exception_rate: { zh: "异常率 %", en: "Exception Rate %" },
    throttle_rate: { zh: "限流率 %", en: "Throttle Rate %" },
    cache_hit_rate: { zh: "缓存命中率 %", en: "Cache Hit Rate %" },
    retry_rate: { zh: "重试率 %", en: "Retry Rate %" },
    output_input_ratio: { zh: "输出/输入比", en: "Output/Input Ratio" },
    // cost structure
    input_cost_pct: { zh: "输入费用占比 %", en: "Input Cost %" },
    output_cost_pct: { zh: "输出费用占比 %", en: "Output Cost %" },
    cache_savings_pct: { zh: "缓存费用占比 %", en: "Cache Cost %" },
    cost_per_1k_tokens: { zh: "千Token成本", en: "Cost/1K Tokens" },
    cost_per_input_1k: { zh: "千输入Token成本", en: "Cost/1K Input" },
    cost_per_output_1k: { zh: "千输出Token成本", en: "Cost/1K Output" },
    // statistics
    unique_models: { zh: "使用模型数", en: "Unique Models" },
    active_users: { zh: "活跃用户数", en: "Active Users" },
    reconciliation_tokens: { zh: "对账Token(不含缓存)", en: "Reconciliation Tokens" },
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
    if (key.endsWith("_rate") || key.endsWith("_pct")) return `${n.toFixed(2)}%`

    // cost fields
    if (key.includes("amount") || key === "avg_cost_per_req" || key === "avg_cost_per_user"
        || key === "cost_per_1k_tokens" || key === "cost_per_input_1k" || key === "cost_per_output_1k") {
        return `¥${n.toFixed(4)}`
    }

    // latency
    if (key.includes("latency") || key.includes("ttfb") || key.includes("time_ms")) {
        return `${n.toFixed(1)} ms`
    }

    // throughput (tokens/second)
    if (key === "tokens_per_second" || key === "output_speed") {
        return `${n.toFixed(1)} /s`
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
    "success_rate", "error_rate", "exception_rate", "throttle_rate",
    "cache_hit_rate", "retry_rate",
    "input_cost_pct", "output_cost_pct", "cache_savings_pct",
])

export const COST_FIELDS = new Set([
    "used_amount", "input_amount", "output_amount", "cached_amount",
    "image_input_amount", "audio_input_amount", "image_output_amount",
    "reasoning_amount", "cache_creation_amount", "web_search_amount",
    "avg_cost_per_req", "avg_cost_per_user",
    "cost_per_1k_tokens", "cost_per_input_1k", "cost_per_output_1k",
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

// ─── Category metadata ─────────────────────────────────────────────────────

export interface CategoryMeta {
    key: string
    labelZh: string
    labelEn: string
    color: string       // tailwind bg class
    textColor: string   // tailwind text class
}

export const CATEGORY_META: Record<string, CategoryMeta> = {
    requests:       { key: "requests",       labelZh: "请求与状态",   labelEn: "Requests",       color: "bg-blue-100 dark:bg-blue-900/30",     textColor: "text-blue-700 dark:text-blue-300" },
    tokens:         { key: "tokens",         labelZh: "Token 用量",   labelEn: "Tokens",         color: "bg-emerald-100 dark:bg-emerald-900/30", textColor: "text-emerald-700 dark:text-emerald-300" },
    cost:           { key: "cost",           labelZh: "费用明细",     labelEn: "Cost",           color: "bg-amber-100 dark:bg-amber-900/30",     textColor: "text-amber-700 dark:text-amber-300" },
    performance:    { key: "performance",    labelZh: "性能",         labelEn: "Performance",    color: "bg-red-100 dark:bg-red-900/30",         textColor: "text-red-700 dark:text-red-300" },
    efficiency:     { key: "efficiency",     labelZh: "效率指标",     labelEn: "Efficiency",     color: "bg-violet-100 dark:bg-violet-900/30",   textColor: "text-violet-700 dark:text-violet-300" },
    per_user:       { key: "per_user",       labelZh: "人效指标",     labelEn: "Per User",       color: "bg-pink-100 dark:bg-pink-900/30",       textColor: "text-pink-700 dark:text-pink-300" },
    rates:          { key: "rates",          labelZh: "比率指标",     labelEn: "Rates",          color: "bg-cyan-100 dark:bg-cyan-900/30",       textColor: "text-cyan-700 dark:text-cyan-300" },
    cost_structure: { key: "cost_structure", labelZh: "成本结构",     labelEn: "Cost Structure", color: "bg-orange-100 dark:bg-orange-900/30",   textColor: "text-orange-700 dark:text-orange-300" },
    statistics:     { key: "statistics",     labelZh: "统计",         labelEn: "Statistics",     color: "bg-gray-100 dark:bg-gray-800/30",       textColor: "text-gray-700 dark:text-gray-300" },
}

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
        const isRate = PERCENTAGE_FIELDS.has(key)
        let rawValue: number

        if (isRate) {
            // Weighted average using request_count as weight
            let weightedSum = 0
            let totalWeight = 0
            for (const r of rows) {
                const v = Number(r[key] ?? 0)
                const w = Number(r["request_count"] ?? 1)
                if (!Number.isNaN(v) && !Number.isNaN(w)) {
                    weightedSum += v * w
                    totalWeight += w
                }
            }
            rawValue = totalWeight > 0 ? weightedSum / totalWeight : 0
        } else {
            const values = rows.map((r) => Number(r[key] ?? 0)).filter((n) => !Number.isNaN(n))
            rawValue = values.reduce((a, b) => a + b, 0)
        }

        return {
            key,
            label: getLabel(key, lang),
            value: formatCellValue(key, rawValue),
            rawValue,
        }
    })
}
