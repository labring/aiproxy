// src/feature/channel/components/DeleteChannelDialog.tsx
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
import { useDeleteChannel } from '../hooks'
import { AnimatePresence, motion } from "motion/react"
import { useTranslation } from 'react-i18next'
import {
    dialogEnterExitAnimation,
    dialogContentAnimation,
    dialogHeaderAnimation,
    dialogContentItemAnimation
} from '@/components/ui/animation/dialog-animation'
import { AnimatedButton } from "@/components/ui/animation/components/animated-button"

interface DeleteChannelDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    channelId: number | null
    onDeleted?: () => void
}

export function DeleteChannelDialog({
    open,
    onOpenChange,
    channelId,
    onDeleted
}: DeleteChannelDialogProps) {
    const { t } = useTranslation()
    const { deleteChannel, isLoading } = useDeleteChannel()

    // Handle delete channel
    const handleDeleteChannel = () => {
        if (!channelId) return

        deleteChannel(channelId, {
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
                                        <AlertDialogTitle className="text-xl">{t("channel.deleteDialog.confirmTitle")}</AlertDialogTitle>
                                        <AlertDialogDescription>
                                            {t("channel.deleteDialog.confirmDescription")}
                                        </AlertDialogDescription>
                                    </AlertDialogHeader>
                                </motion.div>

                                <motion.div
                                    {...dialogContentItemAnimation}
                                    className="px-6 pb-6"
                                >
                                    <AlertDialogFooter className="mt-2 flex justify-end space-x-2">
                                        <AnimatedButton >
                                            <AlertDialogCancel>{t("channel.deleteDialog.cancel")}</AlertDialogCancel>
                                        </AnimatedButton>
                                        <AnimatedButton >
                                            <AlertDialogAction
                                                onClick={handleDeleteChannel}
                                                disabled={isLoading}
                                                className="bg-red-600 hover:bg-red-700"
                                            >
                                                {isLoading ? t("channel.deleteDialog.deleting") : t("channel.deleteDialog.delete")}
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