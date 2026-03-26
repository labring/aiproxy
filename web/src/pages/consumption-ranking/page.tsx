import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { channelApi } from '@/api/channel'
import { modelApi } from '@/api/model'
import { ChannelDialog } from '@/feature/channel/components/ChannelDialog'
import { ConsumptionRankingPanel } from '@/feature/group/components/ConsumptionRankingPanel'
import { GroupDialog } from '@/feature/group/components/GroupDialog'
import { ModelDialog } from '@/feature/model/components/ModelDialog'
import type { Channel } from '@/types/channel'
import type { ModelConfig } from '@/types/model'
import { openResourceDialog, showDeletedResourceToast } from '@/utils/resource-dialog'

export default function ConsumptionRankingPage() {
    const { t } = useTranslation()
    const [groupDialogOpen, setGroupDialogOpen] = useState(false)
    const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null)
    const [channelDialogOpen, setChannelDialogOpen] = useState(false)
    const [selectedChannel, setSelectedChannel] = useState<Channel | null>(null)
    const [modelDialogOpen, setModelDialogOpen] = useState(false)
    const [selectedModel, setSelectedModel] = useState<ModelConfig | null>(null)

    const openGroupDialog = (groupId: string) => {
        setSelectedGroupId(groupId)
        setGroupDialogOpen(true)
    }

    const openChannelDialog = async (channelId: number) => {
        await openResourceDialog({
            fetcher: () => channelApi.getChannel(channelId),
            onSuccess: (channel) => {
                setSelectedChannel(channel)
                setChannelDialogOpen(true)
            },
            onNotFound: () => {
                showDeletedResourceToast(t('channel.deleted'))
            },
            onError: () => {
                showDeletedResourceToast(t('channel.fetchFailed'))
            },
        })
    }

    const openModelDialog = async (modelName: string) => {
        await openResourceDialog({
            fetcher: () => modelApi.getModel(modelName),
            onSuccess: (model) => {
                setSelectedModel(model)
                setModelDialogOpen(true)
            },
            onNotFound: () => {
                showDeletedResourceToast(t('model.deleted', '模型已被删除或不存在'))
            },
            onError: () => {
                showDeletedResourceToast(t('model.fetchFailed', '获取模型信息失败'))
            },
        })
    }

    return (
        <div className="h-full p-6">
            <ConsumptionRankingPanel
                onViewGroup={openGroupDialog}
                onViewChannel={openChannelDialog}
                onViewModel={openModelDialog}
            />

            <GroupDialog
                open={groupDialogOpen}
                onOpenChange={setGroupDialogOpen}
                groupId={selectedGroupId}
            />

            <ChannelDialog
                open={channelDialogOpen}
                onOpenChange={setChannelDialogOpen}
                mode="update"
                channel={selectedChannel}
            />

            <ModelDialog
                open={modelDialogOpen}
                onOpenChange={setModelDialogOpen}
                mode="update"
                model={selectedModel}
            />
        </div>
    )
}
