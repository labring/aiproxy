// src/feature/group/components/GroupModelConfigsTab.tsx
import { useState, useMemo, useRef, useId } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { groupApi } from '@/api/group'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
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
import type { ModelPrice, TimeoutConfig } from '@/types/model'
import {
    IMAGE_GENERATION_COUNT_LIMIT_SUPPORTED_MODEL_TYPES,
    VIDEO_GENERATION_SECONDS_LIMIT_SUPPORTED_MODEL_TYPES,
    VIDEO_GENERATION_COUNT_LIMIT_SUPPORTED_MODEL_TYPES,
} from '@/types/model'
import { PriceFormFields } from '@/components/price/PriceFormFields'
import { PriceDisplay } from '@/components/price/PriceDisplay'
import { Combobox } from '@/components/ui/combobox'
import { toast } from 'sonner'
import { priceSchema } from '@/validation/model'

interface GroupModelConfigsTabProps {
    groupId: string
}

const IMAGE_GENERATION_COUNT_LIMIT_SUPPORTED_TYPES = new Set<number>(IMAGE_GENERATION_COUNT_LIMIT_SUPPORTED_MODEL_TYPES)
const VIDEO_GENERATION_SECONDS_LIMIT_SUPPORTED_TYPES = new Set<number>(VIDEO_GENERATION_SECONDS_LIMIT_SUPPORTED_MODEL_TYPES)
const VIDEO_GENERATION_COUNT_LIMIT_SUPPORTED_TYPES = new Set<number>(VIDEO_GENERATION_COUNT_LIMIT_SUPPORTED_MODEL_TYPES)

const omitKeys = (obj: object, keys: string[]) => {
    const omitted = new Set(keys)
    return Object.fromEntries(Object.entries(obj).filter(([key]) => !omitted.has(key)))
}

// Default empty config for creating
const getDefaultConfig = (): Omit<GroupModelConfigSaveRequest, 'model'> => ({
    override_limit: false,
    rpm: 0,
    tpm: 0,
    override_retry_times: false,
    retry_times: 0,
    override_timeout_config: false,
    timeout_config: {},
    override_force_save_detail: false,
    force_save_detail: false,
    override_max_image_generation_count: false,
    max_image_generation_count: 0,
    override_max_video_generation_seconds: false,
    max_video_generation_seconds: 0,
    override_max_video_generation_count: false,
    max_video_generation_count: 0,
    override_request_body_storage_max_size: false,
    request_body_storage_max_size: 0,
    override_response_body_storage_max_size: false,
    response_body_storage_max_size: 0,
    override_summary_service_tier: false,
    summary_service_tier: false,
    override_summary_claude_long_context: false,
    summary_claude_long_context: false,
})


function ConfigValues({ entries }: { entries: [string, string | number | boolean | undefined][] }) {
    const { t } = useTranslation()
    const active = entries.filter(([, value]) => value !== undefined)
    const formatValue = (value: string | number | boolean | undefined) =>
        typeof value === 'boolean' ? t(value ? 'common.yes' : 'common.no') : String(value)

    if (!active.length) return <span className="text-xs text-muted-foreground">{t('ui.inheritModel')}</span>

    return (
        <Popover>
            <PopoverTrigger asChild>
                <button type="button" onClick={event => event.stopPropagation()} className="grid gap-1 text-left text-xs leading-5 hover:text-primary focus-visible:outline-ring">
                    <span className="whitespace-nowrap">{active[0][0]}: <span className="font-medium tabular-nums">{formatValue(active[0][1])}</span></span>
                    {active.length > 1 && <span className="text-muted-foreground">+{active.length - 1} {t('ui.overrides')}</span>}
                </button>
            </PopoverTrigger>
            <PopoverContent className="w-80 max-w-[calc(100vw-2rem)] p-4" onClick={event => event.stopPropagation()}>
                <dl className="grid gap-3 text-xs">
                    {active.map(([label, value]) => <div key={label} className="flex justify-between gap-4"><dt className="text-muted-foreground">{label}</dt><dd className="shrink-0 font-mono">{formatValue(value)}</dd></div>)}
                </dl>
            </PopoverContent>
        </Popover>
    )
}

