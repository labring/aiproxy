// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { expect, it, vi } from 'vitest'
import { tokenApi } from '@/api/token'
import type { TokensResponse } from '@/types/token'
import { GroupTokensTab } from './GroupTokensTab'

const renders = vi.hoisted(() => ({ count: 0 }))
vi.mock('@tanstack/react-table', async (importOriginal) => {
    const actual = await importOriginal<typeof import('@tanstack/react-table')>()
    return {
        ...actual,
        useReactTable: (options: Parameters<typeof actual.useReactTable>[0]) => {
            // Fail instead of hanging if TanStack's queued page reset triggers a render loop.
            if (++renders.count > 40) throw new Error('Table entered a render loop')
            return actual.useReactTable(options)
        },
    }
})
vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/token', () => ({ tokenApi: { getGroupTokens: vi.fn() } }))
vi.mock('@/feature/monitor/runtime-hooks', () => ({ useGroupTokenMetrics: () => ({}) }))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

it('settles on the empty state when a group has no API keys', async () => {
    vi.useFakeTimers()
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const container = document.createElement('div')
    const root = createRoot(container)
    renders.count = 0
    let resolveTokens!: (result: TokensResponse) => void
    const response = new Promise<TokensResponse>((resolve) => { resolveTokens = resolve })
    vi.mocked(tokenApi.getGroupTokens).mockReturnValue(response)

    function render() {
        return act(async () => root.render(
            <QueryClientProvider client={client}>
                <GroupTokensTab groupId="empty" />
            </QueryClientProvider>,
        ))
    }

    try {
        await render()
        await render()
        await act(async () => {
            resolveTokens({ tokens: [], total: 0 })
            await vi.advanceTimersByTimeAsync(0)
        })

        expect(tokenApi.getGroupTokens).toHaveBeenCalledExactlyOnceWith('empty', 1, 20, undefined)
        expect(container.textContent).toContain('common.noResult')
        expect(container.querySelector<HTMLButtonElement>('[aria-label="ui.nextPage"]')?.disabled).toBe(true)
    } finally {
        await act(async () => root.unmount())
        client.clear()
        vi.useRealTimers()
    }
})
