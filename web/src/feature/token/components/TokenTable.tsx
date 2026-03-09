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
    PowerOff, Power, Copy, Settings
} from 'lucide-react'
import {
    DropdownMenu, DropdownMenuContent,
    DropdownMenuItem, DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Card } from '@/components/ui/card'
import { TokenDialog } from './TokenDialog'
import { TokenQuotaDialog } from './TokenQuotaDialog'
import { Loader2 } from 'lucide-react'
import { DataTable } from '@/components/table/motion-data-table'
import { DeleteTokenDialog } from './DeleteTokenDialog'
import { useTranslation } from 'react-i18next'
import { AnimatedIcon } from '@/components/ui/animation/components/animated-icon'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'
import { Badge } from '@/components/ui/badge'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'

// 计算剩余额度
const calculateRemainingQuota = (token: Token): { total: number; period: number } => {
    const total = token.quota > 0 ? Math.max(0, token.quota - token.used_amount) : -1
    // 周期使用量 = 当前总使用量 - 上次周期更新时的使用量
    const periodUsed = token.used_amount - (token.period_last_update_amount || 0)
    const period = token.period_quota > 0 ? Math.max(0, token.period_quota - periodUsed) : -1
    return { total, period }
}

// 计算下次刷新时间
const calculateNextRefreshTime = (token: Token): Date | null => {
    if (!token.period_quota || token.period_quota <= 0 || !token.period_last_update_time) {
        return null
    }

    const lastUpdate = new Date(token.period_last_update_time)

    switch (token.period_type) {
        case 'daily':
            // 下一天的同一时间
            const nextDay = new Date(lastUpdate)
            nextDay.setDate(nextDay.getDate() + 1)
            return nextDay
        case 'weekly':
            // 7天后
            const nextWeek = new Date(lastUpdate)
            nextWeek.setDate(nextWeek.getDate() + 7)
            return nextWeek
        case 'monthly':
        default:
            // 下个月的同一天
            const nextMonth = new Date(lastUpdate)
            nextMonth.setMonth(nextMonth.getMonth() + 1)
            return nextMonth
    }
}

// 格式化剩余时间
const formatRemainingTime = (targetDate: Date): string => {
    const now = new Date()
    const diff = targetDate.getTime() - now.getTime()

    if (diff <= 0) {
        return "Expired"
    }

    const days = Math.floor(diff / (1000 * 60 * 60 * 24))
    const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
    const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))

    if (days > 0) {
        return `${days}d ${hours}h`
    }
    if (hours > 0) {
        return `${hours}h ${minutes}m`
    }
    return `${minutes}m`
}

export function TokenTable() {
    const { t } = useTranslation()

    // 状态管理
    const [tokenDialogOpen, setTokenDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [quotaDialogOpen, setQuotaDialogOpen] = useState(false)
    const [selectedTokenId, setSelectedTokenId] = useState<number | null>(null)
    const [selectedToken, setSelectedToken] = useState<Token | null>(null)
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

    // 打开限额配置对话框
    const openQuotaDialog = (token: Token) => {
        setSelectedToken(token)
        setQuotaDialogOpen(true)
    }

    // 更新Token状态
    const handleStatusChange = (id: number, currentStatus: number) => {
        // 状态切换: 2 -> 1 (禁用 -> 启用), 1 -> 2 (启用 -> 禁用)
        const newStatus = currentStatus === 2 ? 1 : 2
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
            accessorKey: 'quota',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("token.quota.remainingQuota")}</div>,
            cell: ({ row }) => {
                const token = row.original
                const remaining = calculateRemainingQuota(token)

                // 没有设置限额
                if (remaining.total < 0 && remaining.period < 0) {
                    return (
                        <span className="text-muted-foreground text-sm">
                            {t("token.quota.unlimited")}
                        </span>
                    )
                }

                return (
                    <div className="text-sm space-y-1">
                        {token.quota > 0 && (
                            <div className="flex items-center gap-1">
                                <span className="text-muted-foreground">Total:</span>
                                <span className={cn(
                                    remaining.total < token.quota * 0.1 ? "text-destructive" : "text-emerald-600"
                                )}>
                                    {remaining.total.toFixed(2)}
                                </span>
                            </div>
                        )}
                        {token.period_quota > 0 && (
                            <div className="flex items-center gap-1">
                                <span className="text-muted-foreground">Period:</span>
                                <span className={cn(
                                    remaining.period < token.period_quota * 0.1 ? "text-destructive" : "text-emerald-600"
                                )}>
                                    {remaining.period.toFixed(2)}
                                </span>
                            </div>
                        )}
                    </div>
                )
            },
        },
        {
            accessorKey: 'period_reset',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("token.quota.nextRefresh")}</div>,
            cell: ({ row }) => {
                const token = row.original
                const nextRefresh = calculateNextRefreshTime(token)

                if (!nextRefresh) {
                    return (
                        <span className="text-muted-foreground text-sm">-</span>
                    )
                }

                return (
                    <span className="text-sm">
                        {formatRemainingTime(nextRefresh)}
                    </span>
                )
            },
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
                            onClick={() => copyToClipboard(row.original.key)}
                        >
                            <Copy className="mr-2 h-4 w-4" />
                            {t("token.copyKey")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => openQuotaDialog(row.original)}
                        >
                            <Settings className="mr-2 h-4 w-4" />
                            {t("token.quota.configure")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => handleStatusChange(row.original.id, row.original.status)}
                            disabled={isStatusUpdating}
                        >
                            {row.original.status === 2 ? (
                                <>
                                    <Power className="mr-2 h-4 w-4 text-emerald-600 dark:text-emerald-500" />
                                    {t("token.enable")}
                                </>
                            ) : (
                                <>
                                    <PowerOff className="mr-2 h-4 w-4 text-yellow-600 dark:text-yellow-500" />
                                    {t("token.disable")}
                                </>
                            )}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => openDeleteDialog(row.original.id)}
                        >
                            <Trash2 className="mr-2 h-4 w-4 text-red-600 dark:text-red-500" />
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
                    <h2 className="text-xl font-semibold text-primary dark:text-[#6A6DE6]">
                        {t("token.management")}
                    </h2>
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
                                className="flex items-center gap-1 bg-primary hover:bg-primary/90 dark:bg-[#4A4DA0] dark:hover:bg-[#5155A5]"
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

            {/* Token限额配置对话框 */}
            <TokenQuotaDialog
                open={quotaDialogOpen}
                onOpenChange={setQuotaDialogOpen}
                token={selectedToken}
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
