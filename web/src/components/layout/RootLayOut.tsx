import { useState } from "react"
import { Outlet } from "react-router"
import { Sidebar } from "./SideBar"
import { cn } from "@/lib/utils"
import { ThemeToggle } from "@/components/common/ThemeToggle"
import { LanguageSelector } from "@/components/common/LanguageSelector"
import { BookOpen, Menu } from "lucide-react"
import { useLocation } from "react-router"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetTitle, SheetTrigger } from "@/components/ui/sheet"

export function RootLayout() {
  const [collapsed, setCollapsed] = useState(false)
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const location = useLocation()
  const { t } = useTranslation()
  const titles: Record<string, string> = {
    "/monitor": t("sidebar.monitor"), "/channel": t("sidebar.channel"), "/model": t("sidebar.model"),
    "/log": t("sidebar.log"), "/group": t("sidebar.group"), "/consumption-ranking": t("sidebar.consumptionRanking"),
    "/key": t("sidebar.key"), "/mcp-front": t("sidebar.mcp"),
  }

  return (
    <div className="flex h-dvh bg-background">
      <Sidebar
        className="hidden lg:flex"
        displayConfig={{
          monitor: true,
          key: true,
          channel: true,
          model: true,
          log: true,
          doc: true,
          github: true,
        }}
        collapsed={collapsed}
        onToggle={() => setCollapsed(!collapsed)}
      />

      <main className={cn("min-w-0 flex-1 flex flex-col overflow-hidden bg-background")}>
        <header className="h-14 shrink-0 border-b bg-card flex items-center justify-between gap-3 px-4 sm:px-6">
          <div className="flex min-w-0 items-center gap-3"><Sheet open={mobileNavOpen} onOpenChange={setMobileNavOpen}><SheetTrigger asChild><Button variant="ghost" size="icon" className="lg:hidden shrink-0" aria-label={t("ui.navigation")}><Menu className="h-5 w-5" /></Button></SheetTrigger><SheetContent side="left" className="w-72 border-0 bg-transparent p-0"><SheetTitle className="sr-only">{t("ui.navigation")}</SheetTitle><Sidebar collapsed={false} className="w-full border-0" onNavigate={() => setMobileNavOpen(false)} /></SheetContent></Sheet><div className="min-w-0"><h2 className="truncate text-lg font-semibold tracking-tight">{titles[location.pathname] || "Workspace"}</h2></div></div>
          <div className="flex items-center gap-1"><a href="https://sealos.run/docs/guides/ai-proxy" target="_blank" rel="noopener noreferrer" title={t("sidebar.doc")} aria-label={t("sidebar.doc")} className="flex size-9 items-center justify-center rounded-md text-muted-foreground hover:bg-muted"><BookOpen className="size-4" /></a><div className="mx-2 h-5 w-px bg-border" /><ThemeToggle /><LanguageSelector /></div>
        </header>
        <div className="min-h-0 flex-1 overflow-auto">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
