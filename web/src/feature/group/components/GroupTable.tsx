// src/feature/group/components/GroupTable.tsx
import { useState, useRef, useEffect, useMemo } from 'react'
import {
    useReactTable,
    getCoreRowModel,
    ColumnDef,
} from '@tanstack/react-table'
import { useGroups, useUpdateGroupStatus } from '../hooks'
import type { Group } from '@/types/group'
import { Button } from '@/components/ui/button'
import {
    MoreHorizontal, Plus, Trash2, RefreshCcw,
    PowerOff, Power, Key
} from 'lucide-react'
import {
    DropdownMenu, DropdownMenuContent,
    DropdownMenuItem, DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Card } from '@/components/ui/card'
import { Loader2 } from 'lucide-react'
import { DataTable } from '@/components/table/motion-data-table'
import { DeleteGroupDialog } from './DeleteGroupDialog'
import { GroupDialog } from './GroupDialog'
import { CreateGroupDialog } from './CreateGroupDialog'
import { CreateGroupTokenDialog } from './CreateGroupTokenDialog'
import { useTranslation } from 'react-i18next'
import { AnimatedIcon } from '@/components/ui/animation/components/animated-icon'
import { AnimatedButton } from '@/components/ui/animation/components/animated-button'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { format } from 'date-fns'

// Format currency amount
const formatAmount = (amount: number): string => {
    if (amount >= 1000000) {
        return `${(amount / 1000000).toFixed(2)}M`
    }
    if (amount >= 1000) {
        return `${(amount / 1000).toFixed(2)}K`
    }
    return amount.toFixed(2)
}

// Format timestamp to date string
const formatTimestamp = (timestamp: number): string => {
    if (!timestamp) return '-'
    return format(new Date(timestamp), 'yyyy-MM-dd HH:mm')
}

