import * as React from "react"
import { ChevronLeft, ChevronRight } from "lucide-react"
import { DayPicker } from "react-day-picker"
import { zhCN, enUS } from "react-day-picker/locale"
import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"
import { buttonVariants } from "@/components/ui/button"

function Calendar({
  className,
  classNames,
  showOutsideDays = true,
  ...props
}: React.ComponentProps<typeof DayPicker>) {
  const { i18n } = useTranslation()
  return (
    <DayPicker
      locale={i18n.language.startsWith('zh') ? zhCN : enUS}
      showOutsideDays={showOutsideDays}
      className={cn("relative p-3", className)}
      classNames={{
        months: "flex flex-col sm:flex-row gap-4",
        month: "flex w-63 flex-col gap-2",
        month_caption: "flex h-9 justify-center relative items-center w-full",
        caption_label: "text-sm font-medium",
        nav: "pointer-events-none absolute inset-x-3 top-3 z-10 flex items-center justify-between",
        button_previous: cn(
          buttonVariants({ variant: "outline" }),
          "pointer-events-auto size-9 bg-transparent p-0 text-muted-foreground hover:text-foreground"
        ),
        button_next: cn(
          buttonVariants({ variant: "outline" }),
          "pointer-events-auto size-9 bg-transparent p-0 text-muted-foreground hover:text-foreground"
        ),
        month_grid: "w-full border-collapse space-x-1",
        weekdays: "flex",
        weekday:
          "text-muted-foreground w-9 font-normal text-xs",
        week: "flex w-full mt-1",
        day: cn(
          "relative p-0 text-center text-sm focus-within:relative focus-within:z-20 [&:has([aria-selected])]:bg-accent [&:has([aria-selected].range_end)]:rounded-r-md",
          props.mode === "range"
            ? "[&:has(>.range_end)]:rounded-r-md [&:has(>.range_start)]:rounded-l-md first:[&:has([aria-selected])]:rounded-l-md last:[&:has([aria-selected])]:rounded-r-md"
            : "[&:has([aria-selected])]:rounded-md"
        ),
        day_button: cn(
          buttonVariants({ variant: "ghost" }),
          "size-9 rounded-sm p-0 font-normal aria-selected:opacity-100"
        ),
        range_start:
          "range_start aria-selected:bg-primary aria-selected:text-primary-foreground",
        range_end:
          "range_end aria-selected:bg-primary aria-selected:text-primary-foreground",
        selected:
          "bg-primary text-primary-foreground hover:bg-primary hover:text-primary-foreground focus:bg-primary focus:text-primary-foreground",
        today: "bg-accent text-accent-foreground",
        outside:
          "text-muted-foreground aria-selected:text-muted-foreground",
        disabled: "text-muted-foreground opacity-50",
        range_middle:
          "aria-selected:bg-accent aria-selected:text-accent-foreground",
        hidden: "invisible",
        ...classNames,
      }}
      components={{
        Chevron: ({ orientation, ...chevronProps }) => {
          const Icon = orientation === "left" ? ChevronLeft : ChevronRight
          return <Icon className="size-4" {...chevronProps} />
        },
      }}
      {...props}
    />
  )
}

export { Calendar }
