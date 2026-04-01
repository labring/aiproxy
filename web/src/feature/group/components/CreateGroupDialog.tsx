// src/feature/group/components/CreateGroupDialog.tsx
import { useEffect, useState } from 'react'
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
import { Switch } from '@/components/ui/switch'
import { Loader2 } from 'lucide-react'
import { MultiSelectCombobox } from '@/components/select/MultiSelectCombobox'
import { useCreateGroup, useUpdateGroup } from '../hooks'
import type { Group } from '@/types/group'

interface CreateGroupDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    group?: Group | null
}

export function CreateGroupDialog({ open, onOpenChange, group = null }: CreateGroupDialogProps) {
    const { t } = useTranslation()
    const { createGroup, isLoading } = useCreateGroup()
    const { updateGroup, isLoading: isUpdating } = useUpdateGroup()
    const [groupName, setGroupName] = useState('')
    const [displayName, setDisplayName] = useState('')
    const [availableSets, setAvailableSets] = useState<string[]>([])
    const [rpmRatio, setRpmRatio] = useState<number | undefined>(undefined)
    const [tpmRatio, setTpmRatio] = useState<number | undefined>(undefined)
    const [balanceAlertEnabled, setBalanceAlertEnabled] = useState(false)
    const [balanceAlertThreshold, setBalanceAlertThreshold] = useState<number | undefined>(undefined)

    const loading = isLoading || isUpdating
    const isEdit = !!group

    useEffect(() => {
        if (!open) {
            return
        }

        if (group) {
            setGroupName(group.id)
            setDisplayName(group.name || '')
            setAvailableSets(group.available_sets || [])
            setRpmRatio(group.rpm_ratio || undefined)
            setTpmRatio(group.tpm_ratio || undefined)
            setBalanceAlertEnabled(group.balance_alert_enabled)
            setBalanceAlertThreshold(group.balance_alert_threshold || undefined)
            return
        }

        setGroupName('')
        setDisplayName('')
        setAvailableSets([])
        setRpmRatio(undefined)
        setTpmRatio(undefined)
        setBalanceAlertEnabled(false)
        setBalanceAlertThreshold(undefined)
    }, [open, group])

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()
        if (!groupName.trim()) return
        const payload = {
            name: displayName.trim() || undefined,
            available_sets: availableSets,
            rpm_ratio: rpmRatio ?? 0,
            tpm_ratio: tpmRatio ?? 0,
            balance_alert_enabled: balanceAlertEnabled,
            balance_alert_threshold: balanceAlertThreshold ?? 0,
        }

        if (isEdit && group) {
            updateGroup({
                groupId: group.id,
                data: payload,
            }, {
                onSuccess: () => {
                    onOpenChange(false)
                },
            })
            return
        }

        createGroup(
            {
                groupId: groupName.trim(),
                data: payload,
            },
            {
                onSuccess: () => {
                    setGroupName('')
                    setDisplayName('')
                    setAvailableSets([])
                    setRpmRatio(undefined)
                    setTpmRatio(undefined)
                    setBalanceAlertEnabled(false)
                    setBalanceAlertThreshold(undefined)
                    onOpenChange(false)
                },
            }
        )
    }

    const handleOpenChange = (open: boolean) => {
        if (!open) {
            setGroupName('')
            setDisplayName('')
            setAvailableSets([])
            setRpmRatio(undefined)
            setTpmRatio(undefined)
            setBalanceAlertEnabled(false)
            setBalanceAlertThreshold(undefined)
        }
        onOpenChange(open)
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="sm:max-w-[425px]">
                <DialogHeader>
                    <DialogTitle>{isEdit ? t('group.dialog.updateTitle') : t('group.dialog.createTitle')}</DialogTitle>
                    <DialogDescription>
                        {isEdit ? t('group.dialog.updateDescription') : t('group.dialog.createDescription')}
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
                                disabled={loading || isEdit}
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="group-display-name">{t('group.dialog.displayName')}</Label>
                            <Input
                                id="group-display-name"
                                placeholder={t('group.dialog.displayNamePlaceholder')}
                                value={displayName}
                                onChange={(e) => setDisplayName(e.target.value)}
                                disabled={loading}
                            />
                        </div>
                        <div className="grid grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label htmlFor="group-rpm-ratio">{t('group.dialog.rpmRatio')}</Label>
                                <Input
                                    id="group-rpm-ratio"
                                    type="number"
                                    min={0}
                                    step="0.01"
                                    placeholder={t('group.dialog.rpmRatioPlaceholder')}
                                    value={rpmRatio ?? ''}
                                    onChange={(e) => setRpmRatio(e.target.value === '' ? undefined : parseFloat(e.target.value))}
                                    disabled={loading}
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="group-tpm-ratio">{t('group.dialog.tpmRatio')}</Label>
                                <Input
                                    id="group-tpm-ratio"
                                    type="number"
                                    min={0}
                                    step="0.01"
                                    placeholder={t('group.dialog.tpmRatioPlaceholder')}
                                    value={tpmRatio ?? ''}
                                    onChange={(e) => setTpmRatio(e.target.value === '' ? undefined : parseFloat(e.target.value))}
                                    disabled={loading}
                                />
                            </div>
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
                        <div className="flex items-center justify-between rounded-lg border p-3">
                            <Label htmlFor="group-balance-alert">{t('group.dialog.balanceAlertEnabled')}</Label>
                            <Switch
                                checked={balanceAlertEnabled}
                                onCheckedChange={setBalanceAlertEnabled}
                                disabled={loading}
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="group-balance-threshold">{t('group.dialog.balanceAlertThreshold')}</Label>
                            <Input
                                id="group-balance-threshold"
                                type="number"
                                min={0}
                                step="0.01"
                                placeholder={t('group.dialog.balanceAlertThresholdPlaceholder')}
                                value={balanceAlertThreshold ?? ''}
                                onChange={(e) => setBalanceAlertThreshold(e.target.value === '' ? undefined : parseFloat(e.target.value))}
                                disabled={loading}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            type="button"
                            variant="outline"
                            onClick={() => handleOpenChange(false)}
                            disabled={loading}
                        >
                            {t('common.cancel')}
                        </Button>
                        <Button
                            type="submit"
                            disabled={loading || !groupName.trim()}
                        >
                            {loading ? (
                                <>
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    {t('group.dialog.submitting')}
                                </>
                            ) : (
                                isEdit ? t('group.dialog.update') : t('group.dialog.create')
                            )}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    )
}
