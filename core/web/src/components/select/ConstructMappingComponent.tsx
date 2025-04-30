import { useState, useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { TrashIcon, PlusIcon } from 'lucide-react'
import { CustomSelect } from './Select'

type MapKeyValuePair = { key: string; value: string }

// mapKeys determines the available selection options
export const ConstructMappingComponent = function ({
    mapKeys,
    mapData,
    setMapData
}: {
    mapKeys: string[]
    mapData: Record<string, string>
    setMapData: (mapping: Record<string, string>) => void
}) {
    const { t } = useTranslation()

    const [mapKeyValuePairs, setMapkeyValuePairs] = useState<Array<MapKeyValuePair>>([])
    const [isInternalUpdate, setIsInternalUpdate] = useState(false)

    useEffect(() => {
        if (!isInternalUpdate) {
            const entries = Object.entries(mapData)
            setMapkeyValuePairs(
                entries.length > 0
                    ? entries.map(([key, value]) => ({ key, value }))
                    : [{ key: '', value: '' }]
            )
        }
        setIsInternalUpdate(false)
    }, [mapData])

    const handleDropdownItemDisplay = (dropdownItem: string) => {
        if (dropdownItem === t('channel.dialog.selectModels')) {
            return (
                <span className="text-xs font-normal leading-4 tracking-[0.048px] text-muted-foreground">
                    {t('channel.dialog.selectModels')}
                </span>
            )
        }
        return (
            <span className="text-xs font-normal leading-4 tracking-[0.048px]">
                {dropdownItem}
            </span>
        )
    }

    const handleSeletedItemDisplay = (selectedItem: string) => {
        if (selectedItem === t('channel.dialog.selectModels')) {
            return (
                <span className="text-xs font-normal leading-4 tracking-[0.048px] text-muted-foreground">
                    {t('channel.dialog.selectModels')}
                </span>
            )
        }
        return (
            <div className="max-w-[114px] overflow-x-auto whitespace-nowrap scrollbar-none">
                <span className="text-xs font-normal leading-4 tracking-[0.048px]">
                    {selectedItem}
                </span>
            </div>
        )
    }

    // Handling mapData and mapKeyValuePairs cleanup when map keys change.
    useEffect(() => {
        // 1. Handle mapData cleanup
        const removedKeysFromMapData = Object.keys(mapData).filter((key) => !mapKeys.includes(key))
        if (removedKeysFromMapData.length > 0) {
            const newMapData = { ...mapData }
            removedKeysFromMapData.forEach((key) => {
                delete newMapData[key]
            })
            setIsInternalUpdate(true)
            setMapData(newMapData)
        }

        // 2. Handle mapKeyValuePairs cleanup
        const removedPairs = mapKeyValuePairs.filter((pair) => pair.key && !mapKeys.includes(pair.key))
        if (removedPairs.length > 0) {
            const newMapKeyValuePairs = mapKeyValuePairs.filter(
                (pair) => !pair.key || mapKeys.includes(pair.key)
            )
            setMapkeyValuePairs(newMapKeyValuePairs)
        }
    }, [mapKeys])

    // Get the keys that have been selected
    const getSelectedMapKeys = (currentIndex: number) => {
        const selected = new Set<string>()
        mapKeyValuePairs.forEach((mapKeyValuePair, idx) => {
            if (idx !== currentIndex && mapKeyValuePair.key) {
                selected.add(mapKeyValuePair.key)
            }
        })
        return selected
    }

    // Handling adding a new row
    const handleAddNewMapKeyPair = () => {
        setMapkeyValuePairs([...mapKeyValuePairs, { key: '', value: '' }])
    }

    // Handling deleting a row
    const handleRemoveMapKeyPair = (index: number) => {
        const mapKeyValuePair = mapKeyValuePairs[index]
        const newMapData = { ...mapData }
        if (mapKeyValuePair.key) {
            delete newMapData[mapKeyValuePair.key]
        }
        setIsInternalUpdate(true)
        setMapData(newMapData)

        const newMapKeyValuePairs = mapKeyValuePairs.filter((_, idx) => idx !== index)
        setMapkeyValuePairs(newMapKeyValuePairs)
    }

    // Handling selection/input changes
    const handleInputChange = (index: number, field: 'key' | 'value', value: string) => {
        const newMapKeyValuePairs = [...mapKeyValuePairs]
        const oldValue = newMapKeyValuePairs[index][field]
        newMapKeyValuePairs[index][field] = value

        // Update the mapping relationship
        const newMapData = { ...mapData }
        if (field === 'key') {
            if (oldValue) delete newMapData[oldValue]

            if (!value) {
                newMapKeyValuePairs[index].value = ''
            }

            if (value && newMapKeyValuePairs[index].value) {
                newMapData[value] = newMapKeyValuePairs[index].value
            }
        } else {
            if (newMapKeyValuePairs[index].key) {
                newMapData[newMapKeyValuePairs[index].key] = value
            }
        }

        setMapkeyValuePairs(newMapKeyValuePairs)
        setIsInternalUpdate(true)
        setMapData(newMapData)
    }

    // Check if there are still keys that can be selected
    const hasAvailableKeys = useMemo(() => {
        const usedKeys = new Set(
            mapKeyValuePairs.map((mapKeyValuePair) => mapKeyValuePair.key).filter(Boolean)
        )
        // Ensure mapKeyValuePairs length does not exceed mapKeys length
        return (
            mapKeyValuePairs.length < mapKeys.length && mapKeys.some((mapKey) => !usedKeys.has(mapKey))
        )
    }, [mapKeys, mapKeyValuePairs])

    return (
        <div className="w-full flex flex-col gap-2 items-start">
            <Label
                className="text-sm font-medium leading-5 tracking-[0.1px] flex items-center h-5 m-0"
            >
                {t('channel.dialog.modelMapping')}
            </Label>

            {mapKeyValuePairs.map((row, index) => (
                <div key={`${index}-${row.key}`} className="flex gap-2 w-full items-center">
                    <CustomSelect<string>
                        listItems={mapKeys.filter((key) => !getSelectedMapKeys(index).has(key))}
                        initSelectedItem={row.key !== '' && row.key ? row.key : undefined}
                        // when select placeholder, the newSelectedItem is null
                        handleSelectedItemChange={(newSelectedItem) =>
                            handleInputChange(index, 'key', newSelectedItem)
                        }
                        handleDropdownItemDisplay={handleDropdownItemDisplay}
                        handleSelectedItemDisplay={handleSeletedItemDisplay}
                        placeholder={t('channel.dialog.selectModels')}
                    />

                    <div className="flex-1 w-full">
                        <Input
                            className="h-8 py-2 px-3 text-xs"
                            value={row.value}
                            onChange={(e) => handleInputChange(index, 'value', e.target.value)}
                            placeholder={t('channel.dialog.mappedName')}
                        />
                    </div>

                    <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 p-0 hover:bg-accent hover:text-destructive"
                        onClick={() => handleRemoveMapKeyPair(index)}
                    >
                        <TrashIcon className="h-4 w-4" />
                    </Button>
                </div>
            ))}

            {hasAvailableKeys && (
                <Button
                    type="button"
                    variant="outline"
                    className="h-8 w-full flex items-center justify-center gap-1.5 hover:border-primary-300"
                    onClick={handleAddNewMapKeyPair}
                >
                    <PlusIcon className="h-4 w-4 text-muted-foreground" />
                    <span className="text-xs font-medium leading-4 tracking-[0.5px] text-muted-foreground">
                        {t('channel.dialog.create')}
                    </span>
                </Button>
            )}
        </div>
    )
}

export default ConstructMappingComponent