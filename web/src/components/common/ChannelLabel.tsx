import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

interface ChannelInfo {
    name: string
    type: number
}

interface ChannelLabelProps {
    id: number
    info?: ChannelInfo
    typeName?: string
    compact?: boolean
    className?: string
    onClick?: (e: React.MouseEvent) => void
}

export function ChannelLabel({
    id,
    info,
    typeName,
    compact = false,
    className,
    onClick
}: ChannelLabelProps) {
    const name = info?.name || `#${id}`
    const typeLabel = typeName || ''

    const clickableClass = onClick
        ? 'cursor-pointer hover:text-primary transition-colors'
        : ''

    if (compact) {
        return (
            <span
                className={cn('inline-flex items-center gap-1.5 min-w-0', clickableClass, className)}
                onClick={onClick}
            >
                {typeLabel && (
                    <Badge
                        variant="outline"
                        className="text-[10px] px-1 py-0 font-normal leading-4 shrink-0 max-w-[88px] truncate"
                        title={typeLabel}
                    >
                        {typeLabel}
                    </Badge>
                )}
                <span className="truncate max-w-[140px]" title={name}>{name}</span>
                <span className="text-muted-foreground shrink-0 max-w-[80px] truncate" title={`#${id}`}>#{id}</span>
            </span>
        )
    }

    return (
        <span
            className={cn('inline-flex items-center gap-1.5 min-w-0', clickableClass, className)}
            onClick={onClick}
        >
            {typeLabel && (
                <Badge
                    variant="outline"
                    className="text-[10px] px-1.5 py-0 font-normal leading-4 shrink-0 max-w-[104px] truncate"
                    title={typeLabel}
                >
                    {typeLabel}
                </Badge>
            )}
            <span className="truncate max-w-[180px]" title={name}>{name}</span>
            <span className="text-muted-foreground shrink-0 max-w-[90px] truncate" title={`(#${id})`}>(#{id})</span>
        </span>
    )
}
