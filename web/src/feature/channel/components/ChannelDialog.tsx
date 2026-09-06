// src/feature/channel/components/ChannelDialog.tsx
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle
} from '@/components/ui/dialog'
import { ChannelForm } from './ChannelForm'
import { Channel } from '@/types/channel'
import { AnimatePresence, motion } from "motion/react"
import { useMemo, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { getChannelFormDefaults } from './channel-form-defaults'
import {
    dialogEnterExitAnimation,
    dialogContentAnimation,
    dialogHeaderAnimation,
    dialogContentItemAnimation
} from '@/components/ui/animation/dialog-animation'

interface ChannelDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    mode: 'create' | 'update' | 'copy'
    channel?: Channel | null
}

export function ChannelDialog({
    open,
    onOpenChange,
    mode = 'create',
    channel = null
}: ChannelDialogProps) {
    const { t } = useTranslation()

    // Determine title and description based on mode
    const title = mode === 'update'
        ? t("channel.dialog.updateTitle")
        : mode === 'copy'
            ? t("channel.dialog.copyTitle")
            : t("channel.dialog.createTitle")
    const description = mode === 'update'
        ? t("channel.dialog.updateDescription")
        : mode === 'copy'
            ? t("channel.dialog.copyDescription")
            : t("channel.dialog.createDescription")

    // Default values for form - memoized to avoid new object reference every render
    // Use channel data if available (for both update and copy)
    const defaultValues = useMemo(() => getChannelFormDefaults(channel), [channel])

    const handleSuccess = useCallback(() => onOpenChange(false), [onOpenChange])

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <AnimatePresence mode="wait">
                {open && (
                    <motion.div {...dialogEnterExitAnimation}>
                        <DialogContent className="max-h-[90vh] w-[calc(100vw-1.5rem)] max-w-3xl overflow-y-auto rounded-2xl border-border/70 p-0 shadow-2xl">
                            <motion.div {...dialogContentAnimation}>
                                <motion.div {...dialogHeaderAnimation}>
                                    <DialogHeader className="border-b border-border/60 bg-gradient-to-br from-primary/[0.06] via-transparent to-transparent p-5 pb-4 sm:p-6">
                                        <DialogTitle className="text-xl">{title}</DialogTitle>
                                        <DialogDescription>{description}</DialogDescription>
                                    </DialogHeader>
                                </motion.div>

                                <motion.div
                                    {...dialogContentItemAnimation}
                                    className="px-6 pb-6"
                                >
                                    <ChannelForm
                                        mode={mode}
                                        channelId={channel?.id}
                                        channel={channel}
                                        defaultValues={defaultValues}
                                        onSuccess={handleSuccess}
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
