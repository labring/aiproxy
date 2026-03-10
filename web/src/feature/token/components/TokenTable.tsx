// src/feature/token/components/TokenTable.tsx
import { useState, useCallback, useMemo } from 'react'
import {
    useReactTable,
    getCoreRowModel,
    ColumnDef,
} from '@tanstack/react-table'
import { useTokens, useUpdateTokenStatus } from '../hooks'
import { Token } from '@/types/token'
import { Button } from '@/components/ui/button'
import {
    MoreHorizontal, Trash2, RefreshCcw,
    PowerOff, Power, Copy, Settings, Search
} from 'lucide-react'
import {
    DropdownMenu, DropdownMenuContent,
    DropdownMenuItem, DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { TokenQuotaDialog } from './TokenQuotaDialog'
import { DataTable } from '@/components/table/motion-data-table'
import { ServerPagination } from '@/components/table/server-pagination'
import { DeleteTokenDialog } from './DeleteTokenDialog'
import { useTranslation } from 'react-i18next'
import { AnimatedIcon } from '@/components/ui/animation/components/animated-icon'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'
import { Badge } from '@/components/ui/badge'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { GroupDialog } from '@/feature/group/components/GroupDialog'
import { useRef } from 'react'

// 遮蔽 API Key，只显示前缀和最后4位
const maskApiKey = (key: string): string => {
    if (key.length <= 8) return key
    const prefix = key.slice(0, 6)
    const suffix = key.slice(-4)
    return `${prefix}****${suffix}`
}

// 计算剩余额度
const calculateRemainingQuota = (token: Token): { total: number; period: number } => {
    const total = token.quota > 0 ? Math.max(0, token.quota - token.used_amount) : -1
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
        case 'daily': {
            const nextDay = new Date(lastUpdate)
            nextDay.setDate(nextDay.getDate() + 1)
            return nextDay
        }
        case 'weekly': {
            const nextWeek = new Date(lastUpdate)
            nextWeek.setDate(nextWeek.getDate() + 7)
            return nextWeek
        }
        case 'monthly':
        default: {
            const nextMonth = new Date(lastUpdate)
            nextMonth.setMonth(nextMonth.getMonth() + 1)
            return nextMonth
        }
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
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [quotaDialogOpen, setQuotaDialogOpen] = useState(false)
    const [selectedTokenId, setSelectedTokenId] = useState<number | null>(null)
    const [selectedToken, setSelectedToken] = useState<Token | null>(null)
    const [groupDialogOpen, setGroupDialogOpen] = useState(false)
    const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null)
    const [groupDialogTab, setGroupDialogTab] = useState<string>('dashboard')
    const [groupDialogTokenName, setGroupDialogTokenName] = useState<string | undefined>(undefined)
    const [isRefreshAnimating, setIsRefreshAnimating] = useState(false)
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

    // 获取Token列表
    const {
        data,
        isLoading,
        refetch
    } = useTokens(page, pageSize, searchKeyword)

    // 更新Token状态
    const { updateStatus, isLoading: isStatusUpdating } = useUpdateTokenStatus()

    const tokens = useMemo(() => data?.tokens || [], [data?.tokens])
    const total = data?.total || 0

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
        setTimeout(() => {
            setIsRefreshAnimating(false)
        }, 1000)
    }

    // 表格列定义
    // eslint-disable-next-line react-hooks/exhaustive-deps
    const columns: ColumnDef<Token>[] = useMemo(() => [
        {
            accessorKey: 'name',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("token.name")}</div>,
            cell: ({ row }) => {
                const group = row.original.group
                return (
                    <div
                        className={cn("font-medium", group && "cursor-pointer hover:text-primary hover:underline underline-offset-4 transition-colors")}
                        onClick={() => {
                            if (group) {
                                setGroupDialogTab('tokens')
                                setGroupDialogTokenName(undefined)
                                setSelectedGroupId(group)
                                setGroupDialogOpen(true)
                            }
                        }}
                    >
                        {row.original.name}
                    </div>
                )
            },
        },
        {
            accessorKey: 'group',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("token.group")}</div>,
            cell: ({ row }) => {
                const group = row.original.group
                if (!group) return <div className="text-sm text-muted-foreground">-</div>
                return (
                    <div
                        className="text-sm text-muted-foreground cursor-pointer hover:text-primary hover:underline"
                        onClick={() => {
                            setGroupDialogTab('tokens')
                            setSelectedGroupId(group)
                            setGroupDialogOpen(true)
                        }}
                    >
                        {group}
                    </div>
                )
            },
        },
        {
            accessorKey: 'key',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("token.key")}</div>,
            cell: ({ row }) => (
                <div className="flex items-center space-x-2">
                    <span
                        className="font-mono cursor-pointer hover:text-primary transition-colors"
                        onClick={() => copyToClipboard(row.original.key)}
                    >
                        {maskApiKey(row.original.key)}
                    </span>
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

                if (remaining.total < 0 && remaining.period < 0) {
                    return (
                        <span
                            className="text-muted-foreground text-sm cursor-pointer hover:text-primary transition-colors"
                            onClick={() => openQuotaDialog(token)}
                        >
                            {t("token.quota.unlimited")}
                        </span>
                    )
                }

                return (
                    <div
                        className="text-sm space-y-1 cursor-pointer hover:text-primary transition-colors"
                        onClick={() => openQuotaDialog(token)}
                    >
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
                        <span
                            className="text-muted-foreground text-sm cursor-pointer hover:text-primary transition-colors"
                            onClick={() => openQuotaDialog(token)}
                        >
                            -
                        </span>
                    )
                }

                return (
                    <span
                        className="text-sm cursor-pointer hover:text-primary transition-colors"
                        onClick={() => openQuotaDialog(token)}
                    >
                        {formatRemainingTime(nextRefresh)}
                    </span>
                )
            },
        },
        {
            accessorKey: 'used_amount',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("token.usedAmount")}</div>,
            cell: ({ row }) => {
                const group = row.original.group
                return (
                    <div
                        className={cn(group && "cursor-pointer hover:text-primary transition-colors")}
                        onClick={() => {
                            if (group) {
                                setGroupDialogTab('dashboard')
                                setGroupDialogTokenName(row.original.name)
                                setSelectedGroupId(group)
                                setGroupDialogOpen(true)
                            }
                        }}
                    >
                        ${(row.original.used_amount || 0).toFixed(4)}
                    </div>
                )
            },
        },
        {
            accessorKey: 'request_count',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("token.requestCount")}</div>,
            cell: ({ row }) => {
                const group = row.original.group
                return (
                    <div
                        className={cn(group && "cursor-pointer hover:text-primary transition-colors")}
                        onClick={() => {
                            if (group) {
                                setGroupDialogTab('dashboard')
                                setGroupDialogTokenName(row.original.name)
                                setSelectedGroupId(group)
                                setGroupDialogOpen(true)
                            }
                        }}
                    >
                        {row.original.request_count.toLocaleString()}
                    </div>
                )
            },
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
    ], [t, isStatusUpdating])

    // 初始化表格
    const table = useReactTable({
        data: tokens,
        columns,
        getCoreRowModel: getCoreRowModel(),
    })

    return (
        <>
            <Card className="border-none shadow-none p-6 flex flex-col h-full">
                {/* 标题和操作按钮 */}
                <div className="flex items-center justify-between mb-6">
                    <h2 className="text-xl font-semibold text-primary dark:text-[#6A6DE6]">
                        {t("token.management")}
                    </h2>
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
                                onClick={refreshTokens}
                                className="flex items-center gap-2 justify-center"
                            >
                                <AnimatedIcon animationVariant="continuous-spin" isAnimating={isRefreshAnimating} className="h-4 w-4">
                                    <RefreshCcw className="h-4 w-4" />
                                </AnimatedIcon>
                                {t("token.refresh")}
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

            {/* Group详情对话框 */}
            <GroupDialog
                open={groupDialogOpen}
                onOpenChange={setGroupDialogOpen}
                groupId={selectedGroupId}
                initialTab={groupDialogTab}
                initialTokenName={groupDialogTokenName}
            />
        </>
    )
}
