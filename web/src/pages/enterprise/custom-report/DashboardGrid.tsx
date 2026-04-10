import { useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import type { CustomReportResponse } from "@/api/enterprise"
import { type ChartType, CHART_TYPE_OPTIONS, getLabel } from "./types"
import { ReportChart } from "./ReportChart"

interface DashboardCardConfig {
    measure: string
    chartType: ChartType
}

export function DashboardGrid({
    data,
    dimensions,
    measures,
    lang,
}: {
    data: CustomReportResponse
    dimensions: string[]
    measures: string[]
    lang: string
}) {
    const { t } = useTranslation()

    // Initialize 4 cards with first 4 measures
    const [cards, setCards] = useState<DashboardCardConfig[]>(() =>
        Array.from({ length: 4 }, (_, i) => ({
            measure: measures[i] ?? measures[0] ?? "",
            chartType: "auto" as ChartType,
        })),
    )

    // Reconcile card measures when available measures change
    useEffect(() => {
        setCards((prev) =>
            prev.map((c, i) => ({
                ...c,
                measure: measures.includes(c.measure) ? c.measure : (measures[i] ?? measures[0] ?? ""),
            })),
        )
    }, [measures])

    const updateCard = (idx: number, updates: Partial<DashboardCardConfig>) => {
        setCards((prev) => prev.map((c, i) => (i === idx ? { ...c, ...updates } : c)))
    }

    return (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 p-4">
            {cards.map((card, idx) => {
                const cardMeasures = [card.measure].filter(Boolean)

                return (
                    <div
                        key={idx}
                        className="rounded-xl border bg-background/50 backdrop-blur-sm overflow-hidden"
                    >
                        {/* Card header */}
                        <div className="flex items-center gap-2 px-3 py-2 border-b bg-muted/20">
                            <Select
                                value={card.measure}
                                onValueChange={(v) => updateCard(idx, { measure: v })}
                            >
                                <SelectTrigger className="h-7 text-xs flex-1">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {measures.map((m) => (
                                        <SelectItem key={m} value={m} className="text-xs">
                                            {getLabel(m, lang)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                            <Select
                                value={card.chartType}
                                onValueChange={(v) => updateCard(idx, { chartType: v as ChartType })}
                            >
                                <SelectTrigger className="h-7 text-xs w-[100px]">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {CHART_TYPE_OPTIONS.map((opt) => (
                                        <SelectItem key={opt.type} value={opt.type} className="text-xs">
                                            {opt.icon} {t(opt.labelKey as never)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>

                        {/* Chart */}
                        <div className="p-2" style={{ minHeight: 240 }}>
                            {card.measure && (
                                <ReportChart
                                    data={data}
                                    dimensions={dimensions}
                                    measures={cardMeasures}
                                    chartType={card.chartType}
                                    lang={lang}
                                />
                            )}
                        </div>
                    </div>
                )
            })}
        </div>
    )
}
