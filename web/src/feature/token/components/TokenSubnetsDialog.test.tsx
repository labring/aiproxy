// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { tokenApi } from '@/api/token'
import { TokenSubnetsDialog } from './TokenSubnetsDialog'

vi.mock('react-i18next', () => ({
    useTranslation: () => ({
        t: (key: string, opts?: { invalid?: string }) =>
            key === 'token.ipLimitDialog.invalid'
                ? `Invalid CIDR: ${opts?.invalid ?? ''}`
                : key,
    }),
}))
vi.mock('@/api/token', () => ({ tokenApi: { updateToken: vi.fn() } }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('TokenSubnetsDialog', () => {
    let root: Root
    let container: HTMLDivElement
    let client: QueryClient
    const onClose = vi.fn()
    const token = { id: 7, subnets: ['10.0.0.0/8'] }
    const tokensQuery = ['tokens', 1]
    const groupTokensQuery = ['groupTokens', 'example-group']

    beforeEach(() => {
        vi.useFakeTimers()
        vi.clearAllMocks()
        container = document.createElement('div')
        document.body.append(container)
        root = createRoot(container)
        client = new QueryClient()
        client.setQueryData(tokensQuery, { tokens: [token] })
        client.setQueryData(groupTokensQuery, { tokens: [token] })
    })
    afterEach(async () => {
        await act(async () => root.unmount())
        client.clear()
        container.remove()
        vi.useRealTimers()
    })

    async function render(open = true) {
        await act(async () => root.render(<QueryClientProvider client={client}>
            {open && <TokenSubnetsDialog onClose={onClose} token={token} />}
        </QueryClientProvider>))
    }
    async function changeText(value: string) {
        const textarea = document.querySelector<HTMLTextAreaElement>('[role="dialog"] textarea')!
        await act(async () => {
            Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(textarea, value)
            textarea.dispatchEvent(new Event('input', { bubbles: true }))
        })
    }
    async function submit() {
        await act(async () => {
            document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
        })
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    }

    it('submits the parsed subnets and refreshes both token lists', async () => {
        vi.mocked(tokenApi.updateToken, { partial: true }).mockResolvedValueOnce({ id: token.id })
        await render()
        await changeText('10.0.0.0/8, 2001:db8::/32')
        await submit()
        expect(tokenApi.updateToken).toHaveBeenCalledExactlyOnceWith(7, { subnets: ['10.0.0.0/8', '2001:db8::/32'] })
        expect(onClose).toHaveBeenCalledOnce()
        expect(client.getQueryState(tokensQuery)?.isInvalidated).toBe(true)
        expect(client.getQueryState(groupTokensQuery)?.isInvalidated).toBe(true)
    })

    it('clearing the field removes the restriction', async () => {
        vi.mocked(tokenApi.updateToken, { partial: true }).mockResolvedValueOnce({ id: token.id })
        await render()
        await changeText('')
        await submit()
        expect(tokenApi.updateToken).toHaveBeenCalledExactlyOnceWith(7, { subnets: [] })
    })

    it('rejects invalid CIDR values before submitting', async () => {
        await render()
        await changeText('10.0.0.0/8, 999.1.2.3/24')
        expect(document.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(true)
        expect(document.querySelector('[role="alert"]')?.textContent).toContain('999.1.2.3/24')
        await submit()
        expect(tokenApi.updateToken).not.toHaveBeenCalled()
        await changeText('10.0.0.0/8')
        expect(document.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(false)
    })

    it('keeps the dialog open and displays a server error', async () => {
        vi.mocked(tokenApi.updateToken).mockRejectedValue(new Error('invalid subnet: failed to parse subnet: 10.0.0.0/8'))
        await render()
        await submit()
        expect(document.querySelector('[role="alert"]')?.textContent).toContain('failed to parse subnet')
        expect(onClose).not.toHaveBeenCalled()
    })

    it('resets unsaved input when the dialog is reopened', async () => {
        await render()
        await changeText('192.168.0.0/16')
        await render(false)
        await render()
        expect(document.querySelector<HTMLTextAreaElement>('[role="dialog"] textarea')?.value).toBe('10.0.0.0/8')
    })
})
