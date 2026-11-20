// src/feature/token/components/TokenOneTimeKeyDialog.tsx
import { useTranslation } from 'react-i18next'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Copy } from 'lucide-react'
import { toast } from 'sonner'
import { writeTextToClipboard } from '@/lib/clipboard'

interface TokenOneTimeKeyDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    tokenKey: string
}

// 明文密钥仅展示一次：创建哈希模式密钥与轮转密钥后使用
export function TokenOneTimeKeyDialog({
    open,
    onOpenChange,
    tokenKey,
}: TokenOneTimeKeyDialogProps) {
    const { t } = useTranslation()

    const copyKey = () => {
        writeTextToClipboard(tokenKey).then(() => {
            toast.success(t('common.copied'))
        }).catch(() => {
            toast.error(t('common.copyFailed'))
        })
    }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-[425px]">
                <DialogHeader>
                    <DialogTitle>{t('token.oneTimeKey.title')}</DialogTitle>
                    <DialogDescription>{t('token.oneTimeKey.description')}</DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-2">
                    <div className="rounded-md border bg-muted/50 px-3 py-2.5 font-mono text-sm break-all select-all">
                        {tokenKey}
                    </div>
                    <p className="text-sm text-yellow-600 dark:text-yellow-500">
                        {t('token.oneTimeKey.warning')}
                    </p>
                    <div className="flex justify-end gap-2">
                        <Button variant="outline" onClick={copyKey}>
                            <Copy className="mr-2 h-4 w-4" />
                            {t('token.copyKey')}
                        </Button>
                        <Button onClick={() => onOpenChange(false)}>
                            {t('token.oneTimeKey.acknowledge')}
                        </Button>
                    </div>
                </div>
            </DialogContent>
        </Dialog>
    )
}
