import { useMemo, useState } from "react"
import { Link, Outlet, useLocation, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import {
    BarChart2,
    FileBarChart,
    Trophy,
    Monitor,
    ChevronLeft,
    ChevronRight,
    LogOut,
    Shield,
    Lock,
    Users,
    RefreshCw,
    Key,
} from "lucide-react"
import type React from "react"
import type { TFunction } from "i18next"
import { ROUTES } from "@/routes/constants"
import { cn } from "@/lib/utils"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { Button } from "@/components/ui/button"
import useAuthStore from "@/store/auth"
import { LanguageSelector } from "@/components/common/LanguageSelector"
import { useMyPermissions, useRole, type PermissionKey } from "@/lib/permissions"

interface EnterpriseSidebarItem {
    title: string
    icon: React.ComponentType<{ className?: string }>
    href: string
    divider?: boolean
    external?: boolean
    requiredPermission?: PermissionKey
    adminOnly?: boolean
}

function createEnterpriseSidebarConfig(t: TFunction): EnterpriseSidebarItem[] {
    return [
        {
            title: t("enterprise.sidebar.myAccess"),
            icon: Key,
            href: ROUTES.ENTERPRISE_MY_ACCESS,
        },
        {
            title: t("enterprise.sidebar.dashboard"),
            icon: BarChart2,
            href: ROUTES.ENTERPRISE,
            requiredPermission: "dashboard_view",
        },
        {
            title: t("enterprise.sidebar.ranking"),
            icon: Trophy,
            href: ROUTES.ENTERPRISE_RANKING,
            requiredPermission: "ranking_view",
        },
        {
            title: t("enterprise.sidebar.quota"),
            icon: Shield,
            href: ROUTES.ENTERPRISE_QUOTA,
            requiredPermission: "quota_manage_view",
        },
        {
            title: t("enterprise.sidebar.accessControl"),
            icon: Lock,
            href: ROUTES.ENTERPRISE_ACCESS_CONTROL,
            requiredPermission: "access_control_view",
        },
        {
            title: t("enterprise.sidebar.users"),
            icon: Users,
            href: ROUTES.ENTERPRISE_USERS,
            requiredPermission: "user_manage_view",
        },
        {
            title: t("enterprise.sidebar.customReport"),
            icon: FileBarChart,
            href: ROUTES.ENTERPRISE_CUSTOM_REPORT,
            requiredPermission: "custom_report_view",
        },
        {
            title: t("enterprise.sidebar.ppioSync"),
            icon: RefreshCw,
            href: ROUTES.ENTERPRISE_PPIO_SYNC,
            requiredPermission: "access_control_view",
        },
        {
            title: "",
            icon: () => null,
            href: "",
            divider: true,
        },
        {
            title: t("enterprise.sidebar.adminPanel"),
            icon: Monitor,
            href: ROUTES.MONITOR,
            external: true,
            adminOnly: true,
        },
    ]
}

export function EnterpriseLayout() {
    const [collapsed, setCollapsed] = useState(false)
    const location = useLocation()
    const navigate = useNavigate()
    const { t } = useTranslation()
    const logout = useAuthStore((s) => s.logout)
    const enterpriseUser = useAuthStore((s) => s.enterpriseUser)

    const handleLogout = () => {
        logout()
        navigate("/login")
    }

    const allSidebarItems = createEnterpriseSidebarConfig(t)
    const { data: permData } = useMyPermissions()
    const myPerms = new Set(permData?.permissions || [])
    const currentRole = useRole()

    const sidebarItems = allSidebarItems.filter(item => {
        if (item.divider) return true
        if (item.adminOnly && currentRole !== 'admin') return false
        if (item.external) return true
        if (!item.requiredPermission) return true
        return myPerms.has(item.requiredPermission)
    })

    const particles = useMemo(
        () =>
            Array.from({ length: 25 }).map(() => ({
                width: Math.random() * 6 + 2,
                height: Math.random() * 6 + 2,
                top: Math.random() * 100,
                left: Math.random() * 100,
                delay: Math.random() * 5,
            })),
        [],
    )

    return (
        <div className="flex h-screen bg-background">
            <div
                className={cn(
                    "h-full relative overflow-hidden flex flex-col transition-all duration-300 ease-in-out",
                    "bg-gradient-to-b from-[#6A6DE6] to-[#8A8DF7] dark:from-[#4A4DA0] dark:to-[#5155A5]",
                    collapsed ? "w-20" : "w-64",
                )}
            >
                {/* Particles */}
                <div className="absolute inset-0 overflow-hidden pointer-events-none">
                    {particles.map((p, i) => (
                        <div
                            key={i}
                            className="absolute rounded-full bg-white/10 dark:bg-white/5 sidebar-particle"
                            style={{
                                width: `${p.width}px`,
                                height: `${p.height}px`,
                                top: `${p.top}%`,
                                left: `${p.left}%`,
                                animationDelay: `${p.delay}s`,
                            }}
                        />
                    ))}
                </div>

                {/* Header with user info */}
                <div className="relative z-10 flex items-center justify-between p-6 border-b border-white/20 dark:border-white/10">
                    <div
                        className={cn(
                            "overflow-hidden transition-all duration-300 ease-in-out flex items-center gap-3 flex-shrink-0",
                            collapsed ? "w-0 opacity-0" : "w-auto opacity-100",
                        )}
                    >
                        {enterpriseUser?.avatar ? (
                            <img
                                src={enterpriseUser.avatar}
                                alt={enterpriseUser.name}
                                className="w-8 h-8 rounded-full flex-shrink-0"
                            />
                        ) : (
                            <div className="w-8 h-8 rounded-full bg-white/20 flex items-center justify-center flex-shrink-0">
                                <span className="text-white text-sm font-medium">
                                    {enterpriseUser?.name?.[0] || "U"}
                                </span>
                            </div>
                        )}
                        <div className="min-w-0">
                            <p className="text-sm font-semibold text-white truncate">
                                {enterpriseUser?.name || t("enterprise.title")}
                            </p>
                            <p className="text-xs text-white/70 truncate">{t("enterprise.title")}</p>
                        </div>
                    </div>
                    <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setCollapsed(!collapsed)}
                        className={cn(
                            "rounded-full hover:bg-white/10 hover:text-white transition-all flex-shrink-0 w-8 h-8 flex items-center justify-center text-white",
                            collapsed ? "ml-auto mr-auto" : "ml-auto",
                        )}
                    >
                        {collapsed ? <ChevronRight className="h-5 w-5" /> : <ChevronLeft className="h-5 w-5" />}
                    </Button>
                </div>

                {/* Menu items */}
                <div className="flex-1 py-2 overflow-y-auto relative z-10">
                    <TooltipProvider delayDuration={300}>
                        {sidebarItems.map((item, index) => {
                            if (item.divider) {
                                return (
                                    <div
                                        key={`divider-${index}`}
                                        className="mx-6 my-2 border-t border-white/20 dark:border-white/10"
                                    />
                                )
                            }

                            const isActive = item.href && location.pathname === item.href

                            const content = (
                                <>
                                    <div className="flex items-center justify-center w-5 h-5">
                                        <item.icon
                                            className={cn(
                                                "w-5 h-5 transition-all duration-300 ease-in-out",
                                                isActive ? "text-white" : "text-white/90",
                                                "group-hover:scale-125 group-hover:rotate-6",
                                            )}
                                        />
                                    </div>
                                    <span
                                        className={cn(
                                            "ml-3 font-medium whitespace-nowrap transition-all duration-300 ease-in-out",
                                            isActive ? "text-white" : "text-white/90",
                                            collapsed ? "opacity-0 w-0 overflow-hidden" : "opacity-100 w-auto",
                                        )}
                                    >
                                        {item.title}
                                    </span>
                                </>
                            )

                            const className = cn(
                                "group flex items-center px-6 py-3 my-1 mx-2 rounded-lg transition-all duration-200",
                                isActive
                                    ? "bg-white/15 text-white backdrop-blur-sm shadow-[0_0_10px_rgba(255,255,255,0.15)]"
                                    : "text-white/90 hover:bg-white/10",
                                collapsed ? "justify-center" : "",
                            )

                            const linkElement = item.external ? (
                                <a href={item.href} target="_blank" rel="noopener noreferrer" className={className}>
                                    {content}
                                </a>
                            ) : (
                                <Link to={item.href} className={className}>
                                    {content}
                                </Link>
                            )

                            return (
                                <Tooltip key={item.title}>
                                    <TooltipTrigger asChild>
                                        {linkElement}
                                    </TooltipTrigger>
                                    {collapsed && <TooltipContent side="right">{item.title}</TooltipContent>}
                                </Tooltip>
                            )
                        })}
                    </TooltipProvider>
                </div>

                {/* Language & Logout */}
                <div className="p-4 border-t border-white/20 dark:border-white/10 relative z-10 space-y-2">
                    {/* Language Selector */}
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <div className={cn("flex", collapsed ? "justify-center" : "justify-start px-1")}>
                                <LanguageSelector variant="minimal" />
                            </div>
                        </TooltipTrigger>
                        {collapsed && <TooltipContent side="right">{t("sidebar.logout")}</TooltipContent>}
                    </Tooltip>

                    {/* Logout */}
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button
                                variant="secondary"
                                onClick={handleLogout}
                                className={cn(
                                    "group w-full flex items-center px-4 py-3 rounded-lg transition-all duration-200",
                                    "text-[#6A6DE6] dark:text-[#4A4DA0] bg-white hover:bg-gray-100",
                                    collapsed ? "justify-center" : "justify-start",
                                )}
                            >
                                <div className="flex items-center justify-center w-5 h-5">
                                    <LogOut className="w-5 h-5 transition-all duration-300 ease-in-out group-hover:scale-125 group-hover:rotate-6" />
                                </div>
                                <span
                                    className={cn(
                                        "ml-3 font-medium whitespace-nowrap transition-all duration-300 ease-in-out",
                                        collapsed ? "opacity-0 w-0 overflow-hidden" : "opacity-100 w-auto",
                                    )}
                                >
                                    {t("sidebar.logout")}
                                </span>
                            </Button>
                        </TooltipTrigger>
                        {collapsed && <TooltipContent side="right">{t("sidebar.logout")}</TooltipContent>}
                    </Tooltip>
                </div>
            </div>

            <main className="flex-1 flex flex-col overflow-hidden transition-all duration-300">
                <div className="flex-1 overflow-auto">
                    <Outlet />
                </div>
            </main>
        </div>
    )
}
