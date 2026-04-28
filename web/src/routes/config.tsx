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
import ConsumptionRankingPage from "@/pages/consumption-ranking/page"

// import layout component directly
import { RootLayout } from "@/components/layout/RootLayOut"
import { LoadingFallback } from "@/components/common/LoadingFallBack"

// lazy load login page
const LoginPage = lazy(() => import("@/pages/auth/login"))

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
    ]

    // app routes
    const appRoutes: RouteObject = {
        element: <ProtectedRoute />,
        children: [{
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
        }]
    }

    return [...authRoutes, appRoutes]
}
