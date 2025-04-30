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

    // Default values for form
    const defaultValues = mode === 'update' && model
        ? {
            model: model.model,
            type: model.type
        }
        : {
            model: '',
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
                                        modelId={model?.model}
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