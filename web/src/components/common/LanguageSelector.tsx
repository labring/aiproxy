import { useTranslation } from 'react-i18next'
import { Globe } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

export function LanguageSelector({ variant = 'default' }: { variant?: 'default' | 'minimal' }) {
    const { i18n } = useTranslation()
    const isChinese = (i18n.resolvedLanguage || i18n.language).startsWith('zh')
    const label = isChinese ? 'Switch to English' : '切换到中文'
    return <Tooltip><TooltipTrigger asChild>
        <Button variant={variant === 'minimal' ? 'outline' : 'ghost'} size="icon" className="size-9" aria-label={label} onClick={() => { void i18n.changeLanguage(isChinese ? 'en' : 'zh') }}>
            <Globe className="size-4" />
        </Button>
    </TooltipTrigger><TooltipContent>{label}</TooltipContent></Tooltip>
}
