import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { MultiSelectCombobox } from '@/components/select/MultiSelectCombobox'
import type { ChannelConfigSchema, ChannelTypeMeta } from '@/types/channel'

type ConfigObject = Record<string, unknown>

interface ChannelConfigEditorProps {
    value: string
    onChange: (value: string) => void
    meta?: ChannelTypeMeta | null
    error?: string | null
}

const parseConfigText = (value: string): { parsed: ConfigObject; error: string | null } => {
    const raw = value.trim()
    if (!raw) {
        return { parsed: {}, error: null }
    }

    try {
        const parsed = JSON.parse(raw) as unknown
        if (parsed === null || Array.isArray(parsed) || typeof parsed !== 'object') {
            return { parsed: {}, error: 'object' }
        }
        return { parsed: parsed as ConfigObject, error: null }
    } catch {
        return { parsed: {}, error: 'invalid' }
    }
}

const stringifyConfig = (config: ConfigObject) => {
    if (Object.keys(config).length === 0) {
        return ''
    }
    return JSON.stringify(config, null, 2)
}

const getAtPath = (obj: unknown, path: string[]): unknown => {
    let current = obj
    for (const segment of path) {
        if (!current || typeof current !== 'object' || Array.isArray(current)) {
            return undefined
        }
        current = (current as Record<string, unknown>)[segment]
    }
    return current
}

const setAtPath = (obj: ConfigObject, path: string[], value: unknown): ConfigObject => {
    const next = { ...obj }
    let current: Record<string, unknown> = next

    for (let i = 0; i < path.length - 1; i += 1) {
        const key = path[i]
        const child = current[key]
        if (!child || typeof child !== 'object' || Array.isArray(child)) {
            current[key] = {}
        } else {
            current[key] = { ...(child as Record<string, unknown>) }
        }
        current = current[key] as Record<string, unknown>
    }

    const lastKey = path[path.length - 1]
    if (
        value === undefined ||
        value === null ||
        value === '' ||
        (Array.isArray(value) && value.length === 0)
    ) {
        delete current[lastKey]
    } else {
        current[lastKey] = value
    }

    return next
}

const cleanupEmptyObjects = (value: unknown): unknown => {
    if (Array.isArray(value)) {
        return value.map(cleanupEmptyObjects).filter((item) => item !== undefined)
    }

    if (!value || typeof value !== 'object') {
        return value
    }

    const entries = Object.entries(value as Record<string, unknown>)
        .map(([key, entryValue]) => [key, cleanupEmptyObjects(entryValue)] as const)
        .filter(([, entryValue]) => {
            if (entryValue === undefined || entryValue === null || entryValue === '') {
                return false
            }
            if (Array.isArray(entryValue)) {
                return entryValue.length > 0
            }
            if (typeof entryValue === 'object') {
                return Object.keys(entryValue as Record<string, unknown>).length > 0
            }
            return true
        })

    return Object.fromEntries(entries)
}

interface SchemaFieldProps {
    schema: ChannelConfigSchema
    path: string[]
    rootValue: ConfigObject
    onValueChange: (path: string[], value: unknown) => void
}

