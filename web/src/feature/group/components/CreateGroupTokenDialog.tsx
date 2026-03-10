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
    const [quota, setQuota] = useState<number>(0)
    const [periodQuota, setPeriodQuota] = useState<number>(0)
    const [periodType, setPeriodType] = useState<string>('none')

    const mutation = useMutation({
        mutationFn: (data: { group: string; name: string; quota: number; periodQuota: number; periodType: string }) => {
            return tokenApi.createGroupToken(data.group, {
                name: data.name,
                quota: data.quota || undefined,
                period_quota: data.periodQuota || undefined,
                period_type: data.periodType === 'none' ? undefined : data.periodType,
            })
        },
        onSuccess: (data) => {
            queryClient.invalidateQueries({ queryKey: ['groupTokens'] })
            queryClient.invalidateQueries({ queryKey: ['tokens'] })
            queryClient.invalidateQueries({ queryKey: ['groups'] })
            toast.success(t('common.success'))
            if (data?.key) {
                navigator.clipboard.writeText(data.key).then(() => {
                    toast.success(t('common.copied'))
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

    const resetForm = () => {
        setName('')
        setQuota(0)
        setPeriodQuota(0)
        setPeriodType('none')
    }

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()
        if (!groupId || !name.trim()) return
        mutation.mutate({
            group: groupId,
            name: name.trim(),
            quota,
            periodQuota: periodQuota,
            periodType,
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

                        {/* Total Quota */}
                        <div className="space-y-2">
                            <Label htmlFor="token-quota">{t('token.quota.total')}</Label>
                            <Input
                                id="token-quota"
                                type="number"
                                min={0}
                                step="0.01"
                                placeholder={t('token.quota.totalPlaceholder')}
                                value={quota || ''}
                                onChange={(e) => setQuota(e.target.value === '' ? 0 : parseFloat(e.target.value))}
                                disabled={mutation.isPending}
                            />
                            <p className="text-xs text-muted-foreground">{t('token.quota.totalHelp')}</p>
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
                                value={periodQuota || ''}
                                onChange={(e) => setPeriodQuota(e.target.value === '' ? 0 : parseFloat(e.target.value))}
                                disabled={mutation.isPending}
                            />
                            <p className="text-xs text-muted-foreground">{t('token.quota.periodHelp')}</p>
                        </div>

                        {/* Period Type */}
                        <div className="space-y-2">
                            <Label>{t('token.quota.periodType')}</Label>
                            <Select
                                value={periodType}
                                onValueChange={setPeriodType}
                                disabled={mutation.isPending}
                            >
                                <SelectTrigger>
                                    <SelectValue placeholder={t('token.quota.selectPeriodType')} />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="none">{t('token.quota.noLimit')}</SelectItem>
                                    <SelectItem value="daily">{t('token.quota.daily')}</SelectItem>
                                    <SelectItem value="weekly">{t('token.quota.weekly')}</SelectItem>
                                    <SelectItem value="monthly">{t('token.quota.monthly')}</SelectItem>
                                </SelectContent>
                            </Select>
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
            </DialogContent>
        </Dialog>
    )
}
