// src/feature/token/components/TokenDialog.tsx
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle
} from '@/components/ui/dialog'
import { TokenForm } from './TokenForm'
import { AnimatePresence, motion } from "motion/react"
import { useTranslation } from 'react-i18next'
import {
    dialogEnterExitAnimation,
    dialogContentAnimation,
    dialogHeaderAnimation,
    dialogContentItemAnimation
} from '@/components/ui/animation/dialog-animation'

interface TokenDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
}

export function TokenDialog({
    open,
    onOpenChange
}: TokenDialogProps) {
    const { t } = useTranslation()

    // 标题和描述
    const title = t("token.dialog.createTitle")
    const description = t("token.dialog.createDescription")

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
                                    <TokenForm
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