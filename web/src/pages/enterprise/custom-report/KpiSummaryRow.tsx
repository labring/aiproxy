import { useTranslation } from "react-i18next"
import { Hash, DollarSign, BarChart2, Users, Activity } from "lucide-react"
import { Card, CardContent } from "@/components/ui/card"
import type { CustomReportResponse } from "@/api/enterprise"
import { computeKpis } from "./types"

const ICON_MAP: Record<string, React.ComponentType<{ className?: string }>> = {
    request_count: BarChart2,
    used_amount: DollarSign,
    total_tokens: Hash,
    active_users: Users,
    unique_models: Activity,
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

    const kpis = computeKpis(data.rows, measures, lang)

    // Always include total rows
    const totalRows = data.total

    return (
        <div className="grid grid-cols-2 lg:grid-cols-5 gap-3">
            {/* Row count card */}
            <Card className="border border-gray-100 dark:border-gray-800">
                <CardContent className="p-4">
                    <div className="flex items-center justify-between">
                        <div>
                            <p className="text-xs text-muted-foreground">{t("enterprise.customReport.totalRows")}</p>
                            <p className="text-xl font-bold mt-0.5">{totalRows.toLocaleString()}</p>
                        </div>
                        <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-[#6A6DE6]/10 to-[#8A8DF7]/10 flex items-center justify-center">
                            <Hash className="w-4 h-4 text-[#6A6DE6]" />
                        </div>
                    </div>
                </CardContent>
            </Card>

            {/* KPI cards */}
            {kpis.map((kpi) => {
                const Icon = ICON_MAP[kpi.key] ?? BarChart2
                return (
                    <Card key={kpi.key} className="border border-gray-100 dark:border-gray-800">
                        <CardContent className="p-4">
                            <div className="flex items-center justify-between">
                                <div>
                                    <p className="text-xs text-muted-foreground">{kpi.label}</p>
                                    <p className="text-xl font-bold mt-0.5">{kpi.value}</p>
                                </div>
                                <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-[#6A6DE6]/10 to-[#8A8DF7]/10 flex items-center justify-center">
                                    <Icon className="w-4 h-4 text-[#6A6DE6]" />
                                </div>
                            </div>
                        </CardContent>
                    </Card>
                )
            })}
        </div>
    )
}
