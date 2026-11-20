// src/lib/datetime.ts

// 时间戳(毫秒)转 datetime-local 输入框的值，0/undefined 返回空串
export const timestampMsToDateTimeLocal = (value?: number): string => {
    if (!value) return ''
    const date = new Date(value)
    const pad = (part: number) => String(part).padStart(2, '0')
    return [
        date.getFullYear(),
        '-',
        pad(date.getMonth() + 1),
        '-',
        pad(date.getDate()),
        'T',
        pad(date.getHours()),
        ':',
        pad(date.getMinutes()),
    ].join('')
}

// datetime-local 输入框的值转时间戳(毫秒)，非法输入返回 undefined
export const dateTimeLocalToTimestampMs = (value: string): number | undefined => {
    if (!value) return undefined
    const date = new Date(value)
    return isNaN(date.getTime()) ? undefined : date.getTime()
}
