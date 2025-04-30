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
import { useTranslation } from 'react-i18next'
import {
    dialogEnterExitAnimation,
    dialogContentAnimation,
    dialogHeaderAnimation,
    dialogContentItemAnimation
} from '@/components/ui/animation/dialog-animation'

interface ChannelDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    mode: 'create' | 'update'
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
    const title = mode === 'create' ? t("channel.dialog.createTitle") : t("channel.dialog.updateTitle")
    const description = mode === 'create'
        ? t("channel.dialog.createDescription")
        : t("channel.dialog.updateDescription")

    // Default values for form
    const defaultValues = mode === 'update' && channel
        ? {
            type: channel.type,
            name: channel.name,
            key: channel.key,
            base_url: channel.base_url,
            models: channel.models || [],
            model_mapping: channel.model_mapping || {}
        }
        : {
            type: 0,
            name: '',
            key: '',
            base_url: '',
            models: [],
            model_mapping: {}
        }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <AnimatePresence mode="wait">
                {open && (
                    <motion.div {...dialogEnterExitAnimation}>
                        <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto p-0">
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
                                    <ChannelForm
                                        mode={mode}
                                        channelId={channel?.id}
                                        channel={channel}
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