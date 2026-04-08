import { useState, useMemo, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Bell, Save, AlertTriangle, CheckCircle, Info, Loader2 } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { enterpriseApi, type QuotaNotifConfig, type QuotaAlertHistory } from "@/api/enterprise"
import { toast } from "sonner"
import { format } from "date-fns"
import { useHasPermission } from "@/lib/permissions"

function NotifConfigTab() {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const canManage = useHasPermission("quota_manage_manage")

    const { data, isLoading } = useQuery({
        queryKey: ["notif-config"],
        queryFn: () => enterpriseApi.getNotifConfig(),
    })

    const [form, setForm] = useState<QuotaNotifConfig | null>(null)

    // Initialize form from data
    const cfg = form ?? data ?? null

    const updateField = <K extends keyof QuotaNotifConfig>(key: K, value: QuotaNotifConfig[K]) => {
        setForm(prev => ({ ...(prev ?? data ?? {} as QuotaNotifConfig), [key]: value }))
    }

    const saveMutation = useMutation({
        mutationFn: (config: QuotaNotifConfig) => enterpriseApi.updateNotifConfig(config),
        onSuccess: () => {
            toast.success(t("enterprise.notifications.saved"))
            queryClient.invalidateQueries({ queryKey: ["notif-config"] })
            setForm(null)
        },
        onError: (err: Error) => {
            toast.error(err.message)
        },
    })

    if (isLoading || !cfg) {
        return (
            <div className="py-8 text-center text-muted-foreground">
                <Loader2 className="w-6 h-6 animate-spin mx-auto" />
            </div>
        )
    }

    return (
        <div className="space-y-6">
            {/* P2P Availability */}
            {data && !data.p2p_available && (
                <Card className="border-amber-200 bg-amber-50 dark:bg-amber-900/20 dark:border-amber-800">
                    <CardContent className="pt-4 pb-4">
                        <div className="flex items-center gap-2 text-sm text-amber-700 dark:text-amber-400">
                            <AlertTriangle className="w-4 h-4" />
                            {t("enterprise.notifications.p2pUnavailable")}
                        </div>
                    </CardContent>
                </Card>
            )}

            {/* Enable toggle */}
            <Card>
                <CardContent className="pt-4 pb-4">
                    <div className="flex items-center justify-between">
                        <div>
                            <Label className="text-base font-medium">{t("enterprise.notifications.enableLabel")}</Label>
                            <p className="text-sm text-muted-foreground mt-0.5">{t("enterprise.notifications.enableDesc")}</p>
                        </div>
                        <Switch
                            checked={cfg.enabled}
                            onCheckedChange={(v) => updateField("enabled", v)}
                            disabled={!canManage}
                        />
                    </div>
                </CardContent>
            </Card>

            {/* Tier Templates */}
            {[
                { key: "tier2" as const, titleKey: "tier2_title" as const, bodyKey: "tier2_body" as const, color: "text-orange-600", label: t("enterprise.notifications.tier2Label") },
                { key: "tier3" as const, titleKey: "tier3_title" as const, bodyKey: "tier3_body" as const, color: "text-red-600", label: t("enterprise.notifications.tier3Label") },
                { key: "exhaust" as const, titleKey: "exhaust_title" as const, bodyKey: "exhaust_body" as const, color: "text-red-800", label: t("enterprise.notifications.exhaustLabel") },
                { key: "policy_change" as const, titleKey: "policy_change_title" as const, bodyKey: "policy_change_body" as const, color: "text-blue-600", label: t("enterprise.notifications.policyChangeLabel" as never) },
            ].map(({ titleKey, bodyKey, color, label }) => (
                <Card key={titleKey}>
                    <CardHeader className="pb-3">
                        <CardTitle className={`text-sm ${color}`}>{label}</CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-3">
                        <div>
                            <Label>{t("enterprise.notifications.templateTitle")}</Label>
                            <Input
                                value={cfg[titleKey]}
                                onChange={(e) => updateField(titleKey, e.target.value)}
                                disabled={!canManage}
                            />
                        </div>
                        <div>
                            <Label>{t("enterprise.notifications.templateBody")}</Label>
                            <Textarea
                                value={cfg[bodyKey]}
                                onChange={(e) => updateField(bodyKey, e.target.value)}
                                rows={3}
                                disabled={!canManage}
                            />
                        </div>
                    </CardContent>
                </Card>
            ))}

            {/* Separator */}
            <div className="border-t pt-4">
                <h3 className="text-sm font-semibold mb-3">{t("enterprise.notifications.adminAlertSection")}</h3>
            </div>

            {/* Admin alert enable toggle */}
            <Card>
                <CardContent className="pt-4 pb-4">
                    <div className="flex items-center justify-between">
                        <div>
                            <Label className="text-base font-medium">{t("enterprise.notifications.adminAlertEnable")}</Label>
                            <p className="text-sm text-muted-foreground mt-0.5">{t("enterprise.notifications.adminAlertEnableDesc")}</p>
                        </div>
                        <Switch
                            checked={cfg.admin_alert_enabled}
                            onCheckedChange={(v) => updateField("admin_alert_enabled", v)}
                            disabled={!canManage}
                        />
                    </div>
                </CardContent>
            </Card>

            {/* Admin alert threshold */}
            <Card>
                <CardHeader className="pb-3">
                    <CardTitle className="text-sm text-orange-600">{t("enterprise.notifications.adminAlertThresholdLabel")}</CardTitle>
                </CardHeader>
                <CardContent className="space-y-3">
                    <div>
                        <Label>{t("enterprise.notifications.adminAlertThresholdDesc")}</Label>
                        <div className="flex items-center gap-2 mt-1">
                            <Input
                                type="number"
                                min={1}
                                max={100}
                                value={Math.round((cfg.admin_alert_threshold ?? 0.8) * 100)}
                                onChange={(e) => updateField("admin_alert_threshold", Number(e.target.value) / 100)}
                                className="w-24"
                                disabled={!canManage}
                            />
                            <span className="text-sm text-muted-foreground">%</span>
                        </div>
                    </div>
                    <div>
                        <Label>{t("enterprise.notifications.templateTitle")}</Label>
                        <Input
                            value={cfg.admin_alert_title}
                            onChange={(e) => updateField("admin_alert_title", e.target.value)}
                            disabled={!canManage}
                        />
                    </div>
                    <div>
                        <Label>{t("enterprise.notifications.templateBody")}</Label>
                        <Textarea
                            value={cfg.admin_alert_body}
                            onChange={(e) => updateField("admin_alert_body", e.target.value)}
                            rows={3}
                            disabled={!canManage}
                        />
                    </div>
                </CardContent>
            </Card>

            {/* Variable reference */}
            <Card>
                <CardContent className="pt-4 pb-4">
                    <div className="flex items-center gap-2 text-sm text-muted-foreground mb-2">
                        <Info className="w-4 h-4" />
                        {t("enterprise.notifications.variablesTitle")}
                    </div>
                    <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground">
                        <code>{"{name}"}</code><span>{t("enterprise.notifications.varName")}</span>
                        <code>{"{usage_pct}"}</code><span>{t("enterprise.notifications.varUsagePct")}</span>
                        <code>{"{period_quota}"}</code><span>{t("enterprise.notifications.varPeriodQuota")}</span>
                        <code>{"{period_type}"}</code><span>{t("enterprise.notifications.varPeriodType")}</span>
                        <code>{"{tier_threshold}"}</code><span>{t("enterprise.notifications.varTierThreshold")}</span>
                        <code>{"{admin_threshold}"}</code><span>{t("enterprise.notifications.varAdminThreshold")}</span>
                        <code>{"{policy_name}"}</code><span>{t("enterprise.notifications.varPolicyName" as never)}</span>
                        <code>{"{tier1_ratio}"}</code><span>{t("enterprise.notifications.varTier1Ratio" as never)}</span>
                        <code>{"{tier2_ratio}"}</code><span>{t("enterprise.notifications.varTier2Ratio" as never)}</span>
                    </div>
                </CardContent>
            </Card>

            {/* Save button */}
            {canManage && form && (
                <Button
                    onClick={() => saveMutation.mutate(cfg)}
                    disabled={saveMutation.isPending}
                    className="w-full"
                >
                    <Save className="w-4 h-4 mr-2" />
                    {saveMutation.isPending ? t("common.saving") : t("enterprise.notifications.save")}
                </Button>
            )}
        </div>
    )
}

