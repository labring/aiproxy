import { CircleOff } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

export function ChannelDisabledBadge({ compact = false }: { compact?: boolean }) {
    const { t } = useTranslation()
    const description = t('channel.currentlyDisabled')

    return (
        <Tooltip>
            <TooltipTrigger asChild>
                {compact ? (
                    <span className="inline-flex shrink-0 text-muted-foreground" aria-label={description}>
                        <CircleOff className="size-3.5" aria-hidden="true" />
                    </span>
                ) : (
                    <Badge variant="outline" className="bg-muted/50 text-muted-foreground" aria-label={description}>
                        <CircleOff aria-hidden="true" />
                        {t('channel.disabled')}
                    </Badge>
                )}
            </TooltipTrigger>
            <TooltipContent>{description}</TooltipContent>
        </Tooltip>
    )
}
