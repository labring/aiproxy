// src/feature/channel/components/ChannelTable.tsx
import { useState, useCallback, useRef, useMemo } from 'react'
import {
    useReactTable,
    getCoreRowModel,
    ColumnDef,
} from '@tanstack/react-table'
import { useNavigate } from 'react-router'
import { useChannels, useChannelTypeMetas, useUpdateChannelStatus, useTestChannel, useTestAllChannels } from '../hooks'
import { channelApi } from '@/api/channel'
import { Channel } from '@/types/channel'
import { Button } from '@/components/ui/button'
import {
    MoreHorizontal, Plus, Trash2, RefreshCcw, Pencil,
    PowerOff, Power, FlaskConical, ChevronDown, ChevronRight, Search
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
import { ChannelTestDialog } from './ChannelTestDialog'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { AnimatedIcon } from '@/components/ui/animation/components/animated-icon'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { ROUTES } from '@/routes/constants'

export function ChannelTable() {
    const { t } = useTranslation()
    const navigate = useNavigate()

    // 状态管理
    const [channelDialogOpen, setChannelDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [selectedChannelId, setSelectedChannelId] = useState<number | null>(null)
    const [dialogMode, setDialogMode] = useState<'create' | 'update'>('create')
    const [selectedChannel, setSelectedChannel] = useState<Channel | null>(null)
    const [isRefreshAnimating, setIsRefreshAnimating] = useState(false)
    const [testDialogOpen, setTestDialogOpen] = useState(false)
    const [isTestAll, setIsTestAll] = useState(false)
    const [expandedModels, setExpandedModels] = useState<Record<number, boolean>>({})
    const [searchInput, setSearchInput] = useState('')
    const [searchKeyword, setSearchKeyword] = useState<string | undefined>(undefined)
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

    // 获取渠道列表
    const {
        data,
        isLoading,
        refetch
    } = useChannels(page, pageSize, searchKeyword)

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

    // 切换模型展开
    const toggleModels = (channelId: number) => {
        setExpandedModels(prev => ({ ...prev, [channelId]: !prev[channelId] }))
    }

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
            accessorKey: 'models',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.models")}</div>,
            size: 200,
            maxSize: 200,
            cell: ({ row }) => {
                const models = row.original.models || []
                const isExpanded = expandedModels[row.original.id]
                if (models.length === 0) return <div className="text-muted-foreground text-xs">-</div>

                return (
                    <div style={{ maxWidth: 200 }} className="overflow-hidden">
                        <div
                            className="flex items-center gap-1 cursor-pointer hover:text-primary transition-colors text-sm"
                            onClick={() => toggleModels(row.original.id)}
                        >
                            {isExpanded
                                ? <ChevronDown className="h-3.5 w-3.5 shrink-0" />
                                : <ChevronRight className="h-3.5 w-3.5 shrink-0" />
                            }
                            <span>{models.length} {t("channel.modelsCount")}</span>
                        </div>
                        {isExpanded && (
                            <div
                                className="mt-1 flex flex-wrap gap-1 cursor-pointer max-h-32 overflow-y-auto"
                                onClick={() => openUpdateDialog(row.original)}
                            >
                                {models.map((model, index) => (
                                    <Badge
                                        key={index}
                                        variant="outline"
                                        className="text-xs py-0 px-1.5 font-mono truncate max-w-[180px]"
                                    >
                                        {model}
                                    </Badge>
                                ))}
                            </div>
                        )}
                    </div>
                )
            }
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
                    </DropdownMenuContent>
                </DropdownMenu>
            ),
        },
    ], [t, expandedModels, isTesting, isStatusUpdating])

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
                            isLoading={isLoading}
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
                        try {
                            channel = await channelApi.getChannel(channelId)
                        } catch {
                            toast.error(t("channel.fetchFailed"))
                            return
                        }
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
