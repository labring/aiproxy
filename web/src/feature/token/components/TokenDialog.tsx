// src/feature/token/components/TokenDialog.tsx
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle
} from '@/components/ui/dialog'
import { TokenForm } from './TokenForm'
import { useTranslation } from 'react-i18next'

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
            {open && <DialogContent className="max-w-xl gap-0 p-0">
                <DialogHeader className="border-b px-5 py-4">
                    <DialogTitle>{title}</DialogTitle>
                    <DialogDescription>{description}</DialogDescription>
                </DialogHeader>
                <div className="px-5 pt-5">
                    <TokenForm
                                        onSuccess={() => onOpenChange(false)}
                                    />
                </div>
            </DialogContent>}
        </Dialog>
    )
}
