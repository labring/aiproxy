// src/feature/model/components/ModelForm.tsx
import { useForm } from 'react-hook-form'
import type { FieldErrors } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import {
    Form,
    FormControl,
    FormDescription,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from '@/components/ui/form'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { ChevronDown, ChevronUp, Plus, X } from 'lucide-react'
import { modelCreateSchema } from '@/validation/model'
import { useCreateModel, useUpdateModel } from '../hooks'
import { useTranslation } from 'react-i18next'
import { ModelCreateForm } from '@/validation/model'
import { Plugin, EngineConfig, ModelPrice, ModelCreateRequest } from '@/types/model'
import { AdvancedErrorDisplay } from '@/components/common/error/errorDisplay'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'
import { useState } from 'react'
import { ENV } from '@/utils/env'
import { PriceFormFields } from '@/components/price/PriceFormFields'
import { ValidationErrorDisplay } from '@/components/common/error/validationErrorDisplay'

interface ModelFormProps {
    mode?: 'create' | 'update'
    onSuccess?: () => void
    defaultValues?: {
        model: string
        type: number
        rpm?: number
        tpm?: number
        retry_times?: number
        timeout?: number
        max_error_rate?: number
        force_save_detail?: boolean
        summary_service_tier?: boolean
        summary_claude_long_context?: boolean
        price?: ModelPrice
        plugin?: Plugin
    }
}

export function ModelForm({
    mode = 'create',
    onSuccess,
    defaultValues = {
        model: '',
        type: 1,
    },
}: ModelFormProps) {
    const { t } = useTranslation()

    // Collapsible expanded states
    const [priceExpanded, setPriceExpanded] = useState(false)
    const [cachePluginExpanded, setCachePluginExpanded] = useState(false)
    const [webSearchPluginExpanded, setWebSearchPluginExpanded] = useState(false)

    // API hooks
    const {
        createModel,
        isLoading: isCreating,
        error: createError,
        clearError: clearCreateError
    } = useCreateModel()

    const {
        updateModel,
        isLoading: isUpdating,
        error: updateError,
        clearError: clearUpdateError
    } = useUpdateModel()

    // Combined loading and error states
    const isLoading = isCreating || isUpdating
    const error = mode === 'create' ? createError : updateError
    const clearError = mode === 'create' ? clearCreateError : clearUpdateError

    // Form setup with simplified default values
    const form = useForm<ModelCreateForm>({
        resolver: zodResolver(modelCreateSchema),
        mode: 'onChange', // 启用实时验证
        defaultValues: {
            model: defaultValues.model || '',
            type: defaultValues.type || 1,
            rpm: defaultValues.rpm,
            tpm: defaultValues.tpm,
            retry_times: defaultValues.retry_times,
            timeout: defaultValues.timeout,
            max_error_rate: defaultValues.max_error_rate,
            force_save_detail: defaultValues.force_save_detail ?? false,
            summary_service_tier: defaultValues.summary_service_tier ?? false,
            summary_claude_long_context: defaultValues.summary_claude_long_context ?? false,
            price: defaultValues.price || {},
            plugin: {
                cache: { enable: false, ...defaultValues.plugin?.cache },
                "web-search": { enable: false, search_from: [], ...defaultValues.plugin?.["web-search"] },
                "think-split": { enable: false, ...defaultValues.plugin?.["think-split"] },
                "stream-fake": { enable: false, ...defaultValues.plugin?.["stream-fake"] },
            }
        },
    })

    // Watch plugin enable states
    const cacheEnabled = form.watch('plugin.cache.enable')
    const webSearchEnabled = form.watch('plugin.web-search.enable')
    const searchEngines = form.watch('plugin.web-search.search_from') || []

    // Available search engine types
    const availableEngineTypes = ['bing', 'google', 'arxiv', 'searchxng'] as const

    // Watch form errors for debugging
    const formErrors = form.formState.errors

    // Add search engine
    const addSearchEngine = () => {
        const currentEngines = form.getValues('plugin.web-search.search_from') || []
        const newEngine: EngineConfig = {
            type: 'bing',
            max_results: undefined,
            spec: undefined
        }
        form.setValue('plugin.web-search.search_from', [...currentEngines, newEngine])
    }

    // Remove search engine
    const removeSearchEngine = (index: number) => {
        const currentEngines = form.getValues('plugin.web-search.search_from') || []
        const newEngines = currentEngines.filter((_, i) => i !== index)
        form.setValue('plugin.web-search.search_from', newEngines)
    }

    // Update search engine
    const updateSearchEngine = (index: number, updates: Partial<EngineConfig>) => {
        const currentEngines = form.getValues('plugin.web-search.search_from') || []
        const newEngines = [...currentEngines]
        newEngines[index] = { ...newEngines[index], ...updates }
        form.setValue('plugin.web-search.search_from', newEngines)
    }

    // Render engine spec fields based on type
    const renderEngineSpecFields = (engine: EngineConfig, index: number) => {
        const engineType = engine.type
        const spec = engine.spec || ({} as Record<string, unknown>)

        switch (engineType) {
            case 'google': {
                const googleSpec = spec as { api_key?: string; cx?: string }
                return (
                    <div className="space-y-2">
                        <div>
                            <Label className="text-xs">{t("model.dialog.webSearchPlugin.engineSpec.apiKey")}</Label>
                            <Input
                                placeholder={t("model.dialog.webSearchPlugin.engineSpec.apiKeyPlaceholder")}
                                value={googleSpec?.api_key || ''}
                                onChange={(e) => updateSearchEngine(index, {
                                    spec: { ...spec, api_key: e.target.value }
                                })}
                                className="mt-1"
                            />
                        </div>
                        <div>
                            <Label className="text-xs">{t("model.dialog.webSearchPlugin.engineSpec.cx")}</Label>
                            <Input
                                placeholder={t("model.dialog.webSearchPlugin.engineSpec.cxPlaceholder")}
                                value={googleSpec?.cx || ''}
                                onChange={(e) => updateSearchEngine(index, {
                                    spec: { ...spec, cx: e.target.value }
                                })}
                                className="mt-1"
                            />
                        </div>
                    </div>
                )
            }
            case 'bing': {
                const bingSpec = spec as { api_key?: string }
                return (
                    <div>
                        <Label className="text-xs">{t("model.dialog.webSearchPlugin.engineSpec.apiKey")}</Label>
                        <Input
                            placeholder={t("model.dialog.webSearchPlugin.engineSpec.apiKeyPlaceholder")}
                            value={bingSpec?.api_key || ''}
                            onChange={(e) => updateSearchEngine(index, {
                                spec: { ...spec, api_key: e.target.value }
                            })}
                            className="mt-1"
                        />
                    </div>
                )
            }
            case 'searchxng': {
                const searchxngSpec = spec as { base_url?: string }
                return (
                    <div>
                        <Label className="text-xs">{t("model.dialog.webSearchPlugin.engineSpec.baseUrl")}</Label>
                        <Input
                            placeholder={t("model.dialog.webSearchPlugin.engineSpec.baseUrlPlaceholder")}
                            value={searchxngSpec?.base_url || ''}
                            onChange={(e) => updateSearchEngine(index, {
                                spec: { ...spec, base_url: e.target.value }
                            })}
                            className="mt-1"
                        />
                    </div>
                )
            }
            case 'arxiv':
            default:
                return null
        }
    }

    // Form submission handler
    const handleFormSubmit = (data: ModelCreateForm) => {
        console.log('Form submitted with data:', data)

        // Clear previous errors
        if (clearError) clearError()

        // Prepare plugin data - only include enabled plugins with their configured values
        const pluginData = {}

        // Cache plugin - 如果开启，必须有 enable 字段，其他字段可选
        if (data.plugin?.cache?.enable) {
            Object.assign(pluginData, {
                cache: {
                    enable: true,
                    ...(data.plugin.cache.ttl && { ttl: data.plugin.cache.ttl }),
                    ...(data.plugin.cache.item_max_size && { item_max_size: data.plugin.cache.item_max_size }),
                    ...(data.plugin.cache.add_cache_hit_header !== undefined && { add_cache_hit_header: data.plugin.cache.add_cache_hit_header }),
                    ...(data.plugin.cache.cache_hit_header && { cache_hit_header: data.plugin.cache.cache_hit_header }),
                }
            })
        }

        // Web search plugin - 如果开启，必须有 enable 和 search_from，其他字段可选
        if (data.plugin?.["web-search"]?.enable && data.plugin["web-search"].search_from && data.plugin["web-search"].search_from.length > 0) {
            // Clean up search engines - remove empty spec objects
            const cleanedSearchFrom = data.plugin["web-search"].search_from.map(engine => ({
                type: engine.type,
                ...(engine.max_results && { max_results: engine.max_results }),
                ...(engine.spec && Object.keys(engine.spec).some(key => (engine.spec as Record<string, unknown>)[key]) && { spec: engine.spec })
            }))

            Object.assign(pluginData, {
                "web-search": {
                    enable: true,
                    search_from: cleanedSearchFrom,
                    ...(data.plugin["web-search"].force_search !== undefined && { force_search: data.plugin["web-search"].force_search }),
                    ...(data.plugin["web-search"].max_results && { max_results: data.plugin["web-search"].max_results }),
                    ...(data.plugin["web-search"].need_reference !== undefined && { need_reference: data.plugin["web-search"].need_reference }),
                    ...(data.plugin["web-search"].reference_location && { reference_location: data.plugin["web-search"].reference_location }),
                    ...(data.plugin["web-search"].reference_format && { reference_format: data.plugin["web-search"].reference_format }),
                    ...(data.plugin["web-search"].default_language && { default_language: data.plugin["web-search"].default_language }),
                    ...(data.plugin["web-search"].prompt_template && { prompt_template: data.plugin["web-search"].prompt_template }),
                }
            })
        }

        // Think split plugin - 如果开启，必须有 enable 字段
        if (data.plugin?.["think-split"]?.enable) {
            Object.assign(pluginData, {
                "think-split": {
                    enable: true
                }
            })
        }

        // Stream fake plugin - 如果开启，必须有 enable 字段
        if (data.plugin?.["stream-fake"]?.enable) {
            Object.assign(pluginData, {
                "stream-fake": {
                    enable: true
                }
            })
        }

        // Clean price data - remove zero/undefined values
        const cleanPrice = (p: ModelPrice | undefined) => {
            if (!p) return undefined
            const cleaned: Record<string, unknown> = {}
            for (const [k, v] of Object.entries(p)) {
                if (k === 'conditional_prices') {
                    if (Array.isArray(v) && v.length > 0) cleaned[k] = v
                } else if (v !== undefined && v !== 0) {
                    cleaned[k] = v
                }
            }
            return Object.keys(cleaned).length > 0 ? cleaned : undefined
        }
        const priceData = cleanPrice(data.price as ModelPrice | undefined)

        // Prepare data for API - 如果没有启用的插件，则不传递 plugin 字段
        const formData: Omit<ModelCreateRequest, 'model'> = {
            type: Number(data.type),
            ...(data.rpm !== undefined && { rpm: Number(data.rpm) }),
            ...(data.tpm !== undefined && { tpm: Number(data.tpm) }),
            ...(data.retry_times !== undefined && { retry_times: Number(data.retry_times) }),
            ...(data.timeout !== undefined && { timeout: Number(data.timeout) }),
            ...(data.max_error_rate !== undefined && { max_error_rate: Number(data.max_error_rate) }),
            ...(data.force_save_detail !== undefined && { force_save_detail: data.force_save_detail }),
            ...(data.summary_service_tier !== undefined && { summary_service_tier: data.summary_service_tier }),
            ...(data.summary_claude_long_context !== undefined && { summary_claude_long_context: data.summary_claude_long_context }),
            ...(priceData && { price: priceData }),
            ...(Object.keys(pluginData).length > 0 && { plugin: pluginData as Plugin })
        }

        if (mode === 'create') {
            // For create mode, include the model name
            createModel({
                model: data.model,
                type: Number(data.type),
                ...(data.rpm !== undefined && { rpm: Number(data.rpm) }),
                ...(data.tpm !== undefined && { tpm: Number(data.tpm) }),
                ...(data.retry_times !== undefined && { retry_times: Number(data.retry_times) }),
                ...(data.timeout !== undefined && { timeout: Number(data.timeout) }),
                ...(data.max_error_rate !== undefined && { max_error_rate: Number(data.max_error_rate) }),
                ...(data.force_save_detail !== undefined && { force_save_detail: data.force_save_detail }),
                ...(data.summary_service_tier !== undefined && { summary_service_tier: data.summary_service_tier }),
                ...(data.summary_claude_long_context !== undefined && { summary_claude_long_context: data.summary_claude_long_context }),
                ...(priceData && { price: priceData }),
                ...(Object.keys(pluginData).length > 0 && { plugin: pluginData as Plugin })
            }, {
                onSuccess: () => {
                    // Reset form
                    form.reset()
                    // Notify parent component
                    if (onSuccess) onSuccess()
                }
            })
        } else {
            // For update mode, use the model name as the identifier
            updateModel({
                model: data.model,
                data: formData
            }, {
                onSuccess: () => {
                    // Notify parent component
                    if (onSuccess) onSuccess()
                }
            })
        }
    }

    return (
        <div>
            {/* 使用简化的验证错误显示组件 */}
            <ValidationErrorDisplay
                errors={formErrors as FieldErrors<Record<string, unknown>>}
                className="mb-4"
            />

            <Form {...form}>
                <form onSubmit={form.handleSubmit(handleFormSubmit, (errors) => {
                    // 处理表单验证失败
                    console.error('Form validation failed:', errors)
                    if (ENV.isDevelopment) {
                        console.group('🔴 Form Submission Failed:')
                        console.log('Validation Errors:', errors)
                        console.log('Current Form Values:', form.getValues())
                        console.groupEnd()
                    }
                })} className="space-y-6">
                    {/* API error alert */}
                    {error && (
                        <AdvancedErrorDisplay error={error} />
                    )}

                    {/* Model name field */}
                    <FormField
                        control={form.control}
                        name="model"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("model.dialog.modelName")}</FormLabel>
                                <FormControl>
                                    <Input
                                        placeholder={t("model.dialog.modelNamePlaceholder")}
                                        {...field}
                                        disabled={mode === 'update'}
                                        className={mode === 'update' ? 'bg-muted' : ''}
                                    />
                                </FormControl>
                                <FormMessage />
                                {mode === 'update' && (
                                    <p className="text-xs text-muted-foreground">
                                        {t("model.dialog.modelNameUpdateDisabled")}
                                    </p>
                                )}
                            </FormItem>
                        )}
                    />

                    {/* Model type field */}
                    <FormField
                        control={form.control}
                        name="type"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("model.dialog.modelType")}</FormLabel>
                                <Select
                                    onValueChange={(value) => field.onChange(Number(value))}
                                    defaultValue={String(field.value)}
                                >
                                    <FormControl>
                                        <SelectTrigger>
                                            <SelectValue placeholder={t("model.dialog.selectType")} />
                                        </SelectTrigger>
                                    </FormControl>
                                    <SelectContent>
                                        {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 13].map((type) => (
                                            <SelectItem key={type} value={String(type)}>
                                                {t(`modeType.${type}` as never)}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                <FormMessage />
                            </FormItem>
                        )}
                    />

                    {/* RPM Field */}
                    <FormField
                        control={form.control}
                        name="rpm"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("model.dialog.rpm")}</FormLabel>
                                <FormControl>
                                    <Input
                                        type="number"
                                        placeholder={t("model.dialog.rpmPlaceholder")}
                                        {...field}
                                        onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                    />
                                </FormControl>
                                <FormMessage />
                            </FormItem>
                        )}
                    />

                    {/* TPM Field */}
                    <FormField
                        control={form.control}
                        name="tpm"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("model.dialog.tpm")}</FormLabel>
                                <FormControl>
                                    <Input
                                        type="number"
                                        placeholder={t("model.dialog.tpmPlaceholder")}
                                        {...field}
                                        onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                    />
                                </FormControl>
                                <FormMessage />
                            </FormItem>
                        )}
                    />

                    {/* Retry Times Field */}
                    <FormField
                        control={form.control}
                        name="retry_times"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("model.dialog.retryTimes")}</FormLabel>
                                <FormControl>
                                    <Input
                                        type="number"
                                        placeholder={t("model.dialog.retryTimesPlaceholder")}
                                        {...field}
                                        onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                    />
                                </FormControl>
                                <FormMessage />
                            </FormItem>
                        )}
                    />

                    {/* Timeout Field */}
                    <FormField
                        control={form.control}
                        name="timeout"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("model.dialog.timeout")}</FormLabel>
                                <FormControl>
                                    <Input
                                        type="number"
                                        placeholder={t("model.dialog.timeoutPlaceholder")}
                                        {...field}
                                        onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                    />
                                </FormControl>
                                <FormMessage />
                            </FormItem>
                        )}
                    />

                    {/* Max Error Rate Field */}
                    <FormField
                        control={form.control}
                        name="max_error_rate"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("model.dialog.maxErrorRate")}</FormLabel>
                                <FormControl>
                                    <Input
                                        type="number"
                                        placeholder={t("model.dialog.maxErrorRatePlaceholder")}
                                        {...field}
                                        min="0"
                                        max="1"
                                        step="0.01"
                                        onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                    />
                                </FormControl>
                                <FormMessage />
                            </FormItem>
                        )}
                    />

                    {/* Force Save Detail Switch */}
                    <FormField
                        control={form.control}
                        name="force_save_detail"
                        render={({ field }) => (
                            <FormItem className="flex flex-row items-center justify-between py-2">
                                <div className="space-y-1">
                                    <FormLabel>{t("model.dialog.forceSaveDetail")}</FormLabel>
                                    <FormDescription>{t("model.dialog.forceSaveDetailDescription")}</FormDescription>
                                </div>
                                <FormControl>
                                    <Switch
                                        checked={field.value}
                                        onCheckedChange={field.onChange}
                                    />
                                </FormControl>
                            </FormItem>
                        )}
                    />

                    {/* Record Service Tier Switch */}
                    <FormField
                        control={form.control}
                        name="summary_service_tier"
                        render={({ field }) => (
                            <FormItem className="flex flex-row items-center justify-between py-2">
                                <div className="space-y-1">
                                    <FormLabel>{t("model.dialog.recordServiceTier")}</FormLabel>
                                    <FormDescription>{t("model.dialog.recordServiceTierDescription")}</FormDescription>
                                </div>
                                <FormControl>
                                    <Switch
                                        checked={field.value}
                                        onCheckedChange={field.onChange}
                                    />
                                </FormControl>
                            </FormItem>
                        )}
                    />

                    <FormField
                        control={form.control}
                        name="summary_claude_long_context"
                        render={({ field }) => (
                            <FormItem className="flex flex-row items-center justify-between py-2">
                                <div className="space-y-1">
                                    <FormLabel>{t("model.dialog.recordClaudeLongContext")}</FormLabel>
                                    <FormDescription>{t("model.dialog.recordClaudeLongContextDescription")}</FormDescription>
                                </div>
                                <FormControl>
                                    <Switch
                                        checked={field.value}
                                        onCheckedChange={field.onChange}
                                    />
                                </FormControl>
                            </FormItem>
                        )}
                    />

                    {/* Price Configuration Section */}
                    <Collapsible open={priceExpanded} onOpenChange={setPriceExpanded}>
                        <CollapsibleTrigger className="flex items-center justify-between w-full py-3 px-4 border rounded-lg hover:bg-muted/50 transition-colors">
                            <div className="text-left">
                                <h3 className="text-sm font-medium">{t("group.price.title")}</h3>
                                <p className="text-xs text-muted-foreground">{t("group.price.description")}</p>
                            </div>
                            {priceExpanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                        </CollapsibleTrigger>
                        <CollapsibleContent className="pt-3 px-1">
                            <PriceFormFields
                                price={(form.watch('price') || {}) as ModelPrice}
                                onChange={(p) => form.setValue('price', p as ModelCreateForm['price'])}
                            />
                        </CollapsibleContent>
                    </Collapsible>

                    {/* Plugin Configuration Section */}
                    <div className="space-y-6">
                        <div>
                            <h3 className="text-lg font-medium">{t("model.dialog.pluginConfiguration")}</h3>
                            <p className="text-sm text-muted-foreground">{t("model.dialog.pluginConfigurationDescription")}</p>
                        </div>

                        <hr className="border-border" />

                        {/* Cache Plugin */}
                        <div className="space-y-4">
                            <Collapsible open={cachePluginExpanded} onOpenChange={setCachePluginExpanded}>
                                <div className="flex items-center justify-between py-2">
                                    <div className="flex items-center space-x-3">
                                        <FormField
                                            control={form.control}
                                            name="plugin.cache.enable"
                                            render={({ field }) => (
                                                <FormItem className="flex items-center space-x-2">
                                                    <FormControl>
                                                        <Switch
                                                            checked={field.value}
                                                            onCheckedChange={field.onChange}
                                                        />
                                                    </FormControl>
                                                </FormItem>
                                            )}
                                        />
                                        <div>
                                            <Label className="text-sm font-medium">{t("model.dialog.cachePlugin.title")}</Label>
                                            <p className="text-xs text-muted-foreground">{t("model.dialog.cachePlugin.description")}</p>
                                        </div>
                                    </div>
                                    {cacheEnabled && (
                                        <CollapsibleTrigger asChild>
                                            <Button variant="ghost" size="sm">
                                                {cachePluginExpanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                                            </Button>
                                        </CollapsibleTrigger>
                                    )}
                                </div>
                                {cacheEnabled && (
                                    <CollapsibleContent className="space-y-4 pl-8 pb-4">
                                        {/* TTL Field */}
                                        <FormField
                                            control={form.control}
                                            name="plugin.cache.ttl"
                                            render={({ field }) => (
                                                <FormItem>
                                                    <FormLabel>{t("model.dialog.cachePlugin.ttl")}</FormLabel>
                                                    <FormControl>
                                                        <Input
                                                            type="number"
                                                            placeholder={t("model.dialog.cachePlugin.ttlPlaceholder")}
                                                            {...field}
                                                            onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                                        />
                                                    </FormControl>
                                                    <FormMessage />
                                                </FormItem>
                                            )}
                                        />

                                        {/* Item Max Size Field */}
                                        <FormField
                                            control={form.control}
                                            name="plugin.cache.item_max_size"
                                            render={({ field }) => (
                                                <FormItem>
                                                    <FormLabel>{t("model.dialog.cachePlugin.itemMaxSize")}</FormLabel>
                                                    <FormControl>
                                                        <Input
                                                            type="number"
                                                            placeholder={t("model.dialog.cachePlugin.itemMaxSizePlaceholder")}
                                                            {...field}
                                                            onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                                        />
                                                    </FormControl>
                                                    <FormMessage />
                                                </FormItem>
                                            )}
                                        />

                                        {/* Add Cache Hit Header */}
                                        <FormField
                                            control={form.control}
                                            name="plugin.cache.add_cache_hit_header"
                                            render={({ field }) => (
                                                <FormItem className="flex flex-row items-center justify-between py-2">
                                                    <FormLabel>{t("model.dialog.cachePlugin.addCacheHitHeader")}</FormLabel>
                                                    <FormControl>
                                                        <Switch
                                                            checked={field.value}
                                                            onCheckedChange={field.onChange}
                                                        />
                                                    </FormControl>
                                                </FormItem>
                                            )}
                                        />

                                        {/* Cache Hit Header Name */}
                                        {form.watch('plugin.cache.add_cache_hit_header') && (
                                            <FormField
                                                control={form.control}
                                                name="plugin.cache.cache_hit_header"
                                                render={({ field }) => (
                                                    <FormItem>
                                                        <FormLabel>{t("model.dialog.cachePlugin.cacheHitHeader")}</FormLabel>
                                                        <FormControl>
                                                            <Input placeholder={t("model.dialog.cachePlugin.cacheHitHeaderPlaceholder")} {...field} />
                                                        </FormControl>
                                                        <FormMessage />
                                                    </FormItem>
                                                )}
                                            />
                                        )}
                                    </CollapsibleContent>
                                )}
                            </Collapsible>
                        </div>

                        <hr className="border-border" />

                        {/* Web Search Plugin */}
                        <div className="space-y-4">
                            <Collapsible open={webSearchPluginExpanded} onOpenChange={setWebSearchPluginExpanded}>
                                <div className="flex items-center justify-between py-2">
                                    <div className="flex items-center space-x-3">
                                        <FormField
                                            control={form.control}
                                            name="plugin.web-search.enable"
                                            render={({ field }) => (
                                                <FormItem className="flex items-center space-x-2">
                                                    <FormControl>
                                                        <Switch
                                                            checked={field.value}
                                                            onCheckedChange={field.onChange}
                                                        />
                                                    </FormControl>
                                                </FormItem>
                                            )}
                                        />
                                        <div>
                                            <Label className="text-sm font-medium">{t("model.dialog.webSearchPlugin.title")}</Label>
                                            <p className="text-xs text-muted-foreground">{t("model.dialog.webSearchPlugin.description")}</p>
                                        </div>
                                    </div>
                                    {webSearchEnabled && (
                                        <CollapsibleTrigger asChild>
                                            <Button variant="ghost" size="sm">
                                                {webSearchPluginExpanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                                            </Button>
                                        </CollapsibleTrigger>
                                    )}
                                </div>
                                {webSearchEnabled && (
                                    <CollapsibleContent className="space-y-4 pl-8 pb-4">
                                        {/* Search Engines Configuration */}
                                        <div>
                                            <div className="flex items-center justify-between mb-3">
                                                <Label className="text-sm font-medium">{t("model.dialog.webSearchPlugin.searchFrom")}</Label>
                                                <Button
                                                    type="button"
                                                    variant="outline"
                                                    size="sm"
                                                    onClick={addSearchEngine}
                                                    className="flex items-center gap-1"
                                                >
                                                    <Plus className="h-3 w-3" />
                                                    {t("model.dialog.webSearchPlugin.addEngine")}
                                                </Button>
                                            </div>

                                            <div className="space-y-3">
                                                {searchEngines.map((engine, index) => (
                                                    <div key={index} className="p-4 bg-muted/30 rounded-lg">
                                                        <div className="flex items-start justify-between mb-3">
                                                            <Label className="text-sm font-medium">
                                                                {t("model.dialog.webSearchPlugin.engineConfig")} #{index + 1}
                                                            </Label>
                                                            <Button
                                                                type="button"
                                                                variant="ghost"
                                                                size="sm"
                                                                onClick={() => removeSearchEngine(index)}
                                                                className="h-6 w-6 p-0 text-destructive hover:text-destructive"
                                                            >
                                                                <X className="h-3 w-3" />
                                                            </Button>
                                                        </div>

                                                        <div className="space-y-3">
                                                            {/* Engine Type */}
                                                            <div>
                                                                <Label className="text-xs">{t("model.dialog.webSearchPlugin.engineType")}</Label>
                                                                <Select
                                                                    value={engine.type}
                                                                    onValueChange={(value) => updateSearchEngine(index, { type: value as 'bing' | 'google' | 'arxiv' | 'searchxng' })}
                                                                >
                                                                    <SelectTrigger className="mt-1">
                                                                        <SelectValue />
                                                                    </SelectTrigger>
                                                                    <SelectContent>
                                                                        {availableEngineTypes.map((type) => (
                                                                            <SelectItem key={type} value={type}>
                                                                                {t(`model.dialog.webSearchPlugin.searchEngines.${type}` as 'model.dialog.webSearchPlugin.searchEngines.bing' | 'model.dialog.webSearchPlugin.searchEngines.google' | 'model.dialog.webSearchPlugin.searchEngines.arxiv' | 'model.dialog.webSearchPlugin.searchEngines.searchxng')}
                                                                            </SelectItem>
                                                                        ))}
                                                                    </SelectContent>
                                                                </Select>
                                                            </div>

                                                            {/* Max Results */}
                                                            <div>
                                                                <Label className="text-xs">{t("model.dialog.webSearchPlugin.maxResults")}</Label>
                                                                <Input
                                                                    type="number"
                                                                    placeholder={t("model.dialog.webSearchPlugin.maxResultsPlaceholder")}
                                                                    value={engine.max_results || ''}
                                                                    onChange={(e) => updateSearchEngine(index, {
                                                                        max_results: e.target.value ? Number(e.target.value) : undefined
                                                                    })}
                                                                    className="mt-1"
                                                                />
                                                            </div>

                                                            {/* Engine Specific Configuration */}
                                                            {renderEngineSpecFields(engine, index)}
                                                        </div>
                                                    </div>
                                                ))}

                                                {searchEngines.length === 0 && (
                                                    <div className="text-center py-8 text-muted-foreground text-sm border-2 border-dashed rounded-lg">
                                                        {t("model.dialog.noSearchEngineConfigured")}
                                                    </div>
                                                )}
                                            </div>
                                        </div>

                                        {/* Force Search */}
                                        <FormField
                                            control={form.control}
                                            name="plugin.web-search.force_search"
                                            render={({ field }) => (
                                                <FormItem className="flex flex-row items-center justify-between py-2">
                                                    <FormLabel>{t("model.dialog.webSearchPlugin.forceSearch")}</FormLabel>
                                                    <FormControl>
                                                        <Switch
                                                            checked={field.value}
                                                            onCheckedChange={field.onChange}
                                                        />
                                                    </FormControl>
                                                </FormItem>
                                            )}
                                        />

                                        {/* Global Max Results */}
                                        <FormField
                                            control={form.control}
                                            name="plugin.web-search.max_results"
                                            render={({ field }) => (
                                                <FormItem>
                                                    <FormLabel>{t("model.dialog.webSearchPlugin.maxResults")} ({t("common.global")})</FormLabel>
                                                    <FormControl>
                                                        <Input
                                                            type="number"
                                                            placeholder={t("model.dialog.webSearchPlugin.maxResultsPlaceholder")}
                                                            {...field}
                                                            onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                                        />
                                                    </FormControl>
                                                    <FormMessage />
                                                </FormItem>
                                            )}
                                        />
                                    </CollapsibleContent>
                                )}
                            </Collapsible>
                        </div>

                        <hr className="border-border" />

                        {/* Think Split Plugin */}
                        <div className="flex items-center justify-between py-2">
                            <div className="flex items-center space-x-3">
                                <FormField
                                    control={form.control}
                                    name="plugin.think-split.enable"
                                    render={({ field }) => (
                                        <FormItem className="flex items-center space-x-2">
                                            <FormControl>
                                                <Switch
                                                    checked={field.value}
                                                    onCheckedChange={field.onChange}
                                                />
                                            </FormControl>
                                        </FormItem>
                                    )}
                                />
                                <div>
                                    <Label className="text-sm font-medium">{t("model.dialog.thinkSplitPlugin.title")}</Label>
                                    <p className="text-xs text-muted-foreground">{t("model.dialog.thinkSplitPlugin.description")}</p>
                                </div>
                            </div>
                        </div>

                        <hr className="border-border" />

                        {/* Stream Fake Plugin */}
                        <div className="flex items-center justify-between py-2">
                            <div className="flex items-center space-x-3">
                                <FormField
                                    control={form.control}
                                    name="plugin.stream-fake.enable"
                                    render={({ field }) => (
                                        <FormItem className="flex items-center space-x-2">
                                            <FormControl>
                                                <Switch
                                                    checked={field.value}
                                                    onCheckedChange={field.onChange}
                                                />
                                            </FormControl>
                                        </FormItem>
                                    )}
                                />
                                <div>
                                    <Label className="text-sm font-medium">{t("model.dialog.streamFakePlugin.title")}</Label>
                                    <p className="text-xs text-muted-foreground">{t("model.dialog.streamFakePlugin.description")}</p>
                                </div>
                            </div>
                        </div>
                    </div>

                    {/* Submit button */}
                    <div className="flex justify-end">
                        <AnimatedButton >
                            <Button type="submit" disabled={isLoading}>
                                {isLoading
                                    ? t("model.dialog.submitting")
                                    : mode === 'create'
                                        ? t("model.dialog.create")
                                        : t("model.dialog.update")
                                }
                            </Button>
                        </AnimatedButton>
                    </div>
                </form>
            </Form>
        </div>
    )
}
