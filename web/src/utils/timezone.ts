export const DEFAULT_TIMEZONE = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'

export const getTimeZoneOffsetMs = (date: Date, timeZone: string): number => {
    const formatter = new Intl.DateTimeFormat('en-US', {
        timeZone,
        hour12: false,
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
    })

    const parts = formatter.formatToParts(date)
    const values = Object.fromEntries(
        parts
            .filter(part => part.type !== 'literal')
            .map(part => [part.type, part.value])
    )

    const utcTime = Date.UTC(
        Number(values.year),
        Number(values.month) - 1,
        Number(values.day),
        Number(values.hour),
        Number(values.minute),
        Number(values.second)
    )

    return utcTime - date.getTime()
}

export const zonedBoundaryToUnix = (date: Date, timeZone: string, endOfDay = false): number => {
    const year = date.getFullYear()
    const month = date.getMonth()
    const day = date.getDate()
    const hour = endOfDay ? 23 : 0
    const minute = endOfDay ? 59 : 0
    const second = endOfDay ? 59 : 0

    try {
        let utcMs = Date.UTC(year, month, day, hour, minute, second)
        for (let i = 0; i < 2; i += 1) {
            const offsetMs = getTimeZoneOffsetMs(new Date(utcMs), timeZone)
            utcMs = Date.UTC(year, month, day, hour, minute, second) - offsetMs
        }
        return Math.floor(utcMs / 1000)
    } catch {
        const fallback = new Date(year, month, day, hour, minute, second)
        return Math.floor(fallback.getTime() / 1000)
    }
}

export const zonedBoundaryToUnixMs = (date: Date, timeZone: string, endOfDay = false): number => {
    return zonedBoundaryToUnix(date, timeZone, endOfDay) * 1000 + (endOfDay ? 999 : 0)
}

export const unixMsToZonedDate = (timestampMs: number, timeZone: string): Date => {
    try {
        const formatter = new Intl.DateTimeFormat('en-US', {
            timeZone,
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
        })

        const values = Object.fromEntries(
            formatter
                .formatToParts(new Date(timestampMs))
                .filter(part => part.type !== 'literal')
                .map(part => [part.type, part.value])
        )

        return new Date(
            Number(values.year),
            Number(values.month) - 1,
            Number(values.day)
        )
    } catch {
        return new Date(timestampMs)
    }
}

export const formatRangeTimestamp = (timestampSeconds: number, timeZone: string): string => {
    return new Intl.DateTimeFormat(undefined, {
        timeZone,
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
    }).format(new Date(timestampSeconds * 1000))
}
