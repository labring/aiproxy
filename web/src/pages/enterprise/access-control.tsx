import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Shield, Plus, Trash2, AlertCircle } from "lucide-react"
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
import { enterpriseApi } from "@/api/enterprise"
import { toast } from "sonner"

export default function AccessControlPage() {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const [addDialogOpen, setAddDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [selectedTenantId, setSelectedTenantId] = useState<number | null>(null)
    const [newTenant, setNewTenant] = useState({ tenant_id: "", name: "" })

    // Fetch whitelist data
    const { data, isLoading } = useQuery({
        queryKey: ["tenant-whitelist"],
        queryFn: () => enterpriseApi.getTenantWhitelist(),
    })

    const tenants = data?.tenants || []
    const config = data?.config || { wildcard_mode: true, env_override: false, description: "" }

    // Add tenant mutation
    const addMutation = useMutation({
        mutationFn: (params: { tenant_id: string; name?: string }) =>
            enterpriseApi.addTenantToWhitelist(params.tenant_id, params.name),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["tenant-whitelist"] })
            toast.success(t("enterprise.accessControl.addSuccess"))
            setAddDialogOpen(false)
            setNewTenant({ tenant_id: "", name: "" })
        },
        onError: (error: Error) => {
            toast.error(error.message || t("enterprise.accessControl.addFailed"))
        },
    })

    // Delete tenant mutation
    const deleteMutation = useMutation({
        mutationFn: (id: number) => enterpriseApi.removeTenantFromWhitelist(id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["tenant-whitelist"] })
            toast.success(t("enterprise.accessControl.deleteSuccess"))
            setDeleteDialogOpen(false)
            setSelectedTenantId(null)
        },
        onError: (error: Error) => {
            toast.error(error.message || t("enterprise.accessControl.deleteFailed"))
        },
    })

    // Update config mutation
    const updateConfigMutation = useMutation({
        mutationFn: (config: { wildcard_mode: boolean; env_override: boolean; description?: string }) =>
            enterpriseApi.updateWhitelistConfig(config),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["tenant-whitelist"] })
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

    const handleDeleteTenant = (id: number) => {
        setSelectedTenantId(id)
        setDeleteDialogOpen(true)
    }

    const confirmDelete = () => {
        if (selectedTenantId) {
            deleteMutation.mutate(selectedTenantId)
        }
    }

    const handleWildcardToggle = (checked: boolean) => {
        updateConfigMutation.mutate({
            ...config,
            wildcard_mode: checked,
        })
    }

    const handleEnvOverrideToggle = (checked: boolean) => {
        updateConfigMutation.mutate({
            ...config,
            env_override: checked,
        })
    }

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
                <Button onClick={() => setAddDialogOpen(true)} className="gap-2">
                    <Plus className="w-4 h-4" />
                    {t("enterprise.accessControl.addTenant")}
                </Button>
            </div>

            {/* Configuration Card */}
            <Card>
                <CardHeader>
                    <CardTitle>{t("enterprise.accessControl.configuration")}</CardTitle>
                    <CardDescription>{t("enterprise.accessControl.configDescription")}</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    {/* Wildcard Mode */}
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
                            onCheckedChange={handleWildcardToggle}
                            disabled={updateConfigMutation.isPending}
                        />
                    </div>

                    {/* Environment Override */}
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
                            onCheckedChange={handleEnvOverrideToggle}
                            disabled={updateConfigMutation.isPending}
                        />
                    </div>

                    {/* Info Banner */}
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

            {/* Tenant List Card */}
            <Card>
                <CardHeader>
                    <CardTitle>{t("enterprise.accessControl.allowedTenants")}</CardTitle>
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
                                    <tr className="border-b text-muted-foreground">
                                        <th className="text-left py-3 px-4 font-medium">
                                            {t("enterprise.accessControl.tenantId")}
                                        </th>
                                        <th className="text-left py-3 px-4 font-medium">
                                            {t("enterprise.accessControl.tenantName")}
                                        </th>
                                        <th className="text-left py-3 px-4 font-medium">
                                            {t("enterprise.accessControl.addedBy")}
                                        </th>
                                        <th className="text-left py-3 px-4 font-medium">
                                            {t("enterprise.accessControl.createdAt")}
                                        </th>
                                        <th className="text-right py-3 px-4 font-medium">
                                            操作
                                        </th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {tenants.map((tenant) => (
                                        <tr key={tenant.id} className="border-b last:border-0 hover:bg-muted/50">
                                            <td className="py-3 px-4">
                                                <code className="text-sm bg-muted px-2 py-1 rounded">
                                                    {tenant.tenant_id}
                                                </code>
                                            </td>
                                            <td className="py-3 px-4">
                                                {tenant.name || (
                                                    <span className="text-muted-foreground italic">
                                                        {t("enterprise.accessControl.noName")}
                                                    </span>
                                                )}
                                            </td>
                                            <td className="py-3 px-4">
                                                <Badge variant="secondary">{tenant.added_by}</Badge>
                                            </td>
                                            <td className="py-3 px-4 text-sm text-muted-foreground">
                                                {new Date(tenant.created_at).toLocaleString()}
                                            </td>
                                            <td className="py-3 px-4 text-right">
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={() => handleDeleteTenant(tenant.id)}
                                                    className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-950"
                                                >
                                                    <Trash2 className="w-4 h-4" />
                                                </Button>
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
