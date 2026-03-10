// src/feature/group/components/GroupTokensTab.tsx
import { useState, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import {
    useReactTable,
    getCoreRowModel,
    ColumnDef,
} from '@tanstack/react-table'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { tokenApi } from '@/api/token'
import type { Token } from '@/types/token'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
    MoreHorizontal, Power, PowerOff, Trash2, Plus, Copy, Settings, RefreshCcw, Search
} from 'lucide-react'
import {
    DropdownMenu, DropdownMenuContent,
    DropdownMenuItem, DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { DataTable } from '@/components/table/motion-data-table'
import { ServerPagination } from '@/components/table/server-pagination'
import { TokenQuotaDialog } from '@/feature/token/components/TokenQuotaDialog'
import { DeleteTokenDialog } from '@/feature/token/components/DeleteTokenDialog'
import { CreateGroupTokenDialog } from './CreateGroupTokenDialog'
import { AnimatedIcon } from '@/components/ui/animation/components/animated-icon'
import { cn } from '@/lib/utils'
import { toast } from 'sonner'

// Mask API key - show prefix and last 4 chars
const maskApiKey = (key: string): string => {
    if (key.length <= 8) return key
    const prefix = key.slice(0, 6)
    const suffix = key.slice(-4)
    return `${prefix}****${suffix}`
}

// Calculate remaining quota
const calculateRemainingQuota = (token: Token): { total: number; period: number } => {
    const total = token.quota > 0 ? Math.max(0, token.quota - token.used_amount) : -1
    const periodUsed = token.used_amount - (token.period_last_update_amount || 0)
    const period = token.period_quota > 0 ? Math.max(0, token.period_quota - periodUsed) : -1
    return { total, period }
}

// Calculate next refresh time
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

// Format remaining time
const formatRemainingTime = (targetDate: Date): string => {
    const now = new Date()
    const diff = targetDate.getTime() - now.getTime()
    if (diff <= 0) return "Expired"
    const days = Math.floor(diff / (1000 * 60 * 60 * 24))
    const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
    const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))
    if (days > 0) return `${days}d ${hours}h`
    if (hours > 0) return `${hours}h ${minutes}m`
    return `${minutes}m`
}

interface GroupTokensTabProps {
    groupId: string
}

export function GroupTokensTab({ groupId }: GroupTokensTabProps) {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const [tokenDialogOpen, setTokenDialogOpen] = useState(false)
    const [quotaDialogOpen, setQuotaDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [selectedToken, setSelectedToken] = useState<Token | null>(null)
    const [selectedTokenId, setSelectedTokenId] = useState<number | null>(null)
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

    // Get group tokens with pagination
    const {
        data,
        isLoading,
        refetch
    } = useQuery({
        queryKey: ['groupTokens', groupId, page, pageSize, searchKeyword],
        queryFn: () => tokenApi.getGroupTokens(groupId, page, pageSize, searchKeyword),
        enabled: !!groupId,
    })

    // Update token status
    const statusMutation = useMutation({
        mutationFn: ({ id, status }: { id: number; status: number }) => {
            return tokenApi.updateTokenStatus(id, { status })
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['groupTokens', groupId] })
            toast.success(t('common.success'))
        },
        onError: (err: Error) => {
            toast.error(err.message || 'Failed to update status')
        },
    })

    const tokens = data?.tokens || []
    const total = data?.total || 0

    const openDeleteDialog = (id: number) => {
        setSelectedTokenId(id)
        setDeleteDialogOpen(true)
    }

    const openQuotaDialog = (token: Token) => {
        setSelectedToken(token)
        setQuotaDialogOpen(true)
    }

    const handleStatusChange = (id: number, currentStatus: number) => {
        const newStatus = currentStatus === 2 ? 1 : 2
        statusMutation.mutate({ id, status: newStatus })
    }

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text).then(() => {
            toast.success(t('common.copied'))
        }).catch(() => {
            toast.error(t('common.copyFailed'))
        })
    }

    const refreshTokens = () => {
        setIsRefreshAnimating(true)
        refetch()
        setTimeout(() => setIsRefreshAnimating(false), 1000)
    }

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
                    <span className="font-mono text-xs">{maskApiKey(row.original.key)}</span>
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
                    return <span className="text-muted-foreground text-sm">{t("token.quota.unlimited")}</span>
                }
                return (
                    <div className="text-sm space-y-1">
                        {token.quota > 0 && (
                            <div className="flex items-center gap-1">
                                <span className="text-muted-foreground">Total:</span>
                                <span className={cn(remaining.total < token.quota * 0.1 ? "text-destructive" : "text-emerald-600")}>
                                    {remaining.total.toFixed(2)}
                                </span>
                            </div>
                        )}
                        {token.period_quota > 0 && (
                            <div className="flex items-center gap-1">
                                <span className="text-muted-foreground">Period:</span>
                                <span className={cn(remaining.period < token.period_quota * 0.1 ? "text-destructive" : "text-emerald-600")}>
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
                const nextRefresh = calculateNextRefreshTime(row.original)
                if (!nextRefresh) return <span className="text-muted-foreground text-sm">-</span>
                return <span className="text-sm">{formatRemainingTime(nextRefresh)}</span>
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
                        <Badge variant="outline" className={cn("text-white dark:text-white/90", "bg-destructive dark:bg-red-600/90")}>
                            {t("token.disabled")}
                        </Badge>
                    ) : (
                        <Badge variant="outline" className={cn("text-white dark:text-white/90", "bg-primary dark:bg-[#4A4DA0]")}>
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
                        <DropdownMenuItem onClick={() => copyToClipboard(row.original.key)}>
                            <Copy className="mr-2 h-4 w-4" />
                            {t("token.copyKey")}
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => openQuotaDialog(row.original)}>
                            <Settings className="mr-2 h-4 w-4" />
                            {t("token.quota.configure")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => handleStatusChange(row.original.id, row.original.status)}
                            disabled={statusMutation.isPending}
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
                            className="text-destructive"
                        >
                            <Trash2 className="mr-2 h-4 w-4" />
                            {t("token.delete")}
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            ),
        },
    ]

    const table = useReactTable({
        data: tokens,
        columns,
        getCoreRowModel: getCoreRowModel(),
    })

    return (
        <>
            <div className="flex flex-col h-full">
                {/* Action buttons */}
                <div className="flex items-center justify-end mb-2">
                    <div className="flex gap-2">
                        <div className="relative">
                            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
                            <Input
                                placeholder={t("common.search")}
                                value={searchInput}
                                onChange={(e) => handleSearchChange(e.target.value)}
                                className="h-8 w-40 pl-8 text-sm"
                            />
                        </div>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={refreshTokens}
                            className="flex items-center gap-1.5 h-8"
                        >
                            <AnimatedIcon animationVariant="continuous-spin" isAnimating={isRefreshAnimating} className="h-3.5 w-3.5">
                                <RefreshCcw className="h-3.5 w-3.5" />
                            </AnimatedIcon>
                            {t("token.refresh")}
                        </Button>
                        <Button
                            size="sm"
                            onClick={() => setTokenDialogOpen(true)}
                            className="flex items-center gap-1 h-8"
                        >
                            <Plus className="h-3.5 w-3.5" />
                            {t("token.add")}
                        </Button>
                    </div>
                </div>

                {/* Table */}
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

                    {/* Pagination */}
                    <ServerPagination
                        page={page}
                        pageSize={pageSize}
                        total={total}
                        onPageChange={setPage}
                        onPageSizeChange={(size) => { setPageSize(size); setPage(1) }}
                    />
                </div>
            </div>

            {/* Create token dialog */}
            <CreateGroupTokenDialog
                open={tokenDialogOpen}
                onOpenChange={setTokenDialogOpen}
                groupId={groupId}
                onCreated={() => refetch()}
            />

            {/* Token quota dialog */}
            <TokenQuotaDialog
                open={quotaDialogOpen}
                onOpenChange={setQuotaDialogOpen}
                token={selectedToken}
            />

            {/* Delete token dialog */}
            <DeleteTokenDialog
                open={deleteDialogOpen}
                onOpenChange={setDeleteDialogOpen}
                tokenId={selectedTokenId}
                onDeleted={() => {
                    setSelectedTokenId(null)
                    refetch()
                }}
            />
        </>
    )
}
