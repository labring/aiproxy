import type React from "react"
import { Link, useLocation, useNavigate } from "react-router"
import { Bot, Layers, BarChart2, Database, Calendar, ChevronLeft, ChevronRight, FileText, Github, LogOut, MessageCircle, Trophy, Users, Activity } from "lucide-react"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"
import { ROUTES } from "@/routes/constants"
import { cn } from "@/lib/utils"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { Button } from "@/components/ui/button"
import useAuthStore from "@/store/auth"

interface SidebarItem { title: string; icon: React.ComponentType<{ className?: string }>; href: string; external?: boolean }
function createSidebarConfig(t: TFunction): SidebarItem[] { return [
    { title: t("sidebar.monitor"), icon: BarChart2, href: ROUTES.MONITOR }, { title: t("sidebar.channel"), icon: Database, href: ROUTES.CHANNEL }, { title: t("sidebar.model"), icon: Layers, href: ROUTES.MODEL }, { title: t("sidebar.log"), icon: Calendar, href: ROUTES.LOG }, { title: t("sidebar.group"), icon: Users, href: ROUTES.GROUP }, { title: t("sidebar.consumptionRanking"), icon: Trophy, href: ROUTES.CONSUMPTION_RANKING }, { title: t("sidebar.key"), icon: Bot, href: ROUTES.KEY }, { title: t("sidebar.mcp"), icon: MessageCircle, href: ROUTES.MCP }, { title: t("sidebar.doc"), icon: FileText, href: "https://sealos.run/docs/guides/ai-proxy", external: true }, { title: t("sidebar.github"), icon: Github, href: "https://github.com/labring/aiproxy", external: true },
] }
interface SidebarProps { displayConfig?: Record<string, boolean>; collapsed?: boolean; onToggle?: () => void; className?: string; onNavigate?: () => void }
export function Sidebar({ displayConfig = {}, collapsed = false, onToggle, className, onNavigate }: SidebarProps) {
    const location = useLocation(); const navigate = useNavigate(); const { t } = useTranslation(); const logout = useAuthStore((s) => s.logout)
    const currentPath = "/" + location.pathname.split("/")[1]
    const items = createSidebarConfig(t).filter((item) => { const key = Object.entries(ROUTES).find(([, value]) => value === item.href)?.[0]?.toLowerCase() || ""; const configuredKey = Object.keys(displayConfig).find((name) => name.toLowerCase() === key); return configuredKey ? displayConfig[configuredKey] !== false : true })
    return <aside className={cn("h-full relative flex flex-col transition-all duration-300 bg-card border-r border-border", collapsed ? "w-16" : "w-56", className)}>
        <div className="flex items-center justify-between h-14 shrink-0 px-4 border-b"><div className={cn("overflow-hidden transition-all", collapsed ? "w-0 opacity-0" : "w-auto opacity-100")}><div className="flex items-center gap-3 whitespace-nowrap"><div className="flex h-9 w-9 items-center justify-center rounded-md bg-primary/10"><Activity className="h-5 w-5 text-foreground" /></div><div><h1 className="text-[15px] font-semibold tracking-tight text-foreground">AI Proxy</h1></div></div></div><Button variant="ghost" size="icon" onClick={onToggle} aria-label={t("ui.toggleNavigation")} title={t("ui.toggleNavigation")} className={cn("rounded-lg hover:bg-muted text-muted-foreground hover:text-foreground", collapsed ? "mx-auto" : "ml-auto")}>{collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}</Button></div>
        <nav className="flex-1 py-3 overflow-y-auto"><TooltipProvider delayDuration={300}>{items.map((item) => { const active = !item.external && currentPath === item.href; const content = <><item.icon className={cn("h-[18px] w-[18px] shrink-0", active ? "text-primary" : "text-muted-foreground group-hover:text-primary")} /><span className={cn("ml-3 truncate text-[13px] font-medium", active ? "text-foreground" : "text-muted-foreground group-hover:text-foreground", collapsed && "w-0 opacity-0")}>{item.title}</span></>; const cls = cn("group flex items-center px-4 py-2.5 my-1 mx-3 rounded-lg transition-colors", active ? "bg-accent text-primary" : "text-muted-foreground hover:bg-muted hover:text-foreground", collapsed && "justify-center px-0 mx-2"); return <Tooltip key={item.href}><TooltipTrigger asChild>{item.external ? <a href={item.href} target="_blank" rel="noopener noreferrer" className={cls}>{content}</a> : <Link to={item.href} aria-current={active ? "page" : undefined} onClick={onNavigate} className={cls}>{content}</Link>}</TooltipTrigger>{collapsed && <TooltipContent side="right">{item.title}</TooltipContent>}</Tooltip> })}</TooltipProvider></nav>
        <div className="p-3 border-t border-border"><Button variant="secondary" onClick={() => { onNavigate?.(); logout(); navigate("/login") }} className={cn("group w-full rounded-lg bg-muted text-muted-foreground hover:bg-accent hover:text-foreground", collapsed ? "justify-center px-0" : "justify-start")}><LogOut className="h-[18px] w-[18px]" /><span className={cn("ml-3 text-[13px]", collapsed && "w-0 opacity-0")}>{t("sidebar.logout")}</span></Button></div>
    </aside>
}
