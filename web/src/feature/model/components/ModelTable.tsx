// src/feature/model/components/ModelTable.tsx
import { useState, useMemo } from 'react'
import { useModels } from '../hooks'
import { ModelConfig } from '@/types/model'
import { Button } from '@/components/ui/button'
import {
    MoreHorizontal, Plus, Trash2, RefreshCcw, Pencil, FileText,
} from 'lucide-react'
import {
    DropdownMenu, DropdownMenuContent,
    DropdownMenuItem, DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Card } from '@/components/ui/card'
import { ModelDialog } from './ModelDialog'
import { DeleteModelDialog } from './DeleteModelDialog'
import { useTranslation } from 'react-i18next'
import { DataTable } from '@/components/table/motion-data-table'
import { ColumnDef } from '@tanstack/react-table'
import { useReactTable, getCoreRowModel, getSortedRowModel } from '@tanstack/react-table'
import { AdvancedErrorDisplay } from '@/components/common/error/errorDisplay'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'
import { AnimatedIcon } from '@/components/ui/animation/components/animated-icon'
import ApiDocDrawer from './api-doc/ApiDoc'
import { Badge } from '@/components/ui/badge'

export function ModelTable() {
    const { t } = useTranslation()

    // State management
    const [modelDialogOpen, setModelDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [selectedModelId, setSelectedModelId] = useState<string | null>(null)
    const [dialogMode, setDialogMode] = useState<'create' | 'update'>('create')
    const [selectedModel, setSelectedModel] = useState<ModelConfig | null>(null)
    const [isRefreshAnimating, setIsRefreshAnimating] = useState(false)

    // API Doc drawer state
    const [apiDocOpen, setApiDocOpen] = useState(false)

    // Get models list
    const {
        data: models,
        isLoading,
        error,
        isError,
        refetch
    } = useModels()

    // Sort models by type for stable sorting
    const sortedModels = useMemo(() => {
        if (!models) return []
        return [...models].sort((a, b) => {
            if (a.type === b.type) {
                // Secondary sort by model name for stability
                return a.model.localeCompare(b.model)
            }
            return a.type - b.type
        })
    }, [models])

    // Create table columns
    const columns: ColumnDef<ModelConfig>[] = [
        {
            accessorKey: 'model',
            header: () => <div className="font-medium py-3.5">{t("model.modelName")}</div>,
            cell: ({ row }) => <div className="font-medium">{row.original.model}</div>,
        },
        {
            accessorKey: 'type',
            header: () => <div className="font-medium py-3.5">{t("model.modelType")}</div>,
            cell: ({ row }) => (
                <div className="font-medium">
                    {/* @ts-expect-error 动态翻译键 */}
                    {t(`modeType.${row.original.type}`)}
                </div>
            ),
        },
        {
            accessorKey: 'plugin',
            header: () => <div className="font-medium py-3.5">{t("model.pluginInfo")}</div>,
            cell: ({ row }) => {
                const plugin = row.original.plugin
                if (!plugin) {
                    return (
                        <div className="text-muted-foreground text-sm">
                            {t("model.noPluginConfigured")}
                        </div>
                    )
                }

                const enabledPlugins = []
                
                if (plugin.cache?.enable) {
                    enabledPlugins.push(t("model.cachePlugin"))
                }
                
                if (plugin["web-search"]?.enable) {
                    enabledPlugins.push(t("model.webSearchPlugin"))
                }
                
                if (plugin["think-split"]?.enable) {
                    enabledPlugins.push(t("model.thinkSplitPlugin"))
                }

                if (enabledPlugins.length === 0) {
                    return (
                        <div className="text-muted-foreground text-sm">
                            {t("model.noPluginConfigured")}
                        </div>
                    )
                }

                return (
                    <div className="flex flex-wrap gap-1">
                        {enabledPlugins.map((pluginName) => (
                            <Badge
                                key={pluginName}
                                variant="outline"
                                className="text-xs bg-green-50 text-green-700 border-green-200 dark:bg-green-900/20 dark:text-green-400 dark:border-green-800"
                            >
                                {pluginName}
                            </Badge>
                        ))}
                    </div>
                )
            },
        },
        // {
        //     accessorKey: 'owner',
        //     header: () => <div className="font-medium py-3.5">{t("model.owner")}</div>,
        //     cell: ({ row }) => <div>{row.original.owner}</div>,
        // },
        // {
        //     accessorKey: 'rpm',
        //     header: () => <div className="font-medium py-3.5">{t("model.rpm")}</div>,
        //     cell: ({ row }) => <div>{row.original.rpm}</div>,
        // },
        {
            id: 'actions',
            cell: ({ row }) => (
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon">
                            <MoreHorizontal className="h-4 w-4" />
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                        <DropdownMenuItem
                            onClick={() => openApiDoc(row.original)}
                        >
                            <FileText className="mr-2 h-4 w-4" />
                            {t("model.apiDetails")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => openUpdateDialog(row.original)}
                        >
                            <Pencil className="mr-2 h-4 w-4" />
                            {t("model.edit")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => openDeleteDialog(row.original.model)}
                        >
                            <Trash2 className="mr-2 h-4 w-4 text-red-600 dark:text-red-500" />
                            {t("model.delete")}
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            ),
        },
    ]

    // Initialize table
    const table = useReactTable({
        data: sortedModels,
        columns,
        getCoreRowModel: getCoreRowModel(),
        getSortedRowModel: getSortedRowModel(),
        initialState: {
            sorting: [
                {
                    id: 'type',
                    desc: false,
                },
            ],
        },
    })

    // Open create model dialog
    const openCreateDialog = () => {
        setDialogMode('create')
        setSelectedModel(null)
        setModelDialogOpen(true)
    }

    // Open update model dialog
    const openUpdateDialog = (model: ModelConfig) => {
        setDialogMode('update')
        setSelectedModel(model)
        setModelDialogOpen(true)
    }

    // Open delete dialog
    const openDeleteDialog = (id: string) => {
        setSelectedModelId(id)
        setDeleteDialogOpen(true)
    }

    // Open API documentation drawer
    const openApiDoc = (model: ModelConfig) => {
        setSelectedModel(model)
        setApiDocOpen(true)
    }

    // Refresh models
    const refreshModels = () => {
        setIsRefreshAnimating(true)
        refetch()

        // Stop animation after 1 second
        setTimeout(() => {
            setIsRefreshAnimating(false)
        }, 1000)
    }

    return (
        <>
            <Card className="border-none shadow-none p-6 flex flex-col h-full">
                {/* Title and action buttons */}
                <div className="flex items-center justify-between mb-6">
                    <h2 className="text-xl font-semibold text-primary">{t("model.management")}</h2>
                    <div className="flex gap-2">
                        <AnimatedButton >
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={refreshModels}
                                className="flex items-center gap-2 justify-center"
                            >
                                <AnimatedIcon animationVariant="continuous-spin" isAnimating={isRefreshAnimating} className="h-4 w-4">
                                    <RefreshCcw className="h-4 w-4" />
                                </AnimatedIcon>
                                {t("model.refresh")}
                            </Button>
                        </AnimatedButton>
                        <AnimatedButton >
                            <Button
                                size="sm"
                                onClick={openCreateDialog}
                                className="flex items-center gap-1"
                            >
                                <Plus className="h-4 w-4" />
                                {t("model.add")}
                            </Button>
                        </AnimatedButton>
                    </div>
                </div>

                {/* Table container */}
                <div className="flex-1 overflow-hidden flex flex-col">
                    <div className="overflow-auto h-full">
                        {isError ? (
                            <AdvancedErrorDisplay error={error} onRetry={refetch} />
                        ) : (
                            <DataTable
                                table={table}
                                columns={columns}
                                isLoading={isLoading}
                                loadingStyle="skeleton"
                                fixedHeader={true}
                                animatedRows={true}
                                showScrollShadows={true}
                            />
                        )}
                    </div>
                </div>
            </Card>

            {/* Model Dialog */}
            <ModelDialog
                open={modelDialogOpen}
                onOpenChange={setModelDialogOpen}
                mode={dialogMode}
                model={selectedModel}
            />

            {/* Delete Model Dialog */}
            <DeleteModelDialog
                open={deleteDialogOpen}
                onOpenChange={setDeleteDialogOpen}
                modelId={selectedModelId}
                onDeleted={() => setSelectedModelId(null)}
            />

            {/* API Documentation Drawer */}

            {selectedModel && (
                <ApiDocDrawer
                    isOpen={apiDocOpen}
                    onClose={() => setApiDocOpen(false)}
                    modelConfig={selectedModel}
                />
            )}
        </>
    )
}