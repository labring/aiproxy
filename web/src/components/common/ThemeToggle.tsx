import { Moon, Sun } from 'lucide-react'
import { useTheme } from '@/handler/ThemeContext'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

export function ThemeToggle() {
    const { theme, setTheme } = useTheme()
    return <Tooltip><TooltipTrigger asChild>
        <Button variant="ghost" size="icon" className="size-9" aria-label="Toggle theme" aria-pressed={theme === 'dark'} onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')}>
            {theme === 'dark' ? <Moon className="size-4" /> : <Sun className="size-4" />}
        </Button>
    </TooltipTrigger><TooltipContent>Toggle theme</TooltipContent></Tooltip>
}
