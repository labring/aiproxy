// src/feature/token/components/TokenQuotaDialog.tsx
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useTranslation } from 'react-i18next'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
    Form,
    FormControl,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from '@/components/ui/form'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { useUpdateToken } from '../hooks'
import { Token } from '@/types/token'
import { Skeleton } from '@/components/ui/skeleton'

const tokenQuotaSchema = z.object({
    quota: z.number().min(0).optional(),
    period_quota: z.number().min(0).optional(),
    period_type: z.string().optional().nullable(),
})

type TokenQuotaForm = z.infer<typeof tokenQuotaSchema>

interface TokenQuotaDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    token: Token | null
}

export function TokenQuotaDialog({ open, onOpenChange, token }: TokenQuotaDialogProps) {
    const { t } = useTranslation()
    const { updateToken, isLoading } = useUpdateToken()

    const form = useForm<TokenQuotaForm>({
        resolver: zodResolver(tokenQuotaSchema),
        defaultValues: {
            quota: undefined,
            period_quota: undefined,
            period_type: null,
        },
    })

    // 当token数据变化时，重置表单
    useEffect(() => {
        if (token && open) {
            form.reset({
                quota: token.quota > 0 ? token.quota : undefined,
                period_quota: token.period_quota > 0 ? token.period_quota : undefined,
                period_type: token.period_type || null,
            })
        }
    }, [token, open, form])

    const onSubmit = (data: TokenQuotaForm) => {
        if (!token) return

        const quota = data.quota ?? 0
        const periodQuota = data.period_quota ?? 0
        const periodType = periodQuota > 0 ? (data.period_type || 'monthly') : ''

        updateToken({
            id: token.id,
            data: {
                quota,
                period_quota: periodQuota,
                period_type: periodType,
            },
        }, {
            onSuccess: () => {
                onOpenChange(false)
            },
        })
    }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-[425px]">
                <DialogHeader>
                    <DialogTitle>{t("token.quota.title")}</DialogTitle>
                    <DialogDescription>
                        {t("token.quota.description")}
                    </DialogDescription>
                </DialogHeader>

                {!token ? (
                    <div className="space-y-4">
                        <Skeleton className="h-10 w-full" />
                        <Skeleton className="h-10 w-full" />
                        <Skeleton className="h-10 w-full" />
                    </div>
                ) : (
                    <Form {...form}>
                        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
                            <FormField
                                control={form.control}
                                name="quota"
                                render={({ field }) => (
                                    <FormItem>
                                        <FormLabel>{t("token.quota.total")}</FormLabel>
                                        <FormControl>
                                            <Input
                                                type="number"
                                                min={0}
                                                step="0.01"
                                                placeholder={t("token.quota.totalPlaceholder")}
                                                {...field}
                                                value={field.value ?? ''}
                                                onChange={(e) => {
                                                    const value = e.target.value
                                                    field.onChange(value === '' ? undefined : parseFloat(value))
                                                }}
                                            />
                                        </FormControl>
                                        <p className="text-xs text-muted-foreground">
                                            {t("token.quota.totalHelp")}
                                        </p>
                                        <FormMessage />
                                    </FormItem>
                                )}
                            />

                            <FormField
                                control={form.control}
                                name="period_quota"
                                render={({ field }) => (
                                    <FormItem>
                                        <FormLabel>{t("token.quota.period")}</FormLabel>
                                        <FormControl>
                                            <Input
                                                type="number"
                                                min={0}
                                                step="0.01"
                                                placeholder={t("token.quota.periodPlaceholder")}
                                                {...field}
                                                value={field.value ?? ''}
                                                onChange={(e) => {
                                                    const value = e.target.value
                                                    field.onChange(value === '' ? undefined : parseFloat(value))
                                                }}
                                            />
                                        </FormControl>
                                        <p className="text-xs text-muted-foreground">
                                            {t("token.quota.periodHelp")}
                                        </p>
                                        <FormMessage />
                                    </FormItem>
                                )}
                            />

                            <FormField
                                control={form.control}
                                name="period_type"
                                render={({ field }) => (
                                    <FormItem>
                                        <FormLabel>{t("token.quota.periodType")}</FormLabel>
                                        <Select
                                            onValueChange={field.onChange}
                                            value={field.value || 'monthly'}
                                            disabled={!form.watch('period_quota')}
                                        >
                                            <FormControl>
                                                <SelectTrigger>
                                                    <SelectValue placeholder={t("token.quota.selectPeriodType")} />
                                                </SelectTrigger>
                                            </FormControl>
                                            <SelectContent>
                                                <SelectItem value="daily">{t("token.quota.daily")}</SelectItem>
                                                <SelectItem value="weekly">{t("token.quota.weekly")}</SelectItem>
                                                <SelectItem value="monthly">{t("token.quota.monthly")}</SelectItem>
                                            </SelectContent>
                                        </Select>
                                        {!form.watch('period_quota') && (
                                            <p className="text-xs text-muted-foreground">
                                                {t("token.quota.periodTypeDisabledHelp")}
                                            </p>
                                        )}
                                        <FormMessage />
                                    </FormItem>
                                )}
                            />

                            <div className="flex justify-end pt-4 gap-2">
                                <Button
                                    type="button"
                                    variant="outline"
                                    onClick={() => onOpenChange(false)}
                                    disabled={isLoading}
                                >
                                    {t("token.deleteDialog.cancel")}
                                </Button>
                                <Button type="submit" disabled={isLoading}>
                                    {isLoading ? t("token.quota.updating") : t("token.quota.update")}
                                </Button>
                            </div>
                        </form>
                    </Form>
                )}
            </DialogContent>
        </Dialog>
    )
}
