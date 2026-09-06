"use client"

import * as React from "react"
import { CalendarIcon } from "lucide-react"
import { format } from "date-fns"
import { DateRange } from "react-day-picker"
import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from "@/components/ui/popover"

interface DateRangePickerProps {
    value?: DateRange
    onChange?: (dateRange: DateRange | undefined) => void
    placeholder?: string
    className?: string
    disabled?: boolean
}

const subscribeToViewport = (onChange: () => void) => {
    const query = window.matchMedia('(max-width: 639px)')
    query.addEventListener('change', onChange)
    return () => query.removeEventListener('change', onChange)
}
const isNarrowViewport = () => window.matchMedia('(max-width: 639px)').matches

export function DateRangePicker({
    value,
    onChange,
    placeholder,
    className,
    disabled = false,
}: DateRangePickerProps) {
    const { t } = useTranslation()
    const narrow = React.useSyncExternalStore(subscribeToViewport, isNarrowViewport, () => true)
    const [date, setDate] = React.useState<DateRange | undefined>(value)

    // 当外部 value 变化时更新内部状态
    React.useEffect(() => {
        setDate(value)
    }, [value])

    const handleDateChange = (newDate: DateRange | undefined) => {
        setDate(newDate)
        onChange?.(newDate)
    }

    return (
        <Popover>
            <PopoverTrigger asChild>
                <Button
                    type="button"
                    variant={"outline"}
                    disabled={disabled}
                    className={cn(
                        "w-full justify-start text-left font-normal",
                        !date && "text-muted-foreground",
                        className
                    )}
                >
                    <CalendarIcon className="h-4 w-4 shrink-0" />
                    <span className="min-w-0 truncate">
                    {date?.from ? (
                        date.to ? (
                            <>
                                {format(date.from, "yyyy-MM-dd")} -{" "}
                                {format(date.to, "yyyy-MM-dd")}
                            </>
                        ) : (
                            format(date.from, "yyyy-MM-dd")
                        )
                    ) : (
                        placeholder || t('common.selectDateRange')
                    )}
                    </span>
                </Button>
            </PopoverTrigger>
            <PopoverContent className="max-h-[calc(100dvh-2rem)] w-auto max-w-[calc(100vw-2rem)] overflow-y-auto p-0" align="start">
                <Calendar
                    autoFocus
                    mode="range"
                    defaultMonth={date?.from}
                    selected={date}
                    onSelect={handleDateChange}
                    numberOfMonths={narrow ? 1 : 2}
                />
            </PopoverContent>
        </Popover>
    )
}
