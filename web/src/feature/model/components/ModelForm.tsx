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
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from '@/components/ui/alert-dialog'
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
import {
    Plugin,
    EngineConfig,
    ModelPrice,
    ModelCreateRequest,
    ModelConfig,
    ModelConfigDetail,
    MODEL_TYPE_OPTIONS,
    STREAM_TIMEOUT_SUPPORTED_MODEL_TYPES,
    IMAGE_GENERATION_COUNT_LIMIT_SUPPORTED_MODEL_TYPES,
    VIDEO_GENERATION_SECONDS_LIMIT_SUPPORTED_MODEL_TYPES,
    VIDEO_GENERATION_COUNT_LIMIT_SUPPORTED_MODEL_TYPES,
    RESOLUTION_FUZZY_MATCH_SUPPORTED_MODEL_TYPES,
} from '@/types/model'
import { AdvancedErrorDisplay } from '@/components/common/error/errorDisplay'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'
import { useState } from 'react'
import { ENV } from '@/utils/env'
import { PriceFormFields } from '@/components/price/PriceFormFields'
import { ValidationErrorDisplay } from '@/components/common/error/validationErrorDisplay'

type PendingUpdate = {
    model: string
    data: Omit<ModelCreateRequest, 'model'>
    changes: ChangeSummary[]
}

