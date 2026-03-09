import { Badge } from '@/components/ui/badge'

interface ChannelInfo {
    name: string
    type: number
}

interface ChannelLabelProps {
    id: number
    info?: ChannelInfo
    typeName?: string
    compact?: boolean
    onClick?: (e: React.MouseEvent) => void
}

export function ChannelLabel({ id, info, typeName, compact = false, onClick }: ChannelLabelProps) {
    const name = info?.name || `#${id}`
    const typeLabel = typeName || ''

    const clickableClass = onClick
        ? 'cursor-pointer hover:text-primary transition-colors'
        : ''

    if (compact) {
        return (
            <span className={`inline-flex items-center gap-1.5 ${clickableClass}`} onClick={onClick}>
                {typeLabel && (
                    <Badge variant="outline" className="text-[10px] px-1 py-0 font-normal leading-4 shrink-0">
                        {typeLabel}
                    </Badge>
                )}
                <span className="truncate">{name}</span>
                <span className="text-muted-foreground shrink-0">#{id}</span>
            </span>
        )
    }

    return (
        <span className={`inline-flex items-center gap-1.5 ${clickableClass}`} onClick={onClick}>
            {typeLabel && (
                <Badge variant="outline" className="text-[10px] px-1.5 py-0 font-normal leading-4 shrink-0">
                    {typeLabel}
                </Badge>
            )}
            <span className="truncate">{name}</span>
            <span className="text-muted-foreground shrink-0">(#{id})</span>
        </span>
    )
}
