// src/feature/group/components/GroupModelConfigsTab.tsx
import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { groupApi } from '@/api/group'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Combobox } from '@/components/ui/combobox'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Plus, Pencil, Trash2, RefreshCcw, Loader2, Search } from 'lucide-react'
import { AnimatedIcon } from '@/components/ui/animation/components/animated-icon'
import { useGroupModelConfigs } from '../hooks'
import { useModels } from '@/feature/model/hooks'
import type { GroupModelConfig, GroupModelConfigSaveRequest } from '@/types/group'
import type { ModelPrice } from '@/types/model'
import { PriceFormFields } from '@/components/price/PriceFormFields'
import { toast } from 'sonner'

interface GroupModelConfigsTabProps {
    groupId: string
}

// Default empty config for creating
const getDefaultConfig = (): Omit<GroupModelConfigSaveRequest, 'model'> => ({
    override_limit: false,
    rpm: 0,
    tpm: 0,
    override_retry_times: false,
    retry_times: 0,
    override_force_save_detail: false,
    force_save_detail: false,
})

export function GroupModelConfigsTab({ groupId }: GroupModelConfigsTabProps) {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const { data, isLoading, refetch } = useGroupModelConfigs(groupId)
    const { data: systemModels } = useModels()
    const [searchKeyword, setSearchKeyword] = useState('')

    const filteredData = useMemo(() => {
        if (!data) return []
        if (!searchKeyword) return data
        const keyword = searchKeyword.toLowerCase()
        return data.filter(c => c.model.toLowerCase().includes(keyword))
    }, [data, searchKeyword])

    const modelOptions = useMemo(() => {
        if (!systemModels) return []
        const existingModels = new Set(data?.map(c => c.model) || [])
        return systemModels
            .map(m => ({ value: m.model, label: m.model }))
            .filter(o => !existingModels.has(o.value))
    }, [systemModels, data])

    const [editDialogOpen, setEditDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [isRefreshAnimating, setIsRefreshAnimating] = useState(false)
    const [editingConfig, setEditingConfig] = useState<GroupModelConfig | null>(null)
    const [deletingModel, setDeletingModel] = useState<string | null>(null)
    const [isCreating, setIsCreating] = useState(false)

    // Form state
    const [formModel, setFormModel] = useState('')
    const [formOverrideLimit, setFormOverrideLimit] = useState(false)
    const [formRpm, setFormRpm] = useState(0)
    const [formTpm, setFormTpm] = useState(0)
    const [formOverrideRetryTimes, setFormOverrideRetryTimes] = useState(false)
    const [formRetryTimes, setFormRetryTimes] = useState(0)
    const [formOverrideForceSaveDetail, setFormOverrideForceSaveDetail] = useState(false)
    const [formForceSaveDetail, setFormForceSaveDetail] = useState(false)
    const [formOverridePrice, setFormOverridePrice] = useState(false)
    const [formPrice, setFormPrice] = useState<ModelPrice>({})

    // Save mutation
    const saveMutation = useMutation({
        mutationFn: ({ model, config }: { model: string; config: GroupModelConfigSaveRequest }) =>
            groupApi.saveGroupModelConfig(groupId, model, config),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['groupModelConfigs', groupId] })
            toast.success(t('common.success'))
            setEditDialogOpen(false)
        },
        onError: (err: Error) => {
            toast.error(err.message || 'Failed to save config')
        },
    })

    // Delete mutation
    const deleteMutation = useMutation({
        mutationFn: (model: string) => groupApi.deleteGroupModelConfig(groupId, model),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['groupModelConfigs', groupId] })
            toast.success(t('common.success'))
            setDeleteDialogOpen(false)
            setDeletingModel(null)
        },
        onError: (err: Error) => {
            toast.error(err.message || 'Failed to delete config')
        },
    })

    const resetForm = (config?: GroupModelConfig) => {
        if (config) {
            setFormModel(config.model)
            setFormOverrideLimit(config.override_limit)
            setFormRpm(config.rpm)
            setFormTpm(config.tpm)
            setFormOverrideRetryTimes(config.override_retry_times)
            setFormRetryTimes(config.retry_times)
            setFormOverrideForceSaveDetail(config.override_force_save_detail)
            setFormForceSaveDetail(config.force_save_detail)
            setFormOverridePrice(config.override_price)
            setFormPrice(config.price || {})
        } else {
            const defaults = getDefaultConfig()
            setFormModel('')
            setFormOverrideLimit(defaults.override_limit!)
            setFormRpm(defaults.rpm!)
            setFormTpm(defaults.tpm!)
            setFormOverrideRetryTimes(defaults.override_retry_times!)
            setFormRetryTimes(defaults.retry_times!)
            setFormOverrideForceSaveDetail(defaults.override_force_save_detail!)
            setFormForceSaveDetail(defaults.force_save_detail!)
            setFormOverridePrice(false)
            setFormPrice({})
        }
    }

    const openCreateDialog = () => {
        setIsCreating(true)
        setEditingConfig(null)
        resetForm()
        setEditDialogOpen(true)
    }

    const openEditDialog = (config: GroupModelConfig) => {
        setIsCreating(false)
        setEditingConfig(config)
        resetForm(config)
        setEditDialogOpen(true)
    }

    const openDeleteDialog = (model: string) => {
        setDeletingModel(model)
        setDeleteDialogOpen(true)
    }

    const handleSave = () => {
        const model = isCreating ? formModel.trim() : editingConfig?.model
        if (!model) return

        const config: GroupModelConfigSaveRequest = {
            model,
            override_limit: formOverrideLimit,
            rpm: formRpm,
            tpm: formTpm,
            override_retry_times: formOverrideRetryTimes,
            retry_times: formRetryTimes,
            override_force_save_detail: formOverrideForceSaveDetail,
            force_save_detail: formForceSaveDetail,
            override_price: formOverridePrice,
            ...(formOverridePrice && { price: formPrice }),
        }
        saveMutation.mutate({ model, config })
    }

    const handleRefresh = () => {
        setIsRefreshAnimating(true)
        refetch()
        setTimeout(() => setIsRefreshAnimating(false), 1000)
    }

    if (isLoading) {
        return (
            <div className="space-y-4">
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-32 w-full" />
            </div>
        )
    }

    return (
        <>
            <div className="space-y-4">
                {/* Header */}
                <div className="flex items-center justify-between">
                    <div className="relative w-64">
                        <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input
                            placeholder={t('common.search')}
                            value={searchKeyword}
                            onChange={(e) => setSearchKeyword(e.target.value)}
                            className="pl-9 h-9"
                        />
                    </div>
                    <div className="flex gap-2">
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={handleRefresh}
                            className="flex items-center gap-1.5 h-8"
                        >
                            <AnimatedIcon animationVariant="continuous-spin" isAnimating={isRefreshAnimating} className="h-3.5 w-3.5">
                                <RefreshCcw className="h-3.5 w-3.5" />
                            </AnimatedIcon>
                            {t('group.refresh')}
                        </Button>
                        <Button
                            size="sm"
                            onClick={openCreateDialog}
                            className="flex items-center gap-1 h-8"
                        >
                            <Plus className="h-3.5 w-3.5" />
                            {t('group.modelConfig.add')}
                        </Button>
                    </div>
                </div>

                {/* Table */}
                <div className="border rounded-lg overflow-hidden">
                    <div className="overflow-auto">
                        <table className="w-full">
                            <thead className="bg-muted/50">
                                <tr>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('model.modelName')}</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('group.modelConfig.overrideLimit')}</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">RPM</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">TPM</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('group.modelConfig.overrideRetryTimes')}</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('model.retryTimes')}</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('model.forceSaveDetail')}</th>
                                    <th className="px-4 py-3 text-right text-xs font-medium text-muted-foreground uppercase">{t('group.modelConfig.actions')}</th>
                                </tr>
                            </thead>
                            <tbody>
                                {filteredData.map((config) => (
                                    <tr key={config.model} className="border-t hover:bg-muted/50 transition-colors">
                                        <td className="px-4 py-3 text-sm font-medium">{config.model}</td>
                                        <td className="px-4 py-3 text-sm">
                                            <Badge variant={config.override_limit ? 'default' : 'secondary'} className="text-xs">
                                                {config.override_limit ? t('common.yes') : t('common.no')}
                                            </Badge>
                                        </td>
                                        <td className="px-4 py-3 text-sm font-mono">
                                            {config.override_limit ? config.rpm : '-'}
                                        </td>
                                        <td className="px-4 py-3 text-sm font-mono">
                                            {config.override_limit ? config.tpm : '-'}
                                        </td>
                                        <td className="px-4 py-3 text-sm">
                                            <Badge variant={config.override_retry_times ? 'default' : 'secondary'} className="text-xs">
                                                {config.override_retry_times ? t('common.yes') : t('common.no')}
                                            </Badge>
                                        </td>
                                        <td className="px-4 py-3 text-sm font-mono">
                                            {config.override_retry_times ? config.retry_times : '-'}
                                        </td>
                                        <td className="px-4 py-3 text-sm">
                                            {config.override_force_save_detail ? (
                                                <Badge variant={config.force_save_detail ? 'default' : 'secondary'} className="text-xs">
                                                    {config.force_save_detail ? t('common.yes') : t('common.no')}
                                                </Badge>
                                            ) : '-'}
                                        </td>
                                        <td className="px-4 py-3 text-sm text-right">
                                            <div className="flex items-center justify-end gap-1">
                                                <Button
                                                    variant="ghost"
                                                    size="icon"
                                                    className="h-8 w-8"
                                                    onClick={() => openEditDialog(config)}
                                                >
                                                    <Pencil className="h-3.5 w-3.5" />
                                                </Button>
                                                <Button
                                                    variant="ghost"
                                                    size="icon"
                                                    className="h-8 w-8 text-destructive hover:text-destructive"
                                                    onClick={() => openDeleteDialog(config.model)}
                                                >
                                                    <Trash2 className="h-3.5 w-3.5" />
                                                </Button>
                                            </div>
                                        </td>
                                    </tr>
                                ))}
                                {filteredData.length === 0 && (
                                    <tr>
                                        <td colSpan={8} className="px-4 py-12 text-center text-muted-foreground">
                                            {t('common.noResult')}
                                        </td>
                                    </tr>
                                )}
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>

            {/* Edit / Create Dialog */}
            <Dialog open={editDialogOpen} onOpenChange={setEditDialogOpen}>
                <DialogContent className="sm:max-w-[600px] max-h-[85vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>
                            {isCreating ? t('group.modelConfig.addTitle') : t('group.modelConfig.editTitle')}
                        </DialogTitle>
                        <DialogDescription>
                            {isCreating ? t('group.modelConfig.addDescription') : t('group.modelConfig.editDescription')}
                        </DialogDescription>
                    </DialogHeader>

                    <div className="space-y-4 py-2">
                        {/* Model name */}
                        <div className="space-y-2">
                            <Label>{t('model.modelName')}</Label>
                            {isCreating ? (
                                <Combobox
                                    options={modelOptions}
                                    value={formModel}
                                    onValueChange={setFormModel}
                                    placeholder={t('model.dialog.modelNamePlaceholder')}
                                    emptyText={t('common.noResult')}
                                />
                            ) : (
                                <Input
                                    value={formModel}
                                    disabled
                                />
                            )}
                        </div>

                        {/* Override Limit */}
                        <div className="flex items-center justify-between rounded-lg border p-3">
                            <div className="space-y-0.5">
                                <Label>{t('group.modelConfig.overrideLimit')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideLimitDesc')}</p>
                            </div>
                            <Switch checked={formOverrideLimit} onCheckedChange={setFormOverrideLimit} />
                        </div>

                        {formOverrideLimit && (
                            <div className="grid grid-cols-2 gap-4 pl-4">
                                <div className="space-y-2">
                                    <Label>RPM</Label>
                                    <Input
                                        type="number"
                                        min={0}
                                        value={formRpm}
                                        onChange={(e) => setFormRpm(Number(e.target.value))}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <Label>TPM</Label>
                                    <Input
                                        type="number"
                                        min={0}
                                        value={formTpm}
                                        onChange={(e) => setFormTpm(Number(e.target.value))}
                                    />
                                </div>
                            </div>
                        )}

                        {/* Override Retry Times */}
                        <div className="flex items-center justify-between rounded-lg border p-3">
                            <div className="space-y-0.5">
                                <Label>{t('group.modelConfig.overrideRetryTimes')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideRetryTimesDesc')}</p>
                            </div>
                            <Switch checked={formOverrideRetryTimes} onCheckedChange={setFormOverrideRetryTimes} />
                        </div>

                        {formOverrideRetryTimes && (
                            <div className="pl-4">
                                <div className="space-y-2">
                                    <Label>{t('model.retryTimes')}</Label>
                                    <Input
                                        type="number"
                                        min={0}
                                        value={formRetryTimes}
                                        onChange={(e) => setFormRetryTimes(Number(e.target.value))}
                                    />
                                </div>
                            </div>
                        )}

                        {/* Override Force Save Detail */}
                        <div className="flex items-center justify-between rounded-lg border p-3">
                            <div className="space-y-0.5">
                                <Label>{t('group.modelConfig.overrideForceSaveDetail')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideForceSaveDetailDesc')}</p>
                            </div>
                            <Switch checked={formOverrideForceSaveDetail} onCheckedChange={setFormOverrideForceSaveDetail} />
                        </div>

                        {formOverrideForceSaveDetail && (
                            <div className="flex items-center gap-2 pl-4">
                                <Label>{t('model.forceSaveDetail')}</Label>
                                <Switch checked={formForceSaveDetail} onCheckedChange={setFormForceSaveDetail} />
                            </div>
                        )}

                        {/* Override Price */}
                        <div className="flex items-center justify-between rounded-lg border p-3">
                            <div className="space-y-0.5">
                                <Label>{t('group.modelConfig.overridePrice')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overridePriceDesc')}</p>
                            </div>
                            <Switch checked={formOverridePrice} onCheckedChange={setFormOverridePrice} />
                        </div>

                        {formOverridePrice && (
                            <div className="pl-4">
                                <PriceFormFields price={formPrice} onChange={setFormPrice} />
                            </div>
                        )}
                    </div>

                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setEditDialogOpen(false)}
                            disabled={saveMutation.isPending}
                        >
                            {t('common.cancel')}
                        </Button>
                        <Button
                            onClick={handleSave}
                            disabled={saveMutation.isPending || (isCreating && !formModel.trim())}
                        >
                            {saveMutation.isPending ? (
                                <>
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    {t('common.saving')}
                                </>
                            ) : (
                                t('common.save')
                            )}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Delete Confirmation */}
            <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>{t('group.modelConfig.deleteTitle')}</AlertDialogTitle>
                        <AlertDialogDescription>
                            {t('group.modelConfig.deleteDescription', { model: deletingModel })}
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel disabled={deleteMutation.isPending}>
                            {t('common.cancel')}
                        </AlertDialogCancel>
                        <AlertDialogAction
                            onClick={() => deletingModel && deleteMutation.mutate(deletingModel)}
                            disabled={deleteMutation.isPending}
                            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                        >
                            {deleteMutation.isPending ? (
                                <>
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    {t('group.deleteDialog.deleting')}
                                </>
                            ) : (
                                t('group.deleteDialog.delete')
                            )}
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </>
    )
}
