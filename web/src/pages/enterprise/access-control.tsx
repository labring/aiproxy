import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Shield, Plus, Pencil, Trash2, AlertCircle, Check, X, Users, UserX } from "lucide-react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
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
import { enterpriseApi, type TenantSummaryItem } from "@/api/enterprise"
import { toast } from "sonner"
import { useHasPermission } from "@/lib/permissions"

const SUMMARY_KEY = "tenant-summary"
const CONFIG_KEY = "tenant-whitelist"

export default function AccessControlPage() {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const [addDialogOpen, setAddDialogOpen] = useState(false)
    const [editDialogOpen, setEditDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [selectedWhitelistId, setSelectedWhitelistId] = useState<number | null>(null)
    const [newTenant, setNewTenant] = useState({ tenant_id: "", name: "" })
    const [editTenant, setEditTenant] = useState({ id: 0, name: "" })

    // Fetch config (wildcard mode etc.)
    const { data: configData, isLoading: configLoading } = useQuery({
        queryKey: [CONFIG_KEY],
        queryFn: () => enterpriseApi.getTenantWhitelist(),
    })
    const config = configData?.config ?? { wildcard_mode: false, env_override: false, description: "" }

    // Fetch unified tenant summary (30s polling)
    const { data: summaryData, isLoading: summaryLoading } = useQuery({
        queryKey: [SUMMARY_KEY],
        queryFn: () => enterpriseApi.getTenantSummary(),
        refetchInterval: 30000,
    })
    const tenants: TenantSummaryItem[] = summaryData?.tenants ?? []

    const invalidateAll = () => {
        queryClient.invalidateQueries({ queryKey: [SUMMARY_KEY] })
        queryClient.invalidateQueries({ queryKey: [CONFIG_KEY] })
    }

    // Add / approve tenant mutation
    const addMutation = useMutation({
        mutationFn: (params: { tenant_id: string; name?: string }) =>
            enterpriseApi.addTenantToWhitelist(params.tenant_id, params.name),
        onSuccess: () => {
            invalidateAll()
            toast.success(t("enterprise.accessControl.addSuccess"))
            setAddDialogOpen(false)
            setNewTenant({ tenant_id: "", name: "" })
        },
        onError: (error: Error) => {
            toast.error(error.message || t("enterprise.accessControl.addFailed"))
        },
    })

    // Update tenant name mutation
    const editMutation = useMutation({
        mutationFn: ({ id, name }: { id: number; name: string }) =>
            enterpriseApi.updateTenantWhitelist(id, name),
        onSuccess: () => {
            invalidateAll()
            toast.success(t("enterprise.accessControl.editSuccess"))
            setEditDialogOpen(false)
        },
        onError: (error: Error) => {
            toast.error(error.message || t("enterprise.accessControl.editFailed"))
        },
    })

    // Remove from whitelist mutation
    const deleteMutation = useMutation({
        mutationFn: (id: number) => enterpriseApi.removeTenantFromWhitelist(id),
        onSuccess: () => {
            invalidateAll()
            toast.success(t("enterprise.accessControl.deleteSuccess"))
            setDeleteDialogOpen(false)
            setSelectedWhitelistId(null)
        },
        onError: (error: Error) => {
            toast.error(error.message || t("enterprise.accessControl.deleteFailed"))
        },
    })

    // Dismiss rejected login record
    const dismissMutation = useMutation({
        mutationFn: (id: number) => enterpriseApi.dismissRejectedTenantLogin(id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: [SUMMARY_KEY] })
            toast.success(t("enterprise.accessControl.dismissSuccess"))
        },
        onError: (error: Error) => {
            toast.error(error.message || t("enterprise.accessControl.dismissFailed"))
        },
    })

    // Update config mutation
    const updateConfigMutation = useMutation({
        mutationFn: (cfg: { wildcard_mode: boolean; env_override: boolean; description?: string }) =>
            enterpriseApi.updateWhitelistConfig(cfg),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: [CONFIG_KEY] })
            toast.success(t("enterprise.accessControl.configUpdated"))
        },
        onError: (error: Error) => {
            toast.error(error.message || t("enterprise.accessControl.configUpdateFailed"))
        },
    })

    const handleAddTenant = () => {
        if (!newTenant.tenant_id.trim()) {
            toast.error(t("enterprise.accessControl.tenantIdRequired"))
            return
        }
        addMutation.mutate(newTenant)
    }

    const confirmDelete = () => {
        if (selectedWhitelistId) {
            deleteMutation.mutate(selectedWhitelistId)
        }
    }

    const canManage = useHasPermission('access_control_manage')
    const isLoading = configLoading || summaryLoading

    return (
        <div className="p-6 space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold flex items-center gap-2">
                        <Shield className="w-6 h-6 text-[#6A6DE6]" />
                        {t("enterprise.accessControl.title")}
                    </h1>
                    <p className="text-muted-foreground mt-1">{t("enterprise.accessControl.description")}</p>
                </div>
                {canManage && (
                    <Button onClick={() => setAddDialogOpen(true)} className="gap-2">
                        <Plus className="w-4 h-4" />
                        {t("enterprise.accessControl.addTenant")}
                    </Button>
                )}
            </div>

            {/* Configuration Card */}
            <Card>
                <CardHeader>
                    <CardTitle>{t("enterprise.accessControl.configuration")}</CardTitle>
                    <CardDescription>{t("enterprise.accessControl.configDescription")}</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div className="flex items-center justify-between p-4 border rounded-lg">
                        <div className="space-y-1">
                            <Label htmlFor="wildcard-mode" className="text-base font-medium">
                                {t("enterprise.accessControl.wildcardMode")}
                            </Label>
                            <p className="text-sm text-muted-foreground">
                                {t("enterprise.accessControl.wildcardModeDescription")}
                            </p>
                        </div>
                        <Switch
                            id="wildcard-mode"
                            checked={config.wildcard_mode}
                            onCheckedChange={(v) => updateConfigMutation.mutate({ ...config, wildcard_mode: v })}
                            disabled={!canManage || updateConfigMutation.isPending}
                        />
                    </div>

                    <div className="flex items-center justify-between p-4 border rounded-lg">
                        <div className="space-y-1">
                            <Label htmlFor="env-override" className="text-base font-medium">
                                {t("enterprise.accessControl.envOverride")}
                            </Label>
                            <p className="text-sm text-muted-foreground">
                                {t("enterprise.accessControl.envOverrideDescription")}
                            </p>
                        </div>
                        <Switch
                            id="env-override"
                            checked={config.env_override}
                            onCheckedChange={(v) => updateConfigMutation.mutate({ ...config, env_override: v })}
                            disabled={!canManage || updateConfigMutation.isPending}
                        />
                    </div>

                    {config.wildcard_mode && (
                        <div className="flex items-start gap-3 p-4 bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800 rounded-lg">
                            <AlertCircle className="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5" />
                            <div className="flex-1">
                                <p className="text-sm font-medium text-blue-900 dark:text-blue-100">
                                    {t("enterprise.accessControl.wildcardEnabled")}
                                </p>
                                <p className="text-sm text-blue-700 dark:text-blue-300 mt-1">
                                    {t("enterprise.accessControl.wildcardEnabledDescription")}
                                </p>
                            </div>
                        </div>
                    )}
                </CardContent>
            </Card>

            {/* Unified Tenant Overview */}
            <Card>
                <CardHeader>
                    <CardTitle>{t("enterprise.accessControl.tenantOverview")}</CardTitle>
                    <CardDescription>
                        {t("enterprise.accessControl.tenantsCount", { count: tenants.length })}
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    {isLoading ? (
                        <div className="text-center py-8 text-muted-foreground">{t("common.loading")}</div>
                    ) : tenants.length === 0 ? (
                        <div className="text-center py-8 text-muted-foreground">
                            {t("enterprise.accessControl.noTenants")}
                        </div>
                    ) : (
                        <div className="overflow-x-auto">
                            <table className="w-full">
                                <thead>
                                    <tr className="border-b text-muted-foreground text-sm">
                                        <th className="text-left py-3 px-4 font-medium">
                                            {t("enterprise.accessControl.tenantId")}
                                        </th>
                                        <th className="text-left py-3 px-4 font-medium">
                                            {t("enterprise.accessControl.tenantName")}
                                        </th>
                                        <th className="text-center py-3 px-4 font-medium">
                                            {t("enterprise.accessControl.successfulMembers")}
                                        </th>
                                        <th className="text-center py-3 px-4 font-medium">
                                            {t("enterprise.accessControl.rejectedAttempts")}
                                        </th>
                                        <th className="text-left py-3 px-4 font-medium">
                                            {t("enterprise.accessControl.addedBy")}
                                        </th>
                                        <th className="text-right py-3 px-4 font-medium">
                                            {t("enterprise.accessControl.actions")}
                                        </th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {tenants.map((tenant) => (
                                        <tr key={tenant.tenant_id} className="border-b last:border-0 hover:bg-muted/50">
                                            <td className="py-3 px-4">
                                                <code className="text-sm bg-muted px-2 py-1 rounded">
                                                    {tenant.tenant_id}
                                                </code>
                                            </td>
                                            <td className="py-3 px-4">
                                                <div className="flex items-center gap-2">
                                                    {tenant.name ? (
                                                        <span>{tenant.name}</span>
                                                    ) : (
                                                        <span className="text-muted-foreground italic text-sm">
                                                            {t("enterprise.accessControl.noName")}
                                                        </span>
                                                    )}
                                                    {tenant.is_whitelisted && (
                                                        <Badge variant="default" className="text-xs bg-green-600">
                                                            {t("enterprise.accessControl.whitelisted")}
                                                        </Badge>
                                                    )}
                                                    {!tenant.is_whitelisted && tenant.rejected_attempts > 0 && (
                                                        <Badge variant="destructive" className="text-xs">
                                                            {t("enterprise.accessControl.rejected")}
                                                        </Badge>
                                                    )}
                                                </div>
                                            </td>
                                            <td className="py-3 px-4 text-center">
                                                <div className="flex items-center justify-center gap-1">
                                                    <Users className="w-3.5 h-3.5 text-green-600" />
                                                    <span className="font-medium">{tenant.successful_members}</span>
                                                </div>
                                            </td>
                                            <td className="py-3 px-4 text-center">
                                                {tenant.rejected_attempts > 0 ? (
                                                    <div className="flex items-center justify-center gap-1">
                                                        <UserX className="w-3.5 h-3.5 text-amber-600" />
                                                        <span className="font-medium text-amber-700 dark:text-amber-400">
                                                            {tenant.rejected_attempts}
                                                        </span>
                                                    </div>
                                                ) : (
                                                    <span className="text-muted-foreground">—</span>
                                                )}
                                            </td>
                                            <td className="py-3 px-4">
                                                {tenant.added_by ? (
                                                    <Badge variant="secondary">{tenant.added_by}</Badge>
                                                ) : (
                                                    <span className="text-muted-foreground text-sm">—</span>
                                                )}
                                            </td>
                                            <td className="py-3 px-4 text-right">
                                                <div className="flex items-center justify-end gap-1">
                                                    {canManage && !tenant.is_whitelisted && (
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            onClick={() => addMutation.mutate({
                                                                tenant_id: tenant.tenant_id,
                                                            })}
                                                            disabled={addMutation.isPending}
                                                            className="text-green-600 hover:text-green-700 hover:bg-green-50 dark:hover:bg-green-950"
                                                            title={t("enterprise.accessControl.addToWhitelist")}
                                                        >
                                                            <Check className="w-4 h-4" />
                                                        </Button>
                                                    )}
                                                    {canManage && tenant.is_whitelisted && tenant.whitelist_id && (
                                                        <>
                                                            <Button
                                                                variant="ghost"
                                                                size="sm"
                                                                onClick={() => {
                                                                    setEditTenant({ id: tenant.whitelist_id!, name: tenant.name || "" })
                                                                    setEditDialogOpen(true)
                                                                }}
                                                                className="text-blue-600 hover:text-blue-700 hover:bg-blue-50 dark:hover:bg-blue-950"
                                                                title={t("common.edit")}
                                                            >
                                                                <Pencil className="w-4 h-4" />
                                                            </Button>
                                                            <Button
                                                                variant="ghost"
                                                                size="sm"
                                                                onClick={() => {
                                                                    setSelectedWhitelistId(tenant.whitelist_id!)
                                                                    setDeleteDialogOpen(true)
                                                                }}
                                                                className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-950"
                                                                title={t("enterprise.accessControl.deleteConfirm")}
                                                            >
                                                                <Trash2 className="w-4 h-4" />
                                                            </Button>
                                                        </>
                                                    )}
                                                    {canManage && tenant.rejected_record_id && (
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            onClick={() => dismissMutation.mutate(tenant.rejected_record_id!)}
                                                            disabled={dismissMutation.isPending}
                                                            className="text-muted-foreground hover:text-orange-600 hover:bg-orange-50 dark:hover:bg-orange-950"
                                                            title={t("enterprise.accessControl.dismiss")}
                                                        >
                                                            <X className="w-4 h-4" />
                                                        </Button>
                                                    )}
                                                </div>
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    )}
                </CardContent>
            </Card>

            {/* Add Tenant Dialog */}
            <Dialog open={addDialogOpen} onOpenChange={setAddDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("enterprise.accessControl.addTenant")}</DialogTitle>
                        <DialogDescription>{t("enterprise.accessControl.addTenantDescription")}</DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 py-4">
                        <div className="space-y-2">
                            <Label htmlFor="tenant-id">{t("enterprise.accessControl.tenantId")} *</Label>
                            <Input
                                id="tenant-id"
                                placeholder={t("enterprise.accessControl.tenantIdPlaceholder")}
                                value={newTenant.tenant_id}
                                onChange={(e) => setNewTenant({ ...newTenant, tenant_id: e.target.value })}
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="tenant-name">{t("enterprise.accessControl.tenantName")}</Label>
                            <Input
                                id="tenant-name"
                                placeholder={t("enterprise.accessControl.tenantNamePlaceholder")}
                                value={newTenant.name}
                                onChange={(e) => setNewTenant({ ...newTenant, name: e.target.value })}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setAddDialogOpen(false)}>
                            {t("common.cancel")}
                        </Button>
                        <Button onClick={handleAddTenant} disabled={addMutation.isPending}>
                            {addMutation.isPending ? t("common.saving") : t("common.save")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Edit Tenant Dialog */}
            <Dialog open={editDialogOpen} onOpenChange={setEditDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("enterprise.accessControl.editTenant")}</DialogTitle>
                        <DialogDescription>{t("enterprise.accessControl.editTenantDescription")}</DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 py-4">
                        <div className="space-y-2">
                            <Label>{t("enterprise.accessControl.tenantName")}</Label>
                            <Input
                                value={editTenant.name}
                                onChange={(e) => setEditTenant({ ...editTenant, name: e.target.value })}
                                placeholder={t("enterprise.accessControl.tenantNamePlaceholder")}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setEditDialogOpen(false)}>
                            {t("common.cancel")}
                        </Button>
                        <Button
                            onClick={() => editMutation.mutate(editTenant)}
                            disabled={editMutation.isPending}
                        >
                            {editMutation.isPending ? t("common.saving") : t("common.save")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Delete Confirmation Dialog */}
            <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>{t("enterprise.accessControl.deleteConfirm")}</AlertDialogTitle>
                        <AlertDialogDescription>
                            {t("enterprise.accessControl.deleteConfirmDescription")}
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
                        <AlertDialogAction
                            onClick={confirmDelete}
                            className="bg-red-600 hover:bg-red-700"
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
