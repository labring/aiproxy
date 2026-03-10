import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Plus, Trash2 } from 'lucide-react'
import type { ModelPrice, ConditionalPrice, PriceCondition } from '@/types/model'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'

interface PriceFormFieldsProps {
    price: ModelPrice
    onChange: (price: ModelPrice) => void
    /** Hide conditional prices section (used when rendering inside a conditional price) */
    hideConditional?: boolean
}

function PriceField({ label, unitLabel, value, unitValue, onValueChange, onUnitChange }: {
    label: string
    unitLabel?: string
    value?: number
    unitValue?: number
    onValueChange: (v: number) => void
    onUnitChange?: (v: number) => void
}) {
    return (
        <div className={unitLabel ? "grid grid-cols-2 gap-2" : ""}>
            <div className="space-y-1">
                <Label className="text-xs">{label}</Label>
                <Input
                    type="number"
                    step="any"
                    min={0}
                    value={value || ''}
                    onChange={(e) => onValueChange(parseFloat(e.target.value) || 0)}
                    className="h-8 text-sm"
                />
            </div>
            {unitLabel && onUnitChange && (
                <div className="space-y-1">
                    <Label className="text-xs">{unitLabel}</Label>
                    <Input
                        type="number"
                        step="1"
                        min={0}
                        placeholder="1000"
                        value={unitValue || ''}
                        onChange={(e) => onUnitChange(parseInt(e.target.value) || 0)}
                        className="h-8 text-sm"
                    />
                </div>
            )}
        </div>
    )
}

function ConditionFields({ condition, onChange }: {
    condition: PriceCondition
    onChange: (c: PriceCondition) => void
}) {
    const { t } = useTranslation()
    const anyServiceTier = '__any__'

    return (
        <div className="grid grid-cols-2 gap-2">
            <div className="space-y-1">
                <Label className="text-xs">{t('group.price.inputTokenMin')}</Label>
                <Input type="number" min={0} value={condition.input_token_min || ''} className="h-8 text-sm"
                    onChange={(e) => onChange({ ...condition, input_token_min: parseInt(e.target.value) || 0 })} />
            </div>
            <div className="space-y-1">
                <Label className="text-xs">{t('group.price.inputTokenMax')}</Label>
                <Input type="number" min={0} value={condition.input_token_max || ''} className="h-8 text-sm"
                    onChange={(e) => onChange({ ...condition, input_token_max: parseInt(e.target.value) || 0 })} />
            </div>
            <div className="space-y-1">
                <Label className="text-xs">{t('group.price.outputTokenMin')}</Label>
                <Input type="number" min={0} value={condition.output_token_min || ''} className="h-8 text-sm"
                    onChange={(e) => onChange({ ...condition, output_token_min: parseInt(e.target.value) || 0 })} />
            </div>
            <div className="space-y-1">
                <Label className="text-xs">{t('group.price.outputTokenMax')}</Label>
                <Input type="number" min={0} value={condition.output_token_max || ''} className="h-8 text-sm"
                    onChange={(e) => onChange({ ...condition, output_token_max: parseInt(e.target.value) || 0 })} />
            </div>
            <div className="space-y-1 col-span-2">
                <Label className="text-xs">{t('group.price.serviceTier')}</Label>
                <Select
                    value={condition.service_tier || anyServiceTier}
                    onValueChange={(value) => onChange({
                        ...condition,
                        service_tier: (value === anyServiceTier ? '' : value) as PriceCondition['service_tier']
                    })}
                >
                    <SelectTrigger className="h-8 text-sm">
                        <SelectValue placeholder={t('group.price.serviceTierAny')} />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value={anyServiceTier}>{t('group.price.serviceTierAny')}</SelectItem>
                        <SelectItem value="auto">auto</SelectItem>
                        <SelectItem value="default">default</SelectItem>
                        <SelectItem value="flex">flex</SelectItem>
                        <SelectItem value="scale">scale</SelectItem>
                        <SelectItem value="priority">priority</SelectItem>
                    </SelectContent>
                </Select>
            </div>
        </div>
    )
}

