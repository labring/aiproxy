import { useState } from 'react'

import { GroupRankingPanel } from '@/feature/group/components/GroupRankingPanel'
import { GroupDialog } from '@/feature/group/components/GroupDialog'

export default function GroupRankingPage() {
    const [groupDialogOpen, setGroupDialogOpen] = useState(false)
    const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null)

    const openGroupDialog = (groupId: string) => {
        setSelectedGroupId(groupId)
        setGroupDialogOpen(true)
    }

    return (
        <div className="h-full p-6">
            <GroupRankingPanel onViewGroup={openGroupDialog} />

            <GroupDialog
                open={groupDialogOpen}
                onOpenChange={setGroupDialogOpen}
                groupId={selectedGroupId}
            />
        </div>
    )
}
