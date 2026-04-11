import { useMemo } from "react"
import { useTranslation } from "react-i18next"
import { Hash, DollarSign, BarChart2, Users, Activity, Zap, Timer, Percent, TrendingUp } from "lucide-react"
import type { CustomReportResponse } from "@/api/enterprise"
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
}

const ICON_COLOR_MAP: Record<string, string> = {
    request_count:  "text-blue-600 dark:text-blue-400",
    used_amount:    "text-amber-600 dark:text-amber-400",
    total_tokens:   "text-emerald-600 dark:text-emerald-400",
    active_users:   "text-pink-600 dark:text-pink-400",
    unique_models:  "text-violet-600 dark:text-violet-400",
}

const formatTotalRows = (n: number) => Math.round(n).toLocaleString()

function formatKpiValue(key: string, n: number): string {
    if (COST_FIELDS.has(key)) return `¥${n.toFixed(4)}`
    if (PERCENTAGE_FIELDS.has(key)) return `${n.toFixed(2)}%`
    if (Number.isInteger(n) && n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
    if (Number.isInteger(n) && n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
    if (!Number.isInteger(n)) return n.toFixed(2)
    return n.toLocaleString()
}

export function KpiSummaryRow({
    data,
    measures,
}: {
    data: CustomReportResponse
    measures: string[]
}) {
    const { t, i18n } = useTranslation()
    const lang = i18n.language
    const kpis = useMemo(() => computeKpis(data.rows, measures, lang), [data.rows, measures, lang])
    const totalRows = data.total

    // Pre-create stable formatter references for AnimatedNumber (avoids hooks-in-loop)
    const formatters = useMemo(
        () => new Map(kpis.map((kpi) => [kpi.key, (n: number) => formatKpiValue(kpi.key, n)])),
        [kpis],
    )

    return (
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

                return (
                    <div
                        key={kpi.key}
                        className="rounded-xl border border-white/20 dark:border-gray-700/30 backdrop-blur-xl bg-white/60 dark:bg-gray-900/60 p-4 shadow-sm"
                    >
                        <div className="flex items-center justify-between gap-2">
                            <div className="min-w-0">
                                <p className="text-xs text-muted-foreground truncate">{kpi.label}</p>
                                <p className="text-xl font-bold mt-0.5 tabular-nums truncate">
                                    <AnimatedNumber value={kpi.rawValue} format={formatter} />
                                </p>
                            </div>
                            <div className={`w-9 h-9 rounded-lg bg-gradient-to-br ${gradient} flex items-center justify-center shrink-0`}>
                                <Icon className={`w-4 h-4 ${iconColor}`} />
                            </div>
                        </div>
                    </div>
                )
            })}
        </div>
    )
}
