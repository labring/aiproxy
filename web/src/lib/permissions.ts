import { useQuery } from "@tanstack/react-query"
import { enterpriseApi } from "@/api/enterprise"
import useAuthStore from "@/store/auth"

export type PermissionKey =
    | 'dashboard_view'
    | 'dashboard_manage'
    | 'ranking_view'
    | 'ranking_manage'
    | 'department_detail_view'
    | 'department_detail_manage'
    | 'export_view'
    | 'export_manage'
    | 'custom_report_view'
    | 'custom_report_manage'
    | 'quota_manage_view'
    | 'quota_manage_manage'
    | 'user_manage_view'
    | 'user_manage_manage'
    | 'access_control_view'
    | 'access_control_manage'

export function useMyPermissions() {
    const isAuthenticated = useAuthStore(s => s.isAuthenticated)
    const enterpriseUser = useAuthStore(s => s.enterpriseUser)

    return useQuery({
        queryKey: ['enterprise', 'my-permissions'],
        queryFn: () => enterpriseApi.getMyPermissions(),
        enabled: isAuthenticated && !!enterpriseUser,
        staleTime: 5 * 60 * 1000,
    })
}

export function useHasPermission(key: PermissionKey): boolean {
    const isAuthenticated = useAuthStore(s => s.isAuthenticated)
    if (!isAuthenticated) return false
    const user = useAuthStore(s => s.enterpriseUser)
    // Admin Key login (no enterpriseUser) has all permissions
    if (!user) return true
    const { data } = useMyPermissions()
    if (!data) return false
    return data.permissions.includes(key)
}

export function useCanManage(module: string): boolean {
    return useHasPermission(`${module}_manage` as PermissionKey)
}

export function useRole(): string {
    const isAuthenticated = useAuthStore(s => s.isAuthenticated)
    if (!isAuthenticated) return 'viewer'
    const user = useAuthStore(s => s.enterpriseUser)
    // Admin Key login (no enterpriseUser) has full admin privileges
    if (!user) return 'admin'
    return user.role || 'viewer'
}
