// src/feature/model/components/ModelForm.tsx
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
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
import { modelCreateSchema } from '@/validation/model'
import { useCreateModel } from '../hooks'
import { useTranslation } from 'react-i18next'
import { ModelCreateForm } from '@/validation/model'
import { AdvancedErrorDisplay } from '@/components/common/error/errorDisplay'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'

interface ModelFormProps {
    mode?: 'create' | 'update'
    modelId?: string
    onSuccess?: () => void
    defaultValues?: {
        model: string
        type: number
    }
}

export function ModelForm({
    mode = 'create',
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    modelId,
    onSuccess,
    defaultValues = {
        model: '',
        type: 1,
    },
}: ModelFormProps) {
    const { t } = useTranslation()

    // API hooks
    const {
        createModel,
        isLoading,
        error,
        clearError
    } = useCreateModel()

    // Form setup
    const form = useForm<ModelCreateForm>({
        resolver: zodResolver(modelCreateSchema),
        defaultValues,
    })

    // Form submission handler
    const handleFormSubmit = (data: ModelCreateForm) => {
        // Clear previous errors
        if (clearError) clearError()

        // Prepare data for API
        const formData = {
            model: data.model,
            type: Number(data.type)
        }

        if (mode === 'create') {
            createModel(formData, {
                onSuccess: () => {
                    // Reset form
                    form.reset()
                    // Notify parent component
                    if (onSuccess) onSuccess()
                }
            })
        }
    }

    return (
        <div>
            <Form {...form}>
                <form onSubmit={form.handleSubmit(handleFormSubmit)} className="space-y-6">
                    {/* API error alert */}
                    {error && (
                        <AdvancedErrorDisplay error={error} />
                    )}

                    {/* Model name field */}
                    <FormField
                        control={form.control}
                        name="model"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("model.dialog.modelName")}</FormLabel>
                                <FormControl>
                                    <Input placeholder={t("model.dialog.modelNamePlaceholder")} {...field} />
                                </FormControl>
                                <FormMessage />
                            </FormItem>
                        )}
                    />

                    {/* Model type field */}
                    <FormField
                        control={form.control}
                        name="type"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("model.dialog.modelType")}</FormLabel>
                                <Select
                                    onValueChange={(value) => field.onChange(Number(value))}
                                    defaultValue={String(field.value)}
                                >
                                    <FormControl>
                                        <SelectTrigger>
                                            <SelectValue placeholder={t("model.dialog.selectType")} />
                                        </SelectTrigger>
                                    </FormControl>
                                    <SelectContent>
                                        {Array.from({ length: 11 }, (_, i) => i + 1).map((type) => (
                                            <SelectItem key={type} value={String(type)}>
                                                {t(`modeType.${type}` as never)}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                <FormMessage />
                            </FormItem>
                        )}
                    />

                    {/* Submit button */}
                    <div className="flex justify-end">
                        <AnimatedButton >
                            <Button type="submit" disabled={isLoading}>
                                {isLoading
                                    ? t("model.dialog.submitting")
                                    : mode === 'create'
                                        ? t("model.dialog.create")
                                        : t("model.dialog.update")
                                }
                            </Button>
                        </AnimatedButton>
                    </div>
                </form>
            </Form>
        </div>
    )
}