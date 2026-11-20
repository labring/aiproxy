import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
    Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Token } from '@/types/token'
import { useUpdateToken } from '../hooks'

interface RenameTokenDialogProps {
    token: Pick<Token, 'id' | 'name'>
    onClose: () => void
}

export function RenameTokenDialog({ token, onClose }: RenameTokenDialogProps) {
    const { t } = useTranslation()
    const inputId = useId()
    const [name, setName] = useState(token.name)
    const { updateToken, isLoading, error } = useUpdateToken()
    const trimmedName = name.trim()
    const tooLong = new TextEncoder().encode(trimmedName).length > 32
    const canSubmit = trimmedName.length > 0 && !tooLong && !isLoading
    const errorMessage = tooLong ? t('token.renameDialog.nameTooLong') : error?.message

    return (
        <Dialog open onOpenChange={(open) => { if (!open) onClose() }}>
            <DialogContent className="sm:max-w-[425px]">
                <DialogHeader>
                    <DialogTitle>{t('token.rename')}</DialogTitle>
                    <DialogDescription>{t('token.renameDialog.description')}</DialogDescription>
                </DialogHeader>
                <form className="space-y-4" onSubmit={(event) => {
                    event.preventDefault()
                    if (canSubmit) {
                        updateToken({ id: token.id, data: { name: trimmedName } }, { onSuccess: onClose })
                    }
                }}>
                    <div className="space-y-2">
                        <Label htmlFor={inputId}>{t('token.name')}</Label>
                        <Input
                            id={inputId}
                            value={name}
                            autoFocus
                            disabled={isLoading}
                            aria-invalid={tooLong}
                            aria-describedby={errorMessage ? `${inputId}-error` : undefined}
                            onChange={(event) => setName(event.target.value)}
                        />
                        {errorMessage && (
                            <p id={`${inputId}-error`} role="alert" className="text-sm text-destructive">
                                {errorMessage}
                            </p>
                        )}
                    </div>
                    <DialogFooter>
                        <Button type="button" variant="outline" disabled={isLoading} onClick={onClose}>
                            {t('common.cancel')}
                        </Button>
                        <Button type="submit" disabled={!canSubmit}>
                            {isLoading ? t('common.loading') : t('common.save')}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    )
}
