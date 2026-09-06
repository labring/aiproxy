import { type RouteObject } from "react-router"
import { Navigate } from "react-router"
import { Suspense, lazy } from "react"
import { ROUTES } from "./constants"
import { ProtectedRoute } from "@/feature/auth/components/ProtectedRoute"

//page
const ModelPage = lazy(() => import("@/pages/model/page"))
const ChannelPage = lazy(() => import("@/pages/channel/page"))
const TokenPage = lazy(() => import("@/pages/token/page"))
const MonitorPage = lazy(() => import("@/pages/monitor/page"))
const LogPage = lazy(() => import("@/pages/log/page"))
const MCPPage = lazy(() => import("@/pages/mcp/page"))
const GroupPage = lazy(() => import("@/pages/group/page"))
const ConsumptionRankingPage = lazy(() => import("@/pages/consumption-ranking/page"))

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
                    element: lazyLoad(MonitorPage),
                },
                {
                    path: ROUTES.GROUP,
                    element: lazyLoad(GroupPage),
                },
                {
                    path: ROUTES.CONSUMPTION_RANKING,
                    element: lazyLoad(ConsumptionRankingPage),
                },
                {
                    path: ROUTES.LEGACY_GROUP_RANKING,
                    element: <Navigate to={ROUTES.CONSUMPTION_RANKING} replace />,
                },
                {
                    path: ROUTES.KEY,
                    element: lazyLoad(TokenPage),
                },
                {
                    path: ROUTES.CHANNEL,
                    element: lazyLoad(ChannelPage),
                },
                {
                    path: ROUTES.MODEL,
                    element: lazyLoad(ModelPage),
                },
                {
                    path: ROUTES.LOG,
                    element: lazyLoad(LogPage),
                },
                {
                    path: ROUTES.MCP,
                    element: lazyLoad(MCPPage),
                }
            ]
        }]
    }

    return [...authRoutes, appRoutes]
}
