// src/feature/model/components/ModelDialog.tsx
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle
} from '@/components/ui/dialog'
import { ModelForm } from './ModelForm'
import { ModelConfig } from '@/types/model'
import { useTranslation } from 'react-i18next'
import { useMemo } from 'react'

interface ModelDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    mode: 'create' | 'update'
    model?: ModelConfig | null
    baseModelConfig?: ModelConfig | null
    preserveModelNameOnCreate?: boolean
}

export function ModelDialog({
    open,
    onOpenChange,
    mode = 'create',
    model = null,
    baseModelConfig = model,
    preserveModelNameOnCreate = false
}: ModelDialogProps) {
    const { t } = useTranslation()

    // Determine title and description based on mode
    const title = mode === 'create' ? t("model.dialog.createTitle") : t("model.dialog.updateTitle")
    const description = mode === 'create'
        ? t("model.dialog.createDescription")
        : t("model.dialog.updateDescription")

    // Default values for form - use model data if available (for both update and copy)
    const defaultValues = useMemo(() => model
        ? {
            ...model,
            model: mode === 'create' && !preserveModelNameOnCreate ? '' : model.model,
            owner: model.owner ?? '',
            type: model.type,
            timeout: model.timeout_config?.request_timeout,
            stream_timeout: model.timeout_config?.stream_request_timeout,
            price: model.price,
            plugin: model.plugin
        }
        : {
            model: '',
            owner: '',
            type: 1
        }, [mode, model, preserveModelNameOnCreate])

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            {open && <DialogContent className="max-w-4xl gap-0 p-0">
                <DialogHeader className="border-b px-5 py-4">
                    <DialogTitle>{title}</DialogTitle>
                    <DialogDescription>{description}</DialogDescription>
                </DialogHeader>
                <div className="px-5 pt-5">
                    <ModelForm
                                        mode={mode}
                                        defaultValues={defaultValues}
                                        baseModelConfig={baseModelConfig}
                                        onSuccess={() => onOpenChange(false)}
                                    />
                </div>
            </DialogContent>}
        </Dialog>
    )
}
