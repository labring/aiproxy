// src/feature/token/components/TokenTable.tsx
import { useState, useRef, useEffect, useMemo } from 'react'
import {
    useReactTable,
    getCoreRowModel,
    ColumnDef,
} from '@tanstack/react-table'
import { useTokens, useUpdateTokenStatus } from '../hooks'
import { Token } from '@/types/token'
import { Button } from '@/components/ui/button'
import {
    MoreHorizontal, Plus, Trash2, RefreshCcw,
    PowerOff, Power, Copy
} from 'lucide-react'
import {
    DropdownMenu, DropdownMenuContent,
    DropdownMenuItem, DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Card } from '@/components/ui/card'
import { TokenDialog } from './TokenDialog'
import { Loader2 } from 'lucide-react'
import { DataTable } from '@/components/table/motion-data-table'
import { DeleteTokenDialog } from './DeleteTokenDialog'
import { useTranslation } from 'react-i18next'
import { AnimatedIcon } from '@/components/ui/animation/components/animated-icon'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'
import { Badge } from '@/components/ui/badge'
import { toast } from 'sonner'

export function TokenTable() {
    const { t } = useTranslation()

    // 状态管理
    const [tokenDialogOpen, setTokenDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [selectedTokenId, setSelectedTokenId] = useState<number | null>(null)
    const sentinelRef = useRef<HTMLDivElement>(null)
    const [isRefreshAnimating, setIsRefreshAnimating] = useState(false)

    // 获取Token列表
    const {
        data,
        isLoading,
        fetchNextPage,
        hasNextPage,
        isFetchingNextPage,
        refetch
    } = useTokens()

    // 更新Token状态
    const { updateStatus, isLoading: isStatusUpdating } = useUpdateTokenStatus()

    // 扁平化分页数据
    const flatData = useMemo(() =>
        data?.pages.flatMap(page => page.tokens) || [],
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

    // 打开创建Token对话框
    const openCreateDialog = () => {
        setTokenDialogOpen(true)
    }

    // 打开删除对话框
    const openDeleteDialog = (id: number) => {
        setSelectedTokenId(id)
        setDeleteDialogOpen(true)
    }

    // 更新Token状态
    const handleStatusChange = (id: number, currentStatus: number) => {
        // 状态切换: 1 -> 2 (禁用 -> 启用), 2 -> 1 (启用 -> 禁用)
        const newStatus = currentStatus === 1 ? 2 : 1
        updateStatus({ id, status: { status: newStatus } })
    }

    // 复制Token到剪贴板
    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text).then(() => {
            toast.success(t('common.copied'))
        }).catch(() => {
            toast.error(t('common.copyFailed'))
        })
    }

    // 刷新Token列表
    const refreshTokens = () => {
        setIsRefreshAnimating(true)
        refetch()

        // 停止动画，延迟1秒以匹配动画效果
        setTimeout(() => {
            setIsRefreshAnimating(false)
        }, 1000)
    }

    // 格式化日期时间
    const formatDateTime = (timestamp: number) => {
        if (timestamp < 0) return t('token.never')

        try {
            const date = new Date(timestamp)
            return new Intl.DateTimeFormat('zh-CN', {
                year: 'numeric',
                month: '2-digit',
                day: '2-digit',
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit'
            }).format(date)
        } catch (error) {
            console.error(error)
            return t('token.invalidDate')
        }
    }

    // 表格列定义
    const columns: ColumnDef<Token>[] = [
        {
            accessorKey: 'name',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("token.name")}</div>,
            cell: ({ row }) => <div className="font-medium">{row.original.name}</div>,
        },
        {
            accessorKey: 'key',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("token.key")}</div>,
            cell: ({ row }) => (
                <div className="flex items-center space-x-2">
                    <span className="font-mono">{row.original.key}</span>
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 w-6 p-0"
                        onClick={() => copyToClipboard(row.original.key)}
                    >
                        <Copy className="h-3.5 w-3.5" />
                    </Button>
                </div>
            ),
        },
        {
            accessorKey: 'accessed_at',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("token.lastUsed")}</div>,
            cell: ({ row }) => <div>{formatDateTime(row.original.accessed_at)}</div>,
        },
        {
            accessorKey: 'request_count',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("token.requestCount")}</div>,
            cell: ({ row }) => <div>{row.original.request_count}</div>,
        },
        {
            accessorKey: 'status',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("token.status")}</div>,
            cell: ({ row }) => (
                <div>
                    {row.original.status === 1 ? (
                        <Badge variant="outline" className="text-yellow-600 border-yellow-300 bg-yellow-50">
                            {t("token.disabled")}
                        </Badge>
                    ) : (
                        <Badge variant="outline" className="text-emerald-600 border-emerald-300 bg-emerald-50">
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
                            onClick={() => copyToClipboard(row.original.key)}
                        >
                            <Copy className="mr-2 h-4 w-4" />
                            {t("token.copyKey")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => handleStatusChange(row.original.id, row.original.status)}
                            disabled={isStatusUpdating}
                        >
                            {row.original.status === 1 ? (
                                <>
                                    <Power className="mr-2 h-4 w-4 text-emerald-600" />
                                    {t("token.enable")}
                                </>
                            ) : (
                                <>
                                    <PowerOff className="mr-2 h-4 w-4 text-yellow-600" />
                                    {t("token.disable")}
                                </>
                            )}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => openDeleteDialog(row.original.id)}
                            className="text-red-600"
                        >
                            <Trash2 className="mr-2 h-4 w-4" />
                            {t("token.delete")}
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
                    <h2 className="text-xl font-semibold text-primary">{t("token.management")}</h2>
                    <div className="flex gap-2">
                        <AnimatedButton>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={refreshTokens}
                                className="flex items-center gap-2 justify-center"
                            >
                                <AnimatedIcon animationVariant="continuous-spin" isAnimating={isRefreshAnimating} className="h-4 w-4">
                                    <RefreshCcw className="h-4 w-4" />
                                </AnimatedIcon>
                                {t("token.refresh")}
                            </Button>
                        </AnimatedButton>
                        <AnimatedButton>
                            <Button
                                size="sm"
                                onClick={openCreateDialog}
                                className="flex items-center gap-1"
                            >
                                <Plus className="h-3.5 w-3.5" />
                                {t("token.add")}
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

            {/* Token创建对话框 */}
            <TokenDialog
                open={tokenDialogOpen}
                onOpenChange={setTokenDialogOpen}
            />

            {/* 删除Token对话框 */}
            <DeleteTokenDialog
                open={deleteDialogOpen}
                onOpenChange={setDeleteDialogOpen}
                tokenId={selectedTokenId}
                onDeleted={() => setSelectedTokenId(null)}
            />
        </>
    )
}