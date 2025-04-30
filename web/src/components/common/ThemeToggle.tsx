import { Moon, Sun } from "lucide-react"
import { useTheme } from "@/handler/ThemeContext"

import { Switch } from "@/components/ui/switch"

export function ThemeToggle() {
    const { theme, setTheme } = useTheme()

    const toggleTheme = () => {
        setTheme(theme === "light" ? "dark" : "light")
    }

    return (
        <div className="flex items-center space-x-2 transition-all duration-700 ease-[cubic-bezier(0.34,1.56,0.64,1)]">
            <Sun
                className={`h-[1.2rem] w-[1.2rem] transition-all duration-700 ease-[cubic-bezier(0.34,1.56,0.64,1)] ${theme === "dark" ? "text-[#A1A1AA] scale-75 rotate-12" : "text-foreground scale-100 rotate-0"
                    }`}
            />
            <Switch
                checked={theme === "dark"}
                onCheckedChange={toggleTheme}
                aria-label="Toggle theme"
                className="transition-all duration-700 ease-[cubic-bezier(0.34,1.56,0.64,1)] hover:scale-110"
            />
            <Moon
                className={`h-[1.2rem] w-[1.2rem] transition-all duration-700 ease-[cubic-bezier(0.34,1.56,0.64,1)] ${theme === "light" ? "text-[#A1A1AA] scale-75 rotate-12" : "text-foreground scale-100 rotate-0"
                    }`}
            />
        </div>
    )
}
