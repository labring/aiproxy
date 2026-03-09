// src/feature/group/components/DeleteGroupDialog.tsx
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
import { Loader2 } from 'lucide-react'
import { useDeleteGroup } from '../hooks'

interface DeleteGroupDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    groupId: string | null
    onDeleted?: () => void
}

export function DeleteGroupDialog({
    open,
    onOpenChange,
    groupId,
    onDeleted,
}: DeleteGroupDialogProps) {
    const { t } = useTranslation()
    const { deleteGroup, isLoading } = useDeleteGroup()

    const handleDelete = () => {
        if (!groupId) return
        deleteGroup(groupId, {
            onSuccess: () => {
                onOpenChange(false)
                onDeleted?.()
            },
        })
    }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-[425px]">
                <DialogHeader>
                    <DialogTitle className="text-destructive">
                        {t('group.deleteDialog.confirmTitle')}
                    </DialogTitle>
                    <DialogDescription>
                        {t('group.deleteDialog.confirmDescription')}
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter className="gap-2 sm:gap-0">
                    <Button
                        variant="outline"
                        onClick={() => onOpenChange(false)}
                        disabled={isLoading}
                    >
                        {t('group.deleteDialog.cancel')}
                    </Button>
                    <Button
                        variant="destructive"
                        onClick={handleDelete}
                        disabled={isLoading}
                    >
                        {isLoading ? (
                            <>
                                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                {t('group.deleteDialog.deleting')}
                            </>
                        ) : (
                            t('group.deleteDialog.delete')
                        )}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
