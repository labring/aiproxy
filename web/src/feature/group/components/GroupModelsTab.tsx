// src/feature/group/components/GroupModelsTab.tsx
import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { dashboardApi } from '@/api/dashboard'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
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
import { Copy, Search } from 'lucide-react'
import { useGroupModelMetrics } from '@/feature/monitor/runtime-hooks'

interface GroupModelsTabProps {
    groupId: string
}

export function GroupModelsTab({ groupId }: GroupModelsTabProps) {
    const { t } = useTranslation()
    const [searchKeyword, setSearchKeyword] = useState('')
    const [ownerFilter, setOwnerFilter] = useState('')

    const { data: models, isLoading, error } = useQuery({
        queryKey: ['groupModels', groupId],
        queryFn: () => dashboardApi.getGroupModels(groupId),
        enabled: !!groupId,
    })

    const ownerOptions = useMemo(() => {
        if (!models) return []
        const ownerSet = new Set<string>()
        let hasEmptyOwner = false

        for (const model of models) {
            if (model.owner) {
                ownerSet.add(model.owner)
            } else {
                hasEmptyOwner = true
            }
        }

        const options = [...ownerSet].sort((a, b) => a.localeCompare(b))
        if (hasEmptyOwner) {
            options.push('__empty__')
        }
        return options
    }, [models])

    const filteredModels = useMemo(() => {
        if (!models) return []
        let filtered = models
        if (searchKeyword) {
            const keyword = searchKeyword.toLowerCase()
            filtered = filtered.filter(m =>
                m.model.toLowerCase().includes(keyword) || (m.owner || '').toLowerCase().includes(keyword)
            )
        }
        if (ownerFilter === '__empty__') {
            filtered = filtered.filter((model) => !model.owner)
        } else if (ownerFilter && ownerFilter !== '__all__') {
            filtered = filtered.filter((model) => model.owner === ownerFilter)
        }
        return filtered
    }, [models, searchKeyword, ownerFilter])
    const { data: runtimeMetrics } = useGroupModelMetrics(groupId, !!groupId && filteredModels.length > 0)
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
        <div className="space-y-4">
            <div className="flex gap-2">
                <div className="relative w-64">
                    <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                    <Input
                        placeholder={t('common.search')}
                        value={searchKeyword}
                        onChange={(e) => setSearchKeyword(e.target.value)}
                        className="pl-9 h-9"
                    />
                </div>
                <div className="w-44">
                    <Select value={ownerFilter} onValueChange={setOwnerFilter}>
                        <SelectTrigger className="h-9">
                            <SelectValue placeholder={t('model.ownerFilterPlaceholder')} />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="__all__">{t('model.allOwners')}</SelectItem>
                            {ownerOptions.map((owner) => (
                                <SelectItem key={owner} value={owner}>
                                    {owner === '__empty__' ? t('model.emptyOwner') : owner}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
            </div>

            <div className="rounded-md border">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t('group.models.model')}</TableHead>
                            <TableHead>{t('model.owner')}</TableHead>
                            <TableHead>{t('group.models.type')}</TableHead>
                            <TableHead>{t('common.runtime')}</TableHead>
                            <TableHead>{t('group.models.rpm')}</TableHead>
                            <TableHead>{t('group.models.tpm')}</TableHead>
                            <TableHead>{t('group.price.title')}</TableHead>
                            <TableHead>{t('group.models.plugins')}</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {filteredModels.map((model) => (
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
                                    {model.owner || (
                                        <span className="text-muted-foreground text-sm">{t('model.emptyOwner')}</span>
                                    )}
                                </TableCell>
                                <TableCell>
                                    <Badge variant="outline">
                                        {t(`modeType.${model.type}` as never)}
                                    </Badge>
                                </TableCell>
                                <TableCell>
                                    {(() => {
                                        const metric = runtimeMetrics?.models?.[model.model]
                                        if (!metric) return <span className="text-muted-foreground text-sm">-</span>
                                        return (
                                            <div className="flex flex-wrap gap-1">
                                                <Badge variant="outline" className="text-xs">RPM {metric.rpm.toLocaleString()}</Badge>
                                                <Badge variant="outline" className="text-xs">TPM {metric.tpm.toLocaleString()}</Badge>
                                            </div>
                                        )
                                    })()}
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
                        {filteredModels.length === 0 && (
                            <TableRow>
                                <TableCell colSpan={8} className="text-center text-muted-foreground py-12">
                                    {t('common.noResult')}
                                </TableCell>
                            </TableRow>
                        )}
                    </TableBody>
                </Table>
            </div>
        </div>
    )
}
