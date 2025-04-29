import type React from "react"

import { Link, useLocation, useNavigate } from "react-router"
import {
    Bot,
    Layers,
    BarChart2,
    Database,
    Calendar,
    ChevronLeft,
    ChevronRight,
    FileText,
    Github,
    LogOut,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"
import { ROUTES } from "@/routes/constants"
import { cn } from "@/lib/utils"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { Button } from "@/components/ui/button"
import useAuthStore from "@/store/auth"

// Add custom animation keyframes
const animationStyles = `
  @keyframes bounce-subtle {
    0%, 100% { transform: translateY(0) scale(1.25) rotate(6deg); }
    50% { transform: translateY(-5px) scale(1.25) rotate(6deg); }
  }
  
  @keyframes pulse-glow {
    0%, 100% { filter: drop-shadow(0 0 2px rgba(255, 255, 255, 0.7)); }
    50% { filter: drop-shadow(0 0 6px rgba(255, 255, 255, 0.9)); }
  }
  
  .animate-bounce-subtle {
    animation: bounce-subtle 0.8s ease-in-out infinite, pulse-glow 1.5s ease-in-out infinite;
  }
`

interface SidebarItem {
    title: string
    icon: React.ComponentType<{ className?: string }>
    href: string
    display: boolean
    external?: boolean
}

function createSidebarConfig(t: TFunction): SidebarItem[] {
    return [
        {
            title: t("sidebar.monitor"),
            icon: BarChart2,
            href: ROUTES.MONITOR,
            display: true,
        },
        {
            title: t("sidebar.key"),
            icon: Bot,
            href: ROUTES.KEY,
            display: true,
        },
        {
            title: t("sidebar.channel"),
            icon: Database,
            href: ROUTES.CHANNEL,
            display: true,
        },
        {
            title: t("sidebar.model"),
            icon: Layers,
            href: ROUTES.MODEL,
            display: true,
        },
        {
            title: t("sidebar.log"),
            icon: Calendar,
            href: ROUTES.LOG,
            display: true,
        },
        {
            title: t("sidebar.doc"),
            icon: FileText,
            href: "https://sealos.run/docs/guides/ai-proxy",
            display: true,
            external: true,
        },
        {
            title: t("sidebar.github"),
            icon: Github,
            href: "https://github.com/labring/aiproxy",
            display: true,
            external: true,
        },
    ]
}

interface SidebarDisplayConfig {
    monitor?: boolean
    key?: boolean
    channel?: boolean
    model?: boolean
    log?: boolean
    doc?: boolean
    github?: boolean
}

interface SidebarProps {
    displayConfig?: SidebarDisplayConfig
    collapsed?: boolean
    onToggle?: () => void
}

export function Sidebar({ displayConfig = {}, collapsed = false, onToggle }: SidebarProps) {
    const location = useLocation()
    const navigate = useNavigate()
    const { t } = useTranslation()
    const logout = useAuthStore((s) => s.logout)

    const currentFirstLevelPath = "/" + location.pathname.split("/")[1]

    const sidebarItems = createSidebarConfig(t).map((item) => {
        // Determine which config property based on path name
        let configKey: keyof SidebarDisplayConfig = "monitor"
        if (item.href === ROUTES.KEY) configKey = "key"
        if (item.href === ROUTES.CHANNEL) configKey = "channel"
        if (item.href === ROUTES.MODEL) configKey = "model"
        if (item.href === ROUTES.LOG) configKey = "log"
        if (item.href === "https://sealos.run/docs/guides/ai-proxy") configKey = "doc"
        if (item.href === "https://github.com/labring/aiproxy") configKey = "github"

        const shouldDisplay = displayConfig[configKey] !== undefined ? displayConfig[configKey] : item.display

        return {
            ...item,
            display: shouldDisplay,
        }
    })

    const handleLogout = () => {
        logout()
        navigate("/login")
    }

    return (
        <div
            className={cn(
                "h-full relative overflow-hidden bg-gradient-to-b from-[#6A6DE6] to-[#8A8DF7] flex flex-col transition-all duration-300 ease-in-out",
                collapsed ? "w-20" : "w-64",
            )}
        >
            <style dangerouslySetInnerHTML={{ __html: animationStyles }} />
            {/* 粒子效果 */}
            <div className="absolute inset-0 overflow-hidden pointer-events-none">
                {Array.from({ length: 25 }).map((_, i) => (
                    <div
                        key={i}
                        className="absolute rounded-full bg-white/10 sidebar-particle"
                        style={{
                            width: `${Math.random() * 6 + 2}px`,
                            height: `${Math.random() * 6 + 2}px`,
                            top: `${Math.random() * 100}%`,
                            left: `${Math.random() * 100}%`,
                            animationDelay: `${Math.random() * 5}s`,
                        }}
                    />
                ))}
            </div>

            <div className="relative z-10 flex items-center justify-between p-6 border-b border-white/20">
                <div
                    className={cn(
                        "overflow-hidden transition-all duration-300 ease-in-out flex-shrink-0",
                        collapsed ? "w-0 opacity-0" : "w-auto opacity-100",
                    )}
                >
                    <h1 className="text-lg font-semibold text-white whitespace-nowrap">AI Proxy</h1>
                </div>
                <Button
                    variant="ghost"
                    size="icon"
                    onClick={onToggle}
                    className={cn(
                        "rounded-full hover:bg-white/10 hover:text-white transition-all flex-shrink-0 w-8 h-8 flex items-center justify-center text-white",
                        collapsed ? "ml-auto mr-auto" : "ml-auto",
                    )}
                >
                    {collapsed ? <ChevronRight className="h-5 w-5" /> : <ChevronLeft className="h-5 w-5" />}
                </Button>
            </div>

            <div className="flex-1 py-2 overflow-y-auto relative z-10">
                <TooltipProvider delayDuration={300}>
                    {sidebarItems
                        .filter((item) => item.display)
                        .map((item) => {
                            const isActive = !item.external && currentFirstLevelPath === item.href
                            const content = (
                                <>
                                    <div className="flex items-center justify-center w-5 h-5">
                                        <item.icon
                                            className={cn(
                                                "w-5 h-5 transition-all duration-300 ease-in-out",
                                                isActive ? "text-white" : "text-white/90",
                                                "group-hover:scale-125 group-hover:rotate-6 group-hover:animate-bounce-subtle",
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

                            return (
                                <Tooltip key={item.href}>
                                    <TooltipTrigger asChild>
                                        {item.external ? (
                                            <a
                                                href={item.href}
                                                target="_blank"
                                                rel="noopener noreferrer"
                                                className={cn(
                                                    "group flex items-center px-6 py-3 my-1 mx-2 rounded-lg transition-all duration-200",
                                                    "text-white/90 hover:bg-white/10",
                                                    collapsed ? "justify-center" : "",
                                                )}
                                            >
                                                {content}
                                            </a>
                                        ) : (
                                            <Link
                                                to={item.href}
                                                className={cn(
                                                    "group flex items-center px-6 py-3 my-1 mx-2 rounded-lg transition-all duration-200",
                                                    isActive
                                                        ? "bg-white/15 text-white backdrop-blur-sm shadow-[0_0_10px_rgba(255,255,255,0.15)]"
                                                        : "text-white/90 hover:bg-white/10",
                                                    collapsed ? "justify-center" : "",
                                                )}
                                            >
                                                {content}
                                            </Link>
                                        )}
                                    </TooltipTrigger>
                                    {collapsed && <TooltipContent side="right">{item.title}</TooltipContent>}
                                </Tooltip>
                            )
                        })}
                </TooltipProvider>
            </div>

            {/* Logout button */}
            <div className="p-4 border-t border-white/20 relative z-10">
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Button
                            variant="secondary"
                            onClick={handleLogout}
                            className={cn(
                                "group w-full flex items-center px-4 py-3 rounded-lg transition-all duration-200",
                                "text-[#6A6DE6] bg-white hover:bg-gray-100",
                                collapsed ? "justify-center" : "justify-start",
                            )}
                        >
                            <div className="flex items-center justify-center w-5 h-5">
                                <LogOut className="w-5 h-5 transition-all duration-300 ease-in-out group-hover:scale-125 group-hover:rotate-6 group-hover:animate-bounce-subtle" />
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
    )
}
