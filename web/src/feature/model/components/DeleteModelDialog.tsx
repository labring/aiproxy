// src/feature/model/components/DeleteModelDialog.tsx
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { useDeleteModel } from '../hooks'
import { AnimatePresence, motion } from "motion/react"
import { useTranslation } from 'react-i18next'
import {
    dialogEnterExitAnimation,
    dialogContentAnimation,
    dialogHeaderAnimation,
    dialogContentItemAnimation
} from '@/components/ui/animation/dialog-animation'
import { AnimatedButton } from "@/components/ui/animation/components/animated-button"

interface DeleteModelDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    modelId: string | null
    onDeleted?: () => void
}

export function DeleteModelDialog({
    open,
    onOpenChange,
    modelId,
    onDeleted
}: DeleteModelDialogProps) {
    const { t } = useTranslation()
    const { deleteModel, isLoading } = useDeleteModel()

    // Handle delete model
    const handleDeleteModel = () => {
        if (!modelId) return

        deleteModel(modelId, {
            onSettled: () => {
                onOpenChange(false)
                onDeleted?.()
            }
        })
    }

    return (
        <AlertDialog open={open} onOpenChange={onOpenChange}>
            <AnimatePresence mode="wait">
                {open && (
                    <motion.div {...dialogEnterExitAnimation}>
                        <AlertDialogContent className="p-0 overflow-hidden">
                            <motion.div {...dialogContentAnimation}>
                                <motion.div {...dialogHeaderAnimation}>
                                    <AlertDialogHeader className="p-6 pb-3">
                                        <AlertDialogTitle className="text-xl">{t("model.deleteDialog.confirmTitle")}</AlertDialogTitle>
                                        <AlertDialogDescription>
                                            {t("model.deleteDialog.confirmDescription")}
                                        </AlertDialogDescription>
                                    </AlertDialogHeader>
                                </motion.div>

                                <motion.div
                                    {...dialogContentItemAnimation}
                                    className="px-6 pb-6"
                                >
                                    <AlertDialogFooter className="mt-2 flex justify-end space-x-2">
                                        <AnimatedButton >
                                            <AlertDialogCancel>{t("model.deleteDialog.cancel")}</AlertDialogCancel>
                                        </AnimatedButton>
                                        <AnimatedButton >
                                            <AlertDialogAction
                                                onClick={handleDeleteModel}
                                                disabled={isLoading}
                                                className="bg-red-600 hover:bg-red-700"
                                            >
                                                {isLoading ? t("model.deleteDialog.deleting") : t("model.deleteDialog.delete")}
                                            </AlertDialogAction>
                                        </AnimatedButton>
                                    </AlertDialogFooter>
                                </motion.div>
                            </motion.div>
                        </AlertDialogContent>
                    </motion.div>
                )}
            </AnimatePresence>
        </AlertDialog>
    )
}