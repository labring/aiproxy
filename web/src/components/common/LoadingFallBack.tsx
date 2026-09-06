import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export function LoadingFallback() {
    const { t } = useTranslation()

    return (
        <div className="flex min-h-64 flex-1 items-center justify-center gap-3 bg-background text-sm text-muted-foreground" role="status">
            <Loader2 className="size-5 animate-spin text-primary" aria-hidden="true" />
            <span>{t('common.loading')}</span>
        </div>
    )
}
