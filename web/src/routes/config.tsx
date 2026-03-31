import { type RouteObject } from "react-router"
import { Navigate } from "react-router"
import { Suspense, lazy } from "react"
import { ROUTES } from "./constants"
import { ProtectedRoute } from "@/feature/auth/components/ProtectedRoute"
import { RequirePermission, RequireAdmin } from "@/components/common/RequirePermission"

//page
import ModelPage from "@/pages/model/page"
import ChannelPage from "@/pages/channel/page"
import TokenPage from "@/pages/token/page"
import MonitorPage from "@/pages/monitor/page"
import LogPage from "@/pages/log/page"
import MCPPage from "@/pages/mcp/page"
import GroupPage from "@/pages/group/page"
import ConsumptionRankingPage from "@/pages/consumption-ranking/page"

// import layout component directly
import { RootLayout } from "@/components/layout/RootLayOut"
import { EnterpriseLayout } from "@/components/layout/EnterpriseLayout"
import { LoadingFallback } from "@/components/common/LoadingFallBack"

// lazy load login page
const LoginPage = lazy(() => import("@/pages/auth/login"))
const FeishuCallbackPage = lazy(() => import("@/pages/auth/feishu-callback"))

// lazy load enterprise pages
const EnterpriseDashboard = lazy(() => import("@/pages/enterprise/dashboard"))
const EnterpriseRanking = lazy(() => import("@/pages/enterprise/ranking"))
const EnterpriseDepartment = lazy(() => import("@/pages/enterprise/department"))
const EnterpriseQuota = lazy(() => import("@/pages/enterprise/quota"))
const EnterpriseCustomReport = lazy(() => import("@/pages/enterprise/custom-report"))
const EnterpriseAccessControl = lazy(() => import("@/pages/enterprise/access-control"))
const EnterpriseUsers = lazy(() => import("@/pages/enterprise/users"))
const EnterprisePPIOSync = lazy(() => import("@/pages/enterprise/ppio-sync"))
const EnterpriseNovitaSync = lazy(() => import("@/pages/enterprise/novita-sync"))
const EnterpriseMyAccess = lazy(() => import("@/pages/enterprise/my-access"))
const EnterpriseNotifications = lazy(() => import("@/pages/enterprise/notifications"))

// lazy load component wrapper
const lazyLoad = (Component: React.ComponentType) => (
    <Suspense fallback={<LoadingFallback />}>
        <Component />
    </Suspense>
)



// routes config
export function useRoutes(): RouteObject[] {

    // auth routes
    const authRoutes: RouteObject[] = [
        { path: "/login", element: lazyLoad(LoginPage) },
        { path: ROUTES.FEISHU_CALLBACK, element: lazyLoad(FeishuCallbackPage) },
    ]

    // app routes
    const appRoutes: RouteObject = {
        element: <ProtectedRoute />,
        children: [
            {
                element: <RequireAdmin><RootLayout /></RequireAdmin>,
                children: [
                    {
                        path: "/",
                        element: <Navigate to={`${ROUTES.MONITOR}`} replace />
                    },
                    {
                        path: ROUTES.MONITOR,
                        element: <MonitorPage />,
                    },
                    {
                        path: ROUTES.GROUP,
                        element: <GroupPage />,
                    },
                    {
                        path: ROUTES.CONSUMPTION_RANKING,
                        element: <ConsumptionRankingPage />,
                    },
                    {
                        path: ROUTES.LEGACY_GROUP_RANKING,
                        element: <Navigate to={ROUTES.CONSUMPTION_RANKING} replace />,
                    },
                    {
                        path: ROUTES.KEY,
                        element: <TokenPage />,
                    },
                    {
                        path: ROUTES.CHANNEL,
                        element: <ChannelPage />,
                    },
                    {
                        path: ROUTES.MODEL,
                        element: <ModelPage />,
                    },
                    {
                        path: ROUTES.LOG,
                        element: <LogPage />,
                    },
                    {
                        path: ROUTES.MCP,
                        element: <MCPPage />,
                    }
                ]
            },
            {
                element: <EnterpriseLayout />,
                children: [
                    {
                        path: ROUTES.ENTERPRISE,
                        element: lazyLoad(EnterpriseDashboard),
                    },
                    {
                        path: ROUTES.ENTERPRISE_RANKING,
                        element: <RequirePermission permission="ranking_view">{lazyLoad(EnterpriseRanking)}</RequirePermission>,
                    },
                    {
                        path: `${ROUTES.ENTERPRISE_DEPARTMENT}/:id`,
                        element: <RequirePermission permission="department_detail_view">{lazyLoad(EnterpriseDepartment)}</RequirePermission>,
                    },
                    {
                        path: ROUTES.ENTERPRISE_QUOTA,
                        element: <RequirePermission permission="quota_manage_view">{lazyLoad(EnterpriseQuota)}</RequirePermission>,
                    },
                    {
                        path: ROUTES.ENTERPRISE_CUSTOM_REPORT,
                        element: <RequirePermission permission="custom_report_view">{lazyLoad(EnterpriseCustomReport)}</RequirePermission>,
                    },
                    {
                        path: ROUTES.ENTERPRISE_ACCESS_CONTROL,
                        element: <RequirePermission permission="access_control_view">{lazyLoad(EnterpriseAccessControl)}</RequirePermission>,
                    },
                    {
                        path: ROUTES.ENTERPRISE_USERS,
                        element: <RequirePermission permission="user_manage_view">{lazyLoad(EnterpriseUsers)}</RequirePermission>,
                    },
                    {
                        path: ROUTES.ENTERPRISE_PPIO_SYNC,
                        element: <RequirePermission permission="access_control_view">{lazyLoad(EnterprisePPIOSync)}</RequirePermission>,
                    },
                    {
                        path: ROUTES.ENTERPRISE_NOVITA_SYNC,
                        element: <RequirePermission permission="access_control_view">{lazyLoad(EnterpriseNovitaSync)}</RequirePermission>,
                    },
                    {
                        path: ROUTES.ENTERPRISE_MY_ACCESS,
                        element: lazyLoad(EnterpriseMyAccess),
                    },
                    {
                        path: ROUTES.ENTERPRISE_NOTIFICATIONS,
                        element: <RequirePermission permission="quota_manage_view">{lazyLoad(EnterpriseNotifications)}</RequirePermission>,
                    },
                ]
            }
        ]
    }

    return [...authRoutes, appRoutes]
}
