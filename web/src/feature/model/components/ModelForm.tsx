// src/feature/model/components/ModelForm.tsx
import { useForm } from 'react-hook-form'
import type { FieldErrors } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
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
import { Plugin, EngineConfig, ModelPrice, ModelCreateRequest, ModelConfig, ModelConfigDetail } from '@/types/model'
import { AdvancedErrorDisplay } from '@/components/common/error/errorDisplay'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'
import { useState } from 'react'
import { ENV } from '@/utils/env'
import { PriceFormFields } from '@/components/price/PriceFormFields'
import { ValidationErrorDisplay } from '@/components/common/error/validationErrorDisplay'

const KNOWN_PRICE_KEYS = new Set([
    'input_price',
    'input_price_unit',
    'output_price',
    'output_price_unit',
    'per_request_price',
    'cache_creation_price',
    'cache_creation_price_unit',
    'cached_price',
    'cached_price_unit',
    'image_input_price',
    'image_input_price_unit',
    'image_output_price',
    'image_output_price_unit',
    'audio_input_price',
    'audio_input_price_unit',
    'thinking_mode_output_price',
    'thinking_mode_output_price_unit',
    'web_search_price',
    'web_search_price_unit',
    'conditional_prices',
])

const MANAGED_MODEL_KEYS = new Set([
    'config',
    'owner',
    'type',
    'rpm',
    'tpm',
    'retry_times',
    'timeout_config',
    'max_error_rate',
    'force_save_detail',
    'summary_service_tier',
    'summary_claude_long_context',
    'price',
    'plugin',
])

const MANAGED_PLUGIN_KEYS = {
    cache: new Set([
        'enable',
        'ttl',
        'item_max_size',
        'add_cache_hit_header',
        'cache_hit_header',
    ]),
    'web-search': new Set([
        'enable',
        'force_search',
        'max_results',
        'need_reference',
        'reference_location',
        'reference_format',
        'default_language',
        'prompt_template',
        'search_from',
    ]),
    'think-split': new Set(['enable']),
    'stream-fake': new Set(['enable']),
} as const

const KNOWN_CONFIG_KEYS = new Set([
    'max_input_tokens',
    'max_output_tokens',
    'max_context_tokens',
    'vision',
    'tool_choice',
    'coder',
    'limited_time_free',
    'support_formats',
    'support_voices',
])

