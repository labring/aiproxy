// src/feature/token/components/DeleteTokenDialog.tsx
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
import { useDeleteToken } from '../hooks'
import { AnimatePresence, motion } from "motion/react"
import { useTranslation } from 'react-i18next'
import {
    dialogEnterExitAnimation,
    dialogContentAnimation,
    dialogHeaderAnimation,
    dialogContentItemAnimation
} from '@/components/ui/animation/dialog-animation'
import { AnimatedButton } from "@/components/ui/animation/components/animated-button"

interface DeleteTokenDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    tokenId: number | null
    onDeleted?: () => void
}

export function DeleteTokenDialog({
    open,
    onOpenChange,
    tokenId,
    onDeleted
}: DeleteTokenDialogProps) {
    const { t } = useTranslation()
    const { deleteToken, isLoading } = useDeleteToken()

    // 处理删除token
    const handleDeleteToken = () => {
        if (!tokenId) return

        deleteToken(tokenId, {
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
                                        <AlertDialogTitle className="text-xl">{t("token.deleteDialog.confirmTitle")}</AlertDialogTitle>
                                        <AlertDialogDescription>
                                            {t("token.deleteDialog.confirmDescription")}
                                        </AlertDialogDescription>
                                    </AlertDialogHeader>
                                </motion.div>

                                <motion.div
                                    {...dialogContentItemAnimation}
                                    className="px-6 pb-6"
                                >
                                    <AlertDialogFooter className="mt-2 flex justify-end space-x-2">
                                        <AnimatedButton>
                                            <AlertDialogCancel>{t("token.deleteDialog.cancel")}</AlertDialogCancel>
                                        </AnimatedButton>
                                        <AnimatedButton>
                                            <AlertDialogAction
                                                onClick={handleDeleteToken}
                                                disabled={isLoading}
                                                className="bg-red-600 hover:bg-red-700"
                                            >
                                                {isLoading ? t("token.deleteDialog.deleting") : t("token.deleteDialog.delete")}
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