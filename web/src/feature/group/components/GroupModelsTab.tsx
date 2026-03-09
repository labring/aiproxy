// src/feature/group/components/GroupModelsTab.tsx
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { dashboardApi } from '@/api/dashboard'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'
import { PriceDisplay } from '@/components/price/PriceDisplay'
import { toast } from 'sonner'
import { Copy } from 'lucide-react'

interface GroupModelsTabProps {
    groupId: string
}

export function GroupModelsTab({ groupId }: GroupModelsTabProps) {
    const { t } = useTranslation()

    const { data: models, isLoading, error } = useQuery({
        queryKey: ['groupModels', groupId],
        queryFn: () => dashboardApi.getGroupModels(groupId),
        enabled: !!groupId,
    })

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text).then(() => {
            toast.success(t('common.copied'))
        }).catch(() => {
            toast.error(t('common.copyFailed'))
        })
    }

    if (error) {
        return (
            <div className="flex items-center justify-center h-64 text-muted-foreground">
                <p>{t('error.loading')}</p>
            </div>
        )
    }

    if (isLoading) {
        return (
            <div className="space-y-2">
                {Array.from({ length: 5 }).map((_, i) => (
                    <Skeleton key={i} className="h-12 w-full rounded-lg" />
                ))}
            </div>
        )
    }

    if (!models || models.length === 0) {
        return (
            <div className="flex items-center justify-center h-64 text-muted-foreground">
                <p>{t('common.noResult')}</p>
            </div>
        )
    }

    return (
        <div className="rounded-md border">
            <Table>
                <TableHeader>
                    <TableRow>
                        <TableHead>{t('group.models.model')}</TableHead>
                        <TableHead>{t('group.models.type')}</TableHead>
                        <TableHead>{t('group.models.rpm')}</TableHead>
                        <TableHead>{t('group.models.tpm')}</TableHead>
                        <TableHead>{t('group.price.title')}</TableHead>
                        <TableHead>{t('group.models.plugins')}</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {models.map((model) => (
                        <TableRow key={model.model}>
                            <TableCell>
                                <button
                                    className="font-mono text-sm hover:underline cursor-pointer text-left"
                                    onClick={() => copyToClipboard(model.model)}
                                >
                                    <span className="flex items-center gap-1">
                                        {model.model}
                                        <Copy className="h-3 w-3 text-muted-foreground" />
                                    </span>
                                </button>
                            </TableCell>
                            <TableCell>
                                <Badge variant="outline">
                                    {t(`modeType.${model.type}` as never)}
                                </Badge>
                            </TableCell>
                            <TableCell>{model.rpm || '-'}</TableCell>
                            <TableCell>{model.tpm || '-'}</TableCell>
                            <TableCell>
                                <PriceDisplay price={model.price} />
                            </TableCell>
                            <TableCell>
                                <div className="flex flex-wrap gap-1">
                                    {model.enabled_plugins && model.enabled_plugins.length > 0 ? (
                                        model.enabled_plugins.map((plugin) => (
                                            <Badge key={plugin} variant="secondary" className="text-xs">
                                                {plugin}
                                            </Badge>
                                        ))
                                    ) : (
                                        <span className="text-muted-foreground text-sm">-</span>
                                    )}
                                </div>
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
        </div>
    )
}
