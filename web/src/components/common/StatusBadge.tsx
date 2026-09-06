import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

export function StatusBadge({
    enabled,
    label,
}: {
    enabled: boolean
    label?: string
}) {
    const { t } = useTranslation()
    return (
        <span
            className={cn(
                'inline-flex items-center gap-1.5 whitespace-nowrap text-xs font-medium',
                enabled
                    ? 'text-emerald-700 dark:text-emerald-400'
                    : 'text-muted-foreground',
            )}
        >
            <span
                className={cn(
                    'size-1.5 shrink-0 rounded-full',
                    enabled ? 'bg-emerald-500' : 'bg-muted-foreground/50',
                )}
            />
            {label ?? t(enabled ? 'token.enabled' : 'token.disabled')}
        </span>
    )
}
