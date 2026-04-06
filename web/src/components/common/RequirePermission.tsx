import { Navigate } from "react-router"
import { useMyPermissions, type PermissionKey } from "@/lib/permissions"
import useAuthStore from "@/store/auth"
import { ROUTES, type RoutePath } from "@/routes/constants"

export function RequirePermission({
    permission,
    children,
    fallback,
}: {
    permission: PermissionKey
    children: React.ReactNode
    fallback?: RoutePath
}) {
    const enterpriseUser = useAuthStore(s => s.enterpriseUser)
    // Admin Key login (no enterpriseUser) → full access, same as RequireAdmin
    if (!enterpriseUser) return <>{children}</>
    const { data, isLoading } = useMyPermissions()
    // Wait for permissions to load before deciding — prevents false redirect on first render
    if (isLoading) return null
    const has = !!data?.permissions.includes(permission)
    if (!has) return <Navigate to={fallback ?? ROUTES.ENTERPRISE} replace />
    return <>{children}</>
}

export function RequireAdmin({ children }: { children: React.ReactNode }) {
    const enterpriseUser = useAuthStore(s => s.enterpriseUser)
    // Admin Key login (no enterpriseUser) → always allow admin panel access
    if (!enterpriseUser) return <>{children}</>
    // Feishu login → only admin role can access
    if (enterpriseUser.role !== 'admin') return <Navigate to={ROUTES.ENTERPRISE} replace />
    return <>{children}</>
}
