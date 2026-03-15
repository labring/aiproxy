import { useState, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Download } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { enterpriseApi } from "@/api/enterprise"
import { toast } from "sonner"
import { type TimeRange, getTimeRange, formatNumber, formatAmount } from "@/lib/enterprise"

export default function EnterpriseRanking() {
    const { t } = useTranslation()
    const [timeRange, setTimeRange] = useState<TimeRange>("7d")
    const [departmentFilter, setDepartmentFilter] = useState<string>("all")
    const [limit, setLimit] = useState<number>(50)

    const { start, end } = useMemo(() => getTimeRange(timeRange), [timeRange])

    const { data: rankingData, isLoading } = useQuery({
        queryKey: ["enterprise", "ranking", start, end, departmentFilter, limit],
        queryFn: () =>
            enterpriseApi.getUserRanking(
                departmentFilter === "all" ? undefined : departmentFilter,
                limit,
                start,
                end,
            ),
    })

    const { data: deptData } = useQuery({
        queryKey: ["enterprise", "departments-for-filter", start, end],
        queryFn: () => enterpriseApi.getDepartmentSummary(start, end),
    })

    const ranking = rankingData?.ranking || []
    const departments = deptData?.departments || []

    const handleExport = async () => {
        try {
            await enterpriseApi.exportReport(start, end)
            toast.success(t("common.success"))
        } catch {
            toast.error(t("error.unknown"))
        }
    }

    return (
        <div className="p-6 space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between flex-wrap gap-4">
                <h1 className="text-2xl font-bold">{t("enterprise.ranking.title")}</h1>
                <div className="flex items-center gap-3">
                    <Select value={timeRange} onValueChange={(v) => setTimeRange(v as TimeRange)}>
                        <SelectTrigger className="w-36">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="7d">{t("enterprise.dashboard.last7Days")}</SelectItem>
                            <SelectItem value="30d">{t("enterprise.dashboard.last30Days")}</SelectItem>
                            <SelectItem value="month">{t("enterprise.dashboard.thisMonth")}</SelectItem>
                        </SelectContent>
                    </Select>

                    <Select value={departmentFilter} onValueChange={setDepartmentFilter}>
                        <SelectTrigger className="w-40">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="all">{t("enterprise.ranking.allDepartments")}</SelectItem>
                            {departments.map((dept) => (
                                <SelectItem key={dept.department_id} value={dept.department_id}>
                                    {dept.department_name || dept.department_id}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>

                    <Select value={String(limit)} onValueChange={(v) => setLimit(Number(v))}>
                        <SelectTrigger className="w-32">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="20">{t("enterprise.ranking.top", { count: 20 })}</SelectItem>
                            <SelectItem value="50">{t("enterprise.ranking.top", { count: 50 })}</SelectItem>
                            <SelectItem value="100">{t("enterprise.ranking.top", { count: 100 })}</SelectItem>
                        </SelectContent>
                    </Select>

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
