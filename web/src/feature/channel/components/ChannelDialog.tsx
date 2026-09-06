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
import { useMemo, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { getChannelFormDefaults } from './channel-form-defaults'

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
            {open && <DialogContent className="max-w-3xl gap-0 p-0">
                <DialogHeader className="border-b px-5 py-4">
                    <DialogTitle>{title}</DialogTitle>
                    <DialogDescription>{description}</DialogDescription>
                </DialogHeader>
                <div className="px-5 pt-5">
                    <ChannelForm
                                        mode={mode}
                                        channelId={channel?.id}
                                        channel={channel}
                                        defaultValues={defaultValues}
                                        onSuccess={handleSuccess}
                                    />
                </div>
            </DialogContent>}
        </Dialog>
    )
}