type ChangeSummary = {
    field: string
    before: string
    after: string
}

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
    'video_input_price',
    'video_input_price_unit',
    'audio_output_price',
    'audio_output_price_unit',
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
    'exclude_from_tests',
    'rpm',
    'tpm',
    'retry_times',
    'timeout_config',
    'force_save_detail',
    'max_image_generation_count',
    'max_video_generation_seconds',
    'max_video_generation_count',
    'allowed_resolutions',
    'request_body_storage_max_size',
    'response_body_storage_max_size',
    'summary_service_tier',
    'summary_claude_long_context',
    'disable_resolution_fuzzy_match',
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
    cachefollow: new Set([
        'enable',
        'enable_generic_follow',
        'followed_channel_ttl_seconds',
        'recent_channel_update_debounce_seconds',
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

const DROPPED_CACHEFOLLOW_PLUGIN_KEYS = new Set([
    'last_store_update_window',
])

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

const STREAM_TIMEOUT_SUPPORTED_TYPES = new Set<number>(STREAM_TIMEOUT_SUPPORTED_MODEL_TYPES)
const IMAGE_GENERATION_COUNT_LIMIT_SUPPORTED_TYPES = new Set<number>(IMAGE_GENERATION_COUNT_LIMIT_SUPPORTED_MODEL_TYPES)
const VIDEO_GENERATION_SECONDS_LIMIT_SUPPORTED_TYPES = new Set<number>(VIDEO_GENERATION_SECONDS_LIMIT_SUPPORTED_MODEL_TYPES)
const VIDEO_GENERATION_COUNT_LIMIT_SUPPORTED_TYPES = new Set<number>(VIDEO_GENERATION_COUNT_LIMIT_SUPPORTED_MODEL_TYPES)
const RESOLUTION_FUZZY_MATCH_SUPPORTED_TYPES = new Set<number>(RESOLUTION_FUZZY_MATCH_SUPPORTED_MODEL_TYPES)

const omitKeys = (obj: object, keys: string[]) => {
    const omitted = new Set(keys)
    return Object.fromEntries(Object.entries(obj).filter(([key]) => !omitted.has(key)))
}

const normalizeForCompare = (value: unknown): unknown => {
    if (value === undefined || value === null || value === '' || value === false) return null
    if (Array.isArray(value)) {
        const normalizedItems: unknown[] = value.map(normalizeForCompare).filter((item) => item !== null)
        return normalizedItems.length > 0 ? normalizedItems : null
    }
    if (typeof value === 'object') {
        const normalizedEntries: Array<readonly [string, unknown]> = Object.entries(value as Record<string, unknown>)
            .map(([key, item]) => [key, normalizeForCompare(item)] as const)
            .filter(([, item]) => item !== null)
            .sort(([left], [right]) => left.localeCompare(right))

        return normalizedEntries.length > 0 ? Object.fromEntries(normalizedEntries) : null
    }
    return value
}

const isSameValue = (before: unknown, after: unknown) =>
    JSON.stringify(normalizeForCompare(before)) === JSON.stringify(normalizeForCompare(after))

const formatScalar = (value: unknown, labels: Record<string, string>) => {
    if (value === undefined || value === null || value === '') return labels.unset
    if (typeof value === 'boolean') return value ? labels.yes : labels.no
    if (Array.isArray(value)) return value.length > 0 ? value.join(', ') : labels.none
    return String(value)
}

const summarizeStructuredChange = (
    before: unknown,
    after: unknown,
    labels: Record<string, string>
) => {
    if (isSameValue(before, after)) return null
    const normalizedBefore = normalizeForCompare(before)
    const normalizedAfter = normalizeForCompare(after)
    return {
        before: normalizedBefore == null ? labels.unset : labels.configured,
        after: normalizedAfter == null ? labels.unset : labels.changed,
    }
}

const pickManagedFields = (
    source: Record<string, unknown> | undefined,
    keys: Set<string>
) => {
    if (!source) return undefined

    const picked = Object.fromEntries(
        Object.entries(source).filter(([key]) => keys.has(key))
    )

    return Object.keys(picked).length > 0 ? picked : undefined
}

const normalizePluginForCompare = (plugin: unknown) => {
    const source = plugin as Partial<Record<keyof typeof MANAGED_PLUGIN_KEYS, Record<string, unknown>>> | undefined
    if (!source) return undefined

    return {
        cache: pickManagedFields(source.cache, MANAGED_PLUGIN_KEYS.cache),
        cachefollow: pickManagedFields(source.cachefollow, MANAGED_PLUGIN_KEYS.cachefollow),
        'web-search': pickManagedFields(source['web-search'], MANAGED_PLUGIN_KEYS['web-search']),
        'think-split': pickManagedFields(source['think-split'], MANAGED_PLUGIN_KEYS['think-split']),
        'stream-fake': pickManagedFields(source['stream-fake'], MANAGED_PLUGIN_KEYS['stream-fake']),
    }
}

const buildChangeSummaries = (
    original: ModelConfig | null | undefined,
    next: Omit<ModelCreateRequest, 'model'>,
    labels: Record<string, string>,
    options: { includePlugin?: boolean } = {}
): ChangeSummary[] => {
    if (!original) return []

    const changes: ChangeSummary[] = []
    const addScalar = (key: keyof ModelConfig, label: string, afterValue: unknown) => {
        const beforeValue = original[key]
        if (!isSameValue(beforeValue, afterValue)) {
            changes.push({
                field: label,
                before: formatScalar(beforeValue, labels),
                after: formatScalar(afterValue, labels),
            })
        }
    }

    addScalar('owner', labels.owner, next.owner)
    addScalar('type', labels.type, next.type)
    addScalar('exclude_from_tests', labels.excludeFromTests, next.exclude_from_tests)
    addScalar('rpm', labels.rpm, next.rpm)
    addScalar('tpm', labels.tpm, next.tpm)
    addScalar('retry_times', labels.retryTimes, next.retry_times)
    addScalar('force_save_detail', labels.forceSaveDetail, next.force_save_detail)
    addScalar('summary_service_tier', labels.summaryServiceTier, next.summary_service_tier)
    addScalar('summary_claude_long_context', labels.summaryClaudeLongContext, next.summary_claude_long_context)
    addScalar('disable_resolution_fuzzy_match', labels.disableResolutionFuzzyMatch, next.disable_resolution_fuzzy_match)
    addScalar('allowed_resolutions', labels.allowedResolutions, next.allowed_resolutions)
    addScalar('request_body_storage_max_size', labels.requestBodyStorageMaxSize, next.request_body_storage_max_size)
    addScalar('response_body_storage_max_size', labels.responseBodyStorageMaxSize, next.response_body_storage_max_size)
    addScalar('max_image_generation_count', labels.maxImageGenerationCount, next.max_image_generation_count)
    addScalar('max_video_generation_seconds', labels.maxVideoGenerationSeconds, next.max_video_generation_seconds)
    addScalar('max_video_generation_count', labels.maxVideoGenerationCount, next.max_video_generation_count)

    const timeoutChange = summarizeStructuredChange(
        original.timeout_config,
        next.timeout_config,
        labels
    )
    if (timeoutChange) {
        changes.push({ field: labels.timeoutConfig, ...timeoutChange })
    }

    const configChange = summarizeStructuredChange(original.config, next.config, labels)
    if (configChange) {
        changes.push({ field: labels.displayConfig, ...configChange })
    }

    const priceChange = summarizeStructuredChange(original.price, next.price, labels)
    if (priceChange) {
        changes.push({ field: labels.priceConfig, ...priceChange })
    }

    if (options.includePlugin) {
        const pluginChange = summarizeStructuredChange(
            normalizePluginForCompare(original.plugin),
            normalizePluginForCompare(next.plugin),
            labels
        )
        if (pluginChange) {
            changes.push({ field: labels.pluginConfig, ...pluginChange })
        }
    }

    return changes
}

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
        retry_times?: number
        timeout_config?: ModelConfig['timeout_config']
        timeout?: number
        stream_timeout?: number
        force_save_detail?: boolean
        max_image_generation_count?: number
        max_video_generation_seconds?: number
        max_video_generation_count?: number
        allowed_resolutions?: string[]
        request_body_storage_max_size?: number
        response_body_storage_max_size?: number
        summary_service_tier?: boolean
        summary_claude_long_context?: boolean
        disable_resolution_fuzzy_match?: boolean
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
    const [pendingUpdate, setPendingUpdate] = useState<PendingUpdate | null>(null)
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
            exclude_from_tests: defaultValues.exclude_from_tests ?? false,
            rpm: defaultValues.rpm,
            tpm: defaultValues.tpm,
            retry_times: defaultValues.retry_times,
            timeout: defaultValues.timeout,
            stream_timeout: defaultValues.stream_timeout ?? defaultValues.timeout_config?.stream_request_timeout,
            force_save_detail: defaultValues.force_save_detail ?? false,
            max_image_generation_count: defaultValues.max_image_generation_count,
            max_video_generation_seconds: defaultValues.max_video_generation_seconds,
            max_video_generation_count: defaultValues.max_video_generation_count,
            allowed_resolutions: defaultValues.allowed_resolutions,
            request_body_storage_max_size: defaultValues.request_body_storage_max_size,
            response_body_storage_max_size: defaultValues.response_body_storage_max_size,
            summary_service_tier: defaultValues.summary_service_tier ?? false,
            summary_claude_long_context: defaultValues.summary_claude_long_context ?? false,
            disable_resolution_fuzzy_match: defaultValues.disable_resolution_fuzzy_match ?? false,
            price: defaultValues.price || {},
            plugin: {
                cache: { enable: false, ...defaultValues.plugin?.cache },
                cachefollow: { enable: false, ...defaultValues.plugin?.cachefollow },
                "web-search": { enable: false, search_from: [], ...defaultValues.plugin?.["web-search"] },
                "think-split": { enable: false, ...defaultValues.plugin?.["think-split"] },
                "stream-fake": { enable: false, ...defaultValues.plugin?.["stream-fake"] },
            }
        },
    })

    // Watch plugin enable states
    const watchedType = form.watch('type')
    const cacheEnabled = form.watch('plugin.cache.enable')
    const cacheFollowEnabled = form.watch('plugin.cachefollow.enable')
    const webSearchEnabled = form.watch('plugin.web-search.enable')
    const searchEngines = form.watch('plugin.web-search.search_from') || []

    const supportFormatsValue = form.watch('config.support_formats')
    const supportVoicesValue = form.watch('config.support_voices')
    const allowedResolutionsValue = form.watch('allowed_resolutions')
    const supportStreamTimeout = STREAM_TIMEOUT_SUPPORTED_TYPES.has(watchedType)
    const supportImageGenerationCountLimit = IMAGE_GENERATION_COUNT_LIMIT_SUPPORTED_TYPES.has(watchedType)
    const supportVideoGenerationSecondsLimit = VIDEO_GENERATION_SECONDS_LIMIT_SUPPORTED_TYPES.has(watchedType)
    const supportVideoGenerationCountLimit = VIDEO_GENERATION_COUNT_LIMIT_SUPPORTED_TYPES.has(watchedType)
    const supportResolutionFuzzyMatchConfig = RESOLUTION_FUZZY_MATCH_SUPPORTED_TYPES.has(watchedType)
    const changeSummaryLabels = {
        owner: t("model.owner"),
        type: t("model.dialog.modelType"),
        excludeFromTests: t("model.dialog.excludeFromTests"),
        rpm: t("model.dialog.rpm"),
        tpm: t("model.dialog.tpm"),
        retryTimes: t("model.dialog.retryTimes"),
        forceSaveDetail: t("model.dialog.forceSaveDetail"),
        summaryServiceTier: t("model.dialog.recordServiceTier"),
        summaryClaudeLongContext: t("model.dialog.recordClaudeLongContext"),
        disableResolutionFuzzyMatch: t("model.dialog.disableResolutionFuzzyMatch"),
        allowedResolutions: t("model.dialog.config.allowedResolutions"),
        requestBodyStorageMaxSize: t("model.dialog.requestBodyStorageMaxSize"),
        responseBodyStorageMaxSize: t("model.dialog.responseBodyStorageMaxSize"),
        maxImageGenerationCount: t("model.dialog.maxImageGenerationCount"),
        maxVideoGenerationSeconds: t("model.dialog.maxVideoGenerationSeconds"),
        maxVideoGenerationCount: t("model.dialog.maxVideoGenerationCount"),
        timeoutConfig: t("model.dialog.timeoutConfig"),
        displayConfig: t("model.dialog.config.title"),
        priceConfig: t("group.price.title"),
        pluginConfig: t("model.dialog.pluginConfiguration"),
        unset: t("model.dialog.changeSummary.unset"),
        none: t("model.dialog.changeSummary.none"),
        yes: t("common.yes"),
        no: t("common.no"),
        configured: t("model.dialog.changeSummary.configured"),
        changed: t("model.dialog.changeSummary.changed"),
    }

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
                    showResolutions: false,
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
                    showResolutions: false,
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
                    showResolutions: false,
                }
            case 5:
            case 6:
            case 30:
                return {
                    tokenFields: ['max_input_tokens', 'max_output_tokens', 'max_context_tokens'] as Array<'max_input_tokens' | 'max_output_tokens' | 'max_context_tokens'>,
                    showToolChoice: false,
                    showVision: true,
                    showCoder: false,
                    showLimitedTimeFree: true,
                    showSupportFormats: true,
                    showSupportVoices: false,
                    showResolutions: true,
                }
            case 13:
            case 22:
            case 26:
            case 27:
            case 32:
            case 33:
            case 34:
            case 36:
                return {
                    tokenFields: ['max_input_tokens', 'max_output_tokens', 'max_context_tokens'] as Array<'max_input_tokens' | 'max_output_tokens' | 'max_context_tokens'>,
                    showToolChoice: false,
                    showVision: true,
                    showCoder: false,
                    showLimitedTimeFree: true,
                    showSupportFormats: false,
                    showSupportVoices: false,
                    showResolutions: true,
                }
            case 21:
                return {
                    tokenFields: ['max_input_tokens', 'max_output_tokens', 'max_context_tokens'] as Array<'max_input_tokens' | 'max_output_tokens' | 'max_context_tokens'>,
                    showToolChoice: false,
                    showVision: true,
                    showCoder: false,
                    showLimitedTimeFree: true,
                    showSupportFormats: false,
                    showSupportVoices: false,
                    showResolutions: false,
                }
            case 1:
            case 2:
            case 4:
            case 9:
            default:
                return {
                    tokenFields: ['max_input_tokens', 'max_output_tokens', 'max_context_tokens'] as Array<'max_input_tokens' | 'max_output_tokens' | 'max_context_tokens'>,
                    showToolChoice: true,
                    showVision: true,
                    showCoder: true,
                    showLimitedTimeFree: true,
                    showSupportFormats: false,
                    showSupportVoices: false,
                    showResolutions: false,
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

    const submitUpdate = (model: string, data: Omit<ModelCreateRequest, 'model'>) => {
        updateModel({
            model,
            data,
        }, {
            onSuccess: () => {
                if (onSuccess) onSuccess()
            }
        })
    }

    const confirmPendingUpdate = () => {
        if (!pendingUpdate) return
        submitUpdate(pendingUpdate.model, pendingUpdate.data)
        setPendingUpdate(null)
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

        const baseConfig = baseModelConfig ? omitKeys(baseModelConfig, ['created_at', 'updated_at', 'model']) : null

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
            Object.entries(existingPlugin).filter(([key]) => !['cache', 'cachefollow', 'web-search', 'think-split', 'stream-fake'].includes(key))
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

        if (data.plugin?.cachefollow?.enable) {
            const existingCacheFollowPlugin = (baseModelConfig?.plugin?.cachefollow || {}) as Record<string, unknown>
            const preservedCacheFollowPluginFields = Object.fromEntries(
                Object.entries(existingCacheFollowPlugin).filter(
                    ([key]) => !MANAGED_PLUGIN_KEYS.cachefollow.has(key) && !DROPPED_CACHEFOLLOW_PLUGIN_KEYS.has(key)
                )
            )
            mergedPlugin.cachefollow = {
                ...preservedCacheFollowPluginFields,
                enable: true,
                ...(data.plugin.cachefollow.enable_generic_follow !== undefined && {
                    enable_generic_follow: data.plugin.cachefollow.enable_generic_follow,
                }),
                ...(data.plugin.cachefollow.followed_channel_ttl_seconds !== undefined && {
                    followed_channel_ttl_seconds: data.plugin.cachefollow.followed_channel_ttl_seconds,
                }),
                ...(data.plugin.cachefollow.recent_channel_update_debounce_seconds !== undefined && {
                    recent_channel_update_debounce_seconds: data.plugin.cachefollow.recent_channel_update_debounce_seconds,
                }),
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
            Object.entries((baseConfig?.timeout_config || {}) as Record<string, unknown>).filter(([key]) => (
                key !== 'request_timeout' && key !== 'stream_request_timeout'
            ))
        )
        const mergedTimeoutConfig = (() => {
            if (data.timeout !== undefined || data.stream_timeout !== undefined) {
                return {
                    ...preservedTimeoutConfigFields,
                    ...(data.timeout !== undefined && { request_timeout: Number(data.timeout) }),
                    ...(supportStreamTimeout && data.stream_timeout !== undefined && { stream_request_timeout: Number(data.stream_timeout) }),
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
            ...(data.exclude_from_tests !== undefined && { exclude_from_tests: data.exclude_from_tests }),
            ...(data.rpm !== undefined && { rpm: Number(data.rpm) }),
            ...(data.tpm !== undefined && { tpm: Number(data.tpm) }),
            ...(data.retry_times !== undefined && { retry_times: Number(data.retry_times) }),
            ...(mergedTimeoutConfig && { timeout_config: mergedTimeoutConfig }),
            ...(data.force_save_detail !== undefined && { force_save_detail: data.force_save_detail }),
            ...(supportImageGenerationCountLimit && data.max_image_generation_count !== undefined && {
                max_image_generation_count: Number(data.max_image_generation_count),
            }),
            ...(configFieldVisibility.showResolutions && data.allowed_resolutions !== undefined && {
                allowed_resolutions: data.allowed_resolutions,
            }),
            ...(supportVideoGenerationSecondsLimit && data.max_video_generation_seconds !== undefined && {
                max_video_generation_seconds: Number(data.max_video_generation_seconds),
            }),
            ...(supportVideoGenerationCountLimit && data.max_video_generation_count !== undefined && {
                max_video_generation_count: Number(data.max_video_generation_count),
            }),
            ...(data.request_body_storage_max_size !== undefined && {
                request_body_storage_max_size: Number(data.request_body_storage_max_size),
            }),
            ...(data.response_body_storage_max_size !== undefined && {
                response_body_storage_max_size: Number(data.response_body_storage_max_size),
            }),
            ...(data.summary_service_tier !== undefined && { summary_service_tier: data.summary_service_tier }),
            ...(data.summary_claude_long_context !== undefined && { summary_claude_long_context: data.summary_claude_long_context }),
            ...(supportResolutionFuzzyMatchConfig && data.disable_resolution_fuzzy_match !== undefined && {
                disable_resolution_fuzzy_match: data.disable_resolution_fuzzy_match,
            }),
            ...(mergedPrice && { price: mergedPrice }),
            ...(Object.keys(mergedPlugin).length > 0 && { plugin: mergedPlugin as Plugin })
        }

        if (mode === 'create') {
            // For create mode, include the model name
            createModel({
                model: data.model,
                ...formData,
            }, {
                onSuccess: () => {
                    // Reset form
                    form.reset()
                    // Notify parent component
                    if (onSuccess) onSuccess()
                }
            })
        } else {
            const includePluginChange = !isSameValue(
                normalizePluginForCompare(baseModelConfig?.plugin),
                normalizePluginForCompare(defaultValues.plugin)
            ) || form.formState.dirtyFields.plugin !== undefined
            const changes = buildChangeSummaries(baseModelConfig, formData, changeSummaryLabels, {
                includePlugin: includePluginChange,
            })
            if (changes.length > 0) {
                setPendingUpdate({
                    model: data.model,
                    data: formData,
                    changes,
                })
                return
            }

            submitUpdate(data.model, formData)
        }
    }

    return (
        <div>
            <AlertDialog
                open={pendingUpdate !== null}
                onOpenChange={(open) => {
                    if (!open) {
                        setPendingUpdate(null)
                    }
                }}
            >
                <AlertDialogContent className="sm:max-w-2xl">
                    <AlertDialogHeader>
                        <AlertDialogTitle>{t("model.dialog.confirmUpdateTitle")}</AlertDialogTitle>
                        <AlertDialogDescription>
                            {t("model.dialog.confirmUpdateDescription")}
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <div className="max-h-[50vh] overflow-auto rounded-md border">
                        <div className="grid grid-cols-[9rem_1fr_1fr] border-b bg-muted/40 px-3 py-2 text-xs font-medium text-muted-foreground">
                            <span>{t("model.dialog.changeSummary.field")}</span>
                            <span>{t("model.dialog.changeSummary.before")}</span>
                            <span>{t("model.dialog.changeSummary.after")}</span>
                        </div>
                        <div className="divide-y">
                            {pendingUpdate?.changes.map((change) => (
                                <div
                                    key={change.field}
                                    className="grid grid-cols-[9rem_1fr_1fr] gap-2 px-3 py-2 text-sm"
                                >
                                    <span className="font-medium">{change.field}</span>
                                    <span className="text-muted-foreground">{change.before}</span>
                                    <span>{change.after}</span>
                                </div>
                            ))}
                        </div>
                    </div>
                    <AlertDialogFooter>
                        <AlertDialogCancel disabled={isLoading}>
                            {t("common.cancel")}
                        </AlertDialogCancel>
                        <AlertDialogAction disabled={isLoading} onClick={confirmPendingUpdate}>
                            {isLoading ? t("model.dialog.submitting") : t("model.dialog.confirmUpdate")}
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>

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

                    <section className="form-section"><h3>{t("ui.identity")}</h3><div className="grid gap-4 sm:grid-cols-2">
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
                                        {MODEL_TYPE_OPTIONS.map((type) => (
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

                    </div></section><section className="form-section"><h3>{t("ui.connection")}</h3><div className="grid gap-4 sm:grid-cols-2">
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

                    {supportStreamTimeout && (
                        <FormField
                            control={form.control}
                            name="stream_timeout"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel>{t("model.dialog.streamTimeout")}</FormLabel>
                                    <FormControl>
                                        <Input
                                            type="number"
                                            placeholder={t("model.dialog.streamTimeoutPlaceholder")}
                                            {...field}
                                            onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                        />
                                    </FormControl>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                    )}

                    {supportImageGenerationCountLimit && (
                        <FormField
                            control={form.control}
                            name="max_image_generation_count"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel>{t("model.dialog.maxImageGenerationCount")}</FormLabel>
                                    <FormControl>
                                        <Input
                                            type="number"
                                            min={0}
                                            placeholder={t("model.dialog.maxImageGenerationCountPlaceholder")}
                                            {...field}
                                            onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : 0)}
                                        />
                                    </FormControl>
                                    <FormDescription>{t("model.dialog.maxImageGenerationCountDescription")}</FormDescription>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                    )}

                    {configFieldVisibility.showResolutions && (
                        <FormField
                            control={form.control}
                            name="allowed_resolutions"
                            render={() => (
                                <FormItem>
                                    <FormLabel>{t("model.dialog.config.allowedResolutions")}</FormLabel>
                                    <FormControl>
                                        <Textarea
                                            placeholder={t("model.dialog.config.allowedResolutionsPlaceholder")}
                                            value={(allowedResolutionsValue || []).join('\n')}
                                            onChange={(e) => {
                                                const values = e.target.value
                                                    .split('\n')
                                                    .map((item) => item.trim())
                                                    .filter(Boolean)
                                                form.setValue('allowed_resolutions', values.length > 0 ? values : undefined, { shouldDirty: true })
                                            }}
                                            className="min-h-[96px]"
                                        />
                                    </FormControl>
                                    <FormDescription>{t("model.dialog.config.allowedResolutionsDescription")}</FormDescription>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                    )}

                    {supportVideoGenerationSecondsLimit && (
                        <FormField
                            control={form.control}
                            name="max_video_generation_seconds"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel>{t("model.dialog.maxVideoGenerationSeconds")}</FormLabel>
                                    <FormControl>
                                        <Input
                                            type="number"
                                            min={0}
                                            placeholder={t("model.dialog.maxVideoGenerationSecondsPlaceholder")}
                                            {...field}
                                            onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : 0)}
                                        />
                                    </FormControl>
                                    <FormDescription>{t("model.dialog.maxVideoGenerationSecondsDescription")}</FormDescription>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                    )}

                    {supportVideoGenerationCountLimit && (
                        <FormField
                            control={form.control}
                            name="max_video_generation_count"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel>{t("model.dialog.maxVideoGenerationCount")}</FormLabel>
                                    <FormControl>
                                        <Input
                                            type="number"
                                            min={0}
                                            placeholder={t("model.dialog.maxVideoGenerationCountPlaceholder")}
                                            {...field}
                                            onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : 0)}
                                        />
                                    </FormControl>
                                    <FormDescription>{t("model.dialog.maxVideoGenerationCountDescription")}</FormDescription>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                    )}

                    {/* Force Save Detail Switch */}
                    <FormField
                        control={form.control}
                        name="force_save_detail"
                        render={({ field }) => (
                            <FormItem className="flex flex-row items-center justify-between gap-4 border-t py-3 sm:col-span-2">
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

                    <FormField
                        control={form.control}
                        name="exclude_from_tests"
                        render={({ field }) => (
                            <FormItem className="flex flex-row items-center justify-between gap-4 border-t py-3 sm:col-span-2">
                                <div className="space-y-1">
                                    <FormLabel>{t("model.dialog.excludeFromTests")}</FormLabel>
                                    <FormDescription>{t("model.dialog.excludeFromTestsDescription")}</FormDescription>
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
                        name="request_body_storage_max_size"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("model.dialog.requestBodyStorageMaxSize")}</FormLabel>
                                <FormControl>
                                    <Input
                                        type="number"
                                        placeholder={t("model.dialog.requestBodyStorageMaxSizePlaceholder")}
                                        {...field}
                                        onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : 0)}
                                    />
                                </FormControl>
                                <FormDescription>{t("model.dialog.requestBodyStorageMaxSizeDescription")}</FormDescription>
                                <FormMessage />
                            </FormItem>
                        )}
                    />

                    <FormField
                        control={form.control}
                        name="response_body_storage_max_size"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("model.dialog.responseBodyStorageMaxSize")}</FormLabel>
                                <FormControl>
                                    <Input
                                        type="number"
                                        placeholder={t("model.dialog.responseBodyStorageMaxSizePlaceholder")}
                                        {...field}
                                        onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : 0)}
                                    />
                                </FormControl>
                                <FormDescription>{t("model.dialog.responseBodyStorageMaxSizeDescription")}</FormDescription>
                                <FormMessage />
                            </FormItem>
                        )}
                    />

                    {/* Record Service Tier Switch */}
                    <FormField
                        control={form.control}
                        name="summary_service_tier"
                        render={({ field }) => (
                            <FormItem className="flex flex-row items-center justify-between gap-4 border-t py-3 sm:col-span-2">
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
                            <FormItem className="flex flex-row items-center justify-between gap-4 border-t py-3 sm:col-span-2">
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

                    {supportResolutionFuzzyMatchConfig && (
                        <FormField
                            control={form.control}
                            name="disable_resolution_fuzzy_match"
                            render={({ field }) => (
                                <FormItem className="flex flex-row items-center justify-between gap-4 border-t py-3 sm:col-span-2">
                                    <div className="space-y-1">
                                        <FormLabel>{t("model.dialog.disableResolutionFuzzyMatch")}</FormLabel>
                                        <FormDescription>{t("model.dialog.disableResolutionFuzzyMatchDescription")}</FormDescription>
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
                    )}

                    </div></section>
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
                                                            aria-label={t("model.dialog.cachePlugin.title")}
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
                                                <FormItem className="flex flex-row items-center justify-between gap-4 border-t py-3 sm:col-span-2">
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

                        {/* Cache Follow Plugin */}
                        <div className="flex items-center justify-between py-2">
                            <div className="flex items-center space-x-3">
                                <FormField
                                    control={form.control}
                                    name="plugin.cachefollow.enable"
                                    render={({ field }) => (
                                        <FormItem className="flex items-center space-x-2">
                                            <FormControl>
                                                <Switch
                                                            aria-label={t("model.dialog.cacheFollowPlugin.title")}
                                                    checked={field.value}
                                                    onCheckedChange={field.onChange}
                                                />
                                            </FormControl>
                                        </FormItem>
                                    )}
                                />
                                <div>
                                    <Label className="text-sm font-medium">{t("model.dialog.cacheFollowPlugin.title")}</Label>
                                    <p className="text-xs text-muted-foreground">{t("model.dialog.cacheFollowPlugin.description")}</p>
                                </div>
                            </div>
                        </div>

                        {cacheFollowEnabled && (
                            <div className="pl-8 pb-4 space-y-4">
                                <FormField
                                    control={form.control}
                                    name="plugin.cachefollow.enable_generic_follow"
                                    render={({ field }) => (
                                        <FormItem className="flex items-start space-x-3 rounded-md border p-4">
                                            <FormControl>
                                                <Switch
                                                    checked={field.value ?? false}
                                                    onCheckedChange={field.onChange}
                                                />
                                            </FormControl>
                                            <div className="space-y-1 leading-none">
                                                <FormLabel>{t("model.dialog.cacheFollowPlugin.enableGenericFollow")}</FormLabel>
                                                <FormDescription>{t("model.dialog.cacheFollowPlugin.enableGenericFollowDescription")}</FormDescription>
                                            </div>
                                        </FormItem>
                                    )}
                                />

                                <FormField
                                    control={form.control}
                                    name="plugin.cachefollow.followed_channel_ttl_seconds"
                                    render={({ field }) => (
                                        <FormItem>
                                            <FormLabel>{t("model.dialog.cacheFollowPlugin.followedChannelTTLSeconds")}</FormLabel>
                                            <FormControl>
                                                <Input
                                                    type="number"
                                                    placeholder={t("model.dialog.cacheFollowPlugin.followedChannelTTLSecondsPlaceholder")}
                                                    {...field}
                                                    onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                                />
                                            </FormControl>
                                            <FormDescription>{t("model.dialog.cacheFollowPlugin.followedChannelTTLSecondsDescription")}</FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    )}
                                />

                                <FormField
                                    control={form.control}
                                    name="plugin.cachefollow.recent_channel_update_debounce_seconds"
                                    render={({ field }) => (
                                        <FormItem>
                                            <FormLabel>{t("model.dialog.cacheFollowPlugin.recentChannelUpdateDebounceSeconds")}</FormLabel>
                                            <FormControl>
                                                <Input
                                                    type="number"
                                                    placeholder={t("model.dialog.cacheFollowPlugin.recentChannelUpdateDebounceSecondsPlaceholder")}
                                                    {...field}
                                                    onChange={(e) => field.onChange(e.target.value ? Number(e.target.value) : undefined)}
                                                />
                                            </FormControl>
                                            <FormDescription>{t("model.dialog.cacheFollowPlugin.recentChannelUpdateDebounceSecondsDescription")}</FormDescription>
                                            <FormMessage />
                                        </FormItem>
                                    )}
                                />
                            </div>
                        )}

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
                                                            aria-label={t("model.dialog.webSearchPlugin.title")}
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
                                                <FormItem className="flex flex-row items-center justify-between gap-4 border-t py-3 sm:col-span-2">
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
                                                            aria-label={t("model.dialog.thinkSplitPlugin.title")}
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
                                                            aria-label={t("model.dialog.streamFakePlugin.title")}
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
                    <div className="form-actions">
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
