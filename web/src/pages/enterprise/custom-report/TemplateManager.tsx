import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Pencil, Trash2, Play, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogFooter,
} from "@/components/ui/dialog"
import { enterpriseApi, type SavedTemplate } from "@/api/enterprise"

interface TemplateManagerProps {
    onApply: (dimensions: string[], measures: string[], chartType?: string, viewMode?: string) => void
    currentDimensions: string[]
    currentMeasures: string[]
    currentChartType: string
    currentViewMode: string
    saveDialogOpen?: boolean
    onSaveDialogChange?: (open: boolean) => void
}

export function TemplateManager({
    onApply,
    currentDimensions,
    currentMeasures,
    currentChartType,
    currentViewMode,
    saveDialogOpen: externalSaveOpen,
    onSaveDialogChange,
}: TemplateManagerProps) {
    const { t } = useTranslation()
    const queryClient = useQueryClient()

    const [internalSaveOpen, setInternalSaveOpen] = useState(false)
    const saveDialogOpen = externalSaveOpen ?? internalSaveOpen
    const setSaveDialogOpen = onSaveDialogChange ?? setInternalSaveOpen
    const [editingId, setEditingId] = useState<number | null>(null)
    const [templateName, setTemplateName] = useState("")
    const [deleteConfirm, setDeleteConfirm] = useState<number | null>(null)

    const { data: templates = [], isLoading } = useQuery({
        queryKey: ["report-templates"],
        queryFn: () => enterpriseApi.listReportTemplates(),
        staleTime: 60_000,
    })

    const createMut = useMutation({
        mutationFn: () =>
            enterpriseApi.createReportTemplate({
                name: templateName,
                dimensions: currentDimensions,
                measures: currentMeasures,
                chart_type: currentChartType,
                view_mode: currentViewMode,
            }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["report-templates"] })
            setSaveDialogOpen(false)
            setTemplateName("")
        },
    })

    const updateMut = useMutation({
        mutationFn: ({ id, name }: { id: number; name: string }) =>
            enterpriseApi.updateReportTemplate(id, { name }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["report-templates"] })
            setEditingId(null)
            setTemplateName("")
        },
    })

    const deleteMut = useMutation({
        mutationFn: (id: number) => enterpriseApi.deleteReportTemplate(id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["report-templates"] })
            setDeleteConfirm(null)
        },
    })

    const handleApply = (tpl: SavedTemplate) => {
        try {
            const dims = JSON.parse(tpl.dimensions) as string[]
            const meas = JSON.parse(tpl.measures) as string[]
            onApply(dims, meas, tpl.chart_type, tpl.view_mode)
        } catch {
            // ignore parse errors
        }
    }

    if (isLoading) {
        return <Loader2 className="w-3.5 h-3.5 animate-spin text-muted-foreground mx-auto my-2" />
    }

    return (
        <>
            {/* My templates list */}
            {templates.length > 0 && (
                <div className="space-y-1 mt-2">
                    <div className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
                        {t("enterprise.customReport.myTemplates", "My Templates")}
                    </div>
                    {templates.map((tpl) => (
                        <div
                            key={tpl.id}
                            className="flex items-center gap-1 group"
                        >
                            {editingId === tpl.id ? (
                                <form
                                    className="flex-1 flex gap-1"
                                    onSubmit={(e) => {
                                        e.preventDefault()
                                        if (templateName.trim()) {
                                            updateMut.mutate({ id: tpl.id, name: templateName.trim() })
                                        }
                                    }}
                                >
                                    <Input
                                        value={templateName}
                                        onChange={(e) => setTemplateName(e.target.value)}
                                        className="h-6 text-xs flex-1"
                                        autoFocus
                                    />
                                    <Button type="submit" size="sm" className="h-6 text-xs px-2" disabled={updateMut.isPending}>
                                        OK
                                    </Button>
                                </form>
                            ) : (
                                <>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        className="h-6 text-xs flex-1 justify-start px-2 font-normal"
                                        onClick={() => handleApply(tpl)}
                                    >
                                        <Play className="w-3 h-3 mr-1 text-[#6A6DE6]" />
                                        {tpl.name}
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        size="icon"
                                        className="h-5 w-5 opacity-0 group-hover:opacity-100"
                                        onClick={() => {
                                            setEditingId(tpl.id)
                                            setTemplateName(tpl.name)
                                        }}
                                    >
                                        <Pencil className="w-3 h-3" />
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        size="icon"
                                        className="h-5 w-5 opacity-0 group-hover:opacity-100 hover:text-destructive"
                                        onClick={() => setDeleteConfirm(tpl.id)}
                                    >
                                        <Trash2 className="w-3 h-3" />
                                    </Button>
                                </>
                            )}
                        </div>
                    ))}
                </div>
            )}

            {/* Save dialog */}
            <Dialog open={saveDialogOpen} onOpenChange={setSaveDialogOpen}>
                <DialogContent className="sm:max-w-[360px]">
                    <DialogHeader>
                        <DialogTitle>{t("enterprise.customReport.saveTemplate", "Save Template")}</DialogTitle>
                    </DialogHeader>
                    <Input
                        value={templateName}
                        onChange={(e) => setTemplateName(e.target.value)}
                        placeholder={t("enterprise.customReport.templateName", "Template name")}
                        autoFocus
                    />
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setSaveDialogOpen(false)}>
                            {t("common.cancel", "Cancel")}
                        </Button>
                        <Button
                            onClick={() => createMut.mutate()}
                            disabled={!templateName.trim() || createMut.isPending}
                        >
                            {createMut.isPending && <Loader2 className="w-4 h-4 mr-2 animate-spin" />}
                            {t("common.save", "Save")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Delete confirm dialog */}
            <Dialog open={deleteConfirm !== null} onOpenChange={() => setDeleteConfirm(null)}>
                <DialogContent className="sm:max-w-[360px]">
                    <DialogHeader>
                        <DialogTitle>{t("enterprise.customReport.deleteTemplate", "Delete Template")}</DialogTitle>
                    </DialogHeader>
                    <p className="text-sm text-muted-foreground">
                        {t("enterprise.customReport.deleteTemplateConfirm", "Are you sure you want to delete this template?")}
                    </p>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setDeleteConfirm(null)}>
                            {t("common.cancel", "Cancel")}
                        </Button>
                        <Button
                            variant="destructive"
                            onClick={() => deleteConfirm && deleteMut.mutate(deleteConfirm)}
                            disabled={deleteMut.isPending}
                        >
                            {t("common.delete", "Delete")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    )
}

// Export a hook-like function to open the save dialog from parent
export function useTemplateManager() {
    const [saveOpen, setSaveOpen] = useState(false)
    return { saveOpen, openSave: () => setSaveOpen(true), closeSave: () => setSaveOpen(false) }
}
