import { useMemo } from "react"
import { useTranslation } from "react-i18next"
import { Hash, DollarSign, BarChart2, Users, Activity, Zap, Timer, Percent, TrendingUp, TrendingDown, Info } from "lucide-react"
import type { CustomReportResponse, ComparisonData } from "@/api/enterprise"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { computeKpis, COST_FIELDS, PERCENTAGE_FIELDS } from "./types"
import { AnimatedNumber } from "./AnimatedNumber"

const ICON_MAP: Record<string, React.ComponentType<{ className?: string }>> = {
    request_count: BarChart2,
    used_amount: DollarSign,
    total_tokens: Hash,
    active_users: Users,
    unique_models: Activity,
    success_rate: Percent,
    avg_latency: Timer,
    tokens_per_second: Zap,
    avg_cost_per_user: TrendingUp,
}

const GRADIENT_MAP: Record<string, string> = {
    request_count:     "from-blue-500/20 to-blue-600/10",
    used_amount:       "from-amber-500/20 to-amber-600/10",
    total_tokens:      "from-emerald-500/20 to-emerald-600/10",
    active_users:      "from-pink-500/20 to-pink-600/10",
    unique_models:     "from-violet-500/20 to-violet-600/10",
    input_tokens:      "from-teal-500/20 to-teal-600/10",
    output_tokens:     "from-cyan-500/20 to-cyan-600/10",
    avg_latency:       "from-red-500/20 to-red-600/10",
    success_rate:      "from-green-500/20 to-green-600/10",
    tokens_per_second: "from-yellow-500/20 to-yellow-600/10",
    output_speed:      "from-orange-500/20 to-orange-600/10",
}

const ICON_COLOR_MAP: Record<string, string> = {
    request_count:     "text-blue-600 dark:text-blue-400",
    used_amount:       "text-amber-600 dark:text-amber-400",
    total_tokens:      "text-emerald-600 dark:text-emerald-400",
    active_users:      "text-pink-600 dark:text-pink-400",
    unique_models:     "text-violet-600 dark:text-violet-400",
    tokens_per_second: "text-yellow-600 dark:text-yellow-400",
    output_speed:      "text-orange-600 dark:text-orange-400",
}

const formatTotalRows = (n: number) => Math.round(n).toLocaleString()

// KPI cards show backend `totals` (globally exact aggregation) when available.
// This differs from Σ(rows[k]) for distinct-count measures, because the table
// rows each report COUNT(DISTINCT …) *within their dimension bucket*: a user
// active on two models appears in two rows with active_users=1 each, while the
// KPI shows 1 (true distinct). Surface this discrepancy via tooltip on the
// cards where it matters, so users don't read the mismatch as a bug.
const KPI_HINTS: Record<string, { zh: string; en: string }> = {
    input_tokens: {
        zh: "含缓存读取 + 缓存创建 Token（对齐 OpenAI prompt_tokens 语义）。如需不含缓存的净值请使用「对账 Token」。",
        en: "Includes cached + cache-creation tokens (OpenAI prompt_tokens semantics). Use 'Reconciliation Tokens' for the non-cached portion.",
    },
    active_users: {
        zh: "全局真实去重：同一用户跨多个模型/日期只计一次。表格行内各自按该维度去重，行求和可能大于此值。",
        en: "Globally distinct users. Table rows each show COUNT(DISTINCT) within their dimension bucket, so summing rows may exceed this value.",
    },
    unique_models: {
        zh: "全局真实去重：同一模型跨多个部门/日期只计一次。表格行内各自按该维度去重，行求和可能大于此值。",
        en: "Globally distinct models. Table rows each show COUNT(DISTINCT) within their bucket, so summing rows may exceed this value.",
    },
    avg_cost_per_user: {
        zh: "分母 = 全局活跃用户数（真实去重）。表格行中的『活跃用户人均』分母则限定在该行维度桶内。",
        en: "Denominator is the global active-user count. Per-row values use the bucket-local active_users denominator.",
    },
    avg_tokens_per_user: {
        zh: "分母 = 全局活跃用户数（真实去重）。表格行中的『活跃用户人均』分母则限定在该行维度桶内。",
        en: "Denominator is the global active-user count. Per-row values use the bucket-local active_users denominator.",
    },
    avg_requests_per_user: {
        zh: "分母 = 全局活跃用户数（真实去重）。表格行中的『活跃用户人均』分母则限定在该行维度桶内。",
        en: "Denominator is the global active-user count. Per-row values use the bucket-local active_users denominator.",
    },
    tokens_per_second: {
        zh: "SUM(tokens)/SUM(wall_time)：单请求平均速率；并发下低于系统真实 TPS。",
        en: "SUM(tokens)/SUM(wall_time) — a per-request avg rate; under concurrency it is lower than true system TPS.",
    },
    output_speed: {
        zh: "SUM(output_tokens)/SUM(wall_time)：单请求平均输出速率，非系统吞吐。",
        en: "SUM(output_tokens)/SUM(wall_time) — per-request avg output rate, not system throughput.",
    },
}

// Map KPI keys to comparison API pct fields.
// Only these 4 metrics are supported by the comparison API.
const COMPARISON_MAP: Record<string, keyof ComparisonData["changes"]> = {
    request_count: "request_count_pct",
    total_tokens: "total_tokens_pct",
    used_amount: "used_amount_pct",
    active_users: "active_users_pct",
}

