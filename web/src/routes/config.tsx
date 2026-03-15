import { type RouteObject } from "react-router"
import { Navigate } from "react-router"
import { Suspense, lazy } from "react"
import { ROUTES } from "./constants"
import { ProtectedRoute } from "@/feature/auth/components/ProtectedRoute"

//page
import ModelPage from "@/pages/model/page"
import ChannelPage from "@/pages/channel/page"
import TokenPage from "@/pages/token/page"
import MonitorPage from "@/pages/monitor/page"
import LogPage from "@/pages/log/page"
import MCPPage from "@/pages/mcp/page"
import GroupPage from "@/pages/group/page"

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
                element: <RootLayout />,
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
                        element: lazyLoad(EnterpriseRanking),
                    },
                    {
                        path: `${ROUTES.ENTERPRISE_DEPARTMENT}/:id`,
                        element: lazyLoad(EnterpriseDepartment),
                    },
                    {
                        path: ROUTES.ENTERPRISE_QUOTA,
                        element: lazyLoad(EnterpriseQuota),
                    },
                ]
            }
        ]
    }

    return [...authRoutes, appRoutes]
}