export function GroupModelConfigsTab({ groupId }: GroupModelConfigsTabProps) {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const formId = useId()
    const fileInputRef = useRef<HTMLInputElement>(null)
    const { data, isLoading, refetch } = useGroupModelConfigs(groupId)
    const { data: systemModels } = useModels()
    const [searchKeyword, setSearchKeyword] = useState('')
    const [isImporting, setIsImporting] = useState(false)

    const filteredData = useMemo(() => {
        if (!data) return []
        let filtered = data
        if (searchKeyword) {
            const keyword = searchKeyword.toLowerCase()
            filtered = filtered.filter((config) => config.model.toLowerCase().includes(keyword))
        }
        return filtered
    }, [data, searchKeyword])

    const modelOptions = useMemo(() => {
        if (!systemModels) return []
        const existingModels = new Set(data?.map(c => c.model) || [])
        return systemModels
            .map(m => ({ value: m.model, label: m.model }))
            .filter(o => !existingModels.has(o.value))
    }, [systemModels, data])

    const systemModelTypeByName = useMemo(() => {
        return new Map(systemModels?.map((model) => [model.model, model.type]) || [])
    }, [systemModels])

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
    const [formOverrideTimeoutConfig, setFormOverrideTimeoutConfig] = useState(false)
    const [formTimeoutConfig, setFormTimeoutConfig] = useState<TimeoutConfig>({})
    const [formOverrideForceSaveDetail, setFormOverrideForceSaveDetail] = useState(false)
    const [formForceSaveDetail, setFormForceSaveDetail] = useState(false)
    const [formOverrideMaxImageGenerationCount, setFormOverrideMaxImageGenerationCount] = useState(false)
    const [formMaxImageGenerationCount, setFormMaxImageGenerationCount] = useState(0)
    const [formOverrideMaxVideoGenerationSeconds, setFormOverrideMaxVideoGenerationSeconds] = useState(false)
    const [formMaxVideoGenerationSeconds, setFormMaxVideoGenerationSeconds] = useState(0)
    const [formOverrideMaxVideoGenerationCount, setFormOverrideMaxVideoGenerationCount] = useState(false)
    const [formMaxVideoGenerationCount, setFormMaxVideoGenerationCount] = useState(0)
    const [formOverrideRequestBodyStorageMaxSize, setFormOverrideRequestBodyStorageMaxSize] = useState(false)
    const [formRequestBodyStorageMaxSize, setFormRequestBodyStorageMaxSize] = useState(0)
    const [formOverrideResponseBodyStorageMaxSize, setFormOverrideResponseBodyStorageMaxSize] = useState(false)
    const [formResponseBodyStorageMaxSize, setFormResponseBodyStorageMaxSize] = useState(0)
    const [formOverrideSummaryServiceTier, setFormOverrideSummaryServiceTier] = useState(false)
    const [formSummaryServiceTier, setFormSummaryServiceTier] = useState(false)
    const [formOverrideSummaryClaudeLongContext, setFormOverrideSummaryClaudeLongContext] = useState(false)
    const [formSummaryClaudeLongContext, setFormSummaryClaudeLongContext] = useState(false)
    const [formOverridePrice, setFormOverridePrice] = useState(false)
    const [formPrice, setFormPrice] = useState<ModelPrice>({})

    const selectedModelType = systemModelTypeByName.get(isCreating ? formModel : editingConfig?.model || formModel)
    const imageGenerationCountLimitTypeKnown = selectedModelType !== undefined
    const supportImageGenerationCountLimit = selectedModelType !== undefined &&
        IMAGE_GENERATION_COUNT_LIMIT_SUPPORTED_TYPES.has(selectedModelType)
    const videoGenerationSecondsLimitTypeKnown = selectedModelType !== undefined
    const supportVideoGenerationSecondsLimit = selectedModelType !== undefined &&
        VIDEO_GENERATION_SECONDS_LIMIT_SUPPORTED_TYPES.has(selectedModelType)
    const videoGenerationCountLimitTypeKnown = selectedModelType !== undefined
    const supportVideoGenerationCountLimit = selectedModelType !== undefined &&
        VIDEO_GENERATION_COUNT_LIMIT_SUPPORTED_TYPES.has(selectedModelType)

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
            setFormOverrideTimeoutConfig(config.override_timeout_config)
            setFormTimeoutConfig(config.timeout_config || {})
            setFormOverrideForceSaveDetail(config.override_force_save_detail)
            setFormForceSaveDetail(config.force_save_detail)
            setFormOverrideMaxImageGenerationCount(config.override_max_image_generation_count)
            setFormMaxImageGenerationCount(config.max_image_generation_count)
            setFormOverrideMaxVideoGenerationSeconds(config.override_max_video_generation_seconds)
            setFormMaxVideoGenerationSeconds(config.max_video_generation_seconds)
            setFormOverrideMaxVideoGenerationCount(config.override_max_video_generation_count)
            setFormMaxVideoGenerationCount(config.max_video_generation_count)
            setFormOverrideRequestBodyStorageMaxSize(config.override_request_body_storage_max_size)
            setFormRequestBodyStorageMaxSize(config.request_body_storage_max_size)
            setFormOverrideResponseBodyStorageMaxSize(config.override_response_body_storage_max_size)
            setFormResponseBodyStorageMaxSize(config.response_body_storage_max_size)
            setFormOverrideSummaryServiceTier(config.override_summary_service_tier)
            setFormSummaryServiceTier(config.summary_service_tier)
            setFormOverrideSummaryClaudeLongContext(config.override_summary_claude_long_context)
            setFormSummaryClaudeLongContext(config.summary_claude_long_context)
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
            setFormOverrideTimeoutConfig(defaults.override_timeout_config!)
            setFormTimeoutConfig(defaults.timeout_config || {})
            setFormOverrideForceSaveDetail(defaults.override_force_save_detail!)
            setFormForceSaveDetail(defaults.force_save_detail!)
            setFormOverrideMaxImageGenerationCount(defaults.override_max_image_generation_count!)
            setFormMaxImageGenerationCount(defaults.max_image_generation_count!)
            setFormOverrideMaxVideoGenerationSeconds(defaults.override_max_video_generation_seconds!)
            setFormMaxVideoGenerationSeconds(defaults.max_video_generation_seconds!)
            setFormOverrideMaxVideoGenerationCount(defaults.override_max_video_generation_count!)
            setFormMaxVideoGenerationCount(defaults.max_video_generation_count!)
            setFormOverrideRequestBodyStorageMaxSize(defaults.override_request_body_storage_max_size!)
            setFormRequestBodyStorageMaxSize(defaults.request_body_storage_max_size!)
            setFormOverrideResponseBodyStorageMaxSize(defaults.override_response_body_storage_max_size!)
            setFormResponseBodyStorageMaxSize(defaults.response_body_storage_max_size!)
            setFormOverrideSummaryServiceTier(defaults.override_summary_service_tier!)
            setFormSummaryServiceTier(defaults.summary_service_tier!)
            setFormOverrideSummaryClaudeLongContext(defaults.override_summary_claude_long_context!)
            setFormSummaryClaudeLongContext(defaults.summary_claude_long_context!)
            setFormOverridePrice(false)
            setFormPrice({})
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
        setFormOverrideTimeoutConfig(config.override_timeout_config)
        setFormTimeoutConfig(config.timeout_config || {})
        setFormOverrideForceSaveDetail(config.override_force_save_detail)
        setFormForceSaveDetail(config.force_save_detail)
        setFormOverrideMaxImageGenerationCount(config.override_max_image_generation_count)
        setFormMaxImageGenerationCount(config.max_image_generation_count)
        setFormOverrideMaxVideoGenerationSeconds(config.override_max_video_generation_seconds)
        setFormMaxVideoGenerationSeconds(config.max_video_generation_seconds)
        setFormOverrideMaxVideoGenerationCount(config.override_max_video_generation_count)
        setFormMaxVideoGenerationCount(config.max_video_generation_count)
        setFormOverrideRequestBodyStorageMaxSize(config.override_request_body_storage_max_size)
        setFormRequestBodyStorageMaxSize(config.request_body_storage_max_size)
        setFormOverrideResponseBodyStorageMaxSize(config.override_response_body_storage_max_size)
        setFormResponseBodyStorageMaxSize(config.response_body_storage_max_size)
        setFormOverrideSummaryServiceTier(config.override_summary_service_tier)
        setFormSummaryServiceTier(config.summary_service_tier)
        setFormOverrideSummaryClaudeLongContext(config.override_summary_claude_long_context)
        setFormSummaryClaudeLongContext(config.summary_claude_long_context)
        setFormOverridePrice(config.override_price)
        setFormPrice(config.price || {})
        setEditDialogOpen(true)
    }

    const openDeleteDialog = (model: string) => {
        setDeletingModel(model)
        setDeleteDialogOpen(true)
    }

    const handleSave = () => {
        const model = isCreating ? formModel.trim() : editingConfig?.model
        if (!model) return

        if (formOverridePrice) {
            const result = priceSchema.safeParse(formPrice)
            if (!result.success) {
                toast.error(result.error.issues[0]?.message || t('error.validationDescription'))
                return
            }
        }

        const maxImageGenerationCountConfig = (() => {
            if (supportImageGenerationCountLimit) {
                return {
                    override_max_image_generation_count: formOverrideMaxImageGenerationCount,
                    max_image_generation_count: formMaxImageGenerationCount,
                }
            }

            if (!imageGenerationCountLimitTypeKnown && editingConfig) {
                return {
                    override_max_image_generation_count: editingConfig.override_max_image_generation_count,
                    max_image_generation_count: editingConfig.max_image_generation_count,
                }
            }

            return {
                override_max_image_generation_count: false,
                max_image_generation_count: 0,
            }
        })()

        const maxVideoGenerationSecondsConfig = (() => {
            if (supportVideoGenerationSecondsLimit) {
                return {
                    override_max_video_generation_seconds: formOverrideMaxVideoGenerationSeconds,
                    max_video_generation_seconds: formMaxVideoGenerationSeconds,
                }
            }

            if (!videoGenerationSecondsLimitTypeKnown && editingConfig) {
                return {
                    override_max_video_generation_seconds: editingConfig.override_max_video_generation_seconds,
                    max_video_generation_seconds: editingConfig.max_video_generation_seconds,
                }
            }

            return {
                override_max_video_generation_seconds: false,
                max_video_generation_seconds: 0,
            }
        })()

        const maxVideoGenerationCountConfig = (() => {
            if (supportVideoGenerationCountLimit) {
                return {
                    override_max_video_generation_count: formOverrideMaxVideoGenerationCount,
                    max_video_generation_count: formMaxVideoGenerationCount,
                }
            }

            if (!videoGenerationCountLimitTypeKnown && editingConfig) {
                return {
                    override_max_video_generation_count: editingConfig.override_max_video_generation_count,
                    max_video_generation_count: editingConfig.max_video_generation_count,
                }
            }

            return {
                override_max_video_generation_count: false,
                max_video_generation_count: 0,
            }
        })()

        const config: GroupModelConfigSaveRequest = {
            model,
            override_limit: formOverrideLimit,
            rpm: formRpm,
            tpm: formTpm,
            override_retry_times: formOverrideRetryTimes,
            retry_times: formRetryTimes,
            override_timeout_config: formOverrideTimeoutConfig,
            ...(formOverrideTimeoutConfig && { timeout_config: formTimeoutConfig }),
            override_force_save_detail: formOverrideForceSaveDetail,
            force_save_detail: formForceSaveDetail,
            ...maxImageGenerationCountConfig,
            ...maxVideoGenerationSecondsConfig,
            ...maxVideoGenerationCountConfig,
            override_request_body_storage_max_size: formOverrideRequestBodyStorageMaxSize,
            request_body_storage_max_size: formRequestBodyStorageMaxSize,
            override_response_body_storage_max_size: formOverrideResponseBodyStorageMaxSize,
            response_body_storage_max_size: formResponseBodyStorageMaxSize,
            override_summary_service_tier: formOverrideSummaryServiceTier,
            summary_service_tier: formSummaryServiceTier,
            override_summary_claude_long_context: formOverrideSummaryClaudeLongContext,
            summary_claude_long_context: formSummaryClaudeLongContext,
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

    // Export group model configs to JSON file
    const exportConfigs = () => {
        if (!data || data.length === 0) {
            toast.error(t('group.modelConfig.noDataToExport'))
            return
        }

        const exportData = data.map((config) => omitKeys(config, ['group_id']))

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
                <div className="flex flex-wrap items-center justify-between gap-3">
                    <div className="flex min-w-0 flex-wrap gap-2">
                        <div className="relative w-64">
                            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                            <Input
                                placeholder={t('common.search')}
                                value={searchKeyword}
                                onChange={(e) => setSearchKeyword(e.target.value)}
                                className="h-9 pl-8"
                            />
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
                <div className="min-w-0 border-y">
                    <div className="overflow-auto">
                        <table className="w-full min-w-[900px]">
                            <thead className="bg-muted">
                                <tr>
                                    {(['model.modelName', 'group.modelConfig.overrideLimit', 'ui.requestPolicy', 'ui.generationLimits', 'ui.loggingPolicy', 'group.price.title', 'group.modelConfig.actions'] as const).map(label => <th key={label} scope="col" className="h-10 whitespace-nowrap px-3 text-left text-xs font-medium text-muted-foreground">{t(label)}</th>)}
                                </tr>
                            </thead>
                            <tbody>
                                {filteredData.map((config) => (
                                    <tr
                                        key={config.model}
                                        className="border-t hover:bg-muted/50 focus-visible:bg-accent outline-none transition-colors cursor-pointer"
                                        tabIndex={0}
                                        onKeyDown={event => { if (event.target === event.currentTarget && event.key === "Enter") openEditDialog(config) }}
                                        onClick={() => openEditDialog(config)}
                                    >
                                        <td className="px-3 py-3 text-sm font-medium">{config.model}</td>
                                        <td className="px-3 py-3"><ConfigValues entries={[
                                            ['RPM', config.override_limit ? config.rpm : undefined],
                                            ['TPM', config.override_limit ? config.tpm : undefined],
                                        ]} /></td>
                                        <td className="px-3 py-3"><ConfigValues entries={[
                                            [t('model.retryTimes'), config.override_retry_times ? config.retry_times : undefined],
                                            [t('group.modelConfig.overrideTimeoutConfig'), config.override_timeout_config ? true : undefined],
                                            [t('model.dialog.timeout'), config.override_timeout_config ? config.timeout_config?.request_timeout : undefined],
                                            [t('model.dialog.streamTimeout'), config.override_timeout_config ? config.timeout_config?.stream_request_timeout : undefined],
                                        ]} /></td>
                                        <td className="px-3 py-3"><ConfigValues entries={[
                                            [t('model.maxImageGenerationCount'), config.override_max_image_generation_count ? config.max_image_generation_count : undefined],
                                            [t('model.maxVideoGenerationSeconds'), config.override_max_video_generation_seconds ? config.max_video_generation_seconds : undefined],
                                            [t('model.maxVideoGenerationCount'), config.override_max_video_generation_count ? config.max_video_generation_count : undefined],
                                        ]} /></td>
                                        <td className="px-3 py-3"><ConfigValues entries={[
                                            [t('model.forceSaveDetail'), config.override_force_save_detail ? config.force_save_detail : undefined],
                                            [t('model.requestBodyStorageMaxSize'), config.override_request_body_storage_max_size ? config.request_body_storage_max_size : undefined],
                                            [t('model.responseBodyStorageMaxSize'), config.override_response_body_storage_max_size ? config.response_body_storage_max_size : undefined],
                                            [t('model.recordServiceTier'), config.override_summary_service_tier ? config.summary_service_tier : undefined],
                                            [t('model.recordClaudeLongContext'), config.override_summary_claude_long_context ? config.summary_claude_long_context : undefined],
                                        ]} /></td>
                                        <td className="px-3 py-3" onClick={event => event.stopPropagation()}>
                                            {config.override_price ? <PriceDisplay price={config.price} /> : <span className="text-xs text-muted-foreground">{t('ui.inheritModel')}</span>}
                                        </td>
                                        <td className="px-4 py-3 text-sm text-right" onClick={(e) => e.stopPropagation()}>
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button variant="ghost" size="icon" className="h-8 w-8" aria-label={t("ui.actions")}>
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
                                        <td colSpan={7} className="px-4 py-12 text-center text-muted-foreground">
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
                            <Label htmlFor={`${formId}-model`}>{t('model.modelName')}</Label>
                            {isCreating ? (
                                <Combobox
                                    id={`${formId}-model`}
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
                        <div className="flex items-center justify-between gap-4 border-t py-4">
                            <div className="space-y-0.5">
                                <Label htmlFor={`${formId}-formOverrideLimit`}>{t('group.modelConfig.overrideLimit')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideLimitDesc')}</p>
                            </div>
                            <Switch id={`${formId}-formOverrideLimit`}  checked={formOverrideLimit} onCheckedChange={setFormOverrideLimit} />
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
                        <div className="flex items-center justify-between gap-4 border-t py-4">
                            <div className="space-y-0.5">
                                <Label htmlFor={`${formId}-formOverrideRetryTimes`}>{t('group.modelConfig.overrideRetryTimes')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideRetryTimesDesc')}</p>
                            </div>
                            <Switch id={`${formId}-formOverrideRetryTimes`}  checked={formOverrideRetryTimes} onCheckedChange={setFormOverrideRetryTimes} />
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

                        <div className="flex items-center justify-between gap-4 border-t py-4">
                            <div className="space-y-0.5">
                                <Label htmlFor={`${formId}-formOverrideTimeoutConfig`}>{t('group.modelConfig.overrideTimeoutConfig')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideTimeoutConfigDesc')}</p>
                            </div>
                            <Switch id={`${formId}-formOverrideTimeoutConfig`}  checked={formOverrideTimeoutConfig} onCheckedChange={setFormOverrideTimeoutConfig} />
                        </div>

                        {formOverrideTimeoutConfig && (
                            <div className="grid grid-cols-2 gap-4 pl-4">
                                <div className="space-y-2">
                                    <Label>{t('model.dialog.timeout')}</Label>
                                    <Input
                                        type="number"
                                        min={0}
                                        value={formTimeoutConfig.request_timeout ?? 0}
                                        onChange={(e) => setFormTimeoutConfig((prev) => ({
                                            ...prev,
                                            request_timeout: Number(e.target.value),
                                        }))}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <Label>{t('model.dialog.streamTimeout')}</Label>
                                    <Input
                                        type="number"
                                        min={0}
                                        value={formTimeoutConfig.stream_request_timeout ?? 0}
                                        onChange={(e) => setFormTimeoutConfig((prev) => ({
                                            ...prev,
                                            stream_request_timeout: Number(e.target.value),
                                        }))}
                                    />
                                </div>
                            </div>
                        )}

                        {/* Override Force Save Detail */}
                        <div className="flex items-center justify-between gap-4 border-t py-4">
                            <div className="space-y-0.5">
                                <Label htmlFor={`${formId}-formOverrideForceSaveDetail`}>{t('group.modelConfig.overrideForceSaveDetail')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideForceSaveDetailDesc')}</p>
                            </div>
                            <Switch id={`${formId}-formOverrideForceSaveDetail`}  checked={formOverrideForceSaveDetail} onCheckedChange={setFormOverrideForceSaveDetail} />
                        </div>

                        {formOverrideForceSaveDetail && (
                            <div className="flex items-center gap-2 pl-4">
                                <Label htmlFor={`${formId}-formForceSaveDetail`}>{t('model.forceSaveDetail')}</Label>
                                <Switch id={`${formId}-formForceSaveDetail`}  checked={formForceSaveDetail} onCheckedChange={setFormForceSaveDetail} />
                            </div>
                        )}

                        {supportImageGenerationCountLimit && (
                            <>
                                <div className="flex items-center justify-between gap-4 border-t py-4">
                                    <div className="space-y-0.5">
                                        <Label htmlFor={`${formId}-formOverrideMaxImageGenerationCount`}>{t('group.modelConfig.overrideMaxImageGenerationCount')}</Label>
                                        <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideMaxImageGenerationCountDesc')}</p>
                                    </div>
                                    <Switch id={`${formId}-formOverrideMaxImageGenerationCount`}
                                        checked={formOverrideMaxImageGenerationCount}
                                        onCheckedChange={setFormOverrideMaxImageGenerationCount}
                                    />
                                </div>

                                {formOverrideMaxImageGenerationCount && (
                                    <div className="pl-4">
                                        <div className="space-y-2">
                                            <Label>{t('model.maxImageGenerationCount')}</Label>
                                            <Input
                                                type="number"
                                                min={0}
                                                value={formMaxImageGenerationCount}
                                                onChange={(e) => setFormMaxImageGenerationCount(Number(e.target.value))}
                                            />
                                            <p className="text-xs text-muted-foreground">{t('group.modelConfig.maxImageGenerationCountHint')}</p>
                                        </div>
                                    </div>
                                )}
                            </>
                        )}

                        {supportVideoGenerationSecondsLimit && (
                            <>
                                <div className="flex items-center justify-between gap-4 border-t py-4">
                                    <div className="space-y-0.5">
                                        <Label htmlFor={`${formId}-formOverrideMaxVideoGenerationSeconds`}>{t('group.modelConfig.overrideMaxVideoGenerationSeconds')}</Label>
                                        <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideMaxVideoGenerationSecondsDesc')}</p>
                                    </div>
                                    <Switch id={`${formId}-formOverrideMaxVideoGenerationSeconds`}
                                        checked={formOverrideMaxVideoGenerationSeconds}
                                        onCheckedChange={setFormOverrideMaxVideoGenerationSeconds}
                                    />
                                </div>

                                {formOverrideMaxVideoGenerationSeconds && (
                                    <div className="pl-4">
                                        <div className="space-y-2">
                                            <Label>{t('model.maxVideoGenerationSeconds')}</Label>
                                            <Input
                                                type="number"
                                                min={0}
                                                value={formMaxVideoGenerationSeconds}
                                                onChange={(e) => setFormMaxVideoGenerationSeconds(Number(e.target.value))}
                                            />
                                            <p className="text-xs text-muted-foreground">{t('group.modelConfig.maxVideoGenerationSecondsHint')}</p>
                                        </div>
                                    </div>
                                )}
                            </>
                        )}

                        {supportVideoGenerationCountLimit && (
                            <>
                                <div className="flex items-center justify-between gap-4 border-t py-4">
                                    <div className="space-y-0.5">
                                        <Label htmlFor={`${formId}-formOverrideMaxVideoGenerationCount`}>{t('group.modelConfig.overrideMaxVideoGenerationCount')}</Label>
                                        <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideMaxVideoGenerationCountDesc')}</p>
                                    </div>
                                    <Switch id={`${formId}-formOverrideMaxVideoGenerationCount`}
                                        checked={formOverrideMaxVideoGenerationCount}
                                        onCheckedChange={setFormOverrideMaxVideoGenerationCount}
                                    />
                                </div>

                                {formOverrideMaxVideoGenerationCount && (
                                    <div className="pl-4">
                                        <div className="space-y-2">
                                            <Label>{t('model.maxVideoGenerationCount')}</Label>
                                            <Input
                                                type="number"
                                                min={0}
                                                value={formMaxVideoGenerationCount}
                                                onChange={(e) => setFormMaxVideoGenerationCount(Number(e.target.value))}
                                            />
                                            <p className="text-xs text-muted-foreground">{t('group.modelConfig.maxVideoGenerationCountHint')}</p>
                                        </div>
                                    </div>
                                )}
                            </>
                        )}

                        <div className="flex items-center justify-between gap-4 border-t py-4">
                            <div className="space-y-0.5">
                                <Label htmlFor={`${formId}-formOverrideRequestBodyStorageMaxSize`}>{t('group.modelConfig.overrideRequestBodyStorageMaxSize')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideRequestBodyStorageMaxSizeDesc')}</p>
                            </div>
                            <Switch id={`${formId}-formOverrideRequestBodyStorageMaxSize`}
                                checked={formOverrideRequestBodyStorageMaxSize}
                                onCheckedChange={setFormOverrideRequestBodyStorageMaxSize}
                            />
                        </div>

                        {formOverrideRequestBodyStorageMaxSize && (
                            <div className="pl-4">
                                <div className="space-y-2">
                                    <Label>{t('model.requestBodyStorageMaxSize')}</Label>
                                    <Input
                                        type="number"
                                        value={formRequestBodyStorageMaxSize}
                                        onChange={(e) => setFormRequestBodyStorageMaxSize(Number(e.target.value))}
                                    />
                                    <p className="text-xs text-muted-foreground">{t('group.modelConfig.storageMaxSizeHint')}</p>
                                </div>
                            </div>
                        )}

                        <div className="flex items-center justify-between gap-4 border-t py-4">
                            <div className="space-y-0.5">
                                <Label htmlFor={`${formId}-formOverrideResponseBodyStorageMaxSize`}>{t('group.modelConfig.overrideResponseBodyStorageMaxSize')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideResponseBodyStorageMaxSizeDesc')}</p>
                            </div>
                            <Switch id={`${formId}-formOverrideResponseBodyStorageMaxSize`}
                                checked={formOverrideResponseBodyStorageMaxSize}
                                onCheckedChange={setFormOverrideResponseBodyStorageMaxSize}
                            />
                        </div>

                        {formOverrideResponseBodyStorageMaxSize && (
                            <div className="pl-4">
                                <div className="space-y-2">
                                    <Label>{t('model.responseBodyStorageMaxSize')}</Label>
                                    <Input
                                        type="number"
                                        value={formResponseBodyStorageMaxSize}
                                        onChange={(e) => setFormResponseBodyStorageMaxSize(Number(e.target.value))}
                                    />
                                    <p className="text-xs text-muted-foreground">{t('group.modelConfig.storageMaxSizeHint')}</p>
                                </div>
                            </div>
                        )}

                        {/* Override Record Service Tier */}
                        <div className="flex items-center justify-between gap-4 border-t py-4">
                            <div className="space-y-0.5">
                                <Label htmlFor={`${formId}-formOverrideSummaryServiceTier`}>{t('group.modelConfig.overrideRecordServiceTier')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideRecordServiceTierDesc')}</p>
                            </div>
                            <Switch id={`${formId}-formOverrideSummaryServiceTier`}  checked={formOverrideSummaryServiceTier} onCheckedChange={setFormOverrideSummaryServiceTier} />
                        </div>

                        {formOverrideSummaryServiceTier && (
                            <div className="flex items-center gap-2 pl-4">
                                <Label htmlFor={`${formId}-formSummaryServiceTier`}>{t('model.recordServiceTier')}</Label>
                                <Switch id={`${formId}-formSummaryServiceTier`}  checked={formSummaryServiceTier} onCheckedChange={setFormSummaryServiceTier} />
                            </div>
                        )}

                        <div className="flex items-center justify-between gap-4 border-t py-4">
                            <div className="space-y-0.5">
                                <Label htmlFor={`${formId}-formOverrideSummaryClaudeLongContext`}>{t('group.modelConfig.overrideRecordClaudeLongContext')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overrideRecordClaudeLongContextDesc')}</p>
                            </div>
                            <Switch id={`${formId}-formOverrideSummaryClaudeLongContext`}  checked={formOverrideSummaryClaudeLongContext} onCheckedChange={setFormOverrideSummaryClaudeLongContext} />
                        </div>

                        {formOverrideSummaryClaudeLongContext && (
                            <div className="flex items-center gap-2 pl-4">
                                <Label htmlFor={`${formId}-formSummaryClaudeLongContext`}>{t('model.recordClaudeLongContext')}</Label>
                                <Switch id={`${formId}-formSummaryClaudeLongContext`}  checked={formSummaryClaudeLongContext} onCheckedChange={setFormSummaryClaudeLongContext} />
                            </div>
                        )}

                        {/* Override Price */}
                        <div className="flex items-center justify-between gap-4 border-t py-4">
                            <div className="space-y-0.5">
                                <Label htmlFor={`${formId}-formOverridePrice`}>{t('group.modelConfig.overridePrice')}</Label>
                                <p className="text-xs text-muted-foreground">{t('group.modelConfig.overridePriceDesc')}</p>
                            </div>
                            <Switch id={`${formId}-formOverridePrice`}  checked={formOverridePrice} onCheckedChange={setFormOverridePrice} />
                        </div>

                        {formOverridePrice && (
                            <div className="pl-4">
                                <PriceFormFields price={formPrice} onChange={setFormPrice} />
                            </div>
                        )}
                    </div>

                    <DialogFooter className="sticky bottom-0 border-t bg-card py-3">
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
