// src/feature/channel/components/ChannelForm.tsx
import { useState } from 'react'
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
import { channelCreateSchema } from '@/validation/channel'
import { useChannelTypeMetas, useCreateChannel, useUpdateChannel } from '../hooks'
import { useModels } from '@/feature/model/hooks'
import { useTranslation } from 'react-i18next'
import { ChannelCreateForm } from '@/validation/channel'
import { ModelDialog } from '@/feature/model/components/ModelDialog'
import { Channel } from '@/types/channel'
import { SingleSelectCombobox } from '@/components/select/SingleSelectCombobox'
import { MultiSelectCombobox } from '@/components/select/MultiSelectCombobox'
import { ConstructMappingComponent } from '@/components/select/ConstructMappingComponent'
import { AdvancedErrorDisplay } from '@/components/common/error/errorDisplay'
import { Skeleton } from "@/components/ui/skeleton"
import { AnimatedContainer } from '@/components/ui/animation/components/animated-container'

interface ChannelFormProps {
    mode?: 'create' | 'update'
    channelId?: number
    channel?: Channel | null
    onSuccess?: () => void
    defaultValues?: {
        type: number
        name: string
        key: string
        base_url: string
        models: string[]
        model_mapping?: Record<string, string>
    }
}

export function ChannelForm({
    mode = 'create',
    channelId,
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    channel,
    onSuccess,
    defaultValues = {
        type: 0,
        name: '',
        key: '',
        base_url: '',
        models: [],
        model_mapping: {}
    },
}: ChannelFormProps) {
    const { t } = useTranslation()
    const [modelDialogOpen, setModelDialogOpen] = useState(false)

    // 获取渠道类型元数据
    const { data: typeMetas, isLoading: isTypeMetasLoading } = useChannelTypeMetas()

    // 获取所有模型
    const { data: models, isLoading: isModelsLoading } = useModels()


    // API hooks
    const {
        createChannel,
        isLoading: isCreating,
        error: createError,
        clearError: clearCreateError
    } = useCreateChannel()

    const {
        updateChannel,
        isLoading: isUpdating,
        error: updateError,
        clearError: clearUpdateError
    } = useUpdateChannel()

    // 动态状态
    const isLoading = mode === 'create' ? isCreating : isUpdating
    const error = mode === 'create' ? createError : updateError
    const clearError = mode === 'create' ? clearCreateError : clearUpdateError

    // 表单设置
    const form = useForm<ChannelCreateForm>({
        resolver: zodResolver(channelCreateSchema),
        defaultValues,
    })




    // 表单提交处理
    const handleFormSubmit = (data: ChannelCreateForm) => {
        // 清除之前的错误
        if (clearError) clearError()


        // 准备提交数据
        const formData = {
            type: data.type,
            name: data.name,
            key: data.key,
            base_url: data.base_url,
            models: data.models,
            model_mapping: data.model_mapping
        }

        if (mode === 'create') {
            createChannel(formData, {
                onSuccess: () => {
                    form.reset()
                    if (onSuccess) onSuccess()
                }
            })
        } else if (mode === 'update' && channelId) {
            updateChannel({ id: channelId, data: formData }, {
                onSuccess: () => {
                    form.reset()
                    if (onSuccess) onSuccess()
                }
            })
        }
    }

    // 获取类型对应的字段提示
    const getTypeHelp = (typeId: number) => {
        if (!typeMetas || !typeId) return { keyHelp: '', defaultBaseUrl: '' }
        return typeMetas[typeId] || { keyHelp: '', defaultBaseUrl: '' }
    }

    // 表单骨架屏渲染
    const renderFormSkeleton = () => (
        <div className="space-y-6 animate-pulse">
            {/* 厂商字段骨架 */}
            <div className="space-y-2">
                <Skeleton className="h-5 w-24" />
                <Skeleton className="h-9 w-full" />
            </div>

            {/* 名称字段骨架 */}
            <div className="space-y-2">
                <Skeleton className="h-5 w-32" />
                <Skeleton className="h-9 w-full" />
            </div>

            {/* 模型选择字段骨架 */}
            <div className="space-y-2">
                <Skeleton className="h-5 w-28" />
                <Skeleton className="h-[72px] w-full rounded-md" />
            </div>

            {/* 模型映射字段骨架 */}
            <div className="space-y-2">
                <Skeleton className="h-5 w-36" />
                <Skeleton className="h-32 w-full" />
            </div>

            {/* 密钥字段骨架 */}
            <div className="space-y-2">
                <Skeleton className="h-5 w-24" />
                <Skeleton className="h-9 w-full" />
            </div>

            {/* 代理地址字段骨架 */}
            <div className="space-y-2">
                <Skeleton className="h-5 w-32" />
                <Skeleton className="h-9 w-full" />
            </div>

            {/* 提交按钮骨架 */}
            <div className="flex justify-end">
                <Skeleton className="h-9 w-24" />
            </div>
        </div>
    )

    return (
        <AnimatedContainer>
            <div>
                {isTypeMetasLoading || !typeMetas || isModelsLoading || !models ? (
                    renderFormSkeleton()
                ) : (
                    <Form {...form}>
                        <form onSubmit={form.handleSubmit(handleFormSubmit)} className="space-y-6">
                            {/* API错误提示 */}
                            {error && (
                                <AdvancedErrorDisplay error={error} />
                            )}

                            {/* 厂商字段 */}
                            <FormField
                                control={form.control}
                                name="type"
                                render={({ field }) => {

                                    const availableChannels = Object.values(typeMetas).map(
                                        (type) => type.name
                                    )

                                    const initSelectedItem = field.value
                                        ? typeMetas[String(field.value)].name
                                        : undefined

                                    const getKeyByName = (name: string): string | undefined => {
                                        for (const key in typeMetas) {
                                            if (typeMetas[key].name === name) {
                                                return key
                                            }
                                        }
                                        return undefined
                                    }

                                    return (

                                        <SingleSelectCombobox
                                            dropdownItems={availableChannels}
                                            initSelectedItem={initSelectedItem}
                                            setSelectedItem={(channelName: string) => {
                                                if (channelName) {
                                                    const channelType = getKeyByName(channelName)
                                                    if (channelType) {
                                                        field.onChange(Number(channelType))
                                                        form.setValue('models', [])
                                                        form.setValue('model_mapping', {})
                                                    }
                                                }
                                            }}
                                            handleDropdownItemFilter={(
                                                dropdownItems: string[],
                                                inputValue: string
                                            ) => {
                                                const lowerCasedInput = inputValue.toLowerCase()

                                                return dropdownItems.filter((item) => {
                                                    return (
                                                        !inputValue ||
                                                        item.toLowerCase().includes(lowerCasedInput)
                                                    )
                                                })


                                            }}
                                            handleDropdownItemDisplay={(
                                                dropdownItem: string
                                            ) => {
                                                return (
                                                    dropdownItem
                                                )
                                            }}
                                        />
                                    )
                                }}
                            />

                            {/* 名称字段 */}
                            <FormField
                                control={form.control}
                                name="name"
                                render={({ field }) => (
                                    <FormItem>
                                        <FormLabel>{t("channel.dialog.name")}</FormLabel>
                                        <FormControl>
                                            <Input placeholder={t("channel.dialog.namePlaceholder")} {...field} />
                                        </FormControl>
                                        <FormMessage />
                                    </FormItem>
                                )}
                            />


                            {/* 模型选择字段 */}
                            <FormField
                                control={form.control}
                                name="models"
                                render={({ field }) => {
                                    const allModels = models.map((model) => model.model)

                                    const handleModelFilteredDropdownItems = (
                                        dropdownItems: string[],
                                        selectedItems: string[],
                                        inputValue: string
                                    ) => {
                                        const lowerCasedInputValue = inputValue.toLowerCase()

                                        // 过滤匹配的模型
                                        const filteredModels = dropdownItems.filter(
                                            (item) =>
                                                !selectedItems.includes(item) &&
                                                item.toLowerCase().includes(lowerCasedInputValue)
                                        )

                                        // 始终添加"创建新模型"选项作为第一个选项
                                        const createNewOption = t('model.dialog.createDescription')

                                        // 只在搜索为空或选项匹配"创建"相关文字时显示创建选项
                                        if (!inputValue || createNewOption.toLowerCase().includes(lowerCasedInputValue)) {
                                            return [createNewOption, ...filteredModels]
                                        }

                                        return filteredModels
                                    }

                                    return (
                                        <MultiSelectCombobox<string>
                                            dropdownItems={allModels}
                                            selectedItems={field.value || []}
                                            setSelectedItems={(modelsOrFunction) => {
                                                // Ensure we're working with array
                                                const models = Array.isArray(modelsOrFunction) ? modelsOrFunction : []

                                                // Now we can use includes safely
                                                if (models.includes(t('model.dialog.createDescription'))) {
                                                    const filteredModels = models.filter(m => m !== t('model.dialog.createDescription'))
                                                    field.onChange(filteredModels)
                                                    setModelDialogOpen(true)
                                                } else {
                                                    field.onChange(models)
                                                }
                                            }}
                                            handleFilteredDropdownItems={handleModelFilteredDropdownItems}
                                            handleDropdownItemDisplay={(item) => {
                                                // 为"创建新模型"选项添加特殊样式
                                                if (item === t('model.dialog.createDescription')) {
                                                    return (
                                                        <div className="flex items-center gap-2 text-primary">
                                                            <span className="flex h-4 w-4 items-center justify-center rounded-full border border-primary">
                                                                <span className="text-xs">+</span>
                                                            </span>
                                                            {item}
                                                        </div>
                                                    )
                                                }
                                                return item
                                            }}
                                            handleSelectedItemDisplay={(item) => {
                                                return item
                                            }}
                                        />
                                    )
                                }}
                            />



                            {/* 模型映射字段 */}
                            <FormField
                                control={form.control}
                                name="model_mapping"
                                render={({ field }) => {
                                    const selectedModels = form.watch('models')

                                    return (
                                        <ConstructMappingComponent
                                            mapKeys={selectedModels}
                                            mapData={field.value as Record<string, string>}
                                            setMapData={(mapping) => {
                                                field.onChange(mapping)
                                            }}
                                        />
                                    )
                                }}
                            />


                            {/* 密钥字段 */}
                            <FormField
                                control={form.control}
                                name="key"
                                render={({ field }) => {
                                    const typeId = Number(form.getValues('type'))
                                    const { keyHelp } = getTypeHelp(typeId)

                                    return (
                                        <FormItem>
                                            <FormLabel>{t("channel.dialog.key")}</FormLabel>
                                            <FormControl>
                                                <Input
                                                    placeholder={keyHelp || t("channel.dialog.keyPlaceholder")}
                                                    {...field}
                                                />
                                            </FormControl>
                                            <FormMessage />
                                        </FormItem>
                                    )
                                }}
                            />

                            {/* 代理地址字段 */}
                            <FormField
                                control={form.control}
                                name="base_url"
                                render={({ field }) => {
                                    const typeId = Number(form.getValues('type'))
                                    const { defaultBaseUrl } = getTypeHelp(typeId)

                                    return (
                                        <FormItem>
                                            <FormLabel>{t("channel.dialog.baseUrl")}</FormLabel>
                                            <FormControl>
                                                <Input
                                                    placeholder={defaultBaseUrl || t("channel.dialog.baseUrlPlaceholder")}
                                                    {...field}
                                                />
                                            </FormControl>
                                            <FormMessage />
                                        </FormItem>
                                    )
                                }}
                            />


                            {/* 提交按钮 */}
                            <div className="flex justify-end">
                                <Button type="submit" disabled={isLoading}>
                                    {isLoading ? t("channel.dialog.submitting") : mode === 'create' ? t("channel.dialog.create") : t("channel.dialog.update")}
                                </Button>
                            </div>
                        </form>
                    </Form>
                )}

                {/* 创建模型对话框 */}
                <ModelDialog
                    open={modelDialogOpen}
                    onOpenChange={setModelDialogOpen}
                    mode="create"
                    model={null}
                />
            </div>
        </AnimatedContainer>
    )
}