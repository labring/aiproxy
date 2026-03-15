import { useState, useMemo, useEffect, useRef } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { BarChart2, DollarSign, Hash, Building2, ArrowRight } from "lucide-react"
import * as echarts from "echarts"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { enterpriseApi, type DepartmentSummary } from "@/api/enterprise"
import { ROUTES } from "@/routes/constants"
import { type TimeRange, getTimeRange, formatNumber, formatAmount } from "@/lib/enterprise"

function MetricCard({
    title,
    value,
    icon: Icon,
}: {
    title: string
    value: string | number
    icon: React.ComponentType<{ className?: string }>
}) {
    return (
        <Card className="border border-gray-100 dark:border-gray-800">
            <CardContent className="p-6">
                <div className="flex items-center justify-between">
                    <div>
                        <p className="text-sm text-muted-foreground">{title}</p>
                        <p className="text-2xl font-bold mt-1">{value}</p>
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

    useEffect(() => {
        if (!chartRef.current) return

        if (!chartInstance.current) {
            chartInstance.current = echarts.init(chartRef.current)
        }

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
                formatter: "{b}: {c} ({d}%)",
            },
            series: [
                {
                    type: "pie",
                    radius: ["40%", "70%"],
                    avoidLabelOverlap: true,
                    itemStyle: {
                        borderRadius: 6,
                        borderColor: "#fff",
                        borderWidth: 2,
                    },
                    label: {
                        show: true,
                        formatter: "{b}",
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
    }, [departments])

    return <div ref={chartRef} className="w-full h-80" />
}

export default function EnterpriseDashboard() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [timeRange, setTimeRange] = useState<TimeRange>("7d")

    const { start, end } = useMemo(() => getTimeRange(timeRange), [timeRange])

    const { data, isLoading } = useQuery({
        queryKey: ["enterprise", "department-summary", start, end],
        queryFn: () => enterpriseApi.getDepartmentSummary(start, end),
    })

    const departments = data?.departments || []

    const totals = useMemo(() => {
        return departments.reduce(
            (acc, d) => ({
                requests: acc.requests + (d.request_count || 0),
                amount: acc.amount + (d.used_amount || 0),
                tokens: acc.tokens + (d.token_count || 0),
                activeDepts: acc.activeDepts + 1,
            }),
            { requests: 0, amount: 0, tokens: 0, activeDepts: 0 },
        )
    }, [departments])

    return (
        <div className="p-6 space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <h1 className="text-2xl font-bold">{t("enterprise.dashboard.title")}</h1>
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

            {/* Metric cards */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                <MetricCard
                    title={t("enterprise.dashboard.totalRequests")}
                    value={isLoading ? "..." : formatNumber(totals.requests)}
                    icon={BarChart2}
                />
                <MetricCard
                    title={t("enterprise.dashboard.totalAmount")}
                    value={isLoading ? "..." : formatAmount(totals.amount)}
                    icon={DollarSign}
                />
                <MetricCard
                    title={t("enterprise.dashboard.totalTokens")}
                    value={isLoading ? "..." : formatNumber(totals.tokens)}
                    icon={Hash}
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
                        <CardTitle className="text-lg">{t("enterprise.dashboard.departmentSummary")}</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <div className="overflow-x-auto">
                            <table className="w-full text-sm">
                                <thead>
                                    <tr className="border-b text-muted-foreground">
                                        <th className="text-left py-3 px-2 font-medium">
                                            {t("enterprise.dashboard.department")}
                                        </th>
                                        <th className="text-right py-3 px-2 font-medium">
                                            {t("enterprise.dashboard.requests")}
                                        </th>
                                        <th className="text-right py-3 px-2 font-medium">
                                            {t("enterprise.dashboard.amount")}
                                        </th>
                                        <th className="text-right py-3 px-2 font-medium">
                                            {t("enterprise.dashboard.activeUsers")}
                                        </th>
                                        <th className="text-right py-3 px-2 font-medium" />
                                    </tr>
                                </thead>
                                <tbody>
                                    {isLoading ? (
                                        <tr>
                                            <td colSpan={5} className="text-center py-8 text-muted-foreground">
                                                {t("common.loading")}
                                            </td>
                                        </tr>
                                    ) : departments.length === 0 ? (
                                        <tr>
                                            <td colSpan={5} className="text-center py-8 text-muted-foreground">
                                                {t("common.noResult")}
                                            </td>
                                        </tr>
                                    ) : (
                                        departments.map((dept) => (
                                            <tr
                                                key={dept.department_id}
                                                className="border-b last:border-0 hover:bg-muted/50 cursor-pointer transition-colors"
                                                onClick={() =>
                                                    navigate(`${ROUTES.ENTERPRISE_DEPARTMENT}/${dept.department_id}`)
                                                }
                                            >
                                                <td className="py-3 px-2 font-medium">
                                                    {dept.department_name || dept.department_id}
                                                </td>
                                                <td className="py-3 px-2 text-right">
                                                    {formatNumber(dept.request_count)}
                                                </td>
                                                <td className="py-3 px-2 text-right">
                                                    {formatAmount(dept.used_amount)}
                                                </td>
                                                <td className="py-3 px-2 text-right">{dept.active_users}</td>
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
        </div>
    )
}
