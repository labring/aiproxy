import * as React from "react"

import { cn } from "@/lib/utils"

function Input({ className, type, onWheel, ...props }: React.ComponentProps<"input">) {
  // Prevent scroll wheel from changing number input values
  const handleWheel = React.useCallback(
    (e: React.WheelEvent<HTMLInputElement>) => {
      if (type === "number") {
        ;(e.target as HTMLInputElement).blur()
        e.preventDefault()
      }
      onWheel?.(e)
    },
    [type, onWheel]
  )

  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "file:text-foreground placeholder:text-muted-foreground selection:bg-primary/20 border-input flex h-9 w-full min-w-0 rounded-md border bg-card px-3 py-1.5 text-sm transition-[border-color,box-shadow] outline-none file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium disabled:cursor-not-allowed disabled:bg-muted disabled:opacity-60 hover:border-muted-foreground/50",
        "focus-visible:border-ring focus-visible:ring-ring/20 focus-visible:ring-[3px]",
        "aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive",
        type === "number" && "[appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none",
        className
      )}
      onWheel={handleWheel}
      {...props}
    />
  )
}

export { Input }
