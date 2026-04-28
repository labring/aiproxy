import { useTranslation } from 'react-i18next'
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from '@/components/ui/popover'
import { Badge } from '@/components/ui/badge'
import type { ModelPrice, PriceCondition } from '@/types/model'

interface PriceDisplayProps {
    price?: ModelPrice
}

const DEFAULT_UNIT = 1000
const SERVICE_TIER_LABELS: Record<NonNullable<NonNullable<ModelPrice['conditional_prices']>[number]['condition']['service_tier']>, string> = {
    '': 'Any',
    auto: 'Auto',
    default: 'Default',
    flex: 'Flex',
    scale: 'Scale',
    priority: 'Priority',
}

function formatPriceValue(price?: number, unit?: number): string | null {
    if (price == null) return null
    const effectiveUnit = unit || DEFAULT_UNIT
    return `${price} / ${effectiveUnit}`
}

function formatTimestamp(value?: number): string | null {
    if (!value) return null
    return new Date(value * 1000).toLocaleString()
}

export function PriceDisplay({ price }: PriceDisplayProps) {
    const { t } = useTranslation()

    const getPriceRows = (target?: ModelPrice): { label: string; value: string | null }[] => {
        if (!target) {
            return []
        }

        return [
            { label: t('group.price.inputPrice'), value: formatPriceValue(target.input_price, target.input_price_unit) },
            { label: t('group.price.outputPrice'), value: formatPriceValue(target.output_price, target.output_price_unit) },
            { label: t('group.price.perRequestPrice'), value: target.per_request_price != null ? String(target.per_request_price) : null },
            { label: t('group.price.cachedPrice'), value: formatPriceValue(target.cached_price, target.cached_price_unit) },
            { label: t('group.price.cacheCreationPrice'), value: formatPriceValue(target.cache_creation_price, target.cache_creation_price_unit) },
            { label: t('group.price.imageInputPrice'), value: formatPriceValue(target.image_input_price, target.image_input_price_unit) },
            { label: t('group.price.imageOutputPrice'), value: formatPriceValue(target.image_output_price, target.image_output_price_unit) },
            { label: t('group.price.audioInputPrice'), value: formatPriceValue(target.audio_input_price, target.audio_input_price_unit) },
            { label: t('group.price.thinkingOutputPrice'), value: formatPriceValue(target.thinking_mode_output_price, target.thinking_mode_output_price_unit) },
            { label: t('group.price.webSearchPrice'), value: formatPriceValue(target.web_search_price, target.web_search_price_unit) },
        ].filter((row) => row.value !== null)
    }

    const formatConditionSummary = (condition: PriceCondition): string[] => {
        const parts: string[] = []

        if (condition.service_tier !== undefined) {
            parts.push(`${t('group.price.serviceTier')}: ${SERVICE_TIER_LABELS[condition.service_tier]}`)
        }

        if (condition.input_token_min || condition.input_token_max) {
            parts.push(`${t('group.price.inputPrice')}: ${condition.input_token_min || 0} - ${condition.input_token_max || '∞'}`)
        }

        if (condition.output_token_min || condition.output_token_max) {
            parts.push(`${t('group.price.outputPrice')}: ${condition.output_token_min || 0} - ${condition.output_token_max || '∞'}`)
        }

        if (condition.start_time || condition.end_time) {
            parts.push(
                `${t('group.price.startTime')}: ${formatTimestamp(condition.start_time) || '-'} | ` +
                `${t('group.price.endTime')}: ${formatTimestamp(condition.end_time) || '-'}`
            )
        }

        return parts
    }

    if (!price) {
        return <span className="text-muted-foreground text-sm">-</span>
    }

    const rows = getPriceRows(price)
    const summaryCandidates = [
        formatPriceValue(price.input_price, price.input_price_unit) && `In: ${formatPriceValue(price.input_price, price.input_price_unit)}`,
        formatPriceValue(price.output_price, price.output_price_unit) && `Out: ${formatPriceValue(price.output_price, price.output_price_unit)}`,
        price.per_request_price != null ? `Req: ${price.per_request_price}` : null,
        formatPriceValue(price.cached_price, price.cached_price_unit) && `Cache: ${formatPriceValue(price.cached_price, price.cached_price_unit)}`,
        formatPriceValue(price.cache_creation_price, price.cache_creation_price_unit) && `Cache Create: ${formatPriceValue(price.cache_creation_price, price.cache_creation_price_unit)}`,
        formatPriceValue(price.image_input_price, price.image_input_price_unit) && `Img In: ${formatPriceValue(price.image_input_price, price.image_input_price_unit)}`,
        formatPriceValue(price.image_output_price, price.image_output_price_unit) && `Img Out: ${formatPriceValue(price.image_output_price, price.image_output_price_unit)}`,
        formatPriceValue(price.audio_input_price, price.audio_input_price_unit) && `Audio In: ${formatPriceValue(price.audio_input_price, price.audio_input_price_unit)}`,
        formatPriceValue(price.thinking_mode_output_price, price.thinking_mode_output_price_unit) && `Think Out: ${formatPriceValue(price.thinking_mode_output_price, price.thinking_mode_output_price_unit)}`,
        formatPriceValue(price.web_search_price, price.web_search_price_unit) && `Search: ${formatPriceValue(price.web_search_price, price.web_search_price_unit)}`,
    ].filter(Boolean)

    // Quick summary for cell display
    const summary = summaryCandidates.slice(0, 2).join(' | ')
    if (!summary && !price.conditional_prices?.length) {
        return <span className="text-muted-foreground text-sm">-</span>
    }

    const hasConditional = price.conditional_prices && price.conditional_prices.length > 0

    return (
        <Popover>
            <PopoverTrigger asChild>
                <button className="text-left text-sm font-mono hover:underline cursor-pointer">
                    {summary || t('group.price.conditionalPrices')}
                    {hasConditional && (
                        <Badge variant="secondary" className="text-[10px] ml-1 px-1 py-0">
                            +{price.conditional_prices!.length}
                        </Badge>
                    )}
                </button>
            </PopoverTrigger>
            <PopoverContent className="w-80 p-3" align="start">
                <div className="space-y-2">
                    <h4 className="font-medium text-sm">{t('group.price.title')}</h4>
                    <div className="space-y-1">
                        {rows.map((row) => (
                            <div key={row.label} className="flex justify-between text-xs">
                                <span className="text-muted-foreground">{row.label}</span>
                                <span className="font-mono">{row.value}</span>
                            </div>
                        ))}
                    </div>
                    {hasConditional && (
                        <div className="border-t pt-2 mt-2">
                            <h5 className="font-medium text-xs mb-1">{t('group.price.conditionalPrices')}</h5>
                            {price.conditional_prices!.map((cp, i) => (
                                <div key={i} className="rounded border p-2 mb-1 text-xs space-y-2">
                                    {formatConditionSummary(cp.condition).length > 0 && (
                                        <div className="text-muted-foreground flex flex-wrap gap-x-3 gap-y-1">
                                            {formatConditionSummary(cp.condition).map((item) => (
                                                <span key={item}>{item}</span>
                                            ))}
                                        </div>
                                    )}
                                    {getPriceRows(cp.price).length > 0 ? (
                                        getPriceRows(cp.price).map((row) => (
                                            <div key={`${i}-${row.label}`} className="flex justify-between">
                                                <span className="text-muted-foreground">{row.label}</span>
                                                <span className="font-mono">{row.value}</span>
                                            </div>
                                        ))
                                    ) : (
                                        <div className="text-muted-foreground">{t('group.price.noPrice')}</div>
                                    )}
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            </PopoverContent>
        </Popover>
    )
}
