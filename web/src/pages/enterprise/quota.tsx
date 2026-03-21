import { useState, useEffect, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Pencil, Trash2, Shield, AlertTriangle, Search, Building2, User, Bell } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import {
    DropdownMenu,
    DropdownMenuCheckboxItem,
    DropdownMenuContent,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
    enterpriseApi,
    type QuotaPolicy,
    type QuotaPolicyInput,
    type DepartmentQuotaPolicyBinding,
    type UserQuotaPolicy,
    type FeishuDepartment,
    type FeishuUser,
    type QuotaNotifConfig,
} from "@/api/enterprise"
import { toast } from "sonner"
import { ALL_FILTER } from "@/lib/enterprise"
import { useHasPermission } from "@/lib/permissions"

const defaultPolicy: QuotaPolicyInput = {
    name: "",
    tier1_ratio: 0.7,
    tier2_ratio: 0.9,
    tier1_rpm_multiplier: 1.0,
    tier1_tpm_multiplier: 1.0,
    tier2_rpm_multiplier: 0.5,
    tier2_tpm_multiplier: 0.5,
    tier3_rpm_multiplier: 0.1,
    tier3_tpm_multiplier: 0.1,
    block_at_tier3: false,
    tier2_blocked_models: "",
    tier3_blocked_models: "",
    period_quota: 0,
    period_type: 3,
}

function TierIndicator({ ratio, label }: { ratio: number; label: string }) {
    const clampedRatio = Math.max(0, Math.min(1, ratio))
    return (
        <div className="flex items-center gap-2">
            <div
                className="h-2 rounded-full bg-gradient-to-r from-green-500 via-yellow-500 to-red-500"
                style={{ width: "60px" }}
            >
                <div
                    className="h-2 w-1 bg-black rounded-full relative"
                    style={{ marginLeft: `${clampedRatio * 100}%`, transform: "translateX(-50%)" }}
                />
            </div>
            <span className="text-xs text-muted-foreground">{label}: {(ratio * 100).toFixed(0)}%</span>
        </div>
    )
}

