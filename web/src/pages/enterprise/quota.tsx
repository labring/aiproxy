import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Pencil, Trash2, Shield, AlertTriangle } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
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
import { enterpriseApi, type QuotaPolicy, type QuotaPolicyInput } from "@/api/enterprise"
import { toast } from "sonner"

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
}

function TierIndicator({ ratio, label }: { ratio: number; label: string }) {
    // Clamp ratio to 0-1 range for display
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

export default function QuotaPoliciesPage() {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
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
        })
    }

    const handleSave = () => {
        if (!formData.name.trim()) {
            toast.error(t("enterprise.quota.nameRequired"))
            return
        }
        // Validate: 0 < tier1 < tier2 <= 1
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
                <Button onClick={handleCreate}>
                    <Plus className="w-4 h-4 mr-2" />
                    {t("enterprise.quota.createPolicy")}
                </Button>
            </div>

            {/* Policies Table */}
            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <Shield className="w-5 h-5" />
                        {t("enterprise.quota.policyList")} ({policies.length})
                    </CardTitle>
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
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    )}
                </CardContent>
            </Card>

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
