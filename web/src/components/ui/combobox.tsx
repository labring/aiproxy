import { useState } from 'react'
import { useCombobox } from 'downshift'
import { Check, ChevronDown, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

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
    id?: string
}

export function Combobox({
    options,
    value,
    onValueChange,
    placeholder,
    emptyText,
    disabled = false,
    className,
    id,
}: ComboboxProps) {
    const { t } = useTranslation()
    const [query, setQuery] = useState('')
    const items = options.filter((option) =>
        option.label.toLowerCase().includes(query.toLowerCase()),
    )
    const {
        isOpen,
        highlightedIndex,
        getInputProps,
        getToggleButtonProps,
        getMenuProps,
        getItemProps,
        reset,
    } = useCombobox({
        items,
        selectedItem: options.find((option) => option.value === value) ?? null,
        itemToString: (item) => item?.label ?? '',
        onInputValueChange: ({ inputValue }) => setQuery(inputValue ?? ''),
        onSelectedItemChange: ({ selectedItem }) => {
            onValueChange(selectedItem?.value ?? '')
            setQuery('')
        },
    })

    return (
        <div className={cn('relative min-w-0', className)}>
            <div className="relative flex items-center">
                <Input
                    {...getInputProps({
                        id,
                        disabled,
                        'aria-label': placeholder,
                        placeholder,
                    })}
                    className="pr-18"
                />
                <div className="absolute right-1 flex items-center">
                    {value && (
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="size-8"
                            disabled={disabled}
                            aria-label={t('ui.removeItem', { name: value })}
                            onClick={() => {
                                reset()
                                onValueChange('')
                                setQuery('')
                            }}
                        >
                            <X className="size-3.5" />
                        </Button>
                    )}
                    <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-8"
                        {...getToggleButtonProps({
                            disabled,
                            'aria-label': placeholder,
                        })}
                    >
                        <ChevronDown className="size-4" />
                    </Button>
                </div>
            </div>
            <ul
                {...getMenuProps()}
                className={cn(
                    'absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-md border bg-popover p-1 shadow-md',
                    !isOpen && 'hidden',
                )}
            >
                {isOpen &&
                    (items.length ? (
                        items.map((option, index) => (
                            <li
                                key={option.value}
                                {...getItemProps({ item: option, index })}
                                className={cn(
                                    'flex cursor-pointer items-center gap-2 rounded-sm px-2 py-2 text-sm',
                                    highlightedIndex === index && 'bg-accent',
                                )}
                            >
                                <Check
                                    className={cn(
                                        'size-4 shrink-0 text-primary',
                                        value !== option.value && 'invisible',
                                    )}
                                />
                                <span className="min-w-0 break-all">
                                    {option.label}
                                </span>
                            </li>
                        ))
                    ) : (
                        <li className="px-3 py-5 text-center text-sm text-muted-foreground">
                            {emptyText ?? t('common.noResult')}
                        </li>
                    ))}
            </ul>
        </div>
    )
}