function BasePriceFields({ price, onChange }: { price: ModelPrice; onChange: (price: ModelPrice) => void }) {
    const { t } = useTranslation()

    const updateField = (field: keyof ModelPrice, value: number) => {
        onChange({ ...price, [field]: value || undefined })
    }

    return (
        <div className="space-y-3">
            <PriceField
                label={t('group.price.inputPrice')}
                unitLabel={t('group.price.inputPriceUnit')}
                value={price.input_price}
                unitValue={price.input_price_unit}
                onValueChange={(v) => updateField('input_price', v)}
                onUnitChange={(v) => updateField('input_price_unit', v)}
            />
            <PriceField
                label={t('group.price.outputPrice')}
                unitLabel={t('group.price.outputPriceUnit')}
                value={price.output_price}
                unitValue={price.output_price_unit}
                onValueChange={(v) => updateField('output_price', v)}
                onUnitChange={(v) => updateField('output_price_unit', v)}
            />
            <PriceField
                label={t('group.price.perRequestPrice')}
                value={price.per_request_price}
                onValueChange={(v) => updateField('per_request_price', v)}
            />
            <PriceField
                label={t('group.price.cachedPrice')}
                unitLabel={t('group.price.cachedPriceUnit')}
                value={price.cached_price}
                unitValue={price.cached_price_unit}
                onValueChange={(v) => updateField('cached_price', v)}
                onUnitChange={(v) => updateField('cached_price_unit', v)}
            />
            <PriceField
                label={t('group.price.cacheCreationPrice')}
                unitLabel={t('group.price.cacheCreationPriceUnit')}
                value={price.cache_creation_price}
                unitValue={price.cache_creation_price_unit}
                onValueChange={(v) => updateField('cache_creation_price', v)}
                onUnitChange={(v) => updateField('cache_creation_price_unit', v)}
            />
            <PriceField
                label={t('group.price.imageInputPrice')}
                unitLabel={t('group.price.imageInputPriceUnit')}
                value={price.image_input_price}
                unitValue={price.image_input_price_unit}
                onValueChange={(v) => updateField('image_input_price', v)}
                onUnitChange={(v) => updateField('image_input_price_unit', v)}
            />
            <PriceField
                label={t('group.price.imageOutputPrice')}
                unitLabel={t('group.price.imageOutputPriceUnit')}
                value={price.image_output_price}
                unitValue={price.image_output_price_unit}
                onValueChange={(v) => updateField('image_output_price', v)}
                onUnitChange={(v) => updateField('image_output_price_unit', v)}
            />
            <PriceField
                label={t('group.price.audioInputPrice')}
                unitLabel={t('group.price.audioInputPriceUnit')}
                value={price.audio_input_price}
                unitValue={price.audio_input_price_unit}
                onValueChange={(v) => updateField('audio_input_price', v)}
                onUnitChange={(v) => updateField('audio_input_price_unit', v)}
            />
            <PriceField
                label={t('group.price.thinkingOutputPrice')}
                unitLabel={t('group.price.thinkingOutputPriceUnit')}
                value={price.thinking_mode_output_price}
                unitValue={price.thinking_mode_output_price_unit}
                onValueChange={(v) => updateField('thinking_mode_output_price', v)}
                onUnitChange={(v) => updateField('thinking_mode_output_price_unit', v)}
            />
            <PriceField
                label={t('group.price.webSearchPrice')}
                unitLabel={t('group.price.webSearchPriceUnit')}
                value={price.web_search_price}
                unitValue={price.web_search_price_unit}
                onValueChange={(v) => updateField('web_search_price', v)}
                onUnitChange={(v) => updateField('web_search_price_unit', v)}
            />
        </div>
    )
}

export function PriceFormFields({ price, onChange, hideConditional = false }: PriceFormFieldsProps) {
    const { t } = useTranslation()

    const addConditionalPrice = () => {
        const conditionals = price.conditional_prices || []
        onChange({
            ...price,
            conditional_prices: [
                ...conditionals,
                { condition: {}, price: {} } as ConditionalPrice,
            ]
        })
    }

    const removeConditionalPrice = (index: number) => {
        const conditionals = [...(price.conditional_prices || [])]
        conditionals.splice(index, 1)
        onChange({ ...price, conditional_prices: conditionals.length > 0 ? conditionals : undefined })
    }

    const updateConditionalCondition = (index: number, condition: PriceCondition) => {
        const conditionals = [...(price.conditional_prices || [])]
        conditionals[index] = { ...conditionals[index], condition }
        onChange({ ...price, conditional_prices: conditionals })
    }

    const updateConditionalPrice = (index: number, condPrice: ModelPrice) => {
        const conditionals = [...(price.conditional_prices || [])]
        conditionals[index] = { ...conditionals[index], price: condPrice }
        onChange({ ...price, conditional_prices: conditionals })
    }

    return (
        <div className="space-y-3">
            <BasePriceFields price={price} onChange={(p) => {
                // Preserve conditional_prices when base fields change
                onChange({ ...p, conditional_prices: price.conditional_prices })
            }} />

            {/* Conditional Prices */}
            {!hideConditional && (
                <div className="space-y-2 pt-2">
                    <div className="flex items-center justify-between">
                        <Label className="text-sm font-medium">{t('group.price.conditionalPrices')}</Label>
                        <Button type="button" variant="outline" size="sm" className="h-7 text-xs" onClick={addConditionalPrice}>
                            <Plus className="h-3 w-3 mr-1" />
                            {t('group.price.addCondition')}
                        </Button>
                    </div>
                    {price.conditional_prices?.map((cp, index) => (
                        <div key={index} className="rounded-lg border p-3 space-y-3">
                            <div className="flex items-center justify-between">
                                <Label className="text-xs font-medium">{t('group.price.condition')} #{index + 1}</Label>
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    className="h-6 text-xs text-destructive hover:text-destructive"
                                    onClick={() => removeConditionalPrice(index)}
                                >
                                    <Trash2 className="h-3 w-3 mr-1" />
                                    {t('group.price.removeCondition')}
                                </Button>
                            </div>
                            <ConditionFields
                                condition={cp.condition}
                                onChange={(c) => updateConditionalCondition(index, c)}
                            />
                            <div className="border-t pt-2">
                                <Label className="text-xs font-medium mb-2 block">{t('group.price.conditionPrice')}</Label>
                                <BasePriceFields
                                    price={cp.price}
                                    onChange={(p) => updateConditionalPrice(index, p)}
                                />
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}