// Cost going up = bad (red), users/requests going up = good (green)
const COST_UP_IS_BAD = new Set(["used_amount"])

function formatKpiValue(key: string, n: number): string {
    if (Number.isNaN(n)) return "-"
    if (COST_FIELDS.has(key)) return `¥${n.toFixed(4)}`
    if (PERCENTAGE_FIELDS.has(key)) return `${n.toFixed(2)}%`
    // Abbreviate large integers for KPI card readability (table keeps full precision)
    if (Number.isInteger(n) && n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
    if (Number.isInteger(n) && n >= 10_000) return `${(n / 1_000).toFixed(1)}K`
    if (!Number.isInteger(n)) return n.toFixed(2)
    return n.toLocaleString()
}

export function KpiSummaryRow({
    data,
    measures,
    comparison,
}: {
    data: CustomReportResponse
    measures: string[]
    comparison?: ComparisonData
}) {
    const { t, i18n } = useTranslation()
    const lang = i18n.language
    const isZh = lang.startsWith("zh")
    const kpis = useMemo(
        () => computeKpis(data.rows, measures, lang, data.totals),
        [data.rows, measures, lang, data.totals],
    )
    const totalRows = data.total

    // Pre-create stable formatter references for AnimatedNumber (avoids hooks-in-loop)
    const formatters = useMemo(
        () => new Map(kpis.map((kpi) => [kpi.key, (n: number) => formatKpiValue(kpi.key, n)])),
        [kpis],
    )

    return (
        <TooltipProvider delayDuration={100}>
        <div className="grid grid-cols-2 lg:grid-cols-5 gap-3">
            {/* Row count card */}
            <div className="rounded-xl border border-white/20 dark:border-gray-700/30 backdrop-blur-xl bg-white/60 dark:bg-gray-900/60 p-4 shadow-sm">
                <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0">
                        <p className="text-xs text-muted-foreground">{t("enterprise.customReport.totalRows")}</p>
                        <p className="text-xl font-bold mt-0.5 tabular-nums truncate">
                            <AnimatedNumber value={totalRows} format={formatTotalRows} />
                        </p>
                    </div>
                    <div className="w-9 h-9 rounded-lg bg-gradient-to-br from-[#6A6DE6]/20 to-[#8A8DF7]/10 flex items-center justify-center shrink-0">
                        <Hash className="w-4 h-4 text-[#6A6DE6]" />
                    </div>
                </div>
            </div>

            {/* KPI cards */}
            {kpis.map((kpi) => {
                const Icon = ICON_MAP[kpi.key] ?? BarChart2
                const gradient = GRADIENT_MAP[kpi.key] ?? "from-[#6A6DE6]/20 to-[#8A8DF7]/10"
                const iconColor = ICON_COLOR_MAP[kpi.key] ?? "text-[#6A6DE6]"
                const formatter = formatters.get(kpi.key)!
                const hint = KPI_HINTS[kpi.key]
                const hintText = hint ? (isZh ? hint.zh : hint.en) : null

                // Period-over-period comparison
                const compField = COMPARISON_MAP[kpi.key]
                const compPct = comparison && compField ? comparison.changes[compField] : undefined
                const hasComp = compPct !== undefined && !Number.isNaN(compPct)

                return (
                    <div
                        key={kpi.key}
                        className="rounded-xl border border-white/20 dark:border-gray-700/30 backdrop-blur-xl bg-white/60 dark:bg-gray-900/60 p-4 shadow-sm"
                    >
                        <div className="flex items-center justify-between gap-2">
                            <div className="min-w-0">
                                <div className="flex items-center gap-1 min-w-0">
                                    <p className="text-xs text-muted-foreground truncate">{kpi.label}</p>
                                    {hintText && (
                                        <Tooltip>
                                            <TooltipTrigger asChild>
                                                <Info className="w-3 h-3 text-muted-foreground/60 shrink-0 cursor-help" />
                                            </TooltipTrigger>
                                            <TooltipContent className="max-w-xs text-xs leading-relaxed">
                                                {hintText}
                                            </TooltipContent>
                                        </Tooltip>
                                    )}
                                </div>
                                <p className="text-xl font-bold mt-0.5 tabular-nums truncate">
                                    <AnimatedNumber value={kpi.rawValue} format={formatter} />
                                </p>
                                {hasComp && (
                                    <div className={`flex items-center gap-0.5 text-xs mt-0.5 ${
                                        compPct === 0
                                            ? "text-muted-foreground"
                                            : (compPct > 0) === COST_UP_IS_BAD.has(kpi.key)
                                                ? "text-red-500 dark:text-red-400"
                                                : "text-green-500 dark:text-green-400"
                                    }`}>
                                        {compPct > 0 ? (
                                            <TrendingUp className="w-3 h-3" />
                                        ) : compPct < 0 ? (
                                            <TrendingDown className="w-3 h-3" />
                                        ) : null}
                                        <span>{compPct > 0 ? "+" : ""}{compPct.toFixed(1)}%</span>
                                    </div>
                                )}
                            </div>
                            <div className={`w-9 h-9 rounded-lg bg-gradient-to-br ${gradient} flex items-center justify-center shrink-0`}>
                                <Icon className={`w-4 h-4 ${iconColor}`} />
                            </div>
                        </div>
                    </div>
                )
            })}
        </div>
        </TooltipProvider>
    )
}
