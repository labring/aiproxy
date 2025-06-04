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

export function DateRangePicker({
    value,
    onChange,
    placeholder,
    className,
    disabled = false,
}: DateRangePickerProps) {
    const { t } = useTranslation()
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
                    id="date"
                    variant={"outline"}
                    disabled={disabled}
                    className={cn(
                        "w-full justify-start text-left font-normal",
                        !date && "text-muted-foreground",
                        className
                    )}
                >
                    <CalendarIcon className="mr-2 h-4 w-4" />
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
                        <span>{placeholder || t('common.selectDateRange')}</span>
                    )}
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-auto p-0" align="start">
                <Calendar
                    initialFocus
                    mode="range"
                    defaultMonth={date?.from}
                    selected={date}
                    onSelect={handleDateChange}
                    numberOfMonths={2}
                />
            </PopoverContent>
        </Popover>
    )
} 