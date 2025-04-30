// src/pages/model/page.tsx
import { AnimatedRoute } from '@/components/layout/AnimatedRoute'
import { ModelTable } from '@/feature/model/components/ModelTable'

export default function ModelPage() {
    return (
        <AnimatedRoute>
            <div className="container mx-auto">
                <ModelTable />
            </div>
        </AnimatedRoute>
    )
}