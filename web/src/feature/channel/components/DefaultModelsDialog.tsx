// src/feature/channel/components/DefaultModelsDialog.tsx
import { useState, useMemo, useCallback } from 'react'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { useChannelTypeMetas, useAllChannelDefaultModels } from '../hooks'
import { useModels } from '@/feature/model/hooks'
import { SingleSelectCombobox } from '@/components/select/SingleSelectCombobox'
import { MultiSelectCombobox } from '@/components/select/MultiSelectCombobox'
import { ConstructMappingComponent } from '@/components/select/ConstructMappingComponent'
import { useTranslation } from 'react-i18next'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { get, put } from '@/api/index'
import { AllDefaultModelsResponse } from '@/api/model'
import { toast } from 'sonner'
import { Loader2, Settings, Plus, Pencil, Trash2, ArrowLeft } from 'lucide-react'

interface DefaultModelsDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
}

type ViewMode = 'list' | 'edit'

export function DefaultModelsDialog({ open, onOpenChange }: DefaultModelsDialogProps) {
    const { t } = useTranslation()
    const queryClient = useQueryClient()

    const [viewMode, setViewMode] = useState<ViewMode>('list')
    const [editingType, setEditingType] = useState<number>(0)
    const [editModels, setEditModels] = useState<string[]>([])
    const [editMapping, setEditMapping] = useState<Record<string, string>>({})
    // Counter to force-remount content after mutations, clearing any stuck CSS state
    const [contentKey, setContentKey] = useState(0)

    const { data: typeMetas, isLoading: isTypeMetasLoading } = useChannelTypeMetas()
    const { data: models, isLoading: isModelsLoading } = useModels()
    const { data: allDefaults, isLoading: isAllDefaultsLoading } = useAllChannelDefaultModels()

    const isLoading = isTypeMetasLoading || isModelsLoading || isAllDefaultsLoading

    // Build list of configured types
    const configuredTypes = useMemo(() => {
        if (!allDefaults?.models || !typeMetas) return []
        return Object.entries(allDefaults.models)
            .filter(([, models]) => models && models.length > 0)
            .map(([typeId, models]) => ({
                typeId: Number(typeId),
                typeName: typeMetas[typeId]?.name || `Type ${typeId}`,
                models,
                mapping: allDefaults.mapping?.[typeId as unknown as string] || {},
            }))
            .sort((a, b) => a.typeName.localeCompare(b.typeName))
    }, [allDefaults, typeMetas])

    // Available channel types that don't have defaults yet
    const availableTypesForAdd = useMemo(() => {
        if (!typeMetas) return []
        const configuredIds = new Set(configuredTypes.map(c => String(c.typeId)))
        return Object.entries(typeMetas)
            .filter(([key]) => !configuredIds.has(key))
            .map(([key, meta]) => ({ id: Number(key), name: meta.name }))
            .sort((a, b) => a.name.localeCompare(b.name))
    }, [typeMetas, configuredTypes])

    const allModelNames = useMemo(() => models?.map((m) => m.model) || [], [models])

    // Fetch fresh defaults from API to avoid stale closure issues
    const fetchFreshDefaults = async () => {
        try {
            const fresh = await get<AllDefaultModelsResponse>('models/default')
            return {
                models: { ...(fresh?.models || {}) },
                mapping: { ...(fresh?.mapping || {}) },
            }
        } catch {
            return { models: {} as Record<string, string[]>, mapping: {} as Record<string, Record<string, string>> }
        }
    }

    // Save mutation
    const saveMutation = useMutation({
        mutationFn: async ({ typeId, models, mapping }: {
            typeId: number
            models: string[]
            mapping: Record<string, string>
        }) => {
            const current = await fetchFreshDefaults()

            if (models.length > 0) {
                current.models[String(typeId)] = models
            } else {
                delete current.models[String(typeId)]
            }

            if (Object.keys(mapping).length > 0) {
                current.mapping[String(typeId)] = mapping
            } else {
                delete current.mapping[String(typeId)]
            }

            await put('option/', { key: 'DefaultChannelModels', value: JSON.stringify(current.models) })
            await put('option/', { key: 'DefaultChannelModelMapping', value: JSON.stringify(current.mapping) })
            return current
        },
        onSuccess: async (data) => {
            // Directly update the cache with computed result (synchronous, no refetch)
            queryClient.setQueryData(['allChannelDefaultModels'], data)
            // Also invalidate to ensure consistency
            queryClient.invalidateQueries({ queryKey: ['channelDefaultModels'] })
            toast.success(t('common.success'))
            setContentKey(k => k + 1)
            setViewMode('list')
        },
        onError: (err: Error) => {
            toast.error(err.message)
        },
    })

    // Delete mutation
    const deleteMutation = useMutation({
        mutationFn: async (typeId: number) => {
            const current = await fetchFreshDefaults()
            delete current.models[String(typeId)]
            delete current.mapping[String(typeId)]

            await put('option/', { key: 'DefaultChannelModels', value: JSON.stringify(current.models) })
            await put('option/', { key: 'DefaultChannelModelMapping', value: JSON.stringify(current.mapping) })
            return current
        },
        onSuccess: async (data) => {
            queryClient.setQueryData(['allChannelDefaultModels'], data)
            queryClient.invalidateQueries({ queryKey: ['channelDefaultModels'] })
            toast.success(t('common.success'))
            setContentKey(k => k + 1)
        },
        onError: (err: Error) => {
            toast.error(err.message)
        },
    })

    const handleAdd = useCallback(() => {
        setEditingType(0)
        setEditModels([])
        setEditMapping({})
        setViewMode('edit')
    }, [])

    const handleEdit = useCallback((typeId: number) => {
        const entry = configuredTypes.find(c => c.typeId === typeId)
        setEditingType(typeId)
        setEditModels(entry?.models ? [...entry.models] : [])
        setEditMapping(entry?.mapping ? { ...entry.mapping } : {})
        setViewMode('edit')
    }, [configuredTypes])

    const handleDelete = useCallback((typeId: number) => {
        deleteMutation.mutate(typeId)
    }, [deleteMutation])

    const handleSave = useCallback(() => {
        if (!editingType) return
        saveMutation.mutate({ typeId: editingType, models: editModels, mapping: editMapping })
    }, [editingType, editModels, editMapping, saveMutation])

    const handleBack = useCallback(() => {
        setViewMode('list')
    }, [])

    // Reset view when dialog opens/closes
    const handleOpenChange = useCallback((newOpen: boolean) => {
        if (!newOpen) {
            setViewMode('list')
            setEditingType(0)
        }
        onOpenChange(newOpen)
    }, [onOpenChange])

    const getKeyByName = useCallback((name: string): number | undefined => {
        if (!typeMetas) return undefined
        for (const key in typeMetas) {
            if (typeMetas[key].name === name) return Number(key)
        }
        return undefined
    }, [typeMetas])

    const editingTypeName = useMemo(() => {
        if (!typeMetas || !editingType) return undefined
        return typeMetas[String(editingType)]?.name
    }, [typeMetas, editingType])

    // Render the list view
    const renderListView = () => (
        <div className="space-y-4">
            {/* Add button */}
            <div className="flex justify-end">
                <Button
                    size="sm"
                    onClick={handleAdd}
                    disabled={availableTypesForAdd.length === 0}
                    className="flex items-center gap-1"
                >
                    <Plus className="h-3.5 w-3.5" />
                    {t('channel.defaultModels.add')}
                </Button>
            </div>

            {/* List */}
            {configuredTypes.length === 0 ? (
                <div className="rounded-lg border border-dashed p-8 text-center text-muted-foreground text-sm">
                    {t('channel.defaultModels.empty')}
                </div>
            ) : (
                <div className="space-y-2">
                    {configuredTypes.map((entry) => (
                        <div
                            key={entry.typeId}
                            className="rounded-lg border bg-card p-4 space-y-2.5 hover:border-primary/30 transition-colors"
                        >
                            {/* Header row */}
                            <div className="flex items-center justify-between">
                                <div className="flex items-center gap-2">
                                    <span className="font-medium text-sm">{entry.typeName}</span>
                                    <Badge variant="secondary" className="text-xs">
                                        {entry.models.length} {t('channel.modelsCount')}
                                    </Badge>
                                    {Object.keys(entry.mapping).length > 0 && (
                                        <Badge variant="outline" className="text-xs">
                                            {Object.keys(entry.mapping).length} {t('channel.defaultModels.mappingCount')}
                                        </Badge>
                                    )}
                                </div>
                                <div className="flex items-center gap-1">
                                    <Button
                                        variant="ghost"
                                        size="icon"
                                        className="h-8 w-8"
                                        onClick={() => handleEdit(entry.typeId)}
                                    >
                                        <Pencil className="h-3.5 w-3.5" />
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        size="icon"
                                        className="h-8 w-8 text-destructive hover:text-destructive"
                                        onClick={() => handleDelete(entry.typeId)}
                                        disabled={deleteMutation.isPending}
                                    >
                                        {deleteMutation.isPending ? (
                                            <Loader2 className="h-3.5 w-3.5 animate-spin" />
                                        ) : (
                                            <Trash2 className="h-3.5 w-3.5" />
                                        )}
                                    </Button>
                                </div>
                            </div>

                            {/* Models preview */}
                            <div className="flex flex-wrap gap-1">
                                {entry.models.slice(0, 10).map((model) => (
                                    <Badge key={model} variant="secondary" className="text-xs font-mono py-0 px-1.5">
                                        {model}
                                    </Badge>
                                ))}
                                {entry.models.length > 10 && (
                                    <Badge variant="outline" className="text-xs py-0 px-1.5">
                                        +{entry.models.length - 10}
                                    </Badge>
                                )}
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )

    // Render the edit/add view
    const renderEditView = () => (
        <div className="space-y-5">
            {/* Back button */}
            <Button
                variant="ghost"
                size="sm"
                onClick={handleBack}
                className="flex items-center gap-1 -ml-2"
            >
                <ArrowLeft className="h-4 w-4" />
                {t('channel.defaultModels.backToList')}
            </Button>

            {/* Channel type selector (only for add mode) */}
            {!editingTypeName ? (
                <div className="space-y-2">
                    <label className="text-sm font-medium">{t('channel.dialog.type')}</label>
                    <SingleSelectCombobox
                        dropdownItems={availableTypesForAdd.map(t => t.name)}
                        initSelectedItem={undefined}
                        setSelectedItem={(name: string) => {
                            const id = getKeyByName(name)
                            if (id) setEditingType(id)
                        }}
                        handleDropdownItemFilter={(items: string[], input: string) => {
                            const lower = input.toLowerCase()
                            return items.filter(item => !input || item.toLowerCase().includes(lower))
                        }}
                        handleDropdownItemDisplay={(item: string) => item}
                    />
                </div>
            ) : (
                <div className="space-y-1">
                    <label className="text-sm font-medium">{t('channel.dialog.type')}</label>
                    <div className="flex items-center gap-2 p-2 rounded-md bg-muted/50">
                        <Badge variant="secondary">{editingTypeName}</Badge>
                    </div>
                </div>
            )}

            {editingType > 0 && (
                <>
                    {/* Models selector */}
                    <div className="space-y-2">
                        <label className="text-sm font-medium">{t('channel.dialog.models')}</label>
                        <MultiSelectCombobox<string>
                            dropdownItems={allModelNames}
                            selectedItems={editModels}
                            setSelectedItems={(modelsOrFn) => {
                                const newModels = Array.isArray(modelsOrFn) ? modelsOrFn : []
                                setEditModels(newModels)
                            }}
                            handleFilteredDropdownItems={(items, selected, input) => {
                                const lower = input.toLowerCase()
                                return items.filter(
                                    item => !selected.includes(item) && item.toLowerCase().includes(lower)
                                )
                            }}
                            handleDropdownItemDisplay={(item) => item}
                            handleSelectedItemDisplay={(item) => item}
                        />
                    </div>

                    {/* Mapping editor */}
                    {editModels.length > 0 && (
                        <ConstructMappingComponent
                            mapKeys={editModels}
                            mapData={editMapping}
                            setMapData={(mapping) => setEditMapping(mapping)}
                        />
                    )}

                    {/* Save button */}
                    <div className="flex justify-end gap-2">
                        <Button variant="outline" onClick={handleBack}>
                            {t('common.cancel')}
                        </Button>
                        <Button
                            onClick={handleSave}
                            disabled={editModels.length === 0 || saveMutation.isPending}
                        >
                            {saveMutation.isPending ? (
                                <>
                                    <Loader2 className="h-4 w-4 animate-spin mr-2" />
                                    {t('common.saving')}
                                </>
                            ) : (
                                t('common.save')
                            )}
                        </Button>
                    </div>
                </>
            )}
        </div>
    )

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="sm:max-w-[90vw] w-[90vw] max-h-[90vh] h-[85vh] overflow-hidden flex flex-col">
                <DialogHeader className="flex-shrink-0">
                    <DialogTitle className="text-xl flex items-center gap-2">
                        <Settings className="h-5 w-5" />
                        {t('channel.defaultModels.title')}
                    </DialogTitle>
                    <DialogDescription>{t('channel.defaultModels.description')}</DialogDescription>
                </DialogHeader>

                <div key={contentKey} className="flex-1 overflow-auto min-h-0 mt-4">
                    {isLoading ? (
                        <div className="space-y-4">
                            <Skeleton className="h-9 w-full" />
                            <Skeleton className="h-20 w-full" />
                            <Skeleton className="h-20 w-full" />
                        </div>
                    ) : (
                        viewMode === 'list' ? renderListView() : renderEditView()
                    )}
                </div>
            </DialogContent>
        </Dialog>
    )
}