interface ModelFormProps {
    mode?: 'create' | 'update'
    onSuccess?: () => void
    baseModelConfig?: ModelConfig | null
    defaultValues?: {
        model: string
        config?: ModelConfig['config']
        owner?: string
        type: number
        exclude_from_tests?: boolean
        rpm?: number
        tpm?: number
        image_quality_prices?: ModelConfig['image_quality_prices']
        image_prices?: ModelConfig['image_prices']
        retry_times?: number
        timeout_config?: ModelConfig['timeout_config']
        warn_error_rate?: number
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
    baseModelConfig = null,
    defaultValues = {
        model: '',
        owner: '',
        type: 1,
    },
}: ModelFormProps) {
    const { t } = useTranslation()
    const [configExpanded, setConfigExpanded] = useState(false)

    // Collapsible expanded states
    const [priceExpanded, setPriceExpanded] = useState(false)
    const [cachePluginExpanded, setCachePluginExpanded] = useState(false)
    const [webSearchPluginExpanded, setWebSearchPluginExpanded] = useState(false)
    const [configExtrasText, setConfigExtrasText] = useState(() => {
        const extras = Object.fromEntries(
            Object.entries(defaultValues.config || {}).filter(([key]) => !KNOWN_CONFIG_KEYS.has(key))
        )
        return Object.keys(extras).length > 0 ? JSON.stringify(extras, null, 2) : ''
    })
    const [configExtrasError, setConfigExtrasError] = useState<string | null>(null)

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
            config: {
                max_input_tokens: defaultValues.config?.max_input_tokens,
                max_output_tokens: defaultValues.config?.max_output_tokens,
                max_context_tokens: defaultValues.config?.max_context_tokens,
                vision: defaultValues.config?.vision ?? false,
                tool_choice: defaultValues.config?.tool_choice ?? false,
                coder: defaultValues.config?.coder ?? false,
                limited_time_free: defaultValues.config?.limited_time_free ?? false,
                support_formats: defaultValues.config?.support_formats,
                support_voices: defaultValues.config?.support_voices,
            },
            owner: defaultValues.owner ?? '',
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
    const watchedType = form.watch('type')
    const cacheEnabled = form.watch('plugin.cache.enable')
    const webSearchEnabled = form.watch('plugin.web-search.enable')
    const searchEngines = form.watch('plugin.web-search.search_from') || []

    const supportFormatsValue = form.watch('config.support_formats')
    const supportVoicesValue = form.watch('config.support_voices')

    const configFieldVisibility = (() => {
        switch (watchedType) {
            case 7:
                return {
                    tokenFields: ['max_input_tokens'] as Array<'max_input_tokens' | 'max_output_tokens' | 'max_context_tokens'>,
                    showToolChoice: false,
                    showVision: false,
                    showCoder: false,
                    showLimitedTimeFree: true,
                    showSupportFormats: true,
                    showSupportVoices: true,
                }
            case 8:
                return {
                    tokenFields: ['max_input_tokens'] as Array<'max_input_tokens' | 'max_output_tokens' | 'max_context_tokens'>,
                    showToolChoice: false,
                    showVision: false,
                    showCoder: false,
                    showLimitedTimeFree: true,
                    showSupportFormats: true,
                    showSupportVoices: false,
                }
            case 3:
            case 10:
            case 11:
                return {
                    tokenFields: ['max_input_tokens', 'max_context_tokens'] as Array<'max_input_tokens' | 'max_output_tokens' | 'max_context_tokens'>,
                    showToolChoice: false,
                    showVision: watchedType === 3,
                    showCoder: false,
                    showLimitedTimeFree: true,
                    showSupportFormats: false,
                    showSupportVoices: false,
                }
            case 5:
            case 9:
            case 13:
                return {
                    tokenFields: ['max_input_tokens', 'max_output_tokens', 'max_context_tokens'] as Array<'max_input_tokens' | 'max_output_tokens' | 'max_context_tokens'>,
                    showToolChoice: false,
                    showVision: true,
                    showCoder: false,
                    showLimitedTimeFree: true,
                    showSupportFormats: true,
                    showSupportVoices: false,
                }
            case 1:
            case 2:
            case 4:
            case 6:
            default:
                return {
                    tokenFields: ['max_input_tokens', 'max_output_tokens', 'max_context_tokens'] as Array<'max_input_tokens' | 'max_output_tokens' | 'max_context_tokens'>,
                    showToolChoice: true,
                    showVision: true,
                    showCoder: true,
                    showLimitedTimeFree: true,
                    showSupportFormats: false,
                    showSupportVoices: false,
                }
        }
    })()

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
        setConfigExtrasError(null)

        let configExtras: Record<string, unknown> = {}
        const rawConfigExtras = configExtrasText.trim()
        if (rawConfigExtras) {
            try {
                const parsed = JSON.parse(rawConfigExtras) as unknown
                if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
                    setConfigExtrasError(t('model.dialog.config.extrasJsonObjectError'))
                    return
                }
                configExtras = parsed as Record<string, unknown>
            } catch {
                setConfigExtrasError(t('model.dialog.config.extrasJsonInvalid'))
                return
            }
        }

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

        const baseConfig = baseModelConfig
            ? (({ created_at: _c, updated_at: _u, model: _m, ...rest }) => rest)(baseModelConfig)
            : null

        const preservedTopLevelFields = Object.fromEntries(
            Object.entries((baseConfig || {}) as Record<string, unknown>).filter(([key]) => !MANAGED_MODEL_KEYS.has(key))
        )

        const basePriceUnknown = Object.fromEntries(
            Object.entries(baseModelConfig?.price || {}).filter(([key]) => !KNOWN_PRICE_KEYS.has(key))
        )

        const mergedPrice = (() => {
            if (!priceData && Object.keys(basePriceUnknown).length === 0) {
                return undefined
            }

            return {
                ...basePriceUnknown,
                ...(priceData || {}),
            } as ModelPrice
        })()

        const existingPlugin = (baseModelConfig?.plugin || {}) as Record<string, unknown>
        const mergedPlugin: Record<string, unknown> = Object.fromEntries(
            Object.entries(existingPlugin).filter(([key]) => !['cache', 'web-search', 'think-split', 'stream-fake'].includes(key))
        )

        if (data.plugin?.cache?.enable) {
            const existingCachePlugin = (baseModelConfig?.plugin?.cache || {}) as Record<string, unknown>
            const preservedCachePluginFields = Object.fromEntries(
                Object.entries(existingCachePlugin).filter(([key]) => !MANAGED_PLUGIN_KEYS.cache.has(key))
            )
            mergedPlugin.cache = {
                ...preservedCachePluginFields,
                enable: true,
                ...(data.plugin.cache.ttl !== undefined && { ttl: data.plugin.cache.ttl }),
                ...(data.plugin.cache.item_max_size !== undefined && { item_max_size: data.plugin.cache.item_max_size }),
                ...(data.plugin.cache.add_cache_hit_header !== undefined && { add_cache_hit_header: data.plugin.cache.add_cache_hit_header }),
                ...(data.plugin.cache.cache_hit_header !== undefined && { cache_hit_header: data.plugin.cache.cache_hit_header }),
            }
        }

        if (data.plugin?.['web-search']?.enable && data.plugin['web-search'].search_from && data.plugin['web-search'].search_from.length > 0) {
            const existingWebSearchPlugin = (baseModelConfig?.plugin?.['web-search'] || {}) as Record<string, unknown>
            const preservedWebSearchPluginFields = Object.fromEntries(
                Object.entries(existingWebSearchPlugin).filter(([key]) => !MANAGED_PLUGIN_KEYS['web-search'].has(key))
            )
            const cleanedSearchFrom = data.plugin['web-search'].search_from.map(engine => ({
                type: engine.type,
                ...(engine.max_results !== undefined && { max_results: engine.max_results }),
                ...(engine.spec && Object.keys(engine.spec).some(key => (engine.spec as Record<string, unknown>)[key] !== undefined && (engine.spec as Record<string, unknown>)[key] !== '') && { spec: engine.spec })
            }))

            mergedPlugin['web-search'] = {
                ...preservedWebSearchPluginFields,
                enable: true,
                search_from: cleanedSearchFrom,
                ...(data.plugin['web-search'].force_search !== undefined && { force_search: data.plugin['web-search'].force_search }),
                ...(data.plugin['web-search'].max_results !== undefined && { max_results: data.plugin['web-search'].max_results }),
                ...(data.plugin['web-search'].need_reference !== undefined && { need_reference: data.plugin['web-search'].need_reference }),
                ...(data.plugin['web-search'].reference_location !== undefined && { reference_location: data.plugin['web-search'].reference_location }),
                ...(data.plugin['web-search'].reference_format !== undefined && { reference_format: data.plugin['web-search'].reference_format }),
                ...(data.plugin['web-search'].default_language !== undefined && { default_language: data.plugin['web-search'].default_language }),
                ...(data.plugin['web-search'].prompt_template !== undefined && { prompt_template: data.plugin['web-search'].prompt_template }),
            }
        }

        if (data.plugin?.['think-split']?.enable) {
            const existingThinkSplitPlugin = (baseModelConfig?.plugin?.['think-split'] || {}) as Record<string, unknown>
            const preservedThinkSplitPluginFields = Object.fromEntries(
                Object.entries(existingThinkSplitPlugin).filter(([key]) => !MANAGED_PLUGIN_KEYS['think-split'].has(key))
            )
            mergedPlugin['think-split'] = {
                ...preservedThinkSplitPluginFields,
                enable: true,
            }
        }

        if (data.plugin?.['stream-fake']?.enable) {
            const existingStreamFakePlugin = (baseModelConfig?.plugin?.['stream-fake'] || {}) as Record<string, unknown>
            const preservedStreamFakePluginFields = Object.fromEntries(
                Object.entries(existingStreamFakePlugin).filter(([key]) => !MANAGED_PLUGIN_KEYS['stream-fake'].has(key))
            )
            mergedPlugin['stream-fake'] = {
                ...preservedStreamFakePluginFields,
                enable: true,
            }
        }

        const preservedTimeoutConfigFields = Object.fromEntries(
            Object.entries((baseConfig?.timeout_config || {}) as Record<string, unknown>).filter(([key]) => key !== 'request_timeout')
        )
        const mergedTimeoutConfig = (() => {
            if (data.timeout !== undefined) {
                return {
                    ...preservedTimeoutConfigFields,
                    request_timeout: Number(data.timeout),
                }
            }
            if (Object.keys(preservedTimeoutConfigFields).length > 0) {
                return preservedTimeoutConfigFields
            }
            return undefined
        })()

        const cleanedConfig = (() => {
            const config = data.config || {}
            const nextConfig: Record<string, unknown> = {
                ...configExtras,
            }

            for (const [key, value] of Object.entries(config)) {
                if (Array.isArray(value)) {
                    const trimmed = value.map((item) => String(item).trim()).filter(Boolean)
                    if (trimmed.length > 0) {
                        nextConfig[key] = trimmed
                    }
                    continue
                }

                if (typeof value === 'boolean') {
                    if (value) {
                        nextConfig[key] = value
                    }
                    continue
                }

                if (typeof value === 'string') {
                    if (value !== '') {
                        nextConfig[key] = value
                    }
                    continue
                }

                if (value !== undefined && value !== null) {
                    nextConfig[key] = value
                }
            }

            return Object.keys(nextConfig).length > 0 ? nextConfig as ModelConfigDetail : undefined
        })()

        // Prepare data for API - 如果没有启用的插件，则不传递 plugin 字段
        const formData: Omit<ModelCreateRequest, 'model'> = {
            ...preservedTopLevelFields,
            ...(cleanedConfig && { config: cleanedConfig }),
            owner: data.owner ?? '',
            type: Number(data.type),
            ...(data.rpm !== undefined && { rpm: Number(data.rpm) }),
            ...(data.tpm !== undefined && { tpm: Number(data.tpm) }),
            ...(data.retry_times !== undefined && { retry_times: Number(data.retry_times) }),
            ...(mergedTimeoutConfig && { timeout_config: mergedTimeoutConfig }),
            ...(data.max_error_rate !== undefined && { max_error_rate: Number(data.max_error_rate) }),
            ...(data.force_save_detail !== undefined && { force_save_detail: data.force_save_detail }),
            ...(data.summary_service_tier !== undefined && { summary_service_tier: data.summary_service_tier }),
            ...(data.summary_claude_long_context !== undefined && { summary_claude_long_context: data.summary_claude_long_context }),
            ...(mergedPrice && { price: mergedPrice }),
            ...(Object.keys(mergedPlugin).length > 0 && { plugin: mergedPlugin as Plugin })
        }

        if (mode === 'create') {
            // For create mode, include the model name
            createModel({
                model: data.model,
                ...(cleanedConfig && { config: cleanedConfig }),
                owner: data.owner ?? '',
                type: Number(data.type),
                ...(data.rpm !== undefined && { rpm: Number(data.rpm) }),
                ...(data.tpm !== undefined && { tpm: Number(data.tpm) }),
                ...(data.retry_times !== undefined && { retry_times: Number(data.retry_times) }),
                ...(data.timeout !== undefined && { timeout_config: { request_timeout: Number(data.timeout) } }),
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

                    {configExtrasError && (
                        <AdvancedErrorDisplay error={new Error(configExtrasError)} />
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

                    <FormField
                        control={form.control}
                        name="owner"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("model.owner")}</FormLabel>
                                <FormControl>
                                    <Input
                                        placeholder={t("model.dialog.ownerPlaceholder")}
                                        {...field}
                                        value={field.value ?? ''}
                                    />
                                </FormControl>
                                <FormDescription>{t("model.dialog.ownerDescription")}</FormDescription>
                                <FormMessage />
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

                    <Collapsible open={configExpanded} onOpenChange={setConfigExpanded}>
                        <CollapsibleTrigger className="flex items-center justify-between w-full py-3 px-4 border rounded-lg hover:bg-muted/50 transition-colors">
                            <div className="text-left">
                                <h3 className="text-sm font-medium">{t("model.dialog.config.title")}</h3>
                                <p className="text-xs text-muted-foreground">{t("model.dialog.config.description")}</p>
                            </div>
                            {configExpanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                        </CollapsibleTrigger>
                        <CollapsibleContent className="pt-3 px-1 space-y-4">
                            <div className="rounded-lg border border-amber-200 bg-amber-50/70 p-3 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
                                {t("model.dialog.config.note")}
                            </div>

                            <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
                                {configFieldVisibility.tokenFields.includes('max_context_tokens') && (
                                    <FormField
                                        control={form.control}
                                        name="config.max_context_tokens"
                                        render={({ field }) => (
                                            <FormItem>
                                                <FormLabel>{t("model.dialog.config.maxContextTokens")}</FormLabel>
                                                <FormControl>
                                                    <Input
                                                        type="number"
                                                        placeholder={t("model.dialog.config.maxContextTokensPlaceholder")}
                                                        {...field}
                                                        value={field.value ?? ''}
                                                        onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                                    />
                                                </FormControl>
                                                <FormMessage />
                                            </FormItem>
                                        )}
                                    />
                                )}
                                {configFieldVisibility.tokenFields.includes('max_input_tokens') && (
                                    <FormField
                                        control={form.control}
                                        name="config.max_input_tokens"
                                        render={({ field }) => (
                                            <FormItem>
                                                <FormLabel>{t("model.dialog.config.maxInputTokens")}</FormLabel>
                                                <FormControl>
                                                    <Input
                                                        type="number"
                                                        placeholder={t("model.dialog.config.maxInputTokensPlaceholder")}
                                                        {...field}
                                                        value={field.value ?? ''}
                                                        onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                                    />
                                                </FormControl>
                                                <FormMessage />
                                            </FormItem>
                                        )}
                                    />
                                )}
                                {configFieldVisibility.tokenFields.includes('max_output_tokens') && (
                                    <FormField
                                        control={form.control}
                                        name="config.max_output_tokens"
                                        render={({ field }) => (
                                            <FormItem>
                                                <FormLabel>{t("model.dialog.config.maxOutputTokens")}</FormLabel>
                                                <FormControl>
                                                    <Input
                                                        type="number"
                                                        placeholder={t("model.dialog.config.maxOutputTokensPlaceholder")}
                                                        {...field}
                                                        value={field.value ?? ''}
                                                        onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                                    />
                                                </FormControl>
                                                <FormMessage />
                                            </FormItem>
                                        )}
                                    />
                                )}
                            </div>

                            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                                {configFieldVisibility.showToolChoice && (
                                    <FormField
                                        control={form.control}
                                        name="config.tool_choice"
                                        render={({ field }) => (
                                            <FormItem className="flex flex-row items-center justify-between rounded-lg border p-3">
                                                <div className="space-y-1">
                                                    <FormLabel>{t("model.dialog.config.toolChoice")}</FormLabel>
                                                    <FormDescription>{t("model.dialog.config.toolChoiceDescription")}</FormDescription>
                                                </div>
                                                <FormControl>
                                                    <Switch
                                                        checked={field.value ?? false}
                                                        onCheckedChange={field.onChange}
                                                    />
                                                </FormControl>
                                            </FormItem>
                                        )}
                                    />
                                )}
                                {configFieldVisibility.showVision && (
                                    <FormField
                                        control={form.control}
                                        name="config.vision"
                                        render={({ field }) => (
                                            <FormItem className="flex flex-row items-center justify-between rounded-lg border p-3">
                                                <div className="space-y-1">
                                                    <FormLabel>{t("model.dialog.config.vision")}</FormLabel>
                                                    <FormDescription>{t("model.dialog.config.visionDescription")}</FormDescription>
                                                </div>
                                                <FormControl>
                                                    <Switch
                                                        checked={field.value ?? false}
                                                        onCheckedChange={field.onChange}
                                                    />
                                                </FormControl>
                                            </FormItem>
                                        )}
                                    />
                                )}
                                {configFieldVisibility.showCoder && (
                                    <FormField
                                        control={form.control}
                                        name="config.coder"
                                        render={({ field }) => (
                                            <FormItem className="flex flex-row items-center justify-between rounded-lg border p-3">
                                                <div className="space-y-1">
                                                    <FormLabel>{t("model.dialog.config.coder")}</FormLabel>
                                                    <FormDescription>{t("model.dialog.config.coderDescription")}</FormDescription>
                                                </div>
                                                <FormControl>
                                                    <Switch
                                                        checked={field.value ?? false}
                                                        onCheckedChange={field.onChange}
                                                    />
                                                </FormControl>
                                            </FormItem>
                                        )}
                                    />
                                )}
                                {configFieldVisibility.showLimitedTimeFree && (
                                    <FormField
                                        control={form.control}
                                        name="config.limited_time_free"
                                        render={({ field }) => (
                                            <FormItem className="flex flex-row items-center justify-between rounded-lg border p-3">
                                                <div className="space-y-1">
                                                    <FormLabel>{t("model.dialog.config.limitedTimeFree")}</FormLabel>
                                                    <FormDescription>{t("model.dialog.config.limitedTimeFreeDescription")}</FormDescription>
                                                </div>
                                                <FormControl>
                                                    <Switch
                                                        checked={field.value ?? false}
                                                        onCheckedChange={field.onChange}
                                                    />
                                                </FormControl>
                                            </FormItem>
                                        )}
                                    />
                                )}
                            </div>

                            {configFieldVisibility.showSupportFormats && (
                                <FormField
                                    control={form.control}
                                    name="config.support_formats"
                                    render={() => (
                                        <FormItem>
                                            <FormLabel>{t("model.dialog.config.supportFormats")}</FormLabel>
                                            <FormControl>
                                                <Textarea
                                                    placeholder={t("model.dialog.config.supportFormatsPlaceholder")}
                                                    value={(supportFormatsValue || []).join('\n')}
                                                    onChange={(e) => {
                                                        const values = e.target.value
                                                            .split('\n')
                                                            .map((item) => item.trim())
                                                            .filter(Boolean)
                                                        form.setValue('config.support_formats', values.length > 0 ? values : undefined, { shouldDirty: true })
                                                    }}
                                                    className="min-h-[96px]"
                                                />
                                            </FormControl>
                                            <FormDescription>{t("model.dialog.config.supportFormatsDescription")}</FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    )}
                                />
                            )}

                            {configFieldVisibility.showSupportVoices && (
                                <FormField
                                    control={form.control}
                                    name="config.support_voices"
                                    render={() => (
                                        <FormItem>
                                            <FormLabel>{t("model.dialog.config.supportVoices")}</FormLabel>
                                            <FormControl>
                                                <Textarea
                                                    placeholder={t("model.dialog.config.supportVoicesPlaceholder")}
                                                    value={(supportVoicesValue || []).join('\n')}
                                                    onChange={(e) => {
                                                        const values = e.target.value
                                                            .split('\n')
                                                            .map((item) => item.trim())
                                                            .filter(Boolean)
                                                        form.setValue('config.support_voices', values.length > 0 ? values : undefined, { shouldDirty: true })
                                                    }}
                                                    className="min-h-[140px]"
                                                />
                                            </FormControl>
                                            <FormDescription>{t("model.dialog.config.supportVoicesDescription")}</FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    )}
                                />
                            )}

                            <div className="space-y-2">
                                <Label>{t("model.dialog.config.extrasJson")}</Label>
                                <Textarea
                                    placeholder={t("model.dialog.config.extrasJsonPlaceholder")}
                                    value={configExtrasText}
                                    onChange={(e) => setConfigExtrasText(e.target.value)}
                                    className="min-h-[140px] font-mono text-xs"
                                />
                                <p className="text-xs text-muted-foreground">{t("model.dialog.config.extrasJsonDescription")}</p>
                            </div>
                        </CollapsibleContent>
                    </Collapsible>

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
