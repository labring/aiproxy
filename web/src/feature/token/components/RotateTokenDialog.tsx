// src/feature/token/components/RotateTokenDialog.tsx
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Loader2, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { tokenApi } from '@/api/token'
import { TOKEN_ROTATE_MODES, TokenRotateMode, Token, isHashedToken } from '@/types/token'
import { TokenOneTimeKeyDialog } from './TokenOneTimeKeyDialog'

interface RotateTokenDialogProps {
    token: Pick<Token, 'id' | 'name' | 'key_preview'>
    group?: string
    onClose: () => void
}

export function RotateTokenDialog({ token, group, onClose }: RotateTokenDialogProps) {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const [mode, setMode] = useState<TokenRotateMode>(TOKEN_ROTATE_MODES.KEEP)
    const [newKey, setNewKey] = useState<string | null>(null)

    const mutation = useMutation({
        mutationFn: (rotateMode: TokenRotateMode) => {
            return group
                ? tokenApi.rotateGroupToken(group, token.id, rotateMode)
                : tokenApi.rotateToken(token.id, rotateMode)
        },
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ['tokens'] })
            queryClient.invalidateQueries({ queryKey: ['groupTokens'] })
            toast.success(t('token.rotateDialog.success'))
            setNewKey(data.key)
        },
        onError: (err: Error) => {
            toast.error(err.message || t('token.rotateDialog.failure'))
        },
    })

    const closeRotate = () => {
        if (!newKey) onClose()
    }

    return (
        <>
            <Dialog open onOpenChange={(open) => { if (!open) closeRotate() }}>
                <DialogContent className="sm:max-w-[425px]">
                    <DialogHeader>
                        <DialogTitle>{t('token.rotateDialog.title')}</DialogTitle>
                        <DialogDescription>
                            {t('token.rotateDialog.description', { name: token.name })}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 py-2">
                        <div className="space-y-2">
                            <Label>{t('token.rotateDialog.mode')}</Label>
                            <Select
                                value={mode || 'keep'}
                                onValueChange={(value) => {
                                    setMode(value === 'keep' ? TOKEN_ROTATE_MODES.KEEP : value as TokenRotateMode)
                                }}
                                disabled={mutation.isPending}
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="keep">{t('token.rotateDialog.modeKeep')}</SelectItem>
                                    <SelectItem value="hash">{t('token.rotateDialog.modeHash')}</SelectItem>
                                    <SelectItem value="plain">{t('token.rotateDialog.modePlain')}</SelectItem>
                                </SelectContent>
                            </Select>
                            <p className="text-xs text-muted-foreground">
                                {isHashedToken(token)
                                    ? t('token.rotateDialog.currentHashed')
                                    : t('token.rotateDialog.currentPlain')}
                            </p>
                        </div>
                        <p className="text-sm text-muted-foreground">
                            {t('token.rotateDialog.warning')}
                        </p>
                        <div className="flex justify-end gap-2">
                            <Button
                                variant="outline"
                                onClick={onClose}
                                disabled={mutation.isPending}
                            >
                                {t('common.cancel')}
                            </Button>
                            <Button
                                onClick={() => mutation.mutate(mode)}
                                disabled={mutation.isPending}
                            >
                                {mutation.isPending
                                    ? <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    : <RefreshCw className="mr-2 h-4 w-4" />}
                                {t('token.rotateDialog.submit')}
                            </Button>
                        </div>
                    </div>
                </DialogContent>
            </Dialog>

            {newKey && (
                <TokenOneTimeKeyDialog
                    open
                    onOpenChange={(open) => { if (!open) onClose() }}
                    tokenKey={newKey}
                />
            )}
        </>
    )
}
