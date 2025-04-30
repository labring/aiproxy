import { useState, ReactNode, useEffect, JSX } from 'react'
import { useCombobox, UseComboboxReturnValue } from 'downshift'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { ChevronUpIcon, ChevronDownIcon } from 'lucide-react'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export const SingleSelectCombobox: <T>(props: {
    dropdownItems: T[]
    setSelectedItem: (value: T) => void
    handleDropdownItemFilter: (dropdownItems: T[], inputValue: string) => T[]
    handleDropdownItemDisplay: (dropdownItem: T) => ReactNode
    handleInputDisplay?: (item: T) => string
    initSelectedItem?: T
}) => JSX.Element = function <T>({
    dropdownItems,
    setSelectedItem,
    handleDropdownItemFilter,
    handleDropdownItemDisplay,
    handleInputDisplay,
    initSelectedItem
}: {
    dropdownItems: T[]
    setSelectedItem: (value: T) => void
    handleDropdownItemFilter: (dropdownItems: T[], inputValue: string) => T[]
    handleDropdownItemDisplay: (dropdownItem: T) => ReactNode
    handleInputDisplay?: (item: T) => string
    initSelectedItem?: T
}) {
        const { t } = useTranslation()
        const [getFilteredDropdownItems, setGetFilteredDropdownItems] = useState<T[]>(dropdownItems)

        useEffect(() => {
            setGetFilteredDropdownItems(dropdownItems)
        }, [dropdownItems])

        const {
            isOpen: isComboboxOpen,
            getToggleButtonProps,
            getLabelProps,
            getMenuProps,
            getInputProps,
            highlightedIndex,
            getItemProps,
            selectedItem
        }: UseComboboxReturnValue<T> = useCombobox({
            items: getFilteredDropdownItems,
            onInputValueChange: ({ inputValue }) => {
                setGetFilteredDropdownItems(handleDropdownItemFilter(dropdownItems, inputValue || ''))
            },

            initialSelectedItem: initSelectedItem || undefined,

            itemToString: (item) => {
                if (!item) return ''
                return handleInputDisplay ? handleInputDisplay(item) : String(item)
            },

            onSelectedItemChange: ({ selectedItem }) => {
                const selectedDropdownItem = dropdownItems.find((item) => item === selectedItem)
                if (selectedDropdownItem) {
                    setSelectedItem(selectedDropdownItem)
                }
            }
        })

        return (
            <div className="w-full relative">
                <div className="w-full flex flex-col gap-2 items-start">
                    <Label
                        className="text-sm font-medium leading-5 h-5 m-0 whitespace-nowrap"
                        {...getLabelProps()}
                    >
                        {t('channel.dialog.type')}
                    </Label>

                    <div className="w-full relative flex items-center">
                        <Input
                            className="h-8 py-2 pl-3 pr-11 rounded-md text-xs font-normal leading-4 tracking-[0.048px]"
                            placeholder={t('channel.dialog.selectType')}
                            {...getInputProps()}
                        />
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="absolute right-0 h-8 w-8 p-0 flex items-center justify-center"
                            {...getToggleButtonProps()}
                        >
                            {isComboboxOpen ? (
                                <ChevronUpIcon className="h-4 w-4 text-muted-foreground" />
                            ) : (
                                <ChevronDownIcon className="h-4 w-4 text-muted-foreground" />
                            )}
                        </Button>
                    </div>
                </div>

                <ul
                    className={cn(
                        "absolute mt-1 w-full py-1.5 px-1.5 bg-popover",
                        "border border-input max-h-60 overflow-y-auto z-10 rounded-md",
                        isComboboxOpen && getFilteredDropdownItems.length ? "block" : "hidden"
                    )}
                    {...getMenuProps()}
                >
                    {isComboboxOpen &&
                        getFilteredDropdownItems.map((item, index) => (
                            <li
                                key={index}
                                {...getItemProps({ item, index })}
                                className={cn(
                                    "flex p-2 items-center gap-2 self-stretch rounded",
                                    "text-xs font-normal leading-4 tracking-[0.5px] cursor-pointer",
                                    highlightedIndex === index ? "bg-accent" : "bg-transparent",
                                    selectedItem === item ? "font-bold" : "font-normal",
                                    "hover:bg-accent text-foreground"
                                )}
                            >
                                {handleDropdownItemDisplay(item)}
                            </li>
                        ))}
                </ul>
            </div>
        )
    }