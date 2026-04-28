// src/feature/group/components/GroupDialog.tsx
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useGroup } from '../hooks'
import { GroupDashboardTab } from './GroupDashboardTab'
import { GroupTokensTab } from './GroupTokensTab'
import { GroupModelsTab } from './GroupModelsTab'
import { GroupModelConfigsTab } from './GroupModelConfigsTab'
import { GroupLogsTab } from './GroupLogsTab'
import { Skeleton } from '@/components/ui/skeleton'

interface GroupDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    groupId: string | null
    initialTab?: string
    initialTokenName?: string
}

export function GroupDialog({ open, onOpenChange, groupId, initialTab = 'dashboard', initialTokenName }: GroupDialogProps) {
    const { t } = useTranslation()
    const [activeTab, setActiveTab] = useState(initialTab)
    const [dashboardTokenName, setDashboardTokenName] = useState<string | undefined>(initialTokenName)

    const { data: group, isLoading } = useGroup(groupId || '')

    // Reset to initialTab when dialog opens
    useEffect(() => {
        if (open) {
            setActiveTab(initialTab)
            setDashboardTokenName(initialTokenName)
        }
    }, [open, initialTab, initialTokenName])

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-[90vw] w-[90vw] max-h-[90vh] h-[85vh] overflow-hidden flex flex-col">
                <DialogHeader className="flex-shrink-0">
                    <DialogTitle className="text-xl font-semibold">
                        {isLoading ? (
                            <Skeleton className="h-6 w-32" />
                        ) : (
                            `Group: ${group?.id || groupId}`
                        )}
                    </DialogTitle>
                </DialogHeader>

                <div className="flex-1 overflow-hidden min-h-0">
                    {isLoading ? (
                        <div className="space-y-4">
                            <Skeleton className="h-10 w-full" />
                            <Skeleton className="h-64 w-full" />
                        </div>
                    ) : (
                        <Tabs value={activeTab} onValueChange={setActiveTab} className="h-full flex flex-col">
                            <TabsList className="flex-shrink-0 w-fit">
                                <TabsTrigger value="dashboard">
                                    {t('group.tabs.dashboard')}
                                </TabsTrigger>
                                <TabsTrigger value="tokens">
                                    {t('group.tabs.tokens')}
                                </TabsTrigger>
                                <TabsTrigger value="models">
                                    {t('group.tabs.models')}
                                </TabsTrigger>
                                <TabsTrigger value="modelConfigs">
                                    {t('group.tabs.modelConfigs')}
                                </TabsTrigger>
                                <TabsTrigger value="logs">
                                    {t('group.tabs.logs')}
                                </TabsTrigger>
                            </TabsList>

                            <div className="flex-1 overflow-auto mt-4 min-h-0">
                                <TabsContent value="dashboard" className="h-full m-0">
                                    {groupId && <GroupDashboardTab groupId={groupId} initialTokenName={dashboardTokenName} />}
                                </TabsContent>

                                <TabsContent value="tokens" className="h-full m-0">
                                    {groupId && (
                                        <GroupTokensTab
                                            groupId={groupId}
                                            onNavigateDashboard={(tokenName) => {
                                                setDashboardTokenName(tokenName)
                                                setActiveTab('dashboard')
                                            }}
                                        />
                                    )}
                                </TabsContent>

                                <TabsContent value="models" className="h-full m-0">
                                    {groupId && <GroupModelsTab groupId={groupId} />}
                                </TabsContent>

                                <TabsContent value="modelConfigs" className="h-full m-0">
                                    {groupId && <GroupModelConfigsTab groupId={groupId} />}
                                </TabsContent>

                                <TabsContent value="logs" className="h-full m-0">
                                    {groupId && <GroupLogsTab groupId={groupId} initialTokenName={initialTab === 'logs' ? initialTokenName : undefined} />}
                                </TabsContent>
                            </div>
                        </Tabs>
                    )}
                </div>
            </DialogContent>
        </Dialog>
    )
}
