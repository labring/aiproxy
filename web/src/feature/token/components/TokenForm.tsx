// src/feature/token/components/TokenForm.tsx
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { tokenCreateSchema, TokenCreateForm } from '@/validation/token'
import { useCreateToken } from '../hooks'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
    Form,
    FormControl,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
    FormDescription,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'

interface TokenFormProps {
    onSuccess?: () => void
}

export function TokenForm({ onSuccess }: TokenFormProps) {
    const { t } = useTranslation()
    const { createToken, isLoading } = useCreateToken()

    // 初始化表单
    const form = useForm<TokenCreateForm>({
        resolver: zodResolver(tokenCreateSchema),
        defaultValues: {
            name: '',
            quota: undefined,
            period_quota: undefined,
            period_type: null,
        },
    })
    const periodType = form.watch('period_type')

    // 提交表单
    const onSubmit = (data: TokenCreateForm) => {
        createToken({
            name: data.name,
            quota: data.quota,
            period_quota: data.period_type ? data.period_quota : undefined,
            period_type: data.period_type || undefined,
        }, {
            onSuccess: () => {
                onSuccess?.()
                form.reset()
            }
        })
    }

    return (
        <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="grid gap-4 sm:grid-cols-2">
                <FormField
                    control={form.control}
                    name="name"
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>{t("token.dialog.name")}</FormLabel>
                            <FormControl>
                                <Input
                                    placeholder={t("token.dialog.namePlaceholder")}
                                    {...field}
                                />
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    )}
                />

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
                            <FormDescription>
                                {t("token.quota.totalHelp")}
                            </FormDescription>
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
                                onValueChange={(value) => {
                                    const nextPeriodType = value === 'none' ? null : value
                                    field.onChange(nextPeriodType)
                                    if (!nextPeriodType) {
                                        form.setValue('period_quota', undefined, { shouldDirty: true })
                                    }
                                }}
                                value={field.value || 'none'}
                            >
                                <FormControl>
                                    <SelectTrigger>
                                        <SelectValue placeholder={t("token.quota.selectPeriodType")} />
                                    </SelectTrigger>
                                </FormControl>
                                <SelectContent>
                                    <SelectItem value="none">{t("token.quota.none")}</SelectItem>
                                    <SelectItem value="daily">{t("token.quota.daily")}</SelectItem>
                                    <SelectItem value="weekly">{t("token.quota.weekly")}</SelectItem>
                                    <SelectItem value="monthly">{t("token.quota.monthly")}</SelectItem>
                                </SelectContent>
                            </Select>
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
                                    disabled={!periodType}
                                />
                            </FormControl>
                            <FormDescription>
                                {periodType ? t("token.quota.periodHelp") : t("token.quota.periodQuotaDisabledHelp")}
                            </FormDescription>
                            <FormMessage />
                        </FormItem>
                    )}
                />

                <div className="form-actions sm:col-span-2">
                    <AnimatedButton>
                        <Button
                            type="submit"
                            disabled={isLoading}
                        >
                            {isLoading ? t("token.dialog.submitting") : t("token.dialog.create")}
                        </Button>
                    </AnimatedButton>
                </div>
            </form>
        </Form>
    )
}
