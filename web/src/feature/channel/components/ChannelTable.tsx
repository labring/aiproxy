// src/feature/channel/components/ChannelTable.tsx
import { useState, useRef, useEffect, useMemo } from 'react'
import {
    useReactTable,
    getCoreRowModel,
    ColumnDef,
} from '@tanstack/react-table'
import { useChannels, useChannelTypeMetas, useUpdateChannelStatus } from '../hooks'
import { Channel } from '@/types/channel'
import { Button } from '@/components/ui/button'
import {
    MoreHorizontal, Plus, Trash2, RefreshCcw, Pencil,
    PowerOff, Power
} from 'lucide-react'
import {
    DropdownMenu, DropdownMenuContent,
    DropdownMenuItem, DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Card } from '@/components/ui/card'
import { ChannelDialog } from './ChannelDialog'
import { Loader2 } from 'lucide-react'
import { DataTable } from '@/components/table/motion-data-table'
import { DeleteChannelDialog } from './DeleteChannelDialog'
import { useTranslation } from 'react-i18next'
import { AnimatedIcon } from '@/components/ui/animation/components/animated-icon'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'
import { Badge } from '@/components/ui/badge'

export function ChannelTable() {
    const { t } = useTranslation()

    // 状态管理
    const [channelDialogOpen, setChannelDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [selectedChannelId, setSelectedChannelId] = useState<number | null>(null)
    const sentinelRef = useRef<HTMLDivElement>(null)
    const [dialogMode, setDialogMode] = useState<'create' | 'update'>('create')
    const [selectedChannel, setSelectedChannel] = useState<Channel | null>(null)
    const [isRefreshAnimating, setIsRefreshAnimating] = useState(false)

    // 获取渠道类型元数据
    const { data: typeMetas } = useChannelTypeMetas()

    // 获取渠道列表
    const {
        data,
        isLoading,
        fetchNextPage,
        hasNextPage,
        isFetchingNextPage,
        refetch
    } = useChannels()

    // 更新渠道状态
    const { updateStatus, isLoading: isStatusUpdating } = useUpdateChannelStatus()

    // 扁平化分页数据
    const flatData = useMemo(() =>
        data?.pages.flatMap(page => page.channels) || [],
        [data]
    )

    // 优化的无限滚动实现
    useEffect(() => {
        // 只有当有更多页面可加载时才创建观察器
        if (!hasNextPage) return

        const options = {
            threshold: 0.1,
            rootMargin: '100px 0px'
        }

        const handleObserver = (entries: IntersectionObserverEntry[]) => {
            const [entry] = entries
            if (entry.isIntersecting && hasNextPage && !isFetchingNextPage) {
                fetchNextPage()
            }
        }

        const observer = new IntersectionObserver(handleObserver, options)

        const sentinel = sentinelRef.current
        if (sentinel) {
            observer.observe(sentinel)
        }

        return () => {
            if (sentinel) {
                observer.unobserve(sentinel)
            }
            observer.disconnect()
        }
    }, [hasNextPage, isFetchingNextPage, fetchNextPage])

    // 打开创建渠道对话框
    const openCreateDialog = () => {
        setDialogMode('create')
        setSelectedChannel(null)
        setChannelDialogOpen(true)
    }

    // 打开更新渠道对话框
    const openUpdateDialog = (channel: Channel) => {
        setDialogMode('update')
        setSelectedChannel(channel)
        setChannelDialogOpen(true)
    }

    // 打开删除对话框
    const openDeleteDialog = (id: number) => {
        setSelectedChannelId(id)
        setDeleteDialogOpen(true)
    }

    // 更新渠道状态
    const handleStatusChange = (id: number, currentStatus: number) => {
        // 状态切换: 1 -> 2 (禁用 -> 启用), 2 -> 1 (启用 -> 禁用)
        const newStatus = currentStatus === 1 ? 2 : 1
        updateStatus({ id, status: { status: newStatus } })
    }

    // 刷新渠道列表
    const refreshChannels = () => {
        setIsRefreshAnimating(true)
        refetch()

        // 停止动画，延迟1秒以匹配动画效果
        setTimeout(() => {
            setIsRefreshAnimating(false)
        }, 1000)
    }

    // 获取渠道类型名称
    const getChannelTypeName = (typeId: number): string => {
        if (!typeMetas) return String(typeId)
        const meta = typeMetas[typeId]
        return meta ? meta.name : String(typeId)
    }

    // 表格列定义
    const columns: ColumnDef<Channel>[] = [
        {
            accessorKey: 'id',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.id")}</div>,
            cell: ({ row }) => <div className="font-medium">{row.original.id}</div>,
        },
        {
            accessorKey: 'name',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.name")}</div>,
            cell: ({ row }) => <div className="font-medium">{row.original.name}</div>,
        },
        {
            accessorKey: 'type',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.type")}</div>,
            cell: ({ row }) => (
                <div className="font-medium">
                    {getChannelTypeName(row.original.type)}
                </div>
            ),
        },
        {
            accessorKey: 'request_count',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.requestCount")}</div>,
            cell: ({ row }) => <div>{row.original.request_count}</div>,
        },
        {
            accessorKey: 'status',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("channel.status")}</div>,
            cell: ({ row }) => (
                <div>
                    {row.original.status === 1 ? (
                        <Badge variant="outline" className="text-yellow-600 border-yellow-300 bg-yellow-50">
                            {t("channel.disabled")}
                        </Badge>
                    ) : (
                        <Badge variant="outline" className="text-emerald-600 border-emerald-300 bg-emerald-50">
                            {t("channel.enabled")}
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
                            onClick={() => openUpdateDialog(row.original)}
                        >
                            <Pencil className="mr-2 h-4 w-4" />
                            {t("channel.edit")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => handleStatusChange(row.original.id, row.original.status)}
                            disabled={isStatusUpdating}
                        >
                            {row.original.status === 1 ? (
                                <>
                                    <Power className="mr-2 h-4 w-4 text-emerald-600" />
                                    {t("channel.enable")}
                                </>
                            ) : (
                                <>
                                    <PowerOff className="mr-2 h-4 w-4 text-yellow-600" />
                                    {t("channel.disable")}
                                </>
                            )}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => openDeleteDialog(row.original.id)}
                            className="text-red-600"
                        >
                            <Trash2 className="mr-2 h-4 w-4" />
                            {t("channel.delete")}
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            ),
        },
    ]

    // 初始化表格
    const table = useReactTable({
        data: flatData,
        columns,
        getCoreRowModel: getCoreRowModel(),
    })

    return (
        <>
            <Card className="border-none shadow-none p-6 flex flex-col h-full">
                {/* 标题和操作按钮 - 固定在顶部 */}
                <div className="flex items-center justify-between mb-6">
                    <h2 className="text-xl font-semibold text-primary">{t("channel.management")}</h2>
                    <div className="flex gap-2">
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
                                className="flex items-center gap-1"
                            >
                                <Plus className="h-3.5 w-3.5" />
                                {t("channel.add")}
                            </Button>
                        </AnimatedButton>
                    </div>
                </div>

                {/* 表格容器 - 设置固定高度和滚动 */}
                <div className="flex-1 overflow-hidden flex flex-col">
                    <div className="overflow-auto h-full">
                        <DataTable
                            table={table}
                            loadingStyle="skeleton"
                            columns={columns}
                            isLoading={isLoading}
                            fixedHeader={true}
                            animatedRows={true}
                            showScrollShadows={true}
                        />

                        {/* 无限滚动监测元素 - 在滚动区域内 */}
                        {hasNextPage && <div
                            ref={sentinelRef}
                            className="h-5 flex justify-center items-center mt-4"
                        >
                            {isFetchingNextPage && (
                                <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                            )}
                        </div>}
                    </div>
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
        </>
    )
}