function PolicyForm({
    policy,
    onChange,
}: {
    policy: QuotaPolicyInput
    onChange: (policy: QuotaPolicyInput) => void
}) {
    const { t } = useTranslation()

    return (
        <div className="space-y-6">
            {/* Policy Name */}
            <div className="space-y-2">
                <Label htmlFor="name">{t("enterprise.quota.policyName")}</Label>
                <Input
                    id="name"
                    value={policy.name}
                    onChange={(e) => onChange({ ...policy, name: e.target.value })}
                    placeholder={t("enterprise.quota.policyNamePlaceholder")}
                />
            </div>

            {/* Period Quota */}
            <div className="space-y-4">
                <h4 className="font-medium">{t("enterprise.quota.periodQuota")}</h4>
                <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-2">
                        <Label>{t("enterprise.quota.periodQuota")}</Label>
                        <Input
                            type="number"
                            value={policy.period_quota}
                            onChange={(e) => onChange({ ...policy, period_quota: Math.max(0, parseFloat(e.target.value) || 0) })}
                            min={0}
                            step={10}
                        />
                        <p className="text-xs text-muted-foreground">{t("enterprise.quota.periodQuotaHint")}</p>
                    </div>
                    <div className="space-y-2">
                        <Label>{t("enterprise.quota.periodType")}</Label>
                        <Select
                            value={String(policy.period_type)}
                            onValueChange={(v) => onChange({ ...policy, period_type: parseInt(v) })}
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="1">{t("enterprise.quota.daily")}</SelectItem>
                                <SelectItem value="2">{t("enterprise.quota.weekly")}</SelectItem>
                                <SelectItem value="3">{t("enterprise.quota.monthly")}</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                </div>
            </div>

            {/* Tier Thresholds */}
            <div className="space-y-4">
                <h4 className="font-medium">{t("enterprise.quota.tierThresholds")}</h4>
                <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-2">
                        <Label>{t("enterprise.quota.tier1Ratio")}</Label>
                        <div className="flex items-center gap-2">
                            <Input
                                type="number"
                                value={(policy.tier1_ratio * 100).toFixed(0)}
                                onChange={(e) => {
                                    const val = Math.max(0, Math.min(100, parseFloat(e.target.value) || 0))
                                    onChange({ ...policy, tier1_ratio: val / 100 })
                                }}
                                min={0}
                                max={100}
                                step={5}
                                className="w-24"
                            />
                            <span className="text-sm text-muted-foreground">%</span>
                        </div>
                        <p className="text-xs text-muted-foreground">{t("enterprise.quota.tier1RatioDesc")}</p>
                    </div>
                    <div className="space-y-2">
                        <Label>{t("enterprise.quota.tier2Ratio")}</Label>
                        <div className="flex items-center gap-2">
                            <Input
                                type="number"
                                value={(policy.tier2_ratio * 100).toFixed(0)}
                                onChange={(e) => {
                                    const val = Math.max(0, Math.min(100, parseFloat(e.target.value) || 0))
                                    onChange({ ...policy, tier2_ratio: val / 100 })
                                }}
                                min={0}
                                max={100}
                                step={5}
                                className="w-24"
                            />
                            <span className="text-sm text-muted-foreground">%</span>
                        </div>
                        <p className="text-xs text-muted-foreground">{t("enterprise.quota.tier2RatioDesc")}</p>
                    </div>
                </div>
            </div>

            {/* Tier Multipliers */}
            <div className="space-y-4">
                <h4 className="font-medium">{t("enterprise.quota.tierMultipliers")}</h4>
                <div className="grid grid-cols-3 gap-4">
                    {/* Tier 1 */}
                    <Card className="p-4 border-green-200 bg-green-50/50 dark:bg-green-950/20">
                        <h5 className="text-sm font-medium text-green-700 dark:text-green-400 mb-3">
                            {t("enterprise.quota.tier1")}
                        </h5>
                        <div className="space-y-3">
                            <div>
                                <Label className="text-xs">RPM</Label>
                                <Input
                                    type="number"
                                    value={policy.tier1_rpm_multiplier}
                                    onChange={(e) => {
                                        const val = Math.max(0.01, Math.min(2, parseFloat(e.target.value) || 0.01))
                                        onChange({ ...policy, tier1_rpm_multiplier: val })
                                    }}
                                    step={0.1}
                                    min={0.01}
                                    max={2}
                                    className="h-8"
                                />
                            </div>
                            <div>
                                <Label className="text-xs">TPM</Label>
                                <Input
                                    type="number"
                                    value={policy.tier1_tpm_multiplier}
                                    onChange={(e) => {
                                        const val = Math.max(0.01, Math.min(2, parseFloat(e.target.value) || 0.01))
                                        onChange({ ...policy, tier1_tpm_multiplier: val })
                                    }}
                                    step={0.1}
                                    min={0.01}
                                    max={2}
                                    className="h-8"
                                />
                            </div>
                        </div>
                    </Card>

                    {/* Tier 2 */}
                    <Card className="p-4 border-yellow-200 bg-yellow-50/50 dark:bg-yellow-950/20">
                        <h5 className="text-sm font-medium text-yellow-700 dark:text-yellow-400 mb-3">
                            {t("enterprise.quota.tier2")}
                        </h5>
                        <div className="space-y-3">
                            <div>
                                <Label className="text-xs">RPM</Label>
                                <Input
                                    type="number"
                                    value={policy.tier2_rpm_multiplier}
                                    onChange={(e) => {
                                        const val = Math.max(0.01, Math.min(2, parseFloat(e.target.value) || 0.01))
                                        onChange({ ...policy, tier2_rpm_multiplier: val })
                                    }}
                                    step={0.1}
                                    min={0.01}
                                    max={2}
                                    className="h-8"
                                />
                            </div>
                            <div>
                                <Label className="text-xs">TPM</Label>
                                <Input
                                    type="number"
                                    value={policy.tier2_tpm_multiplier}
                                    onChange={(e) => {
                                        const val = Math.max(0.01, Math.min(2, parseFloat(e.target.value) || 0.01))
                                        onChange({ ...policy, tier2_tpm_multiplier: val })
                                    }}
                                    step={0.1}
                                    min={0.01}
                                    max={2}
                                    className="h-8"
                                />
                            </div>
                            <div>
                                <Label className="text-xs">{t("enterprise.quota.blockedModels")}</Label>
                                <Textarea
                                    value={policy.tier2_blocked_models ? (() => { try { return JSON.parse(policy.tier2_blocked_models).join("\n") } catch { return policy.tier2_blocked_models } })() : ""}
                                    onChange={(e) => {
                                        const lines = e.target.value.split("\n").map(s => s.trim()).filter(Boolean)
                                        onChange({ ...policy, tier2_blocked_models: lines.length > 0 ? JSON.stringify(lines) : "" })
                                    }}
                                    placeholder={t("enterprise.quota.blockedModelsHint")}
                                    rows={2}
                                    className="text-xs"
                                />
                            </div>
                        </div>
                    </Card>

                    {/* Tier 3 */}
                    <Card className="p-4 border-red-200 bg-red-50/50 dark:bg-red-950/20">
                        <h5 className="text-sm font-medium text-red-700 dark:text-red-400 mb-3">
                            {t("enterprise.quota.tier3")}
                        </h5>
                        <div className="space-y-3">
                            <div>
                                <Label className="text-xs">RPM</Label>
                                <Input
                                    type="number"
                                    value={policy.tier3_rpm_multiplier}
                                    onChange={(e) => {
                                        const val = Math.max(0.01, Math.min(2, parseFloat(e.target.value) || 0.01))
                                        onChange({ ...policy, tier3_rpm_multiplier: val })
                                    }}
                                    step={0.1}
                                    min={0.01}
                                    max={2}
                                    className="h-8"
                                    disabled={policy.block_at_tier3}
                                />
                            </div>
                            <div>
                                <Label className="text-xs">TPM</Label>
                                <Input
                                    type="number"
                                    value={policy.tier3_tpm_multiplier}
                                    onChange={(e) => {
                                        const val = Math.max(0.01, Math.min(2, parseFloat(e.target.value) || 0.01))
                                        onChange({ ...policy, tier3_tpm_multiplier: val })
                                    }}
                                    step={0.1}
                                    min={0.01}
                                    max={2}
                                    className="h-8"
                                    disabled={policy.block_at_tier3}
                                />
                            </div>
                            <div>
                                <Label className="text-xs">{t("enterprise.quota.blockedModels")}</Label>
                                <Textarea
                                    value={policy.tier3_blocked_models ? (() => { try { return JSON.parse(policy.tier3_blocked_models).join("\n") } catch { return policy.tier3_blocked_models } })() : ""}
                                    onChange={(e) => {
                                        const lines = e.target.value.split("\n").map(s => s.trim()).filter(Boolean)
                                        onChange({ ...policy, tier3_blocked_models: lines.length > 0 ? JSON.stringify(lines) : "" })
                                    }}
                                    placeholder={t("enterprise.quota.blockedModelsHint")}
                                    rows={2}
                                    className="text-xs"
                                />
                            </div>
                        </div>
                    </Card>
                </div>
            </div>

            {/* Block at Tier 3 */}
            <div className="flex items-center justify-between p-4 border rounded-lg border-red-200 bg-red-50/30 dark:bg-red-950/10">
                <div className="flex items-center gap-3">
                    <AlertTriangle className="w-5 h-5 text-red-500" />
                    <div>
                        <Label htmlFor="block">{t("enterprise.quota.blockAtTier3")}</Label>
                        <p className="text-xs text-muted-foreground">{t("enterprise.quota.blockAtTier3Desc")}</p>
                    </div>
                </div>
                <Switch
                    id="block"
                    checked={policy.block_at_tier3}
                    onCheckedChange={(checked) => onChange({ ...policy, block_at_tier3: checked })}
                />
            </div>
        </div>
    )
}

// ─── Department Binding Tab ─────────────────────────────────────────────────

