import { startOfWeek, endOfWeek, subWeeks, startOfMonth, endOfMonth, subMonths } from "date-fns"
import { useState, useEffect, useCallback, useRef } from "react"
import type * as echarts from "echarts"

/** Sentinel value for "all" / no-filter in Select components. */
export const ALL_FILTER = "__all__"

export type TimeRange = "7d" | "30d" | "month" | "last_week" | "last_month" | "custom"

export function getTimeRange(range: TimeRange, customStart?: number, customEnd?: number): { start: number; end: number } {
    // Snap to the start of the current hour, then subtract 1 second so that
    // `hour_timestamp <= stableEnd` excludes the current incomplete hour.
    // GroupSummary is hourly-granularity; the current hour is still being
    // written to, so including it causes values to change on every query.
    const nowHour = Math.floor(Date.now() / 3_600_000) * 3600
    const stableEnd = nowHour - 1
    switch (range) {
        case "7d":
            return { start: nowHour - 7 * 86400, end: stableEnd }
        case "30d":
            return { start: nowHour - 30 * 86400, end: stableEnd }
        case "month": {
            const d = new Date()
            d.setDate(1)
            d.setHours(0, 0, 0, 0)
            return { start: Math.floor(d.getTime() / 1000), end: stableEnd }
        }
        case "last_week": {
            const lastWeek = subWeeks(new Date(), 1)
            const start = startOfWeek(lastWeek, { weekStartsOn: 1 }) // Monday
            const end = endOfWeek(lastWeek, { weekStartsOn: 1 })
            start.setHours(0, 0, 0, 0)
            end.setHours(23, 59, 59, 999)
            return {
                start: Math.floor(start.getTime() / 1000),
                end: Math.floor(end.getTime() / 1000)
            }
        }
        case "last_month": {
            const lastMonth = subMonths(new Date(), 1)
            const start = startOfMonth(lastMonth)
            const end = endOfMonth(lastMonth)
            start.setHours(0, 0, 0, 0)
            end.setHours(23, 59, 59, 999)
            return {
                start: Math.floor(start.getTime() / 1000),
                end: Math.floor(end.getTime() / 1000)
            }
        }
        case "custom":
            return {
                start: customStart || nowHour - 7 * 86400,
                end: customEnd || stableEnd,
            }
    }
}

export function formatNumber(n: number): string {
    if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`
    if (n >= 1000) return `${(n / 1000).toFixed(1)}K`
    return String(n)
}

export function formatAmount(n: number): string {
    if (n === 0) return "¥0.00"
    const abs = Math.abs(n)
    if (abs < 0.0001) return `¥${n.toExponential(2)}`
    if (abs < 0.01) return `¥${n.toFixed(4)}`
    return `¥${n.toFixed(2)}`
}

/** Detect dark mode and re-render on change. */
export function useDarkMode(): boolean {
    const [isDark, setIsDark] = useState(() =>
        document.documentElement.classList.contains("dark")
    )

    useEffect(() => {
        const observer = new MutationObserver(() => {
            setIsDark(document.documentElement.classList.contains("dark"))
        })
        observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] })
        return () => observer.disconnect()
    }, [])

    return isDark
}

/** Common ECharts theme colors for dark/light mode. */
export function getEChartsTheme(isDark: boolean) {
    return {
        textColor: isDark ? "#e5e7eb" : "#374151",         // gray-200 / gray-700
        subTextColor: isDark ? "#9ca3af" : "#6b7280",      // gray-400 / gray-500
        borderColor: isDark ? "#374151" : "#ffffff",        // gray-700 / white
        backgroundColor: "transparent",
        splitLineColor: isDark ? "#374151" : "#e5e7eb",     // gray-700 / gray-200
    }
}

/**
 * Hook to auto-refresh an ECharts instance on dark mode change.
 * Call this after setOption to re-apply theme-dependent options.
 */
export function useEChartsResize(chartRef: React.RefObject<HTMLDivElement | null>) {
    const instanceRef = useRef<echarts.ECharts | null>(null)

    const setInstance = useCallback((inst: echarts.ECharts | null) => {
        instanceRef.current = inst
    }, [])

    useEffect(() => {
        const handleResize = () => instanceRef.current?.resize()
        window.addEventListener("resize", handleResize)
        return () => {
            window.removeEventListener("resize", handleResize)
            instanceRef.current?.dispose()
            instanceRef.current = null
        }
    }, [chartRef])

    return { instanceRef, setInstance }
}
