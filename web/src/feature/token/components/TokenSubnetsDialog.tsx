import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
    Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import type { Token } from '@/types/token'
import { parseSubnets } from '@/validation/token'
import { useUpdateToken } from '../hooks'

interface TokenSubnetsDialogProps {
    token: Pick<Token, 'id' | 'subnets'>
    onClose: () => void
}

export function TokenSubnetsDialog({ token, onClose }: TokenSubnetsDialogProps) {
    const { t } = useTranslation()
    const textareaId = useId()
    const [text, setText] = useState((token.subnets ?? []).join(', '))
    const { updateToken, isLoading, error } = useUpdateToken()
    const { subnets, invalid } = parseSubnets(text)
    const canSubmit = invalid === undefined && !isLoading
    const errorMessage = invalid !== undefined
        ? t('token.ipLimitDialog.invalid', { invalid })
        : error?.message

    return (
        <Dialog open onOpenChange={(open) => { if (!open) onClose() }}>
            <DialogContent className="sm:max-w-[425px]">
                <DialogHeader>
                    <DialogTitle>{t('token.ipLimit')}</DialogTitle>
                    <DialogDescription>{t('token.ipLimitDialog.description')}</DialogDescription>
                </DialogHeader>
                <form className="space-y-4" onSubmit={(event) => {
                    event.preventDefault()
                    if (canSubmit) {
                        updateToken({ id: token.id, data: { subnets } }, { onSuccess: onClose })
                    }
                }}>
                    <div className="space-y-2">
                        <Label htmlFor={textareaId}>{t('token.ipLimit')}</Label>
                        <Textarea
                            id={textareaId}
                            value={text}
                            autoFocus
                            rows={3}
                            disabled={isLoading}
                            aria-invalid={invalid !== undefined}
                            aria-describedby={errorMessage ? `${textareaId}-error` : undefined}
                            placeholder={t('token.ipLimitDialog.placeholder')}
                            onChange={(event) => setText(event.target.value)}
                        />
                        {errorMessage && (
                            <p id={`${textareaId}-error`} role="alert" className="text-sm text-destructive">
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
