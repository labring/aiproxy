// src/feature/group/components/CreateGroupTokenDialog.tsx
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Loader2 } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { tokenApi } from '@/api/token'
import { toast } from 'sonner'
import { writeTextToClipboard } from '@/lib/clipboard'
import { Switch } from '@/components/ui/switch'
import { Token } from '@/types/token'
import { dateTimeLocalToTimestampMs } from '@/lib/datetime'
import { TokenOneTimeKeyDialog } from '@/feature/token/components/TokenOneTimeKeyDialog'

interface CreateGroupTokenDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    groupId: string | null
    onCreated?: () => void
}

export function CreateGroupTokenDialog({
    open,
    onOpenChange,
    groupId,
    onCreated,
}: CreateGroupTokenDialogProps) {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const [name, setName] = useState('')
    const [hashedKey, setHashedKey] = useState(false)
    const [quota, setQuota] = useState<number | undefined>(undefined)
    const [periodQuota, setPeriodQuota] = useState<number | undefined>(undefined)
    const [periodType, setPeriodType] = useState<string | null>(null)
    const [expiredAt, setExpiredAt] = useState('')
    const [createdToken, setCreatedToken] = useState<Token | null>(null)

    const mutation = useMutation({
        mutationFn: (data: { group: string; name: string; hashedKey: boolean; quota?: number; periodQuota?: number; periodType: string | null; expiredAt: string }) => {
            return tokenApi.createGroupToken(data.group, {
                name: data.name,
                hashed_key: data.hashedKey,
                quota: data.quota,
                period_quota: data.periodType ? data.periodQuota : undefined,
                period_type: data.periodType || undefined,
                expired_at: dateTimeLocalToTimestampMs(data.expiredAt),
            })
        },
        onSuccess: (data, variables) => {
            queryClient.invalidateQueries({ queryKey: ['groupTokens'] })
            queryClient.invalidateQueries({ queryKey: ['tokens'] })
            queryClient.invalidateQueries({ queryKey: ['groups'] })
            toast.success(t('common.success'))
            // 哈希模式密钥仅此一次可见，先展示再关闭（明文模式两种字段都有，按请求标志判断）
            if (variables.hashedKey) {
                setCreatedToken(data)
                return
            }
            if (data?.key) {
                writeTextToClipboard(data.key).then(() => {
                    toast.success(t('common.copied'))
                }).catch(() => {
                    toast.error(t('common.copyFailed'))
                })
            }
            resetForm()
            onOpenChange(false)
            onCreated?.()
        },
        onError: (err: Error) => {
            toast.error(err.message || 'Failed to create token')
        },
    })

    const finishCreated = () => {
        setCreatedToken(null)
        resetForm()
        onOpenChange(false)
        onCreated?.()
    }

    const resetForm = () => {
        setName('')
        setHashedKey(false)
        setQuota(undefined)
        setPeriodQuota(undefined)
        setPeriodType(null)
        setExpiredAt('')
    }

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()
        if (!groupId || !name.trim()) return
        mutation.mutate({
            group: groupId,
            name: name.trim(),
            hashedKey,
            quota,
            periodQuota,
            periodType,
            expiredAt,
        })
    }

    const handleOpenChange = (open: boolean) => {
        if (!open) resetForm()
        onOpenChange(open)
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="sm:max-w-[425px]">
                <DialogHeader>
                    <DialogTitle>{t('group.tokenDialog.createTitle')}</DialogTitle>
                    <DialogDescription>
                        {t('group.tokenDialog.createDescription')}
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={handleSubmit}>
                    <div className="space-y-4 py-4">
                        {/* Name */}
                        <div className="space-y-2">
                            <Label htmlFor="token-name">{t('group.tokenDialog.name')}</Label>
                            <Input
                                id="token-name"
                                placeholder={t('group.tokenDialog.namePlaceholder')}
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                disabled={mutation.isPending}
                            />
                        </div>

                        {/* Hashed key */}
                        <div className="flex flex-row items-center justify-between rounded-lg border p-3">
                            <div className="space-y-0.5">
                                <Label htmlFor="token-hashed-key">{t('token.hashedKey')}</Label>
                                <p className="text-xs text-muted-foreground">{t('token.hashedKeyHelp')}</p>
                            </div>
                            <Switch
                                id="token-hashed-key"
                                checked={hashedKey}
                                onCheckedChange={setHashedKey}
                                disabled={mutation.isPending}
                            />
                        </div>

                        {/* Expiry */}
                        <div className="space-y-2">
                            <Label htmlFor="token-expired-at">{t('token.expiredAt')}</Label>
                            <Input
                                id="token-expired-at"
                                type="datetime-local"
                                value={expiredAt}
                                onChange={(e) => setExpiredAt(e.target.value)}
                                disabled={mutation.isPending}
                            />
                            <p className="text-xs text-muted-foreground">{t('token.expiredAtHelp')}</p>
                        </div>
                        {/* Total Quota */}
                        <div className="space-y-2">
                            <Label htmlFor="token-quota">{t('token.quota.total')}</Label>
                            <Input
                                id="token-quota"
                                type="number"
                                min={0}
                                step="0.01"
                                placeholder={t('token.quota.totalPlaceholder')}
                                value={quota ?? ''}
                                onChange={(e) => setQuota(e.target.value === '' ? undefined : parseFloat(e.target.value))}
                                disabled={mutation.isPending}
                            />
                            <p className="text-xs text-muted-foreground">{t('token.quota.totalHelp')}</p>
                        </div>

                        {/* Period Type */}
                        <div className="space-y-2">
                            <Label>{t('token.quota.periodType')}</Label>
                            <Select
                                value={periodType || 'none'}
                                onValueChange={(value) => {
                                    const nextPeriodType = value === 'none' ? null : value
                                    setPeriodType(nextPeriodType)
                                    if (!nextPeriodType) setPeriodQuota(undefined)
                                }}
                                disabled={mutation.isPending}
                            >
                                <SelectTrigger>
                                    <SelectValue placeholder={t('token.quota.selectPeriodType')} />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="none">{t('token.quota.none')}</SelectItem>
                                    <SelectItem value="daily">{t('token.quota.daily')}</SelectItem>
                                    <SelectItem value="weekly">{t('token.quota.weekly')}</SelectItem>
                                    <SelectItem value="monthly">{t('token.quota.monthly')}</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>

                        {/* Period Quota */}
                        <div className="space-y-2">
                            <Label htmlFor="token-period-quota">{t('token.quota.period')}</Label>
                            <Input
                                id="token-period-quota"
                                type="number"
                                min={0}
                                step="0.01"
                                placeholder={t('token.quota.periodPlaceholder')}
                                value={periodQuota ?? ''}
                                onChange={(e) => setPeriodQuota(e.target.value === '' ? undefined : parseFloat(e.target.value))}
                                disabled={mutation.isPending || !periodType}
                            />
                            <p className="text-xs text-muted-foreground">
                                {periodType ? t('token.quota.periodHelp') : t('token.quota.periodQuotaDisabledHelp')}
                            </p>
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            type="button"
                            variant="outline"
                            onClick={() => handleOpenChange(false)}
                            disabled={mutation.isPending}
                        >
                            {t('common.cancel')}
                        </Button>
                        <Button
                            type="submit"
                            disabled={mutation.isPending || !name.trim()}
                        >
                            {mutation.isPending ? (
                                <>
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    {t('group.tokenDialog.submitting')}
                                </>
                            ) : (
                                t('group.tokenDialog.create')
                            )}
                        </Button>
                    </DialogFooter>
                </form>
                {createdToken && (
                    <TokenOneTimeKeyDialog
                        open
                        onOpenChange={(open) => { if (!open) finishCreated() }}
                        tokenKey={createdToken.key}
                    />
                )}
            </DialogContent>
        </Dialog>
    )
}