function SchemaField({ schema, path, rootValue, onValueChange }: SchemaFieldProps) {
    const key = path[path.length - 1]
    const title = schema.title || key
    const description = schema.description
    const currentValue = getAtPath(rootValue, path)

    if (schema.type === 'object' && schema.properties) {
        return (
            <div className="space-y-3 rounded-lg border p-4">
                {path.length > 0 && (
                    <div className="space-y-1">
                        <Label>{title}</Label>
                        {description && <p className="text-xs text-muted-foreground">{description}</p>}
                    </div>
                )}
                {Object.entries(schema.properties).map(([childKey, childSchema]) => (
                    <SchemaField
                        key={[...path, childKey].join('.')}
                        schema={childSchema}
                        path={[...path, childKey]}
                        rootValue={rootValue}
                        onValueChange={onValueChange}
                    />
                ))}
            </div>
        )
    }

    if (schema.type === 'boolean') {
        return (
            <div className="flex items-center justify-between rounded-lg border p-3">
                <div className="space-y-1 pr-4">
                    <Label>{title}</Label>
                    {description && <p className="text-xs text-muted-foreground">{description}</p>}
                </div>
                <Switch
                    checked={Boolean(currentValue)}
                    onCheckedChange={(checked) => onValueChange(path, checked ? true : undefined)}
                />
            </div>
        )
    }

    if (schema.enum && schema.enum.length > 0) {
        return (
            <div className="space-y-2">
                <Label>{title}</Label>
                <Select
                    value={typeof currentValue === 'string' ? currentValue : ''}
                    onValueChange={(next) => onValueChange(path, next)}
                >
                    <SelectTrigger>
                        <SelectValue placeholder={description || title} />
                    </SelectTrigger>
                    <SelectContent>
                        {schema.enum.map((option) => (
                            <SelectItem key={option} value={option}>
                                {option}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
                {description && <p className="text-xs text-muted-foreground">{description}</p>}
            </div>
        )
    }

    if (schema.type === 'array' && schema.items?.type === 'string') {
        return (
            <div className="space-y-2">
                <MultiSelectCombobox<string>
                    dropdownItems={schema.items.enum || []}
                    selectedItems={Array.isArray(currentValue) ? currentValue.filter((item): item is string => typeof item === 'string') : []}
                    setSelectedItems={(items) => onValueChange(path, items)}
                    handleFilteredDropdownItems={(dropdownItems, selectedItems, inputValue) => {
                        const normalized = inputValue.trim()
                        const filtered = dropdownItems.filter((item) => !selectedItems.includes(item) && item.toLowerCase().includes(normalized.toLowerCase()))
                        if (normalized && !selectedItems.includes(normalized) && !dropdownItems.includes(normalized)) {
                            return [normalized, ...filtered]
                        }
                        return filtered
                    }}
                    handleDropdownItemDisplay={(item) => item}
                    handleSelectedItemDisplay={(item) => item}
                    allowUserCreatedItems={true}
                    placeholder={description || title}
                    label={title}
                />
                {description && <p className="text-xs text-muted-foreground">{description}</p>}
            </div>
        )
    }

    if (schema.type === 'number' || schema.type === 'integer') {
        return (
            <div className="space-y-2">
                <Label>{title}</Label>
                <Input
                    type="number"
                    value={typeof currentValue === 'number' ? currentValue : ''}
                    onChange={(e) => {
                        const raw = e.target.value
                        onValueChange(path, raw === '' ? undefined : Number(raw))
                    }}
                    placeholder={description || title}
                />
                {description && <p className="text-xs text-muted-foreground">{description}</p>}
            </div>
        )
    }

    return (
        <div className="space-y-2">
            <Label>{title}</Label>
            <Input
                value={typeof currentValue === 'string' ? currentValue : ''}
                placeholder={description || title}
                onChange={(e) => onValueChange(path, e.target.value)}
            />
            {description && <p className="text-xs text-muted-foreground">{description}</p>}
        </div>
    )
}

export function ChannelConfigEditor({
    value,
    onChange,
    meta,
    error,
}: ChannelConfigEditorProps) {
    const { t } = useTranslation()
    const [mode, setMode] = useState<'visual' | 'json'>('visual')

    const schema = meta?.configSchema
    const hasVisualConfigs = schema?.type === 'object' && !!schema.properties && Object.keys(schema.properties).length > 0
    const parsedState = useMemo(() => parseConfigText(value || ''), [value])

    const updateConfig = (path: string[], nextValue: unknown) => {
        if (parsedState.error) {
            return
        }

        const next = setAtPath(parsedState.parsed, path, nextValue)
        onChange(stringifyConfig(cleanupEmptyObjects(next) as ConfigObject))
    }

    if (!hasVisualConfigs) {
        return (
            <div className="space-y-2">
                <Textarea
                    placeholder={t('channel.dialog.configsPlaceholder')}
                    value={value || ''}
                    onChange={(e) => onChange(e.target.value)}
                    className="min-h-[160px] font-mono text-xs"
                />
                <p className="text-xs text-muted-foreground">
                    {t('channel.dialog.configsHelp')}
                </p>
                {error && <p className="text-sm font-medium text-destructive">{error}</p>}
            </div>
        )
    }

    return (
        <div className="space-y-3">
            <Tabs value={mode} onValueChange={(next) => setMode(next as 'visual' | 'json')}>
                <TabsList className="grid w-full grid-cols-2">
                    <TabsTrigger value="visual">
                        {t('channel.dialog.configEditor.visual')}
                    </TabsTrigger>
                    <TabsTrigger value="json">
                        {t('channel.dialog.configEditor.json')}
                    </TabsTrigger>
                </TabsList>

                <TabsContent value="visual" className="mt-3 space-y-4">
                    {parsedState.error ? (
                        <div className="rounded-lg border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive">
                            {parsedState.error === 'object'
                                ? t('channel.dialog.configsJsonObjectError')
                                : t('channel.dialog.configsJsonInvalid')}
                        </div>
                    ) : (
                        Object.entries(schema.properties || {}).map(([key, childSchema]) => (
                            <SchemaField
                                key={key}
                                schema={childSchema}
                                path={[key]}
                                rootValue={parsedState.parsed}
                                onValueChange={updateConfig}
                            />
                        ))
                    )}

                    {error && <p className="text-sm font-medium text-destructive">{error}</p>}
                    <p className="text-xs text-muted-foreground">
                        {t('channel.dialog.configEditor.sharedDataHelp')}
                    </p>
                </TabsContent>

                <TabsContent value="json" className="mt-3 space-y-2">
                    <Textarea
                        placeholder={t('channel.dialog.configsPlaceholder')}
                        value={value || ''}
                        onChange={(e) => onChange(e.target.value)}
                        className="min-h-[160px] font-mono text-xs"
                    />
                    <p className="text-xs text-muted-foreground">
                        {t('channel.dialog.configsHelp')}
                    </p>
                    {error && <p className="text-sm font-medium text-destructive">{error}</p>}
                </TabsContent>
            </Tabs>
        </div>
    )
}
