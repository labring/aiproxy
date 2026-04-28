// src/pages/token/page.tsx
import { TokenTable } from '@/feature/token/components/TokenTable'
import { AnimatedRoute } from '@/components/layout/AnimatedRoute'

export default function TokenPage() {
    return (
        <AnimatedRoute>
            <div className="h-full">
                <TokenTable />
            </div>
        </AnimatedRoute>
    )
}