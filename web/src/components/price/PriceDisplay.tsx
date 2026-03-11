import { useTranslation } from 'react-i18next'
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from '@/components/ui/popover'
import { Badge } from '@/components/ui/badge'
import type { ModelPrice } from '@/types/model'

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
    if (!price) return null
    const effectiveUnit = unit || DEFAULT_UNIT
    return `${price} / ${effectiveUnit}`
}

function formatConditionSummary(condition: NonNullable<ModelPrice['conditional_prices']>[number]['condition']): string[] {
    const parts: string[] = []

    if (condition.service_tier !== undefined) {
        parts.push(`Service Tier: ${SERVICE_TIER_LABELS[condition.service_tier]}`)
    }

    if (condition.input_token_min || condition.input_token_max) {
        parts.push(`Input: ${condition.input_token_min || 0} - ${condition.input_token_max || '∞'}`)
    }

    if (condition.output_token_min || condition.output_token_max) {
        parts.push(`Output: ${condition.output_token_min || 0} - ${condition.output_token_max || '∞'}`)
    }

    return parts
}

export function PriceDisplay({ price }: PriceDisplayProps) {
    const { t } = useTranslation()

    if (!price) {
        return <span className="text-muted-foreground text-sm">-</span>
    }

    const inputStr = formatPriceValue(price.input_price, price.input_price_unit)
    const outputStr = formatPriceValue(price.output_price, price.output_price_unit)

    // Quick summary for cell display
    const summary = [inputStr && `In: ${inputStr}`, outputStr && `Out: ${outputStr}`].filter(Boolean).join(' | ')
    if (!summary && !price.per_request_price && !price.conditional_prices?.length) {
        return <span className="text-muted-foreground text-sm">-</span>
    }

    const rows: { label: string; value: string | null }[] = [
        { label: t('group.price.inputPrice'), value: formatPriceValue(price.input_price, price.input_price_unit) },
        { label: t('group.price.outputPrice'), value: formatPriceValue(price.output_price, price.output_price_unit) },
        { label: t('group.price.perRequestPrice'), value: price.per_request_price ? String(price.per_request_price) : null },
        { label: t('group.price.cachedPrice'), value: formatPriceValue(price.cached_price, price.cached_price_unit) },
        { label: t('group.price.cacheCreationPrice'), value: formatPriceValue(price.cache_creation_price, price.cache_creation_price_unit) },
        { label: t('group.price.imageInputPrice'), value: formatPriceValue(price.image_input_price, price.image_input_price_unit) },
        { label: t('group.price.imageOutputPrice'), value: formatPriceValue(price.image_output_price, price.image_output_price_unit) },
        { label: t('group.price.audioInputPrice'), value: formatPriceValue(price.audio_input_price, price.audio_input_price_unit) },
        { label: t('group.price.thinkingOutputPrice'), value: formatPriceValue(price.thinking_mode_output_price, price.thinking_mode_output_price_unit) },
        { label: t('group.price.webSearchPrice'), value: formatPriceValue(price.web_search_price, price.web_search_price_unit) },
    ].filter(r => r.value !== null)

    const hasConditional = price.conditional_prices && price.conditional_prices.length > 0

    return (
        <Popover>
            <PopoverTrigger asChild>
                <button className="text-left text-sm font-mono hover:underline cursor-pointer">
                    {summary || (price.per_request_price ? `Per req: ${price.per_request_price}` : t('group.price.conditionalPrices'))}
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
                                <div key={i} className="rounded border p-2 mb-1 text-xs space-y-1">
                                    {formatConditionSummary(cp.condition).length > 0 && (
                                        <div className="text-muted-foreground flex flex-wrap gap-x-3 gap-y-1">
                                            {formatConditionSummary(cp.condition).map((item) => (
                                                <span key={item}>{item}</span>
                                            ))}
                                        </div>
                                    )}
                                    {cp.price.input_price != null && (
                                        <div className="flex justify-between">
                                            <span className="text-muted-foreground">{t('group.price.inputPrice')}</span>
                                            <span className="font-mono">{formatPriceValue(cp.price.input_price, cp.price.input_price_unit)}</span>
                                        </div>
                                    )}
                                    {cp.price.output_price != null && (
                                        <div className="flex justify-between">
                                            <span className="text-muted-foreground">{t('group.price.outputPrice')}</span>
                                            <span className="font-mono">{formatPriceValue(cp.price.output_price, cp.price.output_price_unit)}</span>
                                        </div>
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
