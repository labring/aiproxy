import { ReactNode } from 'react'
import { useSelect } from 'downshift'
import { cn } from '@/lib/utils'
import { ChevronDownIcon } from 'lucide-react'

export const CustomSelect = function <T>({
    listItems,
    handleSelectedItemChange,
    handleDropdownItemDisplay,
    handleSelectedItemDisplay,
    placeholder,
    initSelectedItem
}: {
    listItems: T[]
    handleSelectedItemChange: (selectedItem: T) => void
    handleDropdownItemDisplay: (dropdownItem: T) => ReactNode
    handleSelectedItemDisplay: (selectedItem: T) => ReactNode
    placeholder?: string
    initSelectedItem?: T
}) {
    const items = [placeholder, ...listItems]

    const {
        isOpen,
        selectedItem,
        getToggleButtonProps,
        getMenuProps,
        getItemProps,
        highlightedIndex
    } = useSelect({
        items: items,
        initialSelectedItem: initSelectedItem,
        onSelectedItemChange: ({ selectedItem: newSelectedItem }) => {
            if (newSelectedItem === placeholder) {
                handleSelectedItemChange(undefined as T)
            } else {
                handleSelectedItemChange(newSelectedItem as T)
            }
        }
    })

    return (
        <div className="w-full relative flex-1">
            <div
                className={cn(
                    "h-8 w-full rounded-md border border-input flex items-center px-3",
                    "bg-background dark:bg-background",
                    "focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]",
                    "hover:border-primary-300 dark:hover:border-primary"
                )}
                {...getToggleButtonProps()}
            >
                {selectedItem ? (
                    handleSelectedItemDisplay(selectedItem as T)
                ) : placeholder ? (
                    <p
                        className="flex-1 text-xs font-normal leading-4 tracking-[0.048px] text-muted-foreground"
                    >
                        {placeholder}
                    </p>
                ) : (
                    <p
                        className="flex-1 text-xs font-normal leading-4 tracking-[0.048px] text-muted-foreground"
                    >
                        Select
                    </p>
                )}
                <div className="ml-auto" style={{ transform: isOpen ? 'rotate(180deg)' : undefined }}>
                    <ChevronDownIcon className="size-3 text-foreground" />
                </div>
            </div>

            <ul
                {...getMenuProps()}
                className={cn(
                    "absolute mt-0.5 w-full py-1.5 px-1.5 bg-popover",
                    "border border-input max-h-60 overflow-y-auto z-10 rounded-md",
                    "dark:bg-popover dark:border-input dark:text-popover-foreground",
                    "shadow-md",
                    isOpen && items.length ? "block" : "hidden"
                )}
            >
                {isOpen &&
                    items.map((item, index) => (
                        <li
                            {...getItemProps({ item, index })}
                            key={index}
                            className={cn(
                                "flex p-2 items-center gap-2 self-stretch rounded",
                                "text-xs font-normal leading-4 tracking-[0.5px] cursor-pointer",
                                highlightedIndex === index ? "bg-accent dark:bg-accent" : "bg-transparent",
                                selectedItem === item ? "font-bold" : "font-normal",
                                "hover:bg-accent dark:hover:bg-accent hover:text-accent-foreground dark:hover:text-accent-foreground text-foreground"
                            )}
                        >
                            {handleDropdownItemDisplay(item as T)}
                        </li>
                    ))}
            </ul>
        </div>
    )
}