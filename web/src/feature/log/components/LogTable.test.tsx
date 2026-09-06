// @vitest-environment jsdom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { channelApi } from '@/api/channel'
import type { LogRecord } from '@/types/log'
import { LogTable } from './LogTable'

vi.mock('react-i18next', () => ({
    useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('@/api/channel', () => ({
    channelApi: {
        getChannelBatchInfo: vi.fn(),
        getTypeMetas: vi.fn().mockResolvedValue({ 1: { name: 'OpenAI' } }),
    },
}))

const reactTestEnvironment = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }
reactTestEnvironment.IS_REACT_ACT_ENVIRONMENT = true

describe('LogTable channel metadata', () => {
    let container: HTMLDivElement
    let root: Root
    let client: QueryClient

    beforeEach(() => {
        vi.clearAllMocks()
        container = document.createElement('div')
        document.body.appendChild(container)
        root = createRoot(container)
        client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } })
        vi.mocked(channelApi.getChannelBatchInfo).mockResolvedValue([
            { id: 7, name: 'Archived channel', type: 1, status: 1, remark: 'Retained metadata' },
            { id: 8, name: 'Standby channel', type: 1, status: 2, backup_only: true },
        ])
    })

    afterEach(async () => {
        await act(async () => root.unmount())
        client.clear()
        container.remove()
    })

    async function renderLogs(channels: number[]) {
        const data = channels.map((channel, index) => ({
            id: index + 1, channel, group: 'test-group', model: 'test-model', code: 200,
            created_at: '2026-01-01T00:00:02Z', request_at: '2026-01-01T00:00:00Z',
        } as LogRecord))
        await act(async () => root.render(
            <QueryClientProvider client={client}>
                <LogTable data={data} total={data.length} page={1} pageSize={10}
                    onPageChange={vi.fn()} onPageSizeChange={vi.fn()} />
            </QueryClientProvider>,
        ))
    }

    it('batches unique channel IDs and reuses metadata in the second column and expanded details', async () => {
        await renderLogs([8, 7, 8, 9, 0])
        await act(async () => {
            await vi.waitFor(() => expect(container.textContent).toContain('Archived channel'))
        })

        expect(channelApi.getChannelBatchInfo).toHaveBeenCalledExactlyOnceWith([7, 8, 9])
        expect(container.querySelectorAll('thead th')[1].textContent).toBe('log.channel')
        const rows = container.querySelectorAll('tbody tr')
        const standbyCell = rows[0].querySelectorAll('td')[1]
        expect(standbyCell.textContent).toContain('Standby channel')
        expect(standbyCell.querySelector('[aria-label="channel.currentlyDisabled"]')).not.toBeNull()
        expect(standbyCell.querySelector('[aria-label="channel.dialog.backupOnly"]')).not.toBeNull()
        expect(rows[1].querySelectorAll('td')[1].querySelector('[title="Archived channel · Retained metadata"]')).not.toBeNull()
        expect(rows[3].querySelectorAll('td')[1].textContent).toContain('#9')
        expect(rows[4].querySelectorAll('td')[1].textContent).toBe('-')

        await act(async () => (rows[1].querySelector('button') as HTMLButtonElement).click())
        expect(container.textContent).toContain('Retained metadata')
        expect(channelApi.getChannelBatchInfo).toHaveBeenCalledTimes(1)
        expect(container.querySelector('td[colspan="11"]')).not.toBeNull()
    })

    it('does not fetch channel metadata for rows without a channel', async () => {
        await renderLogs([0])
        expect(channelApi.getChannelBatchInfo).not.toHaveBeenCalled()
        expect(container.querySelectorAll('tbody td')[1].textContent).toBe('-')
    })
})
