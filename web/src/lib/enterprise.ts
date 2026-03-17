import { startOfWeek, endOfWeek, subWeeks, startOfMonth, endOfMonth, subMonths } from "date-fns"

export type TimeRange = "7d" | "30d" | "month" | "last_week" | "last_month" | "custom"

export function getTimeRange(range: TimeRange, customStart?: number, customEnd?: number): { start: number; end: number } {
    const now = Math.floor(Date.now() / 1000)
    switch (range) {
        case "7d":
            return { start: now - 7 * 86400, end: now }
        case "30d":
            return { start: now - 30 * 86400, end: now }
        case "month": {
            const d = new Date()
            d.setDate(1)
            d.setHours(0, 0, 0, 0)
            return { start: Math.floor(d.getTime() / 1000), end: now }
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
                start: customStart || now - 7 * 86400,
                end: customEnd || now,
            }
    }
}

export function formatNumber(n: number): string {
    if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`
    if (n >= 1000) return `${(n / 1000).toFixed(1)}K`
    return String(n)
}

export function formatAmount(n: number): string {
    return `$${n.toFixed(2)}`
}
