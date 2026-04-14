// src/feature/channel/components/ChannelTable.tsx
import { useState, useCallback, useRef, useMemo } from 'react'
import {
    useReactTable,
    getCoreRowModel,
    ColumnDef,
} from '@tanstack/react-table'
import { useNavigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { useChannels, useChannelTypeMetas, useUpdateChannelStatus, useTestChannel, useTestAllChannels, useAllChannelDefaultModels } from '../hooks'
import { channelApi } from '@/api/channel'
import { Channel, ChannelCreateRequest } from '@/types/channel'
import { Button } from '@/components/ui/button'
import {
    MoreHorizontal, Plus, Trash2, RefreshCcw, Pencil,
    PowerOff, Power, FlaskConical, Search, Settings, Download, Upload, Copy, ChevronDown, CircleHelp
} from 'lucide-react'
import {
    DropdownMenu, DropdownMenuContent,
    DropdownMenuItem, DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { ChannelDialog } from './ChannelDialog'
import { Loader2 } from 'lucide-react'
import { DataTable } from '@/components/table/motion-data-table'
import { ServerPagination } from '@/components/table/server-pagination'
import { DeleteChannelDialog } from './DeleteChannelDialog'
import { DefaultModelsDialog } from './DefaultModelsDialog'
import { ChannelTestDialog } from './ChannelTestDialog'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { AnimatedIcon } from '@/components/ui/animation/components/animated-icon'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'
import { Badge } from '@/components/ui/badge'
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from '@/components/ui/popover'
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'
import { ROUTES } from '@/routes/constants'
import { useRuntimeMetrics } from '@/feature/monitor/runtime-hooks'
import { openResourceDialog, showDeletedResourceToast } from '@/utils/resource-dialog'
import { format } from 'date-fns'
import { getChannelModelMetric, getTemporarilyExcludedModels } from '@/utils/runtime-metrics'

const formatTimestamp = (timestamp: number): string => {
    if (!timestamp) return '-'
    return format(new Date(timestamp), 'yyyy-MM-dd HH:mm')
}

const formatAccessedAt = (timestamp: number, neverLabel: string): string => {
    if (!timestamp || timestamp <= 0) return neverLabel
    return format(new Date(timestamp), 'yyyy-MM-dd HH:mm')
}

export function ChannelTable() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const queryClient = useQueryClient()
    const fileInputRef = useRef<HTMLInputElement>(null)

    // 状态管理
    const [channelDialogOpen, setChannelDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [selectedChannelId, setSelectedChannelId] = useState<number | null>(null)
    const [dialogMode, setDialogMode] = useState<'create' | 'update' | 'copy'>('create')
    const [selectedChannel, setSelectedChannel] = useState<Channel | null>(null)
    const [isRefreshAnimating, setIsRefreshAnimating] = useState(false)
    const [isImporting, setIsImporting] = useState(false)
    const [testDialogOpen, setTestDialogOpen] = useState(false)
    const [isTestAll, setIsTestAll] = useState(false)
    const [defaultModelsDialogOpen, setDefaultModelsDialogOpen] = useState(false)
    const [searchInput, setSearchInput] = useState('')
    const [searchKeyword, setSearchKeyword] = useState<string | undefined>(undefined)
    const [selectedChannelType, setSelectedChannelType] = useState<number | undefined>(undefined)
    const searchTimerRef = useRef<ReturnType<typeof setTimeout>>(null)
    const [page, setPage] = useState(1)
    const [pageSize, setPageSize] = useState(20)

    const handleSearchChange = useCallback((value: string) => {
        setSearchInput(value)
        if (searchTimerRef.current) clearTimeout(searchTimerRef.current)
        searchTimerRef.current = setTimeout(() => {
            setSearchKeyword(value || undefined)
            setPage(1)
        }, 300)
    }, [])

    // 获取渠道类型元数据
    const { data: typeMetas } = useChannelTypeMetas()
    const { data: allDefaultModels } = useAllChannelDefaultModels()
    // 获取渠道列表
    const {
        data,
        isLoading,
        refetch
    } = useChannels(page, pageSize, searchKeyword, selectedChannelType)

    // 更新渠道状态
    const { updateStatus, isLoading: isStatusUpdating } = useUpdateChannelStatus()

    // 测试单个渠道
    const { testChannel, isTesting, results: testResults, clearResults, cancelTest } = useTestChannel()

    // 测试所有渠道
    const {
        testAllChannels,
        isTesting: isTestingAll,
        results: testAllResults,
        clearResults: clearTestAllResults,
        cancelTest: cancelTestAll
    } = useTestAllChannels()

    const channels = useMemo(
        () => (data?.channels || []).filter(channel => channel != null),
        [data?.channels]
    )
    const total = data?.total || 0
    const { data: runtimeMetrics, isLoading: isLoadingRuntimeMetrics } = useRuntimeMetrics()

    // 打开创建渠道对话框
    const openCreateDialog = () => {
        setDialogMode('create')
        setSelectedChannel(null)
        setChannelDialogOpen(true)
    }

    // 打开更新渠道对话框
    const openUpdateDialog = (channel: Channel) => {
        setDialogMode('update')
        setSelectedChannel({...channel})
        setChannelDialogOpen(true)
    }

    // 打开复制渠道对话框
    const openCopyDialog = (channel: Channel) => {
        setDialogMode('copy')
        setSelectedChannel({...channel})
        setChannelDialogOpen(true)
    }

    // 打开删除对话框
    const openDeleteDialog = (id: number) => {
        setSelectedChannelId(id)
        setDeleteDialogOpen(true)
    }

    // 更新渠道状态
    const handleStatusChange = (id: number, currentStatus: number) => {
        const newStatus = currentStatus === 2 ? 1 : 2
        updateStatus({ id, status: { status: newStatus } })
    }

    // 刷新渠道列表
    const refreshChannels = () => {
        setIsRefreshAnimating(true)
        refetch()
        setTimeout(() => {
            setIsRefreshAnimating(false)
        }, 1000)
    }

    // Export channels to JSON file
    const exportChannels = async () => {
        try {
            // Fetch all channels for export
            const allChannels = await channelApi.getAllChannels()
            if (!allChannels || allChannels.length === 0) {
                toast.error(t('channel.noDataToExport'))
                return
            }

            const exportData: ChannelCreateRequest[] = allChannels.map(channel => ({
                type: channel.type,
                name: channel.name,
                key: channel.key,
                base_url: channel.base_url,
                proxy_url: channel.proxy_url,
                models: channel.models,
                model_mapping: channel.model_mapping || undefined,
                sets: channel.sets,
                priority: channel.priority,
                enabled_no_permission_ban: channel.enabled_no_permission_ban,
                warn_error_rate: channel.warn_error_rate,
                max_error_rate: channel.max_error_rate,
                configs: channel.configs || undefined,
            }))

            const blob = new Blob([JSON.stringify(exportData, null, 2)], {
                type: 'application/json',
            })
            const url = URL.createObjectURL(blob)
            const a = document.createElement('a')
            a.href = url
            a.download = `channels_${new Date().toISOString().slice(0, 10)}.json`
            document.body.appendChild(a)
            a.click()
            document.body.removeChild(a)
            URL.revokeObjectURL(url)
            toast.success(t('channel.exportSuccess'))
        } catch {
            toast.error(t('channel.exportFailed'))
        }
    }

    // Export single channel to JSON file
    const exportSingleChannel = (channel: Channel) => {
        const exportData: ChannelCreateRequest[] = [{
            type: channel.type,
            name: channel.name,
            key: channel.key,
            base_url: channel.base_url,
            proxy_url: channel.proxy_url,
            models: channel.models,
            model_mapping: channel.model_mapping || undefined,
            sets: channel.sets,
            priority: channel.priority,
            enabled_no_permission_ban: channel.enabled_no_permission_ban,
            warn_error_rate: channel.warn_error_rate,
            max_error_rate: channel.max_error_rate,
            configs: channel.configs || undefined,
        }]

        const blob = new Blob([JSON.stringify(exportData, null, 2)], {
            type: 'application/json',
        })
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `channel_${channel.name}_${new Date().toISOString().slice(0, 10)}.json`
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        URL.revokeObjectURL(url)
        toast.success(t('channel.exportSuccess'))
    }

    // Import channels from JSON file
    const importChannels = async (event: React.ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0]
        if (!file) return

        setIsImporting(true)
        try {
            const text = await file.text()
            const channels: ChannelCreateRequest[] = JSON.parse(text)

            if (!Array.isArray(channels)) {
                throw new Error(t('channel.invalidFormat'))
            }

            // Import channels one by one
            let successCount = 0
            let failCount = 0

            for (const channel of channels) {
                try {
                    await channelApi.createChannel(channel)
                    successCount++
                } catch {
                    failCount++
                }
            }

            if (successCount > 0) {
                toast.success(t('channel.importSuccess', {
                    success: successCount,
                    fail: failCount
                }))
                queryClient.invalidateQueries({ queryKey: ['channels'] })
            } else {
                toast.error(t('channel.importFailed'))
            }
        } catch (error) {
            toast.error(
                error instanceof Error
                    ? error.message
                    : t('channel.importFailed')
            )
        } finally {
            setIsImporting(false)
            // Reset file input
            if (fileInputRef.current) {
                fileInputRef.current.value = ''
            }
        }
    }

    // Trigger file input click
    const triggerImport = () => {
        fileInputRef.current?.click()
    }

    // 跳转到全局仪表盘
    const navigateToDashboard = (channelId: number) => {
        navigate(`${ROUTES.MONITOR}?channel=${channelId}`)
    }

    // 获取渠道类型名称
    const getChannelTypeName = (typeId: number): string => {
        if (!typeMetas) return String(typeId)
        const meta = typeMetas[typeId]
        return meta ? meta.name : String(typeId)
    }

    const providerOptions = useMemo(() => {
        if (!typeMetas) return []
        return Object.entries(typeMetas)
            .map(([key, meta]) => ({
                value: String(key),
                label: meta.name,
            }))
    }, [typeMetas])

    const getDisplayModels = useCallback((channel: Channel) => {
        if (channel.models && channel.models.length > 0) {
            return {
                models: channel.models,
                usingDefaultModels: false,
            }
        }

        const defaultModels = allDefaultModels?.models?.[String(channel.type)] || []
        return {
            models: defaultModels,
            usingDefaultModels: true,
        }
    }, [allDefaultModels])

    const formatPercent = useCallback((value?: number) => `${((value || 0) * 100).toFixed(1)}%`, [])
    const getExcludedModels = useCallback((channel: Channel) => {
        const { models } = getDisplayModels(channel)
        return getTemporarilyExcludedModels(runtimeMetrics, channel.id, models)
    }, [getDisplayModels, runtimeMetrics])

    // 可点击单元格样式
    const clickableCell = 'cursor-pointer hover:text-primary hover:underline underline-offset-4 transition-colors'
    const dashboardCell = 'cursor-pointer hover:text-primary transition-colors'

    // 表格列定义
    // eslint-disable-next-line react-hooks/exhaustive-deps
    const columns: ColumnDef<Channel>[] = useMemo(() => [
        {
            accessorKey: 'id',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.id")}</div>,
            cell: ({ row }) => (
                <div
                    className={dashboardCell}
                    onClick={() => navigateToDashboard(row.original.id)}
                    title={t("channel.viewDashboard")}
                >
                    {row.original.id}
                </div>
            ),
        },
        {
            accessorKey: 'name',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.name")}</div>,
            cell: ({ row }) => (
                <div
                    className={cn("font-medium", clickableCell)}
                    onClick={() => openUpdateDialog(row.original)}
                >
                    {row.original.name}
                </div>
            ),
        },
        {
            accessorKey: 'type',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.type")}</div>,
            cell: ({ row }) => (
                <div
                    className={clickableCell}
                    onClick={() => openUpdateDialog(row.original)}
                >
                    {getChannelTypeName(row.original.type)}
                </div>
            ),
        },
        {
            accessorKey: 'sets',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.sets")}</div>,
            cell: ({ row }) => {
                const sets = row.original.sets || [];
                if (sets.length === 0) return <div className="text-muted-foreground text-xs">-</div>;

                return (
                    <div
                        className={cn("flex flex-wrap gap-1", "cursor-pointer")}
                        onClick={() => openUpdateDialog(row.original)}
                    >
                        {sets.map((set, index) => (
                            <Badge
                                key={index}
                                variant="secondary"
                                className="text-xs py-0 px-2 hover:bg-secondary/80"
                            >
                                {set}
                            </Badge>
                        ))}
                    </div>
                );
            }
        },
        {
            accessorKey: 'priority',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.priority")}</div>,
            cell: ({ row }) => (
                <div
                    className={clickableCell}
                    onClick={() => openUpdateDialog(row.original)}
                >
                    {row.original.priority || 10}
                </div>
            ),
        },
        {
            id: 'runtime',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("common.runtime")}</div>,
            cell: ({ row }) => {
                const metric = runtimeMetrics?.channels?.[String(row.original.id)]
                if (!metric) {
                    return <div className="text-muted-foreground text-xs">-</div>
                }

                return (
                    <div className="flex flex-wrap gap-1">
                        <Badge variant="outline" className="text-xs">RPM {metric.rpm.toLocaleString()}</Badge>
                        <Badge variant="outline" className="text-xs">TPM {metric.tpm.toLocaleString()}</Badge>
                        <Badge variant="outline" className="text-xs">ERR {formatPercent(metric.error_rate)}</Badge>
                        {metric.banned_models > 0 && (
                            <Badge variant="destructive" className="text-xs">BAN {metric.banned_models}</Badge>
                        )}
                    </div>
                )
            },
        },
        {
            accessorKey: 'models',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.models")}</div>,
            cell: ({ row }) => {
                const { models, usingDefaultModels } = getDisplayModels(row.original)

                if (models.length === 0) {
                    return (
                        usingDefaultModels ? (
                            <button
                                type="button"
                                className="inline-flex cursor-pointer items-center gap-2 rounded-md border border-amber-300/70 bg-amber-50 px-2.5 py-1 text-sm text-amber-700 transition-colors hover:border-amber-400 hover:bg-amber-100 hover:text-amber-800 dark:border-amber-700/70 dark:bg-amber-950/30 dark:text-amber-300 dark:hover:bg-amber-900/40"
                                onClick={() => openUpdateDialog(row.original)}
                            >
                                <span className="whitespace-nowrap">
                                    {t("channel.usingDefaults")} · 0 {t("channel.modelsCount")}
                                </span>
                            </button>
                        ) : (
                            <Badge variant="outline" className="text-xs">
                                -
                            </Badge>
                        )
                    )
                }

                return (
                    <Popover>
                        <PopoverTrigger asChild>
                            <button
                                className={cn(
                                    "inline-flex items-center gap-2 rounded-md border px-2.5 py-1 text-sm transition-colors cursor-pointer",
                                    usingDefaultModels
                                        ? "border-amber-300/70 bg-amber-50 text-amber-700 hover:border-amber-400 hover:bg-amber-100 hover:text-amber-800 dark:border-amber-700/70 dark:bg-amber-950/30 dark:text-amber-300 dark:hover:bg-amber-900/40"
                                        : "border-border bg-background hover:border-primary hover:text-primary"
                                )}
                            >
                                <span className="whitespace-nowrap">
                                    {usingDefaultModels
                                        ? `${t("channel.usingDefaults")} · ${models.length} ${t("channel.modelsCount")}`
                                        : `${models.length} ${t("channel.modelsCount")}`}
                                </span>
                                <ChevronDown className="h-3.5 w-3.5" />
                            </button>
                        </PopoverTrigger>
                        <PopoverContent className="w-auto p-3" align="start">
                            <div className="space-y-2">
                                <div className="flex items-center gap-2">
                                    <h4 className="font-medium text-sm">{t("channel.models")} ({models.length})</h4>
                                    {usingDefaultModels && (
                                        <Badge variant="outline" className="border-amber-300/70 bg-amber-50 text-xs text-amber-700 dark:border-amber-700/70 dark:bg-amber-950/30 dark:text-amber-300">
                                            {t("channel.usingDefaults")}
                                        </Badge>
                                    )}
                                </div>
                                <div className="flex flex-col gap-1 max-h-64 overflow-y-auto">
                                    {models.map((model, index) => (
                                        <button
                                            key={index}
                                            type="button"
                                            className={cn(
                                                "w-fit rounded-md border px-1.5 py-0.5 text-xs font-mono transition-colors text-left",
                                                "hover:border-primary hover:text-primary"
                                            )}
                                            onClick={() => openUpdateDialog(row.original)}
                                        >
                                            <div>{model}</div>
                                            {(() => {
                                                const pair = getChannelModelMetric(runtimeMetrics, row.original.id, model)
                                                if (!pair) return null
                                                return (
                                                    <div className="mt-1 flex flex-wrap gap-1 text-[10px]">
                                                        <span>RPM {pair.rpm}</span>
                                                        <span>TPM {pair.tpm}</span>
                                                        <span>ERR {formatPercent(pair.error_rate)}</span>
                                                        {pair.banned && <span className="text-destructive">{t('channel.temporarilyExcluded')}</span>}
                                                    </div>
                                                )
                                            })()}
                                        </button>
                                    ))}
                                </div>
                            </div>
                        </PopoverContent>
                    </Popover>
                )
            }
        },
        {
            id: 'temporarily_excluded_models',
            header: () => (
                <div className="flex items-center gap-1 font-medium py-3.5 whitespace-nowrap">
                    <span>{t("channel.temporarilyExcludedModels")}</span>
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <button
                                type="button"
                                className="text-muted-foreground hover:text-foreground transition-colors"
                                aria-label={t('channel.temporarilyExcludedHelp')}
                            >
                                <CircleHelp className="h-3.5 w-3.5" />
                            </button>
                        </TooltipTrigger>
                        <TooltipContent side="top" className="max-w-64">
                            {t('channel.temporarilyExcludedHelp')}
                        </TooltipContent>
                    </Tooltip>
                </div>
            ),
            cell: ({ row }) => {
                const excludedModels = getExcludedModels(row.original)
                if (excludedModels.length === 0) {
                    return <div className="text-xs text-muted-foreground">-</div>
                }

                return (
                    <div className="flex flex-wrap gap-1">
                        {excludedModels.map((model) => (
                            <Badge
                                key={model}
                                variant="destructive"
                                className="max-w-[220px] truncate text-xs"
                                title={t('channel.highErrorRateExcluded')}
                            >
                                {model}
                            </Badge>
                        ))}
                    </div>
                )
            },
        },
        {
            accessorKey: 'request_count',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.requestCount")}</div>,
            cell: ({ row }) => (
                <div
                    className={dashboardCell}
                    onClick={() => navigateToDashboard(row.original.id)}
                    title={t("channel.viewDashboard")}
                >
                    {row.original.request_count.toLocaleString()}
                </div>
            ),
        },
        {
            accessorKey: 'retry_count',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.retryCount")}</div>,
            cell: ({ row }) => (
                <div
                    className={dashboardCell}
                    onClick={() => navigateToDashboard(row.original.id)}
                    title={t("channel.viewDashboard")}
                >
                    {(row.original.retry_count || 0).toLocaleString()}
                </div>
            ),
        },
        {
            accessorKey: 'used_amount',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.usedAmount")}</div>,
            cell: ({ row }) => (
                <div
                    className={dashboardCell}
                    onClick={() => navigateToDashboard(row.original.id)}
                    title={t("channel.viewDashboard")}
                >
                    ${(row.original.used_amount || 0).toFixed(4)}
                </div>
            ),
        },
        {
            accessorKey: 'created_at',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.createdAt")}</div>,
            cell: ({ row }) => (
                <div className="text-sm text-muted-foreground">
                    {formatTimestamp(row.original.created_at)}
                </div>
            ),
        },
        {
            accessorKey: 'accessed_at',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.accessedAt")}</div>,
            cell: ({ row }) => (
                <div className="text-sm text-muted-foreground">
                    {formatAccessedAt(row.original.accessed_at, t("token.never"))}
                </div>
            ),
        },
        {
            accessorKey: 'status',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.status")}</div>,
            cell: ({ row }) => (
                <div>
                    {row.original.status === 2 ? (
                        <Badge variant="outline" className={cn(
                            "text-white dark:text-white/90",
                            "bg-destructive dark:bg-red-600/90"
                        )}>
                            {t("token.disabled")}
                        </Badge>
                    ) : (
                        <Badge variant="outline" className={cn(
                            "text-white dark:text-white/90",
                            "bg-primary dark:bg-[#4A4DA0]"
                        )}>
                            {t("token.enabled")}
                        </Badge>
                    )}
                </div>
            ),
        },
        {
            id: 'actions',
            cell: ({ row }) => (
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon">
                            <MoreHorizontal className="h-4 w-4" />
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                        <DropdownMenuItem
                            onClick={() => {
                                clearResults()
                                setIsTestAll(false)
                                setTestDialogOpen(true)
                                testChannel(row.original.id)
                            }}
                            disabled={isTesting}
                        >
                            <FlaskConical className="mr-2 h-4 w-4 text-blue-600 dark:text-blue-500" />
                            {isTesting ? t("channel.testing") : t("channel.test")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => openUpdateDialog(row.original)}
                        >
                            <Pencil className="mr-2 h-4 w-4" />
                            {t("channel.edit")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => openCopyDialog(row.original)}
                        >
                            <Copy className="mr-2 h-4 w-4" />
                            {t("channel.copyFrom")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => handleStatusChange(row.original.id, row.original.status)}
                            disabled={isStatusUpdating}
                        >
                            {row.original.status === 2 ? (
                                <>
                                    <Power className="mr-2 h-4 w-4 text-emerald-600 dark:text-emerald-500" />
                                    {t("channel.enable")}
                                </>
                            ) : (
                                <>
                                    <PowerOff className="mr-2 h-4 w-4 text-yellow-600 dark:text-yellow-500" />
                                    {t("channel.disable")}
                                </>
                            )}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => openDeleteDialog(row.original.id)}
                        >
                            <Trash2 className="mr-2 h-4 w-4 text-red-600 dark:text-red-500" />
                            {t("channel.delete")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => exportSingleChannel(row.original)}
                        >
                            <Download className="mr-2 h-4 w-4" />
                            {t("channel.export")}
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            ),
        },
    ], [t, isTesting, isStatusUpdating, getDisplayModels, runtimeMetrics, formatPercent, getExcludedModels])

    // 初始化表格
    const table = useReactTable({
        data: channels,
        columns,
        getCoreRowModel: getCoreRowModel(),
    })

    return (
        <>
            <Card className="border-none shadow-none p-6 flex flex-col h-full">
                {/* 标题和操作按钮 */}
                <div className="flex items-center justify-between mb-6">
                    <h2 className="text-xl font-semibold text-primary dark:text-[#6A6DE6]">{t("channel.management")}</h2>
                    <div className="flex gap-2">
                        <div className="w-48">
                            <Select
                                value={selectedChannelType ? String(selectedChannelType) : ''}
                                onValueChange={(value) => {
                                    setSelectedChannelType(value === '__all__' ? undefined : Number(value))
                                    setPage(1)
                                }}
                            >
                                <SelectTrigger className="h-9">
                                    <SelectValue placeholder={t('channel.dialog.selectType')} />
                                </SelectTrigger>
                                <SelectContent className="z-[60]">
                                    <SelectItem value="__all__">{t('common.all')}</SelectItem>
                                    {providerOptions.map((option) => (
                                        <SelectItem key={option.value} value={option.value}>
                                            {option.label}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="relative">
                            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                            <Input
                                placeholder={t("common.search")}
                                value={searchInput}
                                onChange={(e) => handleSearchChange(e.target.value)}
                                className="h-9 w-48 pl-8"
                            />
                        </div>
                        <AnimatedButton>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={() => setDefaultModelsDialogOpen(true)}
                                className="flex items-center gap-2 justify-center"
                            >
                                <Settings className="h-4 w-4" />
                                {t("channel.defaultModels.manage")}
                            </Button>
                        </AnimatedButton>
                        <AnimatedButton>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={() => {
                                    clearTestAllResults()
                                    setIsTestAll(true)
                                    setTestDialogOpen(true)
                                    testAllChannels()
                                }}
                                disabled={isTestingAll}
                                className="flex items-center gap-2 justify-center"
                            >
                                {isTestingAll ? (
                                    <Loader2 className="h-4 w-4 animate-spin" />
                                ) : (
                                    <FlaskConical className="h-4 w-4" />
                                )}
                                {t("channel.testAllChannels")}
                            </Button>
                        </AnimatedButton>
                        <AnimatedButton>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={refreshChannels}
                                className="flex items-center gap-2 justify-center"
                            >
                                <AnimatedIcon animationVariant="continuous-spin" isAnimating={isRefreshAnimating} className="h-4 w-4">
                                    <RefreshCcw className="h-4 w-4" />
                                </AnimatedIcon>
                                {t("channel.refresh")}
                            </Button>
                        </AnimatedButton>
                        <AnimatedButton>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={exportChannels}
                                disabled={!channels || channels.length === 0}
                                className="flex items-center gap-2 justify-center"
                            >
                                <Download className="h-4 w-4" />
                                {t("channel.export")}
                            </Button>
                        </AnimatedButton>
                        <AnimatedButton>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={triggerImport}
                                disabled={isImporting}
                                className="flex items-center gap-2 justify-center"
                            >
                                <Upload className="h-4 w-4" />
                                {isImporting ? t("channel.importing") : t("channel.import")}
                            </Button>
                        </AnimatedButton>
                        <input
                            ref={fileInputRef}
                            type="file"
                            accept=".json"
                            onChange={importChannels}
                            className="hidden"
                        />
                        <AnimatedButton>
                            <Button
                                size="sm"
                                onClick={openCreateDialog}
                                className="flex items-center gap-1 bg-primary hover:bg-primary/90 dark:bg-[#4A4DA0] dark:hover:bg-[#5155A5]"
                            >
                                <Plus className="h-3.5 w-3.5" />
                                {t("channel.add")}
                            </Button>
                        </AnimatedButton>
                    </div>
                </div>

                {/* 表格容器 */}
                <div className="flex-1 overflow-hidden flex flex-col">
                    <div className="overflow-auto flex-1">
                        <DataTable
                            table={table}
                            loadingStyle="skeleton"
                            columns={columns}
                            isLoading={isLoading || isLoadingRuntimeMetrics}
                            fixedHeader={true}
                            animatedRows={true}
                            showScrollShadows={true}
                        />
                    </div>

                    {/* 分页 */}
                    <ServerPagination
                        page={page}
                        pageSize={pageSize}
                        total={total}
                        onPageChange={setPage}
                        onPageSizeChange={(size) => { setPageSize(size); setPage(1) }}
                    />
                </div>
            </Card>

            {/* 默认模型管理对话框 */}
            <DefaultModelsDialog
                open={defaultModelsDialogOpen}
                onOpenChange={setDefaultModelsDialogOpen}
            />

            {/* 渠道对话框 */}
            <ChannelDialog
                open={channelDialogOpen}
                onOpenChange={setChannelDialogOpen}
                mode={dialogMode}
                channel={selectedChannel}
            />

            {/* 删除渠道对话框 */}
            <DeleteChannelDialog
                open={deleteDialogOpen}
                onOpenChange={setDeleteDialogOpen}
                channelId={selectedChannelId}
                onDeleted={() => setSelectedChannelId(null)}
            />

            {/* 测试结果对话框 */}
            <ChannelTestDialog
                open={testDialogOpen}
                onOpenChange={(open) => {
                    setTestDialogOpen(open)
                    if (!open) {
                        setIsTestAll(false)
                    }
                }}
                isTesting={isTestAll ? isTestingAll : isTesting}
                results={isTestAll ? testAllResults : testResults}
                showChannelInfo={true}
                onChannelClick={async (channelId: number) => {
                    let channel = channels.find(c => c.id === channelId)

                    if (!channel) {
                        await openResourceDialog({
                            fetcher: () => channelApi.getChannel(channelId),
                            onSuccess: (fullChannel) => {
                                channel = fullChannel
                            },
                            onNotFound: () => {
                                showDeletedResourceToast(t("channel.deleted"))
                            },
                            onError: () => {
                                showDeletedResourceToast(t("channel.fetchFailed"))
                            },
                        })
                    }

                    if (channel) {
                        setSelectedChannel(channel)
                        setDialogMode('update')
                        setChannelDialogOpen(true)
                    }
                }}
                onCancel={() => {
                    if (isTestAll) {
                        cancelTestAll()
                        clearTestAllResults()
                    } else {
                        cancelTest()
                        clearResults()
                    }
                    setTestDialogOpen(false)
                    setIsTestAll(false)
                }}
            />
        </>
    )
}
