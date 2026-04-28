// src/pages/channel/page.tsx
import { AnimatedRoute } from '@/components/layout/AnimatedRoute'
import { ChannelTable } from '@/feature/channel/components/ChannelTable'

export default function ChannelPage() {
    return (
        <AnimatedRoute>
            <div className="h-full">
                <ChannelTable />
            </div>
        </AnimatedRoute>
    )
}