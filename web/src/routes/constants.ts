export const BASE_PATH = '/' as const

export const ROUTES = {
    MONITOR: "/monitor",
    GROUP: "/group",
    KEY: "/key",
    CHANNEL: "/channel",
    MODEL: "/model",
    LOG: "/log",
    MCP: "/mcp-front",
    ENTERPRISE: "/enterprise",
    ENTERPRISE_RANKING: "/enterprise/ranking",
    ENTERPRISE_DEPARTMENT: "/enterprise/department",
    ENTERPRISE_QUOTA: "/enterprise/quota",
    ENTERPRISE_CUSTOM_REPORT: "/enterprise/custom-report",
    ENTERPRISE_ACCESS_CONTROL: "/enterprise/access-control",
    ENTERPRISE_USERS: "/enterprise/users",
    ENTERPRISE_PPIO_SYNC: "/enterprise/ppio-sync",
    FEISHU_CALLBACK: "/feishu/callback",
} as const

export type RouteKey = keyof typeof ROUTES
export type RoutePath = typeof ROUTES[RouteKey]

// get route path by key
export const getRoute = (key: RouteKey): RoutePath => ROUTES[key]
