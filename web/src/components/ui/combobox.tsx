import { useState } from 'react'
import { Check, ChevronsUpDown, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from '@/components/ui/popover'

interface ComboboxOption {
    value: string
    label: string
}

interface ComboboxProps {
    options: ComboboxOption[]
    value: string
    onValueChange: (value: string) => void
    placeholder?: string
    emptyText?: string
    disabled?: boolean
    className?: string
}

export function Combobox({
    options,
    value,
    onValueChange,
    placeholder = 'Select...',
    emptyText = 'No results',
    disabled = false,
    className,
}: ComboboxProps) {
    const [open, setOpen] = useState(false)

    const selectedLabel = options.find(o => o.value === value)?.label

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    disabled={disabled}
                    className={cn('w-full justify-between font-normal', !value && 'text-muted-foreground', className)}
                >
                    <span className="truncate">{selectedLabel || placeholder}</span>
                    <div className="flex items-center gap-1 ml-2 shrink-0">
                        {value && (
                            <X
                                className="h-3.5 w-3.5 opacity-50 hover:opacity-100"
                                onClick={(e) => {
                                    e.stopPropagation()
                                    onValueChange('')
                                }}
                            />
                        )}
                        <ChevronsUpDown className="h-4 w-4 opacity-50" />
                    </div>
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[--radix-popover-trigger-width] p-0" align="start">
                <div className="max-h-60 overflow-auto p-1">
                    {options.length === 0 ? (
                        <div className="py-6 text-center text-sm text-muted-foreground">{emptyText}</div>
                    ) : (
                        options.map((option) => (
                            <div
                                key={option.value}
                                className={cn(
                                    'relative flex cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none hover:bg-accent hover:text-accent-foreground',
                                    value === option.value && 'bg-accent'
                                )}
                                onClick={() => {
                                    onValueChange(option.value === value ? '' : option.value)
                                    setOpen(false)
                                }}
                            >
                                <Check className={cn('mr-2 h-4 w-4', value === option.value ? 'opacity-100' : 'opacity-0')} />
                                {option.label}
                            </div>
                        ))
                    )}
                </div>
            </PopoverContent>
        </Popover>
    )
}
