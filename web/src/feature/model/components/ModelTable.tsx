// src/feature/model/components/ModelTable.tsx
import { useState } from 'react'
import { useModels } from '../hooks'
import { ModelConfig } from '@/types/model'
import { Button } from '@/components/ui/button'
import {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    MoreHorizontal, Plus, Trash2, RefreshCcw, Pencil
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
import { useReactTable, getCoreRowModel } from '@tanstack/react-table'
import { AdvancedErrorDisplay } from '@/components/common/error/errorDisplay'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'
import { AnimatedIcon } from '@/components/ui/animation/components/animated-icon'

export function ModelTable() {
    const { t } = useTranslation()

    // State management
    const [modelDialogOpen, setModelDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [selectedModelId, setSelectedModelId] = useState<string | null>(null)
    const [dialogMode, setDialogMode] = useState<'create' | 'update'>('create')
    const [selectedModel, setSelectedModel] = useState<ModelConfig | null>(null)
    const [isRefreshAnimating, setIsRefreshAnimating] = useState(false)

    // Get models list
    const {
        data: models,
        isLoading,
        error,
        isError,
        refetch
    } = useModels()

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
                        {/* <DropdownMenuItem
                            onClick={() => openUpdateDialog(row.original)}
                        >
                            <Pencil className="mr-2 h-4 w-4" />
                            {t("model.edit")}
                        </DropdownMenuItem> */}
                        <DropdownMenuItem
                            onClick={() => openDeleteDialog(row.original.model)}
                            className="text-red-600"
                        >
                            <Trash2 className="mr-2 h-4 w-4" />
                            {t("model.delete")}
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            ),
        },
    ]

    // Initialize table
    const table = useReactTable({
        data: models || [],
        columns,
        getCoreRowModel: getCoreRowModel(),
    })

    // Open create model dialog
    const openCreateDialog = () => {
        setDialogMode('create')
        setSelectedModel(null)
        setModelDialogOpen(true)
    }

    // Open update model dialog
    // const openUpdateDialog = (model: ModelConfig) => {
    //     setDialogMode('update')
    //     setSelectedModel(model)
    //     setModelDialogOpen(true)
    // }

    // Open delete dialog
    const openDeleteDialog = (id: string) => {
        setSelectedModelId(id)
        setDeleteDialogOpen(true)
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
        </>
    )
}