function AlertHistoryTab() {
    const { t } = useTranslation()
    const [page, setPage] = useState(1)
    const [statusFilter, setStatusFilter] = useState<string>("all")
    const [tierFilter, setTierFilter] = useState<string>("all")
    const [periodTypeFilter, setPeriodTypeFilter] = useState<string>("all")
    const [keyword, setKeyword] = useState("")
    const [debouncedKeyword, setDebouncedKeyword] = useState("")
    const perPage = 20

    // Debounce keyword search
    useEffect(() => {
        const timer = setTimeout(() => setDebouncedKeyword(keyword), 500)
        return () => clearTimeout(timer)
    }, [keyword])

    const filters = useMemo(() => {
        const f: { status?: string; tier?: number; keyword?: string; period_type?: string } = {}
        if (statusFilter !== "all") f.status = statusFilter
        if (tierFilter !== "all") f.tier = Number(tierFilter)
        if (periodTypeFilter !== "all") f.period_type = periodTypeFilter
        if (debouncedKeyword) f.keyword = debouncedKeyword
        return f
    }, [statusFilter, tierFilter, periodTypeFilter, debouncedKeyword])

    const { data, isLoading } = useQuery({
        queryKey: ["alert-history", page, statusFilter, tierFilter, periodTypeFilter, debouncedKeyword],
        queryFn: () => enterpriseApi.getAlertHistory(page, perPage, filters),
    })

    const tierBadge = (tier: number) => {
        const configs: Record<number, { label: string; className: string }> = {
            0: { label: t("enterprise.notifications.tierAdmin"), className: "bg-purple-100 text-purple-800" },
            2: { label: t("enterprise.notifications.tierLevel2"), className: "bg-orange-100 text-orange-800" },
            3: { label: t("enterprise.notifications.tierLevel3"), className: "bg-red-100 text-red-800" },
            4: { label: t("enterprise.notifications.tierExhaust"), className: "bg-red-200 text-red-900" },
            5: { label: t("enterprise.notifications.tierPolicyChange" as never), className: "bg-blue-100 text-blue-800" },
        }
        const cfg = configs[tier] || { label: `T${tier}`, className: "bg-gray-100 text-gray-800" }
        return <Badge className={cfg.className}>{cfg.label}</Badge>
    }

    const statusBadge = (status: string) => {
        if (status === "sent") {
            return (
                <Badge className="bg-green-100 text-green-800">
                    <CheckCircle className="w-3 h-3 mr-1" />
                    {t("enterprise.notifications.statusSent")}
                </Badge>
            )
        }
        return (
            <Badge className="bg-red-100 text-red-800">
                <AlertTriangle className="w-3 h-3 mr-1" />
                {t("enterprise.notifications.statusFailed")}
            </Badge>
        )
    }

    const totalPages = Math.ceil((data?.total ?? 0) / perPage)

    return (
        <div className="space-y-4">
            {/* Filters */}
            <div className="flex items-center gap-3 flex-wrap">
                <Input
                    placeholder={t("enterprise.notifications.searchUser" as never)}
                    value={keyword}
                    onChange={(e) => { setKeyword(e.target.value); setPage(1) }}
                    className="w-48"
                />
                <Select value={statusFilter} onValueChange={v => { setStatusFilter(v); setPage(1) }}>
                    <SelectTrigger className="w-32">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="all">{t("enterprise.notifications.allStatuses")}</SelectItem>
                        <SelectItem value="sent">{t("enterprise.notifications.statusSent")}</SelectItem>
                        <SelectItem value="failed">{t("enterprise.notifications.statusFailed")}</SelectItem>
                    </SelectContent>
                </Select>
                <Select value={tierFilter} onValueChange={v => { setTierFilter(v); setPage(1) }}>
                    <SelectTrigger className="w-32">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="all">{t("enterprise.notifications.allTiers")}</SelectItem>
                        <SelectItem value="0">{t("enterprise.notifications.tierAdmin")}</SelectItem>
                        <SelectItem value="2">{t("enterprise.notifications.tierLevel2")}</SelectItem>
                        <SelectItem value="3">{t("enterprise.notifications.tierLevel3")}</SelectItem>
                        <SelectItem value="4">{t("enterprise.notifications.tierExhaust")}</SelectItem>
                        <SelectItem value="5">{t("enterprise.notifications.tierPolicyChange" as never)}</SelectItem>
                    </SelectContent>
                </Select>
                <Select value={periodTypeFilter} onValueChange={v => { setPeriodTypeFilter(v); setPage(1) }}>
                    <SelectTrigger className="w-32">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="all">{t("enterprise.notifications.allPeriods" as never)}</SelectItem>
                        <SelectItem value="daily">{t("enterprise.quota.daily")}</SelectItem>
                        <SelectItem value="weekly">{t("enterprise.quota.weekly")}</SelectItem>
                        <SelectItem value="monthly">{t("enterprise.quota.monthly")}</SelectItem>
                    </SelectContent>
                </Select>
            </div>

            {/* Table */}
            {isLoading ? (
                <div className="py-8 text-center text-muted-foreground">
                    <Loader2 className="w-6 h-6 animate-spin mx-auto" />
                </div>
            ) : !data?.records?.length ? (
                <div className="py-8 text-center text-sm text-muted-foreground">
                    {t("enterprise.notifications.noAlerts")}
                </div>
            ) : (
                <>
                    <div className="overflow-x-auto">
                        <table className="w-full text-sm">
                            <thead>
                                <tr className="border-b bg-muted/50">
                                    <th className="text-left p-3 font-medium text-muted-foreground">{t("enterprise.notifications.colTime")}</th>
                                    <th className="text-left p-3 font-medium text-muted-foreground">{t("enterprise.notifications.colUser")}</th>
                                    <th className="text-center p-3 font-medium text-muted-foreground">{t("enterprise.notifications.colTier")}</th>
                                    <th className="text-center p-3 font-medium text-muted-foreground">{t("enterprise.notifications.colUsage")}</th>
                                    <th className="text-left p-3 font-medium text-muted-foreground">{t("enterprise.notifications.colStatus")}</th>
                                    <th className="text-left p-3 font-medium text-muted-foreground">{t("enterprise.notifications.colTitle")}</th>
                                </tr>
                            </thead>
                            <tbody>
                                {data.records.map((h: QuotaAlertHistory) => (
                                    <tr key={h.id} className="border-b last:border-b-0 hover:bg-muted/50 transition-colors">
                                        <td className="p-3 whitespace-nowrap">{format(new Date(h.created_at), "yyyy-MM-dd HH:mm")}</td>
                                        <td className="p-3">{h.user_name || h.open_id}</td>
                                        <td className="p-3 text-center">{tierBadge(h.tier)}</td>
                                        <td className="p-3 text-center">{(h.usage_ratio * 100).toFixed(1)}%</td>
                                        <td className="p-3">{statusBadge(h.status)}</td>
                                        <td className="p-3 max-w-[200px] truncate text-muted-foreground">{h.title}</td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>

                    {/* Pagination */}
                    {totalPages > 1 && (
                        <div className="flex items-center justify-center gap-2 pt-2">
                            <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>
                                {t("table.previousPage")}
                            </Button>
                            <span className="text-sm text-muted-foreground">
                                {page} / {totalPages}
                            </span>
                            <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>
                                {t("table.nextPage")}
                            </Button>
                        </div>
                    )}
                </>
            )}
        </div>
    )
}

export default function NotificationsPage() {
    const { t } = useTranslation()

    return (
        <div className="p-6 max-w-4xl mx-auto space-y-6">
            <div>
                <div className="flex items-center gap-2">
                    <Bell className="w-6 h-6 text-purple-600" />
                    <h1 className="text-2xl font-bold text-foreground">{t("enterprise.notifications.title")}</h1>
                </div>
                <p className="text-sm text-muted-foreground mt-1">{t("enterprise.notifications.description")}</p>
            </div>

            <Tabs defaultValue="config">
                <TabsList>
                    <TabsTrigger value="config">{t("enterprise.notifications.configTab")}</TabsTrigger>
                    <TabsTrigger value="history">{t("enterprise.notifications.historyTab")}</TabsTrigger>
                </TabsList>
                <TabsContent value="config" className="mt-4">
                    <NotifConfigTab />
                </TabsContent>
                <TabsContent value="history" className="mt-4">
                    <AlertHistoryTab />
                </TabsContent>
            </Tabs>
        </div>
    )
}
