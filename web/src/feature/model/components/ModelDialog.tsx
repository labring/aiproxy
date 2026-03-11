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
import { AnimatePresence, motion } from "motion/react"
import { useTranslation } from 'react-i18next'
import {
    dialogEnterExitAnimation,
    dialogContentAnimation,
    dialogHeaderAnimation,
    dialogContentItemAnimation
} from '@/components/ui/animation/dialog-animation'

interface ModelDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    mode: 'create' | 'update'
    model?: ModelConfig | null
}

export function ModelDialog({
    open,
    onOpenChange,
    mode = 'create',
    model = null
}: ModelDialogProps) {
    const { t } = useTranslation()

    // Determine title and description based on mode
    const title = mode === 'create' ? t("model.dialog.createTitle") : t("model.dialog.updateTitle")
    const description = mode === 'create'
        ? t("model.dialog.createDescription")
        : t("model.dialog.updateDescription")

    // Default values for form - use model data if available (for both update and copy)
    const defaultValues = model
        ? {
            model: mode === 'create' ? '' : model.model, // Clear model name for copy mode
            owner: model.owner ?? '',
            type: model.type,
            rpm: model.rpm,
            tpm: model.tpm,
            retry_times: model.retry_times,
            timeout: model.timeout,
            max_error_rate: model.max_error_rate,
            force_save_detail: model.force_save_detail,
            summary_service_tier: model.summary_service_tier ?? false,
            summary_claude_long_context: model.summary_claude_long_context ?? false,
            price: model.price,
            plugin: model.plugin
        }
        : {
            model: '',
            owner: '',
            type: 1
        }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <AnimatePresence mode="wait">
                {open && (
                    <motion.div {...dialogEnterExitAnimation}>
                        <DialogContent className="max-w-md max-h-[85vh] overflow-y-auto p-0">
                            <motion.div {...dialogContentAnimation}>
                                <motion.div {...dialogHeaderAnimation}>
                                    <DialogHeader className="p-6 pb-3">
                                        <DialogTitle className="text-xl">{title}</DialogTitle>
                                        <DialogDescription>{description}</DialogDescription>
                                    </DialogHeader>
                                </motion.div>

                                <motion.div
                                    {...dialogContentItemAnimation}
                                    className="px-6 pb-6"
                                >
                                    <ModelForm
                                        mode={mode}
                                        defaultValues={defaultValues}
                                        onSuccess={() => onOpenChange(false)}
                                    />
                                </motion.div>
                            </motion.div>
                        </DialogContent>
                    </motion.div>
                )}
            </AnimatePresence>
        </Dialog>
    )
}
