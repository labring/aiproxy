export type TimeRange = "7d" | "30d" | "month" | "custom"

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
