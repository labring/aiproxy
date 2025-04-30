import { useState, useMemo, Dispatch, SetStateAction, ReactNode, JSX } from 'react'
import { useCombobox, useMultipleSelection } from 'downshift'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { XIcon, ChevronUpIcon, ChevronDownIcon } from 'lucide-react'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export const MultiSelectCombobox = function <T>({
    dropdownItems,
    selectedItems,
    setSelectedItems,
    handleFilteredDropdownItems,
    handleDropdownItemDisplay,
    handleSelectedItemDisplay,
}: {
    dropdownItems: T[]
    selectedItems: T[]
    setSelectedItems: Dispatch<SetStateAction<T[]>>
    handleFilteredDropdownItems: (dropdownItems: T[], selectedItems: T[], inputValue: string) => T[]
    handleDropdownItemDisplay: (dropdownItem: T) => ReactNode
    handleSelectedItemDisplay: (selectedItem: T) => ReactNode
}): JSX.Element {
    const { t } = useTranslation()

    const [inputValue, setInputValue] = useState<string>('')

    // Dropdown list excludes already selected options and includes those matching the input.
    const items = useMemo(
        () => handleFilteredDropdownItems(dropdownItems, selectedItems, inputValue),
        [inputValue, selectedItems, dropdownItems, handleFilteredDropdownItems]
    )

    const { getSelectedItemProps, getDropdownProps, removeSelectedItem } = useMultipleSelection({
        selectedItems,
        onStateChange({ selectedItems: newSelectedItems, type }) {
            switch (type) {
                case useMultipleSelection.stateChangeTypes.SelectedItemKeyDownBackspace:
                case useMultipleSelection.stateChangeTypes.SelectedItemKeyDownDelete:
                case useMultipleSelection.stateChangeTypes.DropdownKeyDownBackspace:
                case useMultipleSelection.stateChangeTypes.FunctionRemoveSelectedItem:
                    if (newSelectedItems) {
                        setSelectedItems(newSelectedItems)
                    }
                    break
                default:
                    break
            }
        }
    })
    const {
        isOpen,
        getToggleButtonProps,
        getLabelProps,
        getMenuProps,
        getInputProps,
        highlightedIndex,
        getItemProps,
        selectedItem
    } = useCombobox({
        items,
        defaultHighlightedIndex: 0, // after selection, highlight the first item.
        selectedItem: null,
        inputValue,
        // @ts-expect-error 忽略未使用参数
        stateReducer(state, actionAndChanges) {
            const { changes, type } = actionAndChanges

            switch (type) {
                case useCombobox.stateChangeTypes.InputKeyDownEnter:
                case useCombobox.stateChangeTypes.ItemClick:
                    return {
                        ...changes,
                        isOpen: true, // keep the menu open after selection.
                        highlightedIndex: 0 // with the first option highlighted.
                    }
                default:
                    return changes
            }
        },
        onStateChange({ inputValue: newInputValue, type, selectedItem: newSelectedItem }) {
            switch (type) {
                case useCombobox.stateChangeTypes.InputKeyDownEnter:
                case useCombobox.stateChangeTypes.ItemClick:
                case useCombobox.stateChangeTypes.InputBlur:
                    if (newSelectedItem) {
                        setSelectedItems([...selectedItems, newSelectedItem])
                        setInputValue('')
                    }
                    break

                case useCombobox.stateChangeTypes.InputChange:
                    setInputValue(newInputValue ?? '')

                    break
                default:
                    break
            }
        }
    })

    return (
        <div className="w-full relative">
            <div className="w-full flex flex-col gap-2 items-start">
                <Label
                    className="flex m-0 w-full h-5 justify-between items-center"
                    {...getLabelProps()}
                >
                    <div className="flex gap-0.5 items-start">
                        <span className="whitespace-nowrap text-sm font-medium leading-5 tracking-[0.1px]">
                            {t('channel.dialog.models')}
                        </span>
                    </div>
                </Label>

                <div
                    className="w-full bg-accent/30 rounded-md border border-input p-2"
                >
                    <div className="flex flex-wrap gap-2 items-center">
                        {selectedItems.map((selectedItemForRender, index) => (
                            <div
                                key={`selected-item-${index}`}
                                className={cn(
                                    "bg-accent/50 rounded-md px-1.5 py-1",
                                    "focus:bg-primary/20"
                                )}
                                {...getSelectedItemProps({
                                    selectedItem: selectedItemForRender,
                                    index: index
                                })}
                            >
                                <div className="flex items-center gap-2">
                                    {handleSelectedItemDisplay(selectedItemForRender)}
                                    <button
                                        className="h-4 w-4 rounded flex items-center justify-center cursor-pointer text-muted-foreground hover:text-foreground"
                                        onClick={(e) => {
                                            e.stopPropagation()
                                            removeSelectedItem(selectedItemForRender)
                                        }}
                                    >
                                        <XIcon className="h-3 w-3" />
                                    </button>
                                </div>
                            </div>
                        ))}

                        <div className="flex flex-1 gap-1">
                            <Input
                                className="border-none shadow-none h-auto p-0 text-xs font-normal leading-4 tracking-[0.048px] bg-transparent"
                                placeholder={t('channel.dialog.selectModels')}
                                {...getInputProps(getDropdownProps({ preventKeyAction: isOpen }))}
                            />

                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8 p-0 flex items-center justify-center shrink-0"
                                {...getToggleButtonProps()}
                            >
                                {isOpen ? (
                                    <ChevronUpIcon className="h-4 w-4 text-muted-foreground" />
                                ) : (
                                    <ChevronDownIcon className="h-4 w-4 text-muted-foreground" />
                                )}
                            </Button>
                        </div>
                    </div>
                </div>
            </div>

            <ul
                className={cn(
                    "absolute mt-1 w-full py-1.5 px-1.5 bg-popover",
                    "border border-input max-h-60 overflow-y-auto z-10 rounded-md",
                    isOpen && items.length ? "block" : "hidden"
                )}
                {...getMenuProps()}
            >
                {isOpen &&
                    items.map((item, index) => (
                        <li
                            key={index}
                            className={cn(
                                "flex p-2 items-center gap-2 self-stretch rounded",
                                "text-xs font-normal leading-4 tracking-[0.5px] cursor-pointer",
                                highlightedIndex === index ? "bg-accent" : "bg-transparent",
                                selectedItem === item ? "font-bold" : "font-normal",
                                "hover:bg-accent text-foreground"
                            )}
                            {...getItemProps({ item, index })}
                        >
                            {handleDropdownItemDisplay(item)}
                        </li>
                    ))}
            </ul>
        </div>
    )
}