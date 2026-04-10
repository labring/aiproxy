import { useTranslation } from "react-i18next"

export function SkeletonChart() {
    return (
        <div className="animate-pulse space-y-4 p-6">
            <div className="flex gap-3">
                {[1, 2, 3, 4].map((i) => (
                    <div key={i} className="h-20 flex-1 rounded-xl bg-gray-200/60 dark:bg-gray-700/40" />
                ))}
            </div>
            <div className="h-72 rounded-xl bg-gray-200/60 dark:bg-gray-700/40" />
            <div className="space-y-2">
                {[1, 2, 3, 4, 5].map((i) => (
                    <div key={i} className="h-8 rounded bg-gray-200/60 dark:bg-gray-700/40" />
                ))}
            </div>
        </div>
    )
}

export function EmptyState() {
    const { t } = useTranslation()
    return (
        <div className="flex flex-col items-center justify-center py-20 text-gray-400 dark:text-gray-500">
            <svg className="mb-4 h-16 w-16 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
            </svg>
            <p className="text-sm font-medium">{t("enterprise.customReport.emptyState", "Select dimensions and measures, then click Generate")}</p>
        </div>
    )
}
