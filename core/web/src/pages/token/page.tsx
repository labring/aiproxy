// src/pages/token/page.tsx
import { TokenTable } from '@/feature/token/components/TokenTable'
import { AnimatedRoute } from '@/components/layout/AnimatedRoute'

export default function TokenPage() {
    return (
        <AnimatedRoute>
            <div className="container mx-auto">
                <TokenTable />
            </div>
        </AnimatedRoute>
    )
}