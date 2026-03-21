import { Navigate } from "react-router"
import { useHasPermission, type PermissionKey } from "@/lib/permissions"
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
