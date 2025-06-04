import { useState } from "react"
import { Outlet } from "react-router"
import { Sidebar } from "./SideBar"
import { cn } from "@/lib/utils"

export function RootLayout() {
  const [collapsed, setCollapsed] = useState(false)

  return (
    <div className="flex h-screen bg-background">
      <Sidebar
        displayConfig={{
          monitor: true,
          key: true,
          channel: true,
          model: true,
          log: false,
          doc: true,
          github: true,
        }}
        collapsed={collapsed}
        onToggle={() => setCollapsed(!collapsed)}
      />

      <main className={cn("flex-1 flex flex-col overflow-hidden transition-all duration-300")}>
        <div className="flex-1 overflow-auto">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
