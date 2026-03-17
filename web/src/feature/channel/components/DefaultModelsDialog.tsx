// src/feature/channel/components/DefaultModelsDialog.tsx
import { useState, useMemo, useCallback, useRef } from 'react'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
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
import { Loader2, Settings, Plus, Pencil, Trash2, ArrowLeft, Download, Upload } from 'lucide-react'

interface DefaultModelsDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
}

type ViewMode = 'list' | 'edit'

const isStringArray = (value: unknown): value is string[] => (
    Array.isArray(value) && value.every((item) => typeof item === 'string')
)

const isStringRecord = (value: unknown): value is Record<string, string> => (
    !!value &&
    typeof value === 'object' &&
    !Array.isArray(value) &&
    Object.values(value).every((item) => typeof item === 'string')
)

const normalizeDefaultModelsImport = (raw: unknown): AllDefaultModelsResponse => {
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
        throw new Error('invalid')
    }

    const data = raw as Record<string, unknown>
    const hasWrappedShape = 'models' in data || 'mapping' in data
    const rawModels = hasWrappedShape ? data.models : data
    const rawMapping = hasWrappedShape ? data.mapping : {}

    if (!rawModels || typeof rawModels !== 'object' || Array.isArray(rawModels)) {
        throw new Error('invalid')
    }
    if (!rawMapping || typeof rawMapping !== 'object' || Array.isArray(rawMapping)) {
        throw new Error('invalid')
    }

    const models: Record<string, string[]> = {}
    for (const [typeId, value] of Object.entries(rawModels as Record<string, unknown>)) {
        if (!isStringArray(value)) {
            throw new Error('invalid')
        }
        models[typeId] = value
    }

    const mapping: Record<string, Record<string, string>> = {}
    for (const [typeId, value] of Object.entries(rawMapping as Record<string, unknown>)) {
        if (!isStringRecord(value)) {
            throw new Error('invalid')
        }
        mapping[typeId] = value
    }

    return { models, mapping }
}

