// src/feature/group/components/GroupModelConfigsTab.tsx
import { useState, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { groupApi } from '@/api/group'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
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
import { Plus, Pencil, Trash2, RefreshCcw, Loader2, Search, Download, Upload, Copy, MoreHorizontal } from 'lucide-react'
import { AnimatedIcon } from '@/components/ui/animation/components/animated-icon'
import { useGroupModelConfigs } from '../hooks'
import { useModels } from '@/feature/model/hooks'
import type { GroupModelConfig, GroupModelConfigSaveRequest } from '@/types/group'
import type { ModelPrice } from '@/types/model'
import { PriceFormFields } from '@/components/price/PriceFormFields'
import { PriceDisplay } from '@/components/price/PriceDisplay'
import { Combobox } from '@/components/ui/combobox'
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
    override_summary_service_tier: false,
    summary_service_tier: false,
    override_summary_claude_long_context: false,
    summary_claude_long_context: false,
})

export function GroupModelConfigsTab({ groupId }: GroupModelConfigsTabProps) {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const fileInputRef = useRef<HTMLInputElement>(null)
    const { data, isLoading, refetch } = useGroupModelConfigs(groupId)
    const { data: systemModels } = useModels()
    const [searchKeyword, setSearchKeyword] = useState('')
    const [ownerFilter, setOwnerFilter] = useState('')
    const [isImporting, setIsImporting] = useState(false)

    const modelOwnerMap = useMemo(() => {
        const map = new Map<string, string>()
        for (const model of systemModels || []) {
            map.set(model.model, model.owner || '')
        }
        return map
    }, [systemModels])

    const filteredData = useMemo(() => {
        if (!data) return []
        let filtered = data
        if (searchKeyword) {
            const keyword = searchKeyword.toLowerCase()
            filtered = filtered.filter((config) => {
                const owner = modelOwnerMap.get(config.model) || ''
                return config.model.toLowerCase().includes(keyword) || owner.toLowerCase().includes(keyword)
            })
        }
        if (ownerFilter === '__empty__') {
            filtered = filtered.filter((config) => !(modelOwnerMap.get(config.model) || ''))
        } else if (ownerFilter && ownerFilter !== '__all__') {
            filtered = filtered.filter((config) => (modelOwnerMap.get(config.model) || '') === ownerFilter)
        }
        return filtered
    }, [data, searchKeyword, ownerFilter, modelOwnerMap])

    const ownerOptions = useMemo(() => {
        if (!data) return []

        const ownerSet = new Set<string>()
        let hasEmptyOwner = false

        for (const config of data) {
            const owner = modelOwnerMap.get(config.model) || ''
            if (owner) {
                ownerSet.add(owner)
            } else {
                hasEmptyOwner = true
            }
        }

        const options = [...ownerSet]
            .sort((a, b) => a.localeCompare(b))
        if (hasEmptyOwner) {
            options.push('__empty__')
        }
        return options
    }, [data, modelOwnerMap])

    const ownerLabel = (owner: string) => {
        if (owner === '__empty__') return t('model.emptyOwner')
        return owner
    }

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
    const [isCopying, setIsCopying] = useState(false)

    // Form state
    const [formModel, setFormModel] = useState('')
    const [formOverrideLimit, setFormOverrideLimit] = useState(false)
    const [formRpm, setFormRpm] = useState(0)
    const [formTpm, setFormTpm] = useState(0)
    const [formOverrideRetryTimes, setFormOverrideRetryTimes] = useState(false)
    const [formRetryTimes, setFormRetryTimes] = useState(0)
    const [formOverrideForceSaveDetail, setFormOverrideForceSaveDetail] = useState(false)
    const [formForceSaveDetail, setFormForceSaveDetail] = useState(false)
    const [formOverrideSummaryServiceTier, setFormOverrideSummaryServiceTier] = useState(false)
    const [formSummaryServiceTier, setFormSummaryServiceTier] = useState(false)
    const [formOverrideSummaryClaudeLongContext, setFormOverrideSummaryClaudeLongContext] = useState(false)
    const [formSummaryClaudeLongContext, setFormSummaryClaudeLongContext] = useState(false)
    const [formOverridePrice, setFormOverridePrice] = useState(false)
    const [formPrice, setFormPrice] = useState<ModelPrice>({})
    const [formImagePrices, setFormImagePrices] = useState<Record<string, number>>({})

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
            setFormOverrideSummaryServiceTier(config.override_summary_service_tier)
            setFormSummaryServiceTier(config.summary_service_tier)
            setFormOverrideSummaryClaudeLongContext(config.override_summary_claude_long_context)
            setFormSummaryClaudeLongContext(config.summary_claude_long_context)
            setFormOverridePrice(config.override_price)
            setFormPrice(config.price || {})
            setFormImagePrices(config.image_prices || {})
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
            setFormOverrideSummaryServiceTier(defaults.override_summary_service_tier!)
            setFormSummaryServiceTier(defaults.summary_service_tier!)
            setFormOverrideSummaryClaudeLongContext(defaults.override_summary_claude_long_context!)
            setFormSummaryClaudeLongContext(defaults.summary_claude_long_context!)
            setFormOverridePrice(false)
            setFormPrice({})
            setFormImagePrices({})
        }
    }

    const openCreateDialog = () => {
        setIsCreating(true)
        setIsCopying(false)
        setEditingConfig(null)
        resetForm()
        setEditDialogOpen(true)
    }

    const openEditDialog = (config: GroupModelConfig) => {
        setIsCreating(false)
        setIsCopying(false)
        setEditingConfig(config)
        resetForm(config)
        setEditDialogOpen(true)
    }

    const openCopyDialog = (config: GroupModelConfig) => {
        setIsCreating(true)
        setIsCopying(true)
        setEditingConfig(null)
        // Reset form with copied data but clear model name
        setFormModel('')
        setFormOverrideLimit(config.override_limit)
        setFormRpm(config.rpm)
        setFormTpm(config.tpm)
        setFormOverrideRetryTimes(config.override_retry_times)
        setFormRetryTimes(config.retry_times)
        setFormOverrideForceSaveDetail(config.override_force_save_detail)
        setFormForceSaveDetail(config.force_save_detail)
        setFormOverrideSummaryServiceTier(config.override_summary_service_tier)
        setFormSummaryServiceTier(config.summary_service_tier)
        setFormOverrideSummaryClaudeLongContext(config.override_summary_claude_long_context)
        setFormSummaryClaudeLongContext(config.summary_claude_long_context)
        setFormOverridePrice(config.override_price)
        setFormPrice(config.price || {})
        setFormImagePrices(config.image_prices || {})
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
            override_summary_service_tier: formOverrideSummaryServiceTier,
            summary_service_tier: formSummaryServiceTier,
            override_summary_claude_long_context: formOverrideSummaryClaudeLongContext,
            summary_claude_long_context: formSummaryClaudeLongContext,
            override_price: formOverridePrice,
            ...(formOverridePrice && { price: formPrice }),
            ...(formOverridePrice && Object.keys(formImagePrices).length > 0 && { image_prices: formImagePrices }),
        }
        saveMutation.mutate({ model, config })
    }

    const handleRefresh = () => {
        setIsRefreshAnimating(true)
        refetch()
        setTimeout(() => setIsRefreshAnimating(false), 1000)
    }

    // Export group model configs to JSON file
    const exportConfigs = () => {
        if (!data || data.length === 0) {
            toast.error(t('group.modelConfig.noDataToExport'))
            return
        }

        const exportData = data.map(({ group_id, ...config }) => config)

        const blob = new Blob([JSON.stringify(exportData, null, 2)], {
            type: 'application/json',
        })
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `group_${groupId}_model_configs_${new Date().toISOString().slice(0, 10)}.json`
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        URL.revokeObjectURL(url)
        toast.success(t('group.modelConfig.exportSuccess'))
    }

    // Import group model configs from JSON file
    const importConfigs = async (event: React.ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0]
        if (!file) return

        setIsImporting(true)
        try {
            const text = await file.text()
            const configs: GroupModelConfigSaveRequest[] = JSON.parse(text)

            if (!Array.isArray(configs)) {
                throw new Error(t('group.modelConfig.invalidFormat'))
            }

            await groupApi.saveGroupModelConfigs(groupId, configs)
            toast.success(t('group.modelConfig.importSuccess', { count: configs.length }))
            queryClient.invalidateQueries({ queryKey: ['groupModelConfigs', groupId] })
        } catch (error) {
            toast.error(
                error instanceof Error
                    ? error.message
                    : t('group.modelConfig.importFailed')
            )
        } finally {
            setIsImporting(false)
            // Reset file input
            if (fileInputRef.current) {
                fileInputRef.current.value = ''
            }
        }
    }

    // Trigger file input click
    const triggerImport = () => {
        fileInputRef.current?.click()
    }

    const formatImagePriceSummary = (imagePrices?: Record<string, number>) => {
        if (!imagePrices || Object.keys(imagePrices).length === 0) {
            return null
        }

        return Object.entries(imagePrices)
            .slice(0, 2)
            .map(([size, value]) => `${size}: ${value}`)
            .join(' | ')
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
                    <div className="flex gap-2">
                        <div className="relative w-64">
                            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                            <Input
                                placeholder={t('common.search')}
                                value={searchKeyword}
                                onChange={(e) => setSearchKeyword(e.target.value)}
                                className="h-9 pl-8"
                            />
                        </div>
                        <div className="w-44">
                            <Select value={ownerFilter} onValueChange={setOwnerFilter}>
                                <SelectTrigger className="h-9">
                                    <SelectValue placeholder={t('model.ownerFilterPlaceholder')} />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="__all__">{t('model.allOwners')}</SelectItem>
                                    {ownerOptions.map((owner) => (
                                        <SelectItem key={owner} value={owner}>{ownerLabel(owner)}</SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
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
                            variant="outline"
                            size="sm"
                            onClick={exportConfigs}
                            disabled={!data || data.length === 0}
                            className="flex items-center gap-1.5 h-8"
                        >
                            <Download className="h-3.5 w-3.5" />
                            {t('group.modelConfig.export')}
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={triggerImport}
                            disabled={isImporting}
                            className="flex items-center gap-1.5 h-8"
                        >
                            <Upload className="h-3.5 w-3.5" />
                            {isImporting ? t('group.modelConfig.importing') : t('group.modelConfig.import')}
                        </Button>
                        <input
                            ref={fileInputRef}
                            type="file"
                            accept=".json"
                            onChange={importConfigs}
                            className="hidden"
                        />
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
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('model.owner')}</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('group.modelConfig.overrideLimit')}</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">RPM</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">TPM</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('group.modelConfig.overrideRetryTimes')}</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('model.retryTimes')}</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('model.forceSaveDetail')}</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('model.recordServiceTier')}</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('model.recordClaudeLongContext')}</th>
                                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground uppercase">{t('group.modelConfig.overridePrice')}</th>
                                    <th className="px-4 py-3 text-right text-xs font-medium text-muted-foreground uppercase">{t('group.modelConfig.actions')}</th>
                                </tr>
                            </thead>
                            <tbody>
                                {filteredData.map((config) => (
                                    <tr
                                        key={config.model}
                                        className="border-t hover:bg-muted/50 transition-colors cursor-pointer"
                                        onClick={() => openEditDialog(config)}
                                    >
                                        <td className="px-4 py-3 text-sm font-medium">{config.model}</td>
                                        <td className="px-4 py-3 text-sm">
                                            {modelOwnerMap.get(config.model) || (
                                                <span className="text-muted-foreground">{t('model.emptyOwner')}</span>
                                            )}
                                        </td>
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
                                        <td className="px-4 py-3 text-sm">
                                            {config.override_summary_service_tier ? (
                                                <Badge variant={config.summary_service_tier ? 'default' : 'secondary'} className="text-xs">
                                                    {config.summary_service_tier ? t('common.yes') : t('common.no')}
                                                </Badge>
                                            ) : '-'}
                                        </td>
                                        <td className="px-4 py-3 text-sm">
                                            {config.override_summary_claude_long_context ? (
                                                <Badge variant={config.summary_claude_long_context ? 'default' : 'secondary'} className="text-xs">
                                                    {config.summary_claude_long_context ? t('common.yes') : t('common.no')}
                                                </Badge>
                                            ) : '-'}
                                        </td>
                                        <td className="px-4 py-3 text-sm">
                                            {config.override_price ? (
                                                <div className="space-y-1">
                                                    <PriceDisplay price={config.price} />
                                                    {formatImagePriceSummary(config.image_prices) && (
                                                        <div className="text-xs text-muted-foreground font-mono">
                                                            {formatImagePriceSummary(config.image_prices)}
                                                        </div>
                                                    )}
                                                </div>
                                            ) : '-'}
                                        </td>
                                        <td className="px-4 py-3 text-sm text-right" onClick={(e) => e.stopPropagation()}>
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button variant="ghost" size="icon" className="h-8 w-8">
                                                        <MoreHorizontal className="h-4 w-4" />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    <DropdownMenuItem onClick={() => openEditDialog(config)}>
                                                        <Pencil className="mr-2 h-4 w-4" />
                                                        {t('common.edit')}
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem onClick={() => openCopyDialog(config)}>
                                                        <Copy className="mr-2 h-4 w-4" />
                                                        {t('group.modelConfig.copyFrom')}
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem
                                                        onClick={() => openDeleteDialog(config.model)}
                                                        className="text-destructive focus:text-destructive"
                                                    >
                                                        <Trash2 className="mr-2 h-4 w-4" />
                                                        {t('common.delete')}
                                                    </DropdownMenuItem>
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </td>
                                    </tr>
                                ))}
                                {filteredData.length === 0 && (
                                    <tr>
                                        <td colSpan={12} className="px-4 py-12 text-center text-muted-foreground">
                                            {t('common.noResult')}
                                        </td>
                                    </tr>
                                )}
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>

            {/* Edit / Create / Copy Dialog */}
            <Dialog open={editDialogOpen} onOpenChange={setEditDialogOpen}>
                <DialogContent className="sm:max-w-[600px] max-h-[85vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>
                            {isCopying ? t('group.modelConfig.copyTitle') : isCreating ? t('group.modelConfig.addTitle') : t('group.modelConfig.editTitle')}
                        </DialogTitle>
                        <DialogDescription>
                            {isCopying ? t('group.modelConfig.copyDescription') : isCreating ? t('group.modelConfig.addDescription') : t('group.modelConfig.editDescription')}
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

                        {/* Override Record Service Tier */}
                        <div className="flex items-center justify-between rounded-lg border p-3">
                            <div className="space-y-0.5">
                                <Label>{t('group.modelConfig.overrideRecordServiceTier')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideRecordServiceTierDesc')}</p>
                            </div>
                            <Switch checked={formOverrideSummaryServiceTier} onCheckedChange={setFormOverrideSummaryServiceTier} />
                        </div>

                        {formOverrideSummaryServiceTier && (
                            <div className="flex items-center gap-2 pl-4">
                                <Label>{t('model.recordServiceTier')}</Label>
                                <Switch checked={formSummaryServiceTier} onCheckedChange={setFormSummaryServiceTier} />
                            </div>
                        )}

                        <div className="flex items-center justify-between rounded-lg border p-3">
                            <div className="space-y-0.5">
                                <Label>{t('group.modelConfig.overrideRecordClaudeLongContext')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideRecordClaudeLongContextDesc')}</p>
                            </div>
                            <Switch checked={formOverrideSummaryClaudeLongContext} onCheckedChange={setFormOverrideSummaryClaudeLongContext} />
                        </div>

                        {formOverrideSummaryClaudeLongContext && (
                            <div className="flex items-center gap-2 pl-4">
                                <Label>{t('model.recordClaudeLongContext')}</Label>
                                <Switch checked={formSummaryClaudeLongContext} onCheckedChange={setFormSummaryClaudeLongContext} />
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
