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
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
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
        },
    })

    // 提交表单
    const onSubmit = (data: TokenCreateForm) => {
        createToken({ name: data.name }, {
            onSuccess: () => {
                onSuccess?.()
                form.reset()
            }
        })
    }

    return (
        <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
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

                <div className="flex justify-end pt-4">
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