// src/feature/group/components/CreateGroupDialog.tsx
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Loader2 } from 'lucide-react'
import { MultiSelectCombobox } from '@/components/select/MultiSelectCombobox'
import { useCreateGroup } from '../hooks'

interface CreateGroupDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
}

export function CreateGroupDialog({ open, onOpenChange }: CreateGroupDialogProps) {
    const { t } = useTranslation()
    const { createGroup, isLoading } = useCreateGroup()
    const [groupName, setGroupName] = useState('')
    const [availableSets, setAvailableSets] = useState<string[]>([])

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()
        if (!groupName.trim()) return
        createGroup(
            {
                groupId: groupName.trim(),
                data: {
                    available_sets: availableSets.length > 0 ? availableSets : undefined,
                },
            },
            {
                onSuccess: () => {
                    setGroupName('')
                    setAvailableSets([])
                    onOpenChange(false)
                },
            }
        )
    }

    const handleOpenChange = (open: boolean) => {
        if (!open) {
            setGroupName('')
            setAvailableSets([])
        }
        onOpenChange(open)
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="sm:max-w-[425px]">
                <DialogHeader>
                    <DialogTitle>{t('group.dialog.createTitle')}</DialogTitle>
                    <DialogDescription>
                        {t('group.dialog.createDescription')}
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={handleSubmit}>
                    <div className="space-y-4 py-4">
                        <div className="space-y-2">
                            <Label htmlFor="group-name">{t('group.dialog.name')}</Label>
                            <Input
                                id="group-name"
                                placeholder={t('group.dialog.namePlaceholder')}
                                value={groupName}
                                onChange={(e) => setGroupName(e.target.value)}
                                disabled={isLoading}
                            />
                        </div>
                        <div className="space-y-2">
                            <MultiSelectCombobox<string>
                                dropdownItems={[]}
                                selectedItems={availableSets}
                                setSelectedItems={setAvailableSets}
                                handleFilteredDropdownItems={(dropdownItems, selectedItems, inputValue) => {
                                    if (inputValue && !selectedItems.includes(inputValue) && !dropdownItems.includes(inputValue)) {
                                        return [inputValue, ...dropdownItems]
                                    }
                                    return dropdownItems
                                }}
                                handleDropdownItemDisplay={(item) => item}
                                handleSelectedItemDisplay={(item) => item}
                                allowUserCreatedItems={true}
                                placeholder={t('group.dialog.availableSetsPlaceholder')}
                                label={t('group.dialog.availableSets')}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            type="button"
                            variant="outline"
                            onClick={() => handleOpenChange(false)}
                            disabled={isLoading}
                        >
                            {t('common.cancel')}
                        </Button>
                        <Button
                            type="submit"
                            disabled={isLoading || !groupName.trim()}
                        >
                            {isLoading ? (
                                <>
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    {t('group.dialog.submitting')}
                                </>
                            ) : (
                                t('group.dialog.create')
                            )}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    )
}