export function GroupTable() {
    const { t } = useTranslation()

    // State management
    const [groupDialogOpen, setGroupDialogOpen] = useState(false)
    const [createGroupDialogOpen, setCreateGroupDialogOpen] = useState(false)
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
    const [tokenDialogOpen, setTokenDialogOpen] = useState(false)
    const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null)
    const sentinelRef = useRef<HTMLDivElement>(null)
    const [isRefreshAnimating, setIsRefreshAnimating] = useState(false)

    // Get groups list
    const {
        data,
        isLoading,
        fetchNextPage,
        hasNextPage,
        isFetchingNextPage,
        refetch
    } = useGroups()

    // Update group status
    const { updateStatus, isLoading: isStatusUpdating } = useUpdateGroupStatus()

    // Flatten paginated data
    const flatData = useMemo(() =>
        data?.pages.flatMap(page => page.groups) || [],
        [data]
    )

    // Optimized infinite scroll implementation
    useEffect(() => {
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

    // Open create group dialog
    const openCreateDialog = () => {
        setCreateGroupDialogOpen(true)
    }

    // Open group detail dialog
    const openDetailDialog = (groupId: string) => {
        setSelectedGroupId(groupId)
        setGroupDialogOpen(true)
    }

    // Open delete dialog
    const openDeleteDialog = (groupId: string) => {
        setSelectedGroupId(groupId)
        setDeleteDialogOpen(true)
    }

    // Open token creation dialog
    const openTokenDialog = (groupId: string) => {
        setSelectedGroupId(groupId)
        setTokenDialogOpen(true)
    }

    // Handle status change
    const handleStatusChange = (groupId: string, currentStatus: number) => {
        const newStatus = currentStatus === 2 ? 1 : 2
        updateStatus({ groupId, status: { status: newStatus } })
    }

    // Refresh groups list
    const refreshGroups = () => {
        setIsRefreshAnimating(true)
        refetch()
        setTimeout(() => {
            setIsRefreshAnimating(false)
        }, 1000)
    }

    // Table column definitions
    const columns: ColumnDef<Group>[] = [
        {
            accessorKey: 'id',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("group.name")}</div>,
            cell: ({ row }) => (
                <div
                    className="font-medium cursor-pointer hover:text-primary"
                    onClick={() => openDetailDialog(row.original.id)}
                >
                    {row.original.id}
                </div>
            ),
        },
        {
            accessorKey: 'status',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("group.status")}</div>,
            cell: ({ row }) => (
                <div>
                    {row.original.status === 2 ? (
                        <Badge variant="outline" className={cn(
                            "text-white dark:text-white/90",
                            "bg-destructive dark:bg-red-600/90"
                        )}>
                            {t("group.disabled")}
                        </Badge>
                    ) : (
                        <Badge variant="outline" className={cn(
                            "text-white dark:text-white/90",
                            "bg-primary dark:bg-[#4A4DA0]"
                        )}>
                            {t("group.enabled")}
                        </Badge>
                    )}
                </div>
            ),
        },
        {
            accessorKey: 'request_count',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("group.requestCount")}</div>,
            cell: ({ row }) => (
                <div className="font-mono">
                    {row.original.request_count?.toLocaleString() || 0}
                </div>
            ),
        },
        {
            accessorKey: 'used_amount',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("group.usedAmount")}</div>,
            cell: ({ row }) => (
                <div className="font-mono">
                    ${formatAmount(row.original.used_amount || 0)}
                </div>
            ),
        },
        {
            accessorKey: 'created_at',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("group.createdAt")}</div>,
            cell: ({ row }) => (
                <div className="text-sm text-muted-foreground">
                    {formatTimestamp(row.original.created_at)}
                </div>
            ),
        },
        {
            accessorKey: 'accessed_at',
            header: () => <div className="font-medium py-3.5 whitespace-nowrap">{t("group.accessedAt")}</div>,
            cell: ({ row }) => (
                <div className="text-sm text-muted-foreground">
                    {formatTimestamp(row.original.accessed_at)}
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
                            onClick={() => openTokenDialog(row.original.id)}
                        >
                            <Key className="mr-2 h-4 w-4" />
                            {t("group.createKey")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => handleStatusChange(row.original.id, row.original.status)}
                            disabled={isStatusUpdating}
                        >
                            {row.original.status === 2 ? (
                                <>
                                    <Power className="mr-2 h-4 w-4 text-emerald-600 dark:text-emerald-500" />
                                    {t("group.enable")}
                                </>
                            ) : (
                                <>
                                    <PowerOff className="mr-2 h-4 w-4 text-yellow-600 dark:text-yellow-500" />
                                    {t("group.disable")}
                                </>
                            )}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onClick={() => openDeleteDialog(row.original.id)}
                            className="text-destructive"
                        >
                            <Trash2 className="mr-2 h-4 w-4" />
                            {t("group.delete")}
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            ),
        },
    ]

    // Initialize table
    const table = useReactTable({
        data: flatData,
        columns,
        getCoreRowModel: getCoreRowModel(),
    })

    return (
        <>
            <Card className="border-none shadow-none p-6 flex flex-col h-full">
                {/* Title and action buttons */}
                <div className="flex items-center justify-between mb-6">
                    <h2 className="text-xl font-semibold text-primary dark:text-[#6A6DE6]">
                        {t("group.management")}
                    </h2>
                    <div className="flex gap-2">
                        <AnimatedButton>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={refreshGroups}
                                className="flex items-center gap-2 justify-center"
                            >
                                <AnimatedIcon animationVariant="continuous-spin" isAnimating={isRefreshAnimating} className="h-4 w-4">
                                    <RefreshCcw className="h-4 w-4" />
                                </AnimatedIcon>
                                {t("group.refresh")}
                            </Button>
                        </AnimatedButton>
                        <AnimatedButton>
                            <Button
                                size="sm"
                                onClick={openCreateDialog}
                                className="flex items-center gap-1 bg-primary hover:bg-primary/90 dark:bg-[#4A4DA0] dark:hover:bg-[#5155A5]"
                            >
                                <Plus className="h-3.5 w-3.5" />
                                {t("group.add")}
                            </Button>
                        </AnimatedButton>
                    </div>
                </div>

                {/* Table container */}
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

                        {/* Infinite scroll sentinel */}
                        {hasNextPage && (
                            <div
                                ref={sentinelRef}
                                className="h-5 flex justify-center items-center mt-4"
                            >
                                {isFetchingNextPage && (
                                    <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                                )}
                            </div>
                        )}
                    </div>
                </div>
            </Card>

            {/* Group detail dialog */}
            <GroupDialog
                open={groupDialogOpen}
                onOpenChange={setGroupDialogOpen}
                groupId={selectedGroupId}
            />

            {/* Create group dialog */}
            <CreateGroupDialog
                open={createGroupDialogOpen}
                onOpenChange={setCreateGroupDialogOpen}
            />

            {/* Delete group dialog */}
            <DeleteGroupDialog
                open={deleteDialogOpen}
                onOpenChange={setDeleteDialogOpen}
                groupId={selectedGroupId}
                onDeleted={() => setSelectedGroupId(null)}
            />

            {/* Create token dialog */}
            <CreateGroupTokenDialog
                open={tokenDialogOpen}
                onOpenChange={setTokenDialogOpen}
                groupId={selectedGroupId}
                onCreated={() => setSelectedGroupId(null)}
            />
        </>
    )
}
