import { useState, useMemo, useCallback, useRef } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Users, RefreshCcw, Shield, Pencil, ArrowUpDown, ArrowUp, ArrowDown } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { DataTable } from "@/components/table/motion-data-table"
import { ServerPagination } from "@/components/table/server-pagination"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from "@/components/ui/dialog"
import { enterpriseApi, type FeishuUser } from "@/api/enterprise"
import { toast } from "sonner"
import { ColumnDef, useReactTable, getCoreRowModel } from "@tanstack/react-table"
import { format } from "date-fns"
import { Label } from "@/components/ui/label"

const roleColors = {
    admin: "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
    analyst: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
    viewer: "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200",
}

export default function UsersPage() {
    const { t } = useTranslation()
    const queryClient = useQueryClient()
    const [page, setPage] = useState(1)
    const [pageSize, setPageSize] = useState(20)
    const [searchInput, setSearchInput] = useState("")
    const [keyword, setKeyword] = useState("")
    const [sortBy, setSortBy] = useState<string>("id")
    const [sortOrder, setSortOrder] = useState<"asc" | "desc">("desc")
    const [level1Department, setLevel1Department] = useState<string>("all")
    const [level2Department, setLevel2Department] = useState<string>("all")
    const [roleDialogOpen, setRoleDialogOpen] = useState(false)
    const [quotaDialogOpen, setQuotaDialogOpen] = useState(false)
    const [selectedUser, setSelectedUser] = useState<FeishuUser | null>(null)
    const [selectedRole, setSelectedRole] = useState<string>("")
    const [selectedPolicyId, setSelectedPolicyId] = useState<number | null>(null)
    const searchTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

    // Debounced search handler
    const handleSearchChange = useCallback((value: string) => {
        setSearchInput(value)
        if (searchTimerRef.current) clearTimeout(searchTimerRef.current)
        searchTimerRef.current = setTimeout(() => {
            setKeyword(value || "")
            setPage(1)
        }, 300)
    }, [])

    // Department filter handlers
    const handleLevel1Change = useCallback((value: string) => {
        setLevel1Department(value)
        setLevel2Department("all") // Reset level2 when level1 changes
        setPage(1)
    }, [])

    const handleLevel2Change = useCallback((value: string) => {
        setLevel2Department(value)
        setPage(1)
    }, [])

    const handleClearFilters = useCallback(() => {
        setLevel1Department("all")
        setLevel2Department("all")
        setSearchInput("")
        setKeyword("")
        setPage(1)
    }, [])

    // Sort handler
    const handleSort = useCallback((field: string) => {
        if (sortBy === field) {
            // Toggle order if same field
            setSortOrder(sortOrder === "asc" ? "desc" : "asc")
        } else {
            // New field, default to ascending
            setSortBy(field)
            setSortOrder("asc")
        }
        setPage(1) // Reset to first page when sorting
    }, [sortBy, sortOrder])

    // Render sort icon
    const renderSortIcon = useCallback((field: string) => {
        if (sortBy !== field) {
            return <ArrowUpDown className="w-4 h-4 ml-1 opacity-40" />
        }
        return sortOrder === "asc"
            ? <ArrowUp className="w-4 h-4 ml-1" />
            : <ArrowDown className="w-4 h-4 ml-1" />
    }, [sortBy, sortOrder])

    // Fetch users
    const { data, isLoading, refetch } = useQuery({
        queryKey: ["feishu-users", page, pageSize, keyword, sortBy, sortOrder, level1Department, level2Department],
        queryFn: () => enterpriseApi.getFeishuUsers(
            page,
            pageSize,
            keyword,
            sortBy,
            sortOrder,
            level1Department === "all" ? undefined : level1Department,
            level2Department === "all" ? undefined : level2Department
        ),
        staleTime: 30000, // 30 seconds
        refetchOnWindowFocus: false,
    })

    // Fetch department levels for filters
    const { data: deptLevelsData } = useQuery({
        queryKey: ["dept-levels", level1Department],
        queryFn: () => enterpriseApi.getDepartmentLevels(
            level1Department === "all" ? undefined : level1Department
        ),
        staleTime: 60000, // 1 minute
        refetchOnWindowFocus: false,
    })

    // Fetch policies for assignment
    const { data: policiesData } = useQuery({
        queryKey: ["quota-policies"],
        queryFn: () => enterpriseApi.listQuotaPolicies(1, 100),
        staleTime: 60000, // 1 minute
        refetchOnWindowFocus: false,
    })

    // Sync mutation
    const syncMutation = useMutation({
        mutationFn: () => enterpriseApi.triggerFeishuSync(),
        onSuccess: () => {
            toast.success(t("enterprise.users.syncStarted"))
            setTimeout(() => refetch(), 3000)
        },
        onError: (error: Error) => {
            toast.error(error.message || t("enterprise.users.syncFailed"))
        },
    })

    // Update role mutation
    const updateRoleMutation = useMutation({
        mutationFn: ({ open_id, role }: { open_id: string; role: string }) =>
            enterpriseApi.updateFeishuUserRole(open_id, role),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["feishu-users"] })
            toast.success(t("enterprise.users.roleUpdated"))
            setRoleDialogOpen(false)
        },
        onError: (error: Error) => {
            toast.error(error.message || t("enterprise.users.roleUpdateFailed"))
        },
    })

    // Bind quota mutation
    const bindQuotaMutation = useMutation({
        mutationFn: ({ open_id, policy_id }: { open_id: string; policy_id: number }) =>
            enterpriseApi.bindPolicyToUser(open_id, policy_id),
        onSuccess: () => {
            toast.success(t("enterprise.users.quotaAssigned"))
            setQuotaDialogOpen(false)
        },
        onError: (error: Error) => {
            toast.error(error.message || t("enterprise.users.quotaAssignFailed"))
        },
    })

    const handleSync = useCallback(() => {
        syncMutation.mutate()
    }, [syncMutation])

    const handleRoleEdit = useCallback((user: FeishuUser) => {
        setSelectedUser(user)
        setSelectedRole(user.role)
        setRoleDialogOpen(true)
    }, [])

    const handleRoleSave = useCallback(() => {
        if (selectedUser && selectedRole) {
            updateRoleMutation.mutate({ open_id: selectedUser.open_id, role: selectedRole })
        }
    }, [selectedUser, selectedRole, updateRoleMutation])

    const handleQuotaAssign = useCallback((user: FeishuUser) => {
        setSelectedUser(user)
        setSelectedPolicyId(null)
        setQuotaDialogOpen(true)
    }, [])

    const handleQuotaSave = useCallback(() => {
        if (selectedUser && selectedPolicyId) {
            bindQuotaMutation.mutate({ open_id: selectedUser.open_id, policy_id: selectedPolicyId })
        }
    }, [selectedUser, selectedPolicyId, bindQuotaMutation])

    const columns: ColumnDef<FeishuUser>[] = useMemo(() => [
        {
            accessorKey: "name",
            header: () => (
                <div
                    className="font-medium flex items-center cursor-pointer hover:text-primary"
                    onClick={() => handleSort("name")}
                >
                    {t("enterprise.users.name")}
                    {renderSortIcon("name")}
                </div>
            ),
            cell: ({ row }) => (
                <div className="flex items-center gap-2">
                    {row.original.avatar && (
                        <img src={row.original.avatar} alt="" className="w-8 h-8 rounded-full" />
                    )}
                    <div>
                        <div className="font-medium">{row.original.name}</div>
                        <div className="text-xs text-muted-foreground">{row.original.email}</div>
                    </div>
                </div>
            ),
        },
        {
            accessorKey: "role",
            header: () => (
                <div
                    className="font-medium flex items-center cursor-pointer hover:text-primary"
                    onClick={() => handleSort("role")}
                >
                    {t("enterprise.users.role")}
                    {renderSortIcon("role")}
                </div>
            ),
            cell: ({ row }) => (
                <Badge className={roleColors[row.original.role as keyof typeof roleColors]}>
                    {t(`enterprise.users.roles.${row.original.role}`)}
                </Badge>
            ),
        },
        {
            accessorKey: "department_id",
            header: () => (
                <div
                    className="font-medium flex items-center cursor-pointer hover:text-primary"
                    onClick={() => handleSort("department_id")}
                >
                    {t("enterprise.users.department")}
                    {renderSortIcon("department_id")}
                </div>
            ),
            cell: ({ row }) => {
                const deptPath = row.original.department_path
                if (!deptPath || !deptPath.full_path) {
                    return <span className="text-muted-foreground">-</span>
                }
                return (
                    <div className="text-sm">
                        <div className="font-medium">{deptPath.level1_name || "-"}</div>
                        {deptPath.level2_name && (
                            <div className="text-xs text-muted-foreground">
                                {deptPath.level2_name}
                            </div>
                        )}
                    </div>
                )
            },
        },
        {
            accessorKey: "group_id",
            header: () => (
                <div
                    className="font-medium flex items-center cursor-pointer hover:text-primary"
                    onClick={() => handleSort("group_id")}
                >
                    {t("enterprise.users.group")}
                    {renderSortIcon("group_id")}
                </div>
            ),
            cell: ({ row }) => <code className="text-xs">{row.original.group_id}</code>,
        },
        {
            accessorKey: "created_at",
            header: () => (
                <div
                    className="font-medium flex items-center cursor-pointer hover:text-primary"
                    onClick={() => handleSort("created_at")}
                >
                    {t("enterprise.users.createdAt")}
                    {renderSortIcon("created_at")}
                </div>
            ),
            cell: ({ row }) => format(new Date(row.original.created_at), "yyyy-MM-dd HH:mm"),
        },
        {
            id: "actions",
            header: () => <div className="text-right font-medium">操作</div>,
            cell: ({ row }) => (
                <div className="flex justify-end gap-2">
                    <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => handleRoleEdit(row.original)}
                    >
                        <Pencil className="w-4 h-4" />
                    </Button>
                    <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => handleQuotaAssign(row.original)}
                    >
                        <Shield className="w-4 h-4" />
                    </Button>
                </div>
            ),
        },
    ], [t, handleRoleEdit, handleQuotaAssign, handleSort, renderSortIcon])

    const users = data?.users || []
    const total = data?.total || 0
    const policies = policiesData?.policies || []

    // Create table instance
    const table = useReactTable({
        data: users,
        columns,
        getCoreRowModel: getCoreRowModel(),
        manualPagination: true,
    })

    return (
        <div className="p-6 space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold flex items-center gap-2">
                        <Users className="w-6 h-6 text-[#6A6DE6]" />
                        {t("enterprise.users.title")}
                    </h1>
                    <p className="text-muted-foreground mt-1">{t("enterprise.users.description")}</p>
                </div>
                <Button onClick={handleSync} disabled={syncMutation.isPending} className="gap-2">
                    <RefreshCcw className={`w-4 h-4 ${syncMutation.isPending ? "animate-spin" : ""}`} />
                    {t("enterprise.users.syncNow")}
                </Button>
            </div>

            {/* Search and Table Card */}
            <Card>
                <CardHeader>
                    <div className="flex items-center justify-between gap-4">
                        <CardTitle>{t("enterprise.users.userList")}</CardTitle>
                        <div className="flex items-center gap-2">
                            {/* Level 1 Department Filter */}
                            <Select value={level1Department} onValueChange={handleLevel1Change}>
                                <SelectTrigger className="w-40">
                                    <SelectValue placeholder={t("enterprise.users.level1Department")} />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="all">{t("enterprise.users.allDepartments")}</SelectItem>
                                    {deptLevelsData?.level1_departments
                                        ?.filter(dept => dept.department_id && dept.department_id !== "")
                                        .map((dept) => (
                                            <SelectItem key={dept.department_id} value={dept.department_id}>
                                                {dept.name || dept.department_id}
                                            </SelectItem>
                                        ))}
                                </SelectContent>
                            </Select>

                            {/* Level 2 Department Filter */}
                            {level1Department && level1Department !== "all" && (
                                <Select value={level2Department} onValueChange={handleLevel2Change}>
                                    <SelectTrigger className="w-40">
                                        <SelectValue placeholder={t("enterprise.users.level2Department")} />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="all">{t("enterprise.users.allSubDepartments")}</SelectItem>
                                        {deptLevelsData?.level2_departments
                                            ?.filter(dept => dept.department_id && dept.department_id !== "")
                                            .map((dept) => (
                                                <SelectItem key={dept.department_id} value={dept.department_id}>
                                                    {dept.name || dept.department_id}
                                                </SelectItem>
                                            ))}
                                    </SelectContent>
                                </Select>
                            )}

                            {/* Search Input */}
                            <Input
                                placeholder={t("enterprise.users.searchPlaceholder")}
                                value={searchInput}
                                onChange={(e) => handleSearchChange(e.target.value)}
                                className="w-64"
                            />

                            {/* Clear Filters */}
                            {(level1Department !== "all" || level2Department !== "all" || keyword) && (
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    onClick={handleClearFilters}
                                >
                                    {t("common.clearFilters")}
                                </Button>
                            )}
                        </div>
                    </div>
                </CardHeader>
                <CardContent>
                    <DataTable table={table} columns={columns} isLoading={isLoading} />
                    <ServerPagination
                        page={page}
                        pageSize={pageSize}
                        total={total}
                        onPageChange={setPage}
                        onPageSizeChange={setPageSize}
                    />
                </CardContent>
            </Card>

            {/* Role Edit Dialog */}
            <Dialog open={roleDialogOpen} onOpenChange={setRoleDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("enterprise.users.editRole")}</DialogTitle>
                        <DialogDescription>
                            {t("enterprise.users.editRoleDescription")}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 py-4">
                        <div>
                            <p className="text-sm text-muted-foreground mb-2">
                                {t("enterprise.users.userName")}: <strong>{selectedUser?.name}</strong>
                            </p>
                        </div>
                        <div className="space-y-2">
                            <Label>{t("enterprise.users.selectRole")}</Label>
                            <Select value={selectedRole} onValueChange={setSelectedRole}>
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="viewer">{t("enterprise.users.roles.viewer")}</SelectItem>
                                    <SelectItem value="analyst">{t("enterprise.users.roles.analyst")}</SelectItem>
                                    <SelectItem value="admin">{t("enterprise.users.roles.admin")}</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setRoleDialogOpen(false)}>
                            {t("common.cancel")}
                        </Button>
                        <Button onClick={handleRoleSave} disabled={updateRoleMutation.isPending}>
                            {updateRoleMutation.isPending ? t("common.saving") : t("common.save")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Quota Assignment Dialog */}
            <Dialog open={quotaDialogOpen} onOpenChange={setQuotaDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("enterprise.users.assignQuota")}</DialogTitle>
                        <DialogDescription>
                            {t("enterprise.users.assignQuotaDescription")}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 py-4">
                        <div>
                            <p className="text-sm text-muted-foreground mb-2">
                                {t("enterprise.users.userName")}: <strong>{selectedUser?.name}</strong>
                            </p>
                        </div>
                        <div className="space-y-2">
                            <Label>{t("enterprise.users.selectPolicy")}</Label>
                            <Select
                                value={selectedPolicyId?.toString()}
                                onValueChange={(v) => setSelectedPolicyId(Number(v))}
                            >
                                <SelectTrigger>
                                    <SelectValue placeholder={t("enterprise.users.selectPolicyPlaceholder")} />
                                </SelectTrigger>
                                <SelectContent>
                                    {policies
                                        .filter(p => p.id && p.id.toString() !== "")
                                        .map((p) => (
                                            <SelectItem key={p.id} value={p.id.toString()}>
                                                {p.name}
                                            </SelectItem>
                                        ))}
                                </SelectContent>
                            </Select>
                        </div>
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setQuotaDialogOpen(false)}>
                            {t("common.cancel")}
                        </Button>
                        <Button onClick={handleQuotaSave} disabled={bindQuotaMutation.isPending}>
                            {bindQuotaMutation.isPending ? t("common.saving") : t("common.save")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