export function DefaultModelsDialog({ open, onOpenChange }: DefaultModelsDialogProps) {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const fileInputRef = useRef<HTMLInputElement>(null)

    const [viewMode, setViewMode] = useState<ViewMode>('list')
    const [editingType, setEditingType] = useState<number>(0)
    const [isCreatingNew, setIsCreatingNew] = useState(false)
    const [editModels, setEditModels] = useState<string[]>([])
    const [editMapping, setEditMapping] = useState<Record<string, string>>({})
    const [isImporting, setIsImporting] = useState(false)
    const [deleteTarget, setDeleteTarget] = useState<{ typeId: number; typeName: string } | null>(null)
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

    const persistDefaults = useCallback(async (data: AllDefaultModelsResponse) => {
        await put('option/', { key: 'DefaultChannelModels', value: JSON.stringify(data.models) })
        await put('option/', { key: 'DefaultChannelModelMapping', value: JSON.stringify(data.mapping) })
        return data
    }, [])

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

            return persistDefaults(current)
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

            return persistDefaults(current)
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

    const importMutation = useMutation({
        mutationFn: async (data: AllDefaultModelsResponse) => persistDefaults(data),
        onSuccess: async (data) => {
            queryClient.setQueryData(['allChannelDefaultModels'], data)
            queryClient.invalidateQueries({ queryKey: ['channelDefaultModels'] })
            toast.success(t('channel.defaultModels.importSuccess'))
            setContentKey((key) => key + 1)
            setViewMode('list')
        },
        onError: () => {
            toast.error(t('channel.defaultModels.importFailed'))
        },
    })

    const handleAdd = useCallback(() => {
        setIsCreatingNew(true)
        setEditingType(0)
        setEditModels([])
        setEditMapping({})
        setViewMode('edit')
    }, [])

    const handleEdit = useCallback((typeId: number) => {
        setIsCreatingNew(false)
        const entry = configuredTypes.find(c => c.typeId === typeId)
        setEditingType(typeId)
        setEditModels(entry?.models ? [...entry.models] : [])
        setEditMapping(entry?.mapping ? { ...entry.mapping } : {})
        setViewMode('edit')
    }, [configuredTypes])

    const handleDelete = useCallback((typeId: number, typeName: string) => {
        setDeleteTarget({ typeId, typeName })
    }, [])

    const handleConfirmDelete = useCallback(() => {
        if (!deleteTarget) return
        deleteMutation.mutate(deleteTarget.typeId, {
            onSettled: () => {
                setDeleteTarget(null)
            },
        })
    }, [deleteMutation, deleteTarget])

    const handleSave = useCallback(() => {
        if (!editingType) return
        saveMutation.mutate({ typeId: editingType, models: editModels, mapping: editMapping })
    }, [editingType, editModels, editMapping, saveMutation])

    const handleBack = useCallback(() => {
        setViewMode('list')
        setIsCreatingNew(false)
    }, [])

    const exportDefaults = useCallback(() => {
        const exportData: AllDefaultModelsResponse = {
            models: { ...(allDefaults?.models || {}) },
            mapping: { ...(allDefaults?.mapping || {}) },
        }

        if (Object.keys(exportData.models).length === 0 && Object.keys(exportData.mapping).length === 0) {
            toast.error(t('channel.defaultModels.noDataToExport'))
            return
        }

        const blob = new Blob([JSON.stringify(exportData, null, 2)], {
            type: 'application/json',
        })
        const url = URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = url
        link.download = `channel_default_models_${new Date().toISOString().slice(0, 10)}.json`
        document.body.appendChild(link)
        link.click()
        document.body.removeChild(link)
        URL.revokeObjectURL(url)
        toast.success(t('channel.defaultModels.exportSuccess'))
    }, [allDefaults, t])

    const triggerImport = useCallback(() => {
        fileInputRef.current?.click()
    }, [])

    const importDefaults = useCallback(async (event: React.ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0]
        if (!file) return

        setIsImporting(true)
        try {
            const text = await file.text()
            const parsed = normalizeDefaultModelsImport(JSON.parse(text))
            await importMutation.mutateAsync(parsed)
        } catch (error) {
            if (error instanceof SyntaxError || (error instanceof Error && error.message === 'invalid')) {
                toast.error(t('channel.defaultModels.invalidFormat'))
            } else if (!(error instanceof Error)) {
                toast.error(t('channel.defaultModels.importFailed'))
            }
        } finally {
            setIsImporting(false)
            if (fileInputRef.current) {
                fileInputRef.current.value = ''
            }
        }
    }, [importMutation, t])

    // Reset view when dialog opens/closes
    const handleOpenChange = useCallback((newOpen: boolean) => {
        if (!newOpen) {
            setViewMode('list')
            setEditingType(0)
            setIsCreatingNew(false)
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
            {/* Action buttons */}
            <div className="flex flex-wrap justify-end gap-2">
                <Button
                    size="sm"
                    variant="outline"
                    onClick={exportDefaults}
                >
                    <Download className="h-3.5 w-3.5 mr-1.5" />
                    {t('channel.defaultModels.export')}
                </Button>
                <Button
                    size="sm"
                    variant="outline"
                    onClick={triggerImport}
                    disabled={isImporting || importMutation.isPending}
                >
                    {isImporting || importMutation.isPending ? (
                        <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" />
                    ) : (
                        <Upload className="h-3.5 w-3.5 mr-1.5" />
                    )}
                    {isImporting || importMutation.isPending
                        ? t('channel.defaultModels.importing')
                        : t('channel.defaultModels.import')}
                </Button>
                <Button
                    size="sm"
                    onClick={handleAdd}
                    disabled={availableTypesForAdd.length === 0}
                    className="flex items-center gap-1"
                >
                    <Plus className="h-3.5 w-3.5" />
                    {t('channel.defaultModels.add')}
                </Button>
                <input
                    ref={fileInputRef}
                    type="file"
                    accept=".json,application/json"
                    className="hidden"
                    onChange={importDefaults}
                />
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
                                            onClick={() => handleDelete(entry.typeId, entry.typeName)}
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
            {isCreatingNew ? (
                <div className="space-y-2">
                    <SingleSelectCombobox
                        key={`channel-default-model-type-${editingType || 0}`}
                        dropdownItems={availableTypesForAdd.map(t => t.name)}
                        initSelectedItem={editingTypeName}
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
            <>
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

                <AlertDialog
                    open={!!deleteTarget}
                    onOpenChange={(nextOpen) => {
                        if (!nextOpen && !deleteMutation.isPending) {
                            setDeleteTarget(null)
                        }
                    }}
                >
                    <AlertDialogContent>
                        <AlertDialogHeader>
                            <AlertDialogTitle>{t('channel.defaultModels.deleteDialog.confirmTitle')}</AlertDialogTitle>
                            <AlertDialogDescription>
                                {t('channel.defaultModels.deleteDialog.confirmDescription', {
                                    typeName: deleteTarget?.typeName ?? '',
                                })}
                            </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                            <AlertDialogCancel disabled={deleteMutation.isPending}>
                                {t('common.cancel')}
                            </AlertDialogCancel>
                            <AlertDialogAction
                                onClick={handleConfirmDelete}
                                disabled={deleteMutation.isPending}
                                className="bg-red-600 hover:bg-red-700"
                            >
                                {deleteMutation.isPending
                                    ? t('channel.deleteDialog.deleting')
                                    : t('channel.deleteDialog.delete')}
                            </AlertDialogAction>
                        </AlertDialogFooter>
                    </AlertDialogContent>
                </AlertDialog>
            </>
        </Dialog>
    )
}
