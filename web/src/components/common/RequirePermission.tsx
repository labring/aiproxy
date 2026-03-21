import { Navigate } from "react-router"
import { useHasPermission, type PermissionKey } from "@/lib/permissions"
import useAuthStore from "@/store/auth"
import { ROUTES } from "@/routes/constants"

export function RequirePermission({
    permission,
    children,
}: {
    permission: PermissionKey
    children: React.ReactNode
}) {
    const has = useHasPermission(permission)
    if (!has) return <Navigate to={ROUTES.ENTERPRISE} replace />
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
