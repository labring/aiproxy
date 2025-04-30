import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { Globe } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { AnimatedIcon } from "../ui/animation/components/animated-icon"
import { cn } from "@/lib/utils"

interface LanguageSelectorProps {
    variant?: "default" | "minimal"
}

export function LanguageSelector({ variant = "default" }: LanguageSelectorProps) {
    const { i18n } = useTranslation()
    const [language, setLanguage] = useState(i18n.language || 'zh')

    // 初始化时从本地存储获取语言设置
    useEffect(() => {
        const savedLanguage = localStorage.getItem('i18nextLng')
        if (savedLanguage && savedLanguage !== language) {
            setLanguage(savedLanguage)
            i18n.changeLanguage(savedLanguage)
        }
    }, []) // 只在组件挂载时执行一次

    const toggleLanguage = () => {
        const newLanguage = language === 'zh' ? 'en' : 'zh'
        setLanguage(newLanguage)
        i18n.changeLanguage(newLanguage)
        localStorage.setItem('i18nextLng', newLanguage)
    }

    const displayText = language === 'zh' ? '中' : 'En'

    const isMinimal = variant === "minimal"

    return (
        <TooltipProvider delayDuration={300}>
            <Tooltip>
                <TooltipTrigger asChild>
                    <Button
                        variant={isMinimal ? "outline" : "ghost"}
                        size="icon"
                        onClick={toggleLanguage}
                        className={cn(
                            "h-10 w-16 rounded-md",
                            isMinimal 
                                ? "bg-white/80 dark:bg-gray-800/80 hover:bg-white dark:hover:bg-gray-800 border border-gray-200 dark:border-gray-700 backdrop-blur-sm"
                                : "bg-primary/10 text-primary hover:bg-primary/20"
                        )}
                    >
                        <AnimatedIcon animationVariant="pulse" className="flex items-center justify-center">
                            <span className="text-sm font-medium">{displayText}</span>
                            <Globe className="h-4 w-4 ml-1" />
                        </AnimatedIcon>
                    </Button>
                </TooltipTrigger>
                <TooltipContent side={isMinimal ? "left" : "bottom"}>
                    {language === 'zh' ? 'Switch to English' : '切换到中文'}
                </TooltipContent>
            </Tooltip>
        </TooltipProvider>
    )
}