import { useState, useMemo, useEffect, useRef } from "react"
import { useParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { ArrowLeft } from "lucide-react"
import * as echarts from "echarts"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { enterpriseApi } from "@/api/enterprise"
import { ROUTES } from "@/routes/constants"
import { type TimeRange, getTimeRange, useDarkMode, getEChartsTheme } from "@/lib/enterprise"

function TrendChart({
    trend,
}: {
    trend: { hour_timestamp: number; request_count: number; used_amount: number; total_tokens: number }[]
}) {
    const chartRef = useRef<HTMLDivElement>(null)
    const chartInstance = useRef<echarts.ECharts | null>(null)
    const isDark = useDarkMode()

    useEffect(() => {
        if (!chartRef.current) return

        if (!chartInstance.current) {
            chartInstance.current = echarts.init(chartRef.current)
        }

        const theme = getEChartsTheme(isDark)
        const xData = trend.map((p) => {
            const d = new Date(p.hour_timestamp * 1000)
            return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, "0")}:00`
        })

        chartInstance.current.setOption({
            tooltip: {
                trigger: "axis",
            },
            legend: {
                data: ["Requests", "Amount ($)", "Tokens"],
                bottom: 0,
                textStyle: { color: theme.textColor },
            },
            grid: {
                left: "3%",
                right: "4%",
                bottom: "12%",
                top: "8%",
                containLabel: true,
            },
            xAxis: {
                type: "category",
                data: xData,
                axisLabel: { rotate: 30, color: theme.subTextColor },
            },
            yAxis: [
                {
                    type: "value",
                    name: "Requests",
                    position: "left",
                    nameTextStyle: { color: theme.subTextColor },
                    axisLabel: { color: theme.subTextColor },
                    splitLine: { lineStyle: { color: theme.splitLineColor } },
                },
                {
                    type: "value",
                    name: "Amount ($)",
                    position: "right",
                    nameTextStyle: { color: theme.subTextColor },
                    axisLabel: { color: theme.subTextColor },
                    splitLine: { show: false },
                },
            ],
            series: [
                {
                    name: "Requests",
                    type: "bar",
                    data: trend.map((p) => p.request_count),
                    itemStyle: { color: "#6A6DE6" },
                },
                {
                    name: "Amount ($)",
                    type: "line",
                    yAxisIndex: 1,
                    data: trend.map((p) => Math.round(p.used_amount * 100) / 100),
                    itemStyle: { color: "#F59E0B" },
                    smooth: true,
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
    }, [trend, isDark])

    return <div ref={chartRef} className="w-full h-96" />
}

export default function EnterpriseDepartment() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const { id } = useParams<{ id: string }>()
    const [timeRange, setTimeRange] = useState<TimeRange>("7d")

    const { start, end } = useMemo(() => getTimeRange(timeRange), [timeRange])

    const { data, isLoading } = useQuery({
        queryKey: ["enterprise", "department-trend", id, start, end],
        queryFn: () => enterpriseApi.getDepartmentTrend(id!, start, end),
        enabled: !!id,
    })

    const trend = data?.trend || []

    return (
        <div className="p-6 space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <Button variant="ghost" size="icon" onClick={() => navigate(ROUTES.ENTERPRISE)}>
                        <ArrowLeft className="w-5 h-5" />
                    </Button>
                    <div>
                        <h1 className="text-2xl font-bold">{t("enterprise.department.title")}</h1>
                        <p className="text-sm text-muted-foreground">{id}</p>
                    </div>
                </div>
                <Select value={timeRange} onValueChange={(v) => setTimeRange(v as TimeRange)}>
                    <SelectTrigger className="w-40">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="7d">{t("enterprise.dashboard.last7Days")}</SelectItem>
                        <SelectItem value="30d">{t("enterprise.dashboard.last30Days")}</SelectItem>
                        <SelectItem value="month">{t("enterprise.dashboard.thisMonth")}</SelectItem>
                    </SelectContent>
                </Select>
            </div>

            {/* Trend chart */}
            <Card>
                <CardHeader>
                    <CardTitle className="text-lg">{t("enterprise.department.trend")}</CardTitle>
                </CardHeader>
                <CardContent>
                    {isLoading ? (
                        <div className="h-96 flex items-center justify-center text-muted-foreground">
                            {t("common.loading")}
                        </div>
                    ) : trend.length === 0 ? (
                        <div className="h-96 flex items-center justify-center text-muted-foreground">
                            {t("common.noResult")}
                        </div>
                    ) : (
                        <TrendChart trend={trend} />
                    )}
                </CardContent>
            </Card>
        </div>
    )
}