function DepartmentBindingTab({ policies, canManage }: { policies: QuotaPolicy[]; canManage: boolean }) {
    const { t } = useTranslation()
    const queryClient = useQueryClient()

    // ── Filter state ──
    const [filterLevel1, setFilterLevel1] = useState<string>("")
    const [filterLevel2, setFilterLevel2] = useState<string>("")

    // ── Query results state ──
    const [queryDepts, setQueryDepts] = useState<FeishuDepartment[]>([])
    const [hasQueried, setHasQueried] = useState(false)
    const [checkedDeptIds, setCheckedDeptIds] = useState<Set<string>>(new Set())
    const [bindPolicyId, setBindPolicyId] = useState<string>("")

    // Fetch level1 departments (always)
    const { data: deptLevels } = useQuery({
        queryKey: ["enterprise", "department-levels"],
        queryFn: () => enterpriseApi.getDepartmentLevels(),
    })

    // Fetch level2 for selected level1 filter
    const { data: level2Data } = useQuery({
        queryKey: ["enterprise", "department-levels", filterLevel1],
        queryFn: () => enterpriseApi.getDepartmentLevels(filterLevel1),
        enabled: !!filterLevel1 && filterLevel1 !== ALL_FILTER,
    })

    const { data: bindings, isLoading: bindingsLoading } = useQuery({
        queryKey: ["enterprise", "dept-bindings"],
        queryFn: () => enterpriseApi.listDepartmentPolicyBindings(),
    })

    const batchBindMutation = useMutation({
        mutationFn: ({ department_ids, quota_policy_id }: { department_ids: string[]; quota_policy_id: number }) =>
            enterpriseApi.batchBindPolicyToDepartments(department_ids, quota_policy_id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["enterprise", "dept-bindings"] })
            setCheckedDeptIds(new Set())
            setBindPolicyId("")
            toast.success(t("enterprise.quota.batchBindSuccess"))
        },
        onError: (err: Error) => toast.error(err.message),
    })

    const unbindMutation = useMutation({
        mutationFn: (department_id: string) => enterpriseApi.unbindPolicyFromDepartment(department_id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["enterprise", "dept-bindings"] })
            toast.success(t("enterprise.quota.unbindSuccess"))
        },
        onError: (err: Error) => toast.error(err.message),
    })

    const level1Depts = deptLevels?.level1_departments || []
    const level2Depts = level2Data?.level2_departments || []
    const bindingList = bindings?.bindings || []

    // Reset level2 filter when level1 changes
    useEffect(() => { setFilterLevel2("") }, [filterLevel1])

    const handleQuery = () => {
        let results: FeishuDepartment[] = []
        const hasLevel1 = filterLevel1 && filterLevel1 !== ALL_FILTER
        const hasLevel2 = filterLevel2 && filterLevel2 !== ALL_FILTER
        if (hasLevel2) {
            // Show only the selected level2 department
            const found = level2Depts.find((d) => d.department_id === filterLevel2)
            results = found ? [found] : []
        } else if (hasLevel1) {
            // Show the level1 + all its level2 children
            const l1 = level1Depts.find((d) => d.department_id === filterLevel1)
            results = [...(l1 ? [l1] : []), ...level2Depts]
        } else {
            // No filter — show all level1 departments
            results = [...level1Depts]
        }
        setQueryDepts(results)
        setHasQueried(true)
        setCheckedDeptIds(new Set())
    }

    const toggleCheck = (id: string) => {
        setCheckedDeptIds((prev) => {
            const next = new Set(prev)
            if (next.has(id)) next.delete(id)
            else next.add(id)
            return next
        })
    }

    const toggleAllChecked = () => {
        if (checkedDeptIds.size === queryDepts.length) {
            setCheckedDeptIds(new Set())
        } else {
            setCheckedDeptIds(new Set(queryDepts.map((d) => d.department_id)))
        }
    }

    const handleBatchBind = () => {
        if (checkedDeptIds.size === 0 || !bindPolicyId) return
        batchBindMutation.mutate({
            department_ids: Array.from(checkedDeptIds),
            quota_policy_id: parseInt(bindPolicyId),
        })
    }

    // Pre-build lookup for O(1) binding check in render
    const bindingByDeptId = useMemo(
        () => new Map(bindingList.map((b: DepartmentQuotaPolicyBinding) => [b.department_id, b])),
        [bindingList],
    )

    return (
        <div className="space-y-6">
            {/* Step 1: Filter & Query */}
            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <Building2 className="w-5 h-5" />
                        {t("enterprise.quota.selectDepartments")}
                    </CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="flex items-end gap-4">
                        <div className="space-y-2 min-w-[180px]">
                            <Label>{t("enterprise.quota.level1Department")}</Label>
                            <Select value={filterLevel1} onValueChange={setFilterLevel1}>
                                <SelectTrigger>
                                    <SelectValue placeholder={t("enterprise.quota.allDepartments")} />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value={ALL_FILTER}>{t("enterprise.quota.allDepartments")}</SelectItem>
                                    {level1Depts.map((d) => (
                                        <SelectItem key={d.department_id} value={d.department_id}>
                                            {d.name || d.department_id}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="space-y-2 min-w-[180px]">
                            <Label>{t("enterprise.quota.level2Department")}</Label>
                            <Select
                                value={filterLevel2}
                                onValueChange={setFilterLevel2}
                                disabled={!filterLevel1 || filterLevel1 === ALL_FILTER || level2Depts.length === 0}
                            >
                                <SelectTrigger>
                                    <SelectValue placeholder={t("enterprise.quota.allDepartments")} />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value={ALL_FILTER}>{t("enterprise.quota.allDepartments")}</SelectItem>
                                    {level2Depts.map((d) => (
                                        <SelectItem key={d.department_id} value={d.department_id}>
                                            {d.name || d.department_id}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                        <Button onClick={handleQuery}>
                            <Search className="w-4 h-4 mr-1.5" />
                            {t("enterprise.quota.query")}
                        </Button>
                    </div>
                </CardContent>
            </Card>

            {/* Step 2: Query Results with checkboxes */}
            {hasQueried && (
                <Card>
                    <CardHeader>
                        <CardTitle className="flex items-center justify-between">
                            <span>{t("enterprise.quota.queryResults")} ({queryDepts.length})</span>
                            {canManage && queryDepts.length > 0 && checkedDeptIds.size > 0 && (
                                <div className="flex items-center gap-3">
                                    <span className="text-sm font-normal text-muted-foreground">
                                        {t("enterprise.quota.selectedCount", { count: checkedDeptIds.size })}
                                    </span>
                                    <Select value={bindPolicyId} onValueChange={setBindPolicyId}>
                                        <SelectTrigger className="w-[180px]">
                                            <SelectValue placeholder={t("enterprise.quota.selectPolicy")} />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {policies.map((p) => (
                                                <SelectItem key={p.id} value={String(p.id)}>{p.name}</SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                    <Button
                                        onClick={handleBatchBind}
                                        disabled={!bindPolicyId || batchBindMutation.isPending}
                                        size="sm"
                                    >
                                        {t("enterprise.quota.bindSelected")}
                                    </Button>
                                </div>
                            )}
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        {queryDepts.length === 0 ? (
                            <div className="text-center py-8 text-muted-foreground">{t("enterprise.quota.noQueryResults")}</div>
                        ) : (
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead className="w-12">
                                            <input
                                                type="checkbox"
                                                checked={checkedDeptIds.size === queryDepts.length && queryDepts.length > 0}
                                                onChange={toggleAllChecked}
                                                className="rounded"
                                            />
                                        </TableHead>
                                        <TableHead>{t("enterprise.quota.department")}</TableHead>
                                        <TableHead>{t("enterprise.quota.memberCount")}</TableHead>
                                        <TableHead>{t("enterprise.quota.currentPolicy")}</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {queryDepts.map((d) => {
                                        const existing = bindingByDeptId.get(d.department_id)
                                        return (
                                            <TableRow key={d.department_id}>
                                                <TableCell>
                                                    <input
                                                        type="checkbox"
                                                        checked={checkedDeptIds.has(d.department_id)}
                                                        onChange={() => toggleCheck(d.department_id)}
                                                        className="rounded"
                                                    />
                                                </TableCell>
                                                <TableCell className="font-medium">{d.name || d.department_id}</TableCell>
                                                <TableCell>{d.member_count > 0 ? `${d.member_count} ${t("enterprise.quota.membersUnit")}` : "—"}</TableCell>
                                                <TableCell>
                                                    {existing?.quota_policy ? (
                                                        <Badge variant="secondary">{existing.quota_policy.name}</Badge>
                                                    ) : (
                                                        <span className="text-muted-foreground">{t("enterprise.quota.noPolicy")}</span>
                                                    )}
                                                </TableCell>
                                            </TableRow>
                                        )
                                    })}
                                </TableBody>
                            </Table>
                        )}
                    </CardContent>
                </Card>
            )}

            {/* Step 3: Current Bindings */}
            <Card>
                <CardHeader>
                    <CardTitle>{t("enterprise.quota.departmentBinding")}</CardTitle>
                </CardHeader>
                <CardContent>
                    {bindingsLoading ? (
                        <div className="text-center py-8 text-muted-foreground">{t("common.loading")}</div>
                    ) : bindingList.length === 0 ? (
                        <div className="text-center py-8 text-muted-foreground">{t("enterprise.quota.noDeptBindings")}</div>
                    ) : (
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>{t("enterprise.quota.level1Department")}</TableHead>
                                    <TableHead>{t("enterprise.quota.level2Department")}</TableHead>
                                    <TableHead>{t("enterprise.quota.policy")}</TableHead>
                                    <TableHead>{t("enterprise.quota.memberCount")}</TableHead>
                                    <TableHead>{t("enterprise.quota.overrideCount")}</TableHead>
                                    <TableHead>{t("enterprise.quota.policyUpdatedAt")}</TableHead>
                                    <TableHead className="w-24">{t("common.edit")}</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {bindingList.map((b: DepartmentQuotaPolicyBinding) => (
                                    <TableRow key={b.id}>
                                        <TableCell>{b.level1_name || "—"}</TableCell>
                                        <TableCell>{b.level2_name || "—"}</TableCell>
                                        <TableCell>
                                            <Badge variant="secondary">
                                                {b.quota_policy?.name || `#${b.quota_policy_id}`}
                                            </Badge>
                                        </TableCell>
                                        <TableCell>
                                            {b.member_count ?? 0}{t("enterprise.quota.membersUnit") ? ` ${t("enterprise.quota.membersUnit")}` : ""}
                                        </TableCell>
                                        <TableCell>
                                            {b.override_count ?? 0}{t("enterprise.quota.membersUnit") ? ` ${t("enterprise.quota.membersUnit")}` : ""}
                                        </TableCell>
                                        <TableCell className="text-sm text-muted-foreground">
                                            {new Date(b.updated_at).toLocaleString()}
                                        </TableCell>
                                        <TableCell>
                                            {canManage && (
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="text-red-500 hover:text-red-600"
                                                    onClick={() => unbindMutation.mutate(b.department_id)}
                                                    disabled={unbindMutation.isPending}
                                                >
                                                    {t("enterprise.quota.unbind")}
                                                </Button>
                                            )}
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    )}
                </CardContent>
            </Card>
        </div>
    )
}

// ─── User Override Tab ──────────────────────────────────────────────────────

function UserOverrideTab({ policies, canManage }: { policies: QuotaPolicy[]; canManage: boolean }) {
    const { t } = useTranslation()
    const queryClient = useQueryClient()

    // Search & filter state
    const [searchKeyword, setSearchKeyword] = useState("")
    const [selectedPolicyFilters, setSelectedPolicyFilters] = useState<Set<string>>(new Set())
    const [queryUsers, setQueryUsers] = useState<FeishuUser[]>([])
    const [hasQueried, setHasQueried] = useState(false)
    const [checkedOpenIds, setCheckedOpenIds] = useState<Set<string>>(new Set())
    const [bindPolicyId, setBindPolicyId] = useState<string>("")

    // Search users query (triggered on demand, fetch larger set for client-side policy filtering)
    const searchQuery = useQuery({
        queryKey: ["enterprise", "feishu-users-search", searchKeyword],
        queryFn: () => enterpriseApi.getFeishuUsers(1, 500, searchKeyword || undefined),
        enabled: false,
    })

    const { data: userBindings, isLoading: userBindingsLoading } = useQuery({
        queryKey: ["enterprise", "user-bindings"],
        queryFn: () => enterpriseApi.listUserPolicyBindings(),
    })

    const batchBindMutation = useMutation({
        mutationFn: ({ open_ids, quota_policy_id }: { open_ids: string[]; quota_policy_id: number }) =>
            enterpriseApi.batchBindPolicyToUsers(open_ids, quota_policy_id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["enterprise", "user-bindings"] })
            setCheckedOpenIds(new Set())
            setBindPolicyId("")
            toast.success(t("enterprise.quota.bindSuccess"))
        },
        onError: (err: Error) => toast.error(err.message),
    })

    const unbindMutation = useMutation({
        mutationFn: (open_id: string) => enterpriseApi.unbindPolicyFromUser(open_id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["enterprise", "user-bindings"] })
            toast.success(t("enterprise.quota.unbindSuccess"))
        },
        onError: (err: Error) => toast.error(err.message),
    })

    const bindingList = userBindings?.bindings || []

    const togglePolicyFilter = (policyName: string) => {
        setSelectedPolicyFilters(prev => {
            const next = new Set(prev)
            if (next.has(policyName)) next.delete(policyName)
            else next.add(policyName)
            return next
        })
    }

    const handleQuery = async () => {
        const result = await searchQuery.refetch()
        const users = result.data?.users || []
        setQueryUsers(users)
        setHasQueried(true)
        setCheckedOpenIds(new Set())
    }

    // Apply client-side policy filter on query results
    const filteredUsers = useMemo(() => queryUsers.filter(u => {
        if (selectedPolicyFilters.size === 0) return true
        const policyName = u.effective_policy || ""
        return selectedPolicyFilters.has(policyName)
    }), [queryUsers, selectedPolicyFilters])

    const toggleUser = (openId: string) => {
        setCheckedOpenIds(prev => {
            const next = new Set(prev)
            if (next.has(openId)) next.delete(openId)
            else next.add(openId)
            return next
        })
    }

    const toggleAllUsers = () => {
        if (checkedOpenIds.size === filteredUsers.length) {
            setCheckedOpenIds(new Set())
        } else {
            setCheckedOpenIds(new Set(filteredUsers.map(u => u.open_id)))
        }
    }

    const handleBatchBind = () => {
        if (checkedOpenIds.size === 0 || !bindPolicyId) return
        batchBindMutation.mutate({
            open_ids: Array.from(checkedOpenIds),
            quota_policy_id: parseInt(bindPolicyId),
        })
    }

    return (
        <div className="space-y-6">
            {/* Card 1: Search & Policy Filter */}
            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <User className="w-5 h-5" />
                        {t("enterprise.quota.searchUser")}
                    </CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="flex items-end gap-4">
                        <div className="flex-1 space-y-2">
                            <Label>{t("enterprise.quota.searchUser")}</Label>
                            <div className="relative">
                                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                                <Input
                                    value={searchKeyword}
                                    onChange={(e) => setSearchKeyword(e.target.value)}
                                    placeholder={t("enterprise.quota.searchUser")}
                                    className="pl-9"
                                    onKeyDown={(e) => { if (e.key === "Enter") handleQuery() }}
                                />
                            </div>
                        </div>
                        <div className="space-y-2 min-w-[200px]">
                            <Label>{t("enterprise.quota.filterByPolicy")}</Label>
                            <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                    <Button variant="outline" className="w-full justify-start gap-1.5">
                                        <Shield className="w-4 h-4" />
                                        {selectedPolicyFilters.size === 0
                                            ? t("enterprise.quota.allPolicies")
                                            : t("enterprise.quota.selectedCount", { count: selectedPolicyFilters.size })}
                                    </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="start" className="w-56">
                                    <DropdownMenuLabel>{t("enterprise.quota.filterByPolicy")}</DropdownMenuLabel>
                                    <DropdownMenuSeparator />
                                    {policies.map((p) => (
                                        <DropdownMenuCheckboxItem
                                            key={p.id}
                                            checked={selectedPolicyFilters.has(p.name)}
                                            onCheckedChange={() => togglePolicyFilter(p.name)}
                                        >
                                            {p.name}
                                        </DropdownMenuCheckboxItem>
                                    ))}
                                    <DropdownMenuSeparator />
                                    <DropdownMenuCheckboxItem
                                        checked={selectedPolicyFilters.has("")}
                                        onCheckedChange={() => togglePolicyFilter("")}
                                    >
                                        {t("enterprise.quota.noPolicy")}
                                    </DropdownMenuCheckboxItem>
                                </DropdownMenuContent>
                            </DropdownMenu>
                        </div>
                        <Button onClick={handleQuery} disabled={searchQuery.isFetching}>
                            <Search className="w-4 h-4 mr-1" />
                            {t("enterprise.quota.query")}
                        </Button>
                    </div>
                </CardContent>
            </Card>

            {/* Card 2: Search Results with Checkboxes */}
            {hasQueried && (
                <Card>
                    <CardHeader>
                        <div className="flex items-center justify-between">
                            <CardTitle>
                                {t("enterprise.quota.queryResults")}
                                {selectedPolicyFilters.size > 0 && queryUsers.length !== filteredUsers.length && (
                                    <span className="ml-2 text-sm font-normal text-muted-foreground">
                                        ({filteredUsers.length}/{queryUsers.length})
                                    </span>
                                )}
                            </CardTitle>
                            {canManage && filteredUsers.length > 0 && checkedOpenIds.size > 0 && (
                                <div className="flex items-center gap-3">
                                    <span className="text-sm text-muted-foreground">
                                        {t("enterprise.quota.selectedCount", { count: checkedOpenIds.size })}
                                    </span>
                                    <Select value={bindPolicyId} onValueChange={setBindPolicyId}>
                                        <SelectTrigger className="w-48">
                                            <SelectValue placeholder={t("enterprise.quota.selectPolicy")} />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {policies.map((p) => (
                                                <SelectItem key={p.id} value={String(p.id)}>{p.name}</SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                    <Button
                                        onClick={handleBatchBind}
                                        disabled={!bindPolicyId || batchBindMutation.isPending}
                                        size="sm"
                                    >
                                        {t("enterprise.quota.bindSelected")}
                                    </Button>
                                </div>
                            )}
                        </div>
                    </CardHeader>
                    <CardContent>
                        {filteredUsers.length === 0 ? (
                            <div className="text-center py-8 text-muted-foreground">
                                {t("enterprise.quota.noUserResults")}
                            </div>
                        ) : (
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead className="w-12">
                                            <input
                                                type="checkbox"
                                                checked={checkedOpenIds.size === filteredUsers.length && filteredUsers.length > 0}
                                                onChange={toggleAllUsers}
                                            />
                                        </TableHead>
                                        <TableHead>{t("enterprise.quota.user")}</TableHead>
                                        <TableHead>{t("enterprise.users.email")}</TableHead>
                                        <TableHead>{t("enterprise.quota.level1Department")}</TableHead>
                                        <TableHead>{t("enterprise.quota.currentPolicy")}</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {filteredUsers.map((u) => (
                                        <TableRow key={u.open_id}>
                                            <TableCell>
                                                <input
                                                    type="checkbox"
                                                    checked={checkedOpenIds.has(u.open_id)}
                                                    onChange={() => toggleUser(u.open_id)}
                                                />
                                            </TableCell>
                                            <TableCell className="font-medium">{u.name || u.open_id}</TableCell>
                                            <TableCell className="text-sm text-muted-foreground">{u.email || "-"}</TableCell>
                                            <TableCell className="text-sm">{u.level1_dept_name || "-"}</TableCell>
                                            <TableCell>
                                                {u.effective_policy ? (
                                                    <Badge variant={u.policy_source === "user" ? "default" : "outline"}>
                                                        {u.effective_policy}
                                                    </Badge>
                                                ) : (
                                                    <span className="text-muted-foreground">-</span>
                                                )}
                                            </TableCell>
                                        </TableRow>
                                    ))}
                                </TableBody>
                            </Table>
                        )}
                    </CardContent>
                </Card>
            )}

            {/* Card 3: Current User Overrides */}
            <Card>
                <CardHeader>
                    <CardTitle>{t("enterprise.quota.personalOverride")}</CardTitle>
                </CardHeader>
                <CardContent>
                    {userBindingsLoading ? (
                        <div className="text-center py-8 text-muted-foreground">{t("common.loading")}</div>
                    ) : bindingList.length === 0 ? (
                        <div className="text-center py-8 text-muted-foreground">{t("enterprise.quota.noUserOverrides")}</div>
                    ) : (
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>{t("enterprise.quota.user")}</TableHead>
                                    <TableHead>{t("enterprise.quota.policy")}</TableHead>
                                    <TableHead className="w-24">{t("common.edit")}</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {bindingList.map((b: UserQuotaPolicy) => (
                                    <TableRow key={b.id}>
                                        <TableCell>{b.user_name || b.open_id}</TableCell>
                                        <TableCell>
                                            <Badge variant="outline">
                                                {b.quota_policy?.name || `#${b.quota_policy_id}`}
                                            </Badge>
                                        </TableCell>
                                        <TableCell>
                                            {canManage && (
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="text-red-500 hover:text-red-600"
                                                    onClick={() => unbindMutation.mutate(b.open_id)}
                                                    disabled={unbindMutation.isPending}
                                                >
                                                    {t("enterprise.quota.unbind")}
                                                </Button>
                                            )}
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    )}
                </CardContent>
            </Card>
        </div>
    )
}

// ─── Notification Config Tab ────────────────────────────────────────────────

const DEFAULT_NOTIF_CONFIG = {
    enabled: false,
    tier2_title: "AI 用量提醒",
    tier2_body: "您好 {name}，您本{period_type}的 AI 用量已达 {usage_pct}（阈值 {tier_threshold}，周期额度 {period_quota}），已进入二级限速，RPM/TPM 有所降低，请注意控制用量。",
    tier3_title: "AI 用量紧张提醒",
    tier3_body: "您好 {name}，您本{period_type}的 AI 用量已达 {usage_pct}（阈值 {tier_threshold}，周期额度 {period_quota}），已进入三级限速，请控制用量以避免服务中断。",
    exhaust_title: "AI 用量已耗尽",
    exhaust_body: "您好 {name}，您本{period_type}的 AI 用量已耗尽（周期额度 {period_quota}），所有请求将被拒绝，请联系管理员或等待下一周期重置。",
}

function NotifConfigTab({ canManage }: { canManage: boolean }) {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const [draft, setDraft] = useState<QuotaNotifConfig | null>(null)

    const { data: serverCfg, isLoading } = useQuery({
        queryKey: ["enterprise", "quota-notif-config"],
        queryFn: () => enterpriseApi.getNotifConfig(),
    })

    const saveMutation = useMutation({
        mutationFn: (cfg: QuotaNotifConfig) => enterpriseApi.updateNotifConfig(cfg),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["enterprise", "quota-notif-config"] })
            toast.success(t("enterprise.quota.notif.saveSuccess" as never))
        },
        onError: (err: Error) => toast.error(err.message),
    })

    const cfg: QuotaNotifConfig = draft ?? serverCfg ?? DEFAULT_NOTIF_CONFIG

    const VARIABLES = [
        { key: "{name}", label: t("enterprise.quota.notif.varName" as never) },
        { key: "{usage_pct}", label: t("enterprise.quota.notif.varUsagePct" as never) },
        { key: "{tier_threshold}", label: t("enterprise.quota.notif.varTierThreshold" as never) },
        { key: "{period_quota}", label: t("enterprise.quota.notif.varPeriodQuota" as never) },
        { key: "{period_type}", label: t("enterprise.quota.notif.varPeriodType" as never) },
    ]

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text).catch(() => {})
        toast.success(`已复制 ${text}`)
    }

    if (isLoading) {
        return <div className="text-center py-8 text-muted-foreground">{t("common.loading")}</div>
    }

    return (
        <div className="space-y-6">
            {/* P2P warning — only shown when backend reports P2P is not configured */}
            {serverCfg && !serverCfg.p2p_available && (
                <Card className="border-amber-200 bg-amber-50/50 dark:bg-amber-950/20">
                    <CardContent className="pt-4 pb-3">
                        <div className="flex gap-2 items-start text-sm text-amber-700 dark:text-amber-400">
                            <AlertTriangle className="w-4 h-4 mt-0.5 shrink-0" />
                            <span>{t("enterprise.quota.notif.noP2PWarning" as never)}</span>
                        </div>
                    </CardContent>
                </Card>
            )}

            {/* Enable switch */}
            <Card>
                <CardContent className="pt-4">
                    <div className="flex items-center justify-between">
                        <div>
                            <Label className="text-base font-medium">{t("enterprise.quota.notif.enable" as never)}</Label>
                            <p className="text-sm text-muted-foreground mt-0.5">{t("enterprise.quota.notif.enableDesc" as never)}</p>
                        </div>
                        <Switch
                            checked={cfg.enabled}
                            onCheckedChange={(v) => setDraft({ ...cfg, enabled: v })}
                            disabled={!canManage}
                        />
                    </div>
                </CardContent>
            </Card>

            {/* Template sections */}
            {[
                { key: "tier2", titleField: "tier2_title" as const, bodyField: "tier2_body" as const, color: "yellow" },
                { key: "tier3", titleField: "tier3_title" as const, bodyField: "tier3_body" as const, color: "red" },
                { key: "exhaust", titleField: "exhaust_title" as const, bodyField: "exhaust_body" as const, color: "red" },
            ].map(({ key, titleField, bodyField, color }) => (
                <Card key={key} className={color === "yellow" ? "border-yellow-200" : "border-red-200"}>
                    <CardHeader className="pb-3">
                        <CardTitle className="text-sm font-medium">
                            {t(`enterprise.quota.notif.${key}` as never)}
                        </CardTitle>
                        <p className="text-xs text-muted-foreground">{t(`enterprise.quota.notif.${key}Desc` as never)}</p>
                    </CardHeader>
                    <CardContent className="space-y-3">
                        <div className="space-y-1">
                            <Label className="text-xs">{t("enterprise.quota.notif.msgTitle" as never)}</Label>
                            <Input
                                value={cfg[titleField]}
                                onChange={(e) => setDraft({ ...cfg, [titleField]: e.target.value })}
                                disabled={!canManage}
                            />
                        </div>
                        <div className="space-y-1">
                            <Label className="text-xs">{t("enterprise.quota.notif.msgBody" as never)}</Label>
                            <Textarea
                                value={cfg[bodyField]}
                                onChange={(e) => setDraft({ ...cfg, [bodyField]: e.target.value })}
                                rows={3}
                                disabled={!canManage}
                            />
                        </div>
                    </CardContent>
                </Card>
            ))}

            {/* Variable hints */}
            <Card>
                <CardContent className="pt-4">
                    <p className="text-xs font-medium text-muted-foreground mb-2">{t("enterprise.quota.notif.variables" as never)}</p>
                    <div className="flex flex-wrap gap-2">
                        {VARIABLES.map(({ key, label }) => (
                            <button
                                key={key}
                                onClick={() => copyToClipboard(key)}
                                className="inline-flex items-center gap-1 px-2 py-1 rounded border text-xs font-mono bg-muted hover:bg-muted/80 transition-colors"
                            >
                                {label}
                            </button>
                        ))}
                    </div>
                </CardContent>
            </Card>

            {/* Actions */}
            {canManage && (
                <div className="flex justify-end gap-2">
                    <Button
                        variant="outline"
                        onClick={() => {
                            setDraft(DEFAULT_NOTIF_CONFIG)
                            toast.success(t("enterprise.quota.notif.resetSuccess" as never))
                        }}
                    >
                        {t("enterprise.quota.notif.resetDefaults" as never)}
                    </Button>
                    <Button
                        onClick={() => draft && saveMutation.mutate(draft)}
                        disabled={saveMutation.isPending}
                    >
                        {saveMutation.isPending ? t("common.saving") : t("enterprise.quota.notif.save" as never)}
                    </Button>
                </div>
            )}
        </div>
    )
}

// ─── Main Page ──────────────────────────────────────────────────────────────

export default function QuotaPoliciesPage() {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const canManage = useHasPermission('quota_manage_manage')
    const [editingPolicy, setEditingPolicy] = useState<QuotaPolicy | null>(null)
    const [isCreating, setIsCreating] = useState(false)
    const [formData, setFormData] = useState<QuotaPolicyInput>(defaultPolicy)
    const [deleteTarget, setDeleteTarget] = useState<QuotaPolicy | null>(null)

    const { data, isLoading } = useQuery({
        queryKey: ["enterprise", "quota-policies"],
        queryFn: () => enterpriseApi.listQuotaPolicies(),
    })

    const createMutation = useMutation({
        mutationFn: enterpriseApi.createQuotaPolicy,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["enterprise", "quota-policies"] })
            setIsCreating(false)
            setFormData(defaultPolicy)
            toast.success(t("enterprise.quota.createSuccess"))
        },
        onError: (err: Error) => {
            toast.error(err.message)
        },
    })

    const updateMutation = useMutation({
        mutationFn: ({ id, data }: { id: number; data: QuotaPolicyInput }) =>
            enterpriseApi.updateQuotaPolicy(id, data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["enterprise", "quota-policies"] })
            setEditingPolicy(null)
            toast.success(t("enterprise.quota.updateSuccess"))
        },
        onError: (err: Error) => {
            toast.error(err.message)
        },
    })

    const deleteMutation = useMutation({
        mutationFn: enterpriseApi.deleteQuotaPolicy,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["enterprise", "quota-policies"] })
            setDeleteTarget(null)
            toast.success(t("enterprise.quota.deleteSuccess"))
        },
        onError: (err: Error) => {
            toast.error(err.message)
        },
    })

    const policies = data?.policies || []

    const periodTypeLabel = (pt: number) => {
        switch (pt) {
            case 1: return t("enterprise.quota.daily")
            case 2: return t("enterprise.quota.weekly")
            case 3: return t("enterprise.quota.monthly")
            default: return "-"
        }
    }

    const handleCreate = () => {
        setFormData(defaultPolicy)
        setIsCreating(true)
    }

    const handleEdit = (policy: QuotaPolicy) => {
        setEditingPolicy(policy)
        setFormData({
            name: policy.name,
            tier1_ratio: policy.tier1_ratio,
            tier2_ratio: policy.tier2_ratio,
            tier1_rpm_multiplier: policy.tier1_rpm_multiplier,
            tier1_tpm_multiplier: policy.tier1_tpm_multiplier,
            tier2_rpm_multiplier: policy.tier2_rpm_multiplier,
            tier2_tpm_multiplier: policy.tier2_tpm_multiplier,
            tier3_rpm_multiplier: policy.tier3_rpm_multiplier,
            tier3_tpm_multiplier: policy.tier3_tpm_multiplier,
            block_at_tier3: policy.block_at_tier3,
            tier2_blocked_models: policy.tier2_blocked_models || "",
            tier3_blocked_models: policy.tier3_blocked_models || "",
            period_quota: policy.period_quota,
            period_type: policy.period_type,
        })
    }

    const handleSave = () => {
        if (!formData.name.trim()) {
            toast.error(t("enterprise.quota.nameRequired"))
            return
        }
        if (formData.tier1_ratio <= 0 || formData.tier1_ratio >= formData.tier2_ratio || formData.tier2_ratio > 1) {
            toast.error(t("enterprise.quota.ratioError"))
            return
        }

        if (editingPolicy) {
            updateMutation.mutate({ id: editingPolicy.id, data: formData })
        } else {
            createMutation.mutate(formData)
        }
    }

    return (
        <div className="p-6 space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold">{t("enterprise.quota.title")}</h1>
                    <p className="text-muted-foreground">{t("enterprise.quota.description")}</p>
                </div>
            </div>

            <Tabs defaultValue="policies">
                <TabsList>
                    <TabsTrigger value="policies">
                        <Shield className="w-4 h-4 mr-1.5" />
                        {t("enterprise.quota.policyListTab")}
                    </TabsTrigger>
                    <TabsTrigger value="departments">
                        <Building2 className="w-4 h-4 mr-1.5" />
                        {t("enterprise.quota.departmentBinding")}
                    </TabsTrigger>
                    <TabsTrigger value="users">
                        <User className="w-4 h-4 mr-1.5" />
                        {t("enterprise.quota.userOverride")}
                    </TabsTrigger>
                    <TabsTrigger value="notif">
                        <Bell className="w-4 h-4 mr-1.5" />
                        {t("enterprise.quota.notif.tab" as never)}
                    </TabsTrigger>
                </TabsList>

                {/* Tab 1: Policy List */}
                <TabsContent value="policies">
                    <Card>
                        <CardHeader>
                            <div className="flex items-center justify-between">
                                <CardTitle className="flex items-center gap-2">
                                    <Shield className="w-5 h-5" />
                                    {t("enterprise.quota.policyList")} ({policies.length})
                                </CardTitle>
                                {canManage && (
                                    <Button onClick={handleCreate}>
                                        <Plus className="w-4 h-4 mr-2" />
                                        {t("enterprise.quota.createPolicy")}
                                    </Button>
                                )}
                            </div>
                        </CardHeader>
                        <CardContent>
                            {isLoading ? (
                                <div className="text-center py-8 text-muted-foreground">{t("common.loading")}</div>
                            ) : policies.length === 0 ? (
                                <div className="text-center py-8 text-muted-foreground">{t("enterprise.quota.noPolicies")}</div>
                            ) : (
                                <Table>
                                    <TableHeader>
                                        <TableRow>
                                            <TableHead>{t("enterprise.quota.name")}</TableHead>
                                            <TableHead>{t("enterprise.quota.periodQuota")}</TableHead>
                                            <TableHead>{t("enterprise.quota.thresholds")}</TableHead>
                                            <TableHead>{t("enterprise.quota.tier1")}</TableHead>
                                            <TableHead>{t("enterprise.quota.tier2")}</TableHead>
                                            <TableHead>{t("enterprise.quota.tier3")}</TableHead>
                                            <TableHead className="w-24">{t("common.edit")}</TableHead>
                                        </TableRow>
                                    </TableHeader>
                                    <TableBody>
                                        {policies.map((policy) => (
                                            <TableRow key={policy.id}>
                                                <TableCell className="font-medium">{policy.name}</TableCell>
                                                <TableCell>
                                                    {policy.period_quota > 0 ? (
                                                        <div className="text-sm">
                                                            <span className="font-medium">{policy.period_quota}</span>
                                                            <span className="text-muted-foreground ml-1">/ {periodTypeLabel(policy.period_type)}</span>
                                                        </div>
                                                    ) : (
                                                        <span className="text-xs text-muted-foreground">-</span>
                                                    )}
                                                </TableCell>
                                                <TableCell>
                                                    <div className="space-y-1">
                                                        <TierIndicator ratio={policy.tier1_ratio} label="T1" />
                                                        <TierIndicator ratio={policy.tier2_ratio} label="T2" />
                                                    </div>
                                                </TableCell>
                                                <TableCell>
                                                    <div className="text-xs space-y-0.5">
                                                        <div>RPM: {policy.tier1_rpm_multiplier}x</div>
                                                        <div>TPM: {policy.tier1_tpm_multiplier}x</div>
                                                    </div>
                                                </TableCell>
                                                <TableCell>
                                                    <div className="text-xs space-y-0.5">
                                                        <div>RPM: {policy.tier2_rpm_multiplier}x</div>
                                                        <div>TPM: {policy.tier2_tpm_multiplier}x</div>
                                                    </div>
                                                </TableCell>
                                                <TableCell>
                                                    {policy.block_at_tier3 ? (
                                                        <span className="text-xs text-red-500 font-medium">{t("enterprise.quota.blocked")}</span>
                                                    ) : (
                                                        <div className="text-xs space-y-0.5">
                                                            <div>RPM: {policy.tier3_rpm_multiplier}x</div>
                                                            <div>TPM: {policy.tier3_tpm_multiplier}x</div>
                                                        </div>
                                                    )}
                                                </TableCell>
                                                <TableCell>
                                                    {canManage && (
                                                        <div className="flex items-center gap-1">
                                                            <Button variant="ghost" size="icon" onClick={() => handleEdit(policy)}>
                                                                <Pencil className="w-4 h-4" />
                                                            </Button>
                                                            <Button
                                                                variant="ghost"
                                                                size="icon"
                                                                className="text-red-500 hover:text-red-600"
                                                                onClick={() => setDeleteTarget(policy)}
                                                            >
                                                                <Trash2 className="w-4 h-4" />
                                                            </Button>
                                                        </div>
                                                    )}
                                                </TableCell>
                                            </TableRow>
                                        ))}
                                    </TableBody>
                                </Table>
                            )}
                        </CardContent>
                    </Card>
                </TabsContent>

                {/* Tab 2: Department Binding */}
                <TabsContent value="departments">
                    <DepartmentBindingTab policies={policies} canManage={canManage} />
                </TabsContent>

                {/* Tab 3: User Override */}
                <TabsContent value="users">
                    <UserOverrideTab policies={policies} canManage={canManage} />
                </TabsContent>

                {/* Tab 4: Notification Settings */}
                <TabsContent value="notif">
                    <NotifConfigTab canManage={canManage} />
                </TabsContent>
            </Tabs>

            {/* Create/Edit Dialog */}
            <Dialog open={isCreating || !!editingPolicy} onOpenChange={(open) => {
                if (!open) {
                    setIsCreating(false)
                    setEditingPolicy(null)
                    setFormData(defaultPolicy)
                }
            }}>
                <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>
                            {editingPolicy ? t("enterprise.quota.editPolicy") : t("enterprise.quota.createPolicy")}
                        </DialogTitle>
                        <DialogDescription>
                            {t("enterprise.quota.formDescription")}
                        </DialogDescription>
                    </DialogHeader>
                    <PolicyForm policy={formData} onChange={setFormData} />
                    <DialogFooter>
                        <Button variant="outline" onClick={() => { setIsCreating(false); setEditingPolicy(null) }}>
                            {t("common.cancel")}
                        </Button>
                        <Button
                            onClick={handleSave}
                            disabled={createMutation.isPending || updateMutation.isPending}
                        >
                            {(createMutation.isPending || updateMutation.isPending) ? t("common.saving") : t("common.save")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Delete Confirmation */}
            <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>{t("enterprise.quota.deleteConfirmTitle")}</AlertDialogTitle>
                        <AlertDialogDescription>
                            {t("enterprise.quota.deleteConfirmDesc", { name: deleteTarget?.name })}
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
                        <AlertDialogAction
                            className="bg-red-500 hover:bg-red-600"
                            onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
                            disabled={deleteMutation.isPending}
                        >
                            {deleteMutation.isPending ? t("common.deleting") : t("common.delete")}
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </div>
    )
}
