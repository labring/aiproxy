// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { tokenApi } from '@/api/token'
import { RotateTokenDialog } from './RotateTokenDialog'

vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/token', () => ({
    tokenApi: { rotateToken: vi.fn(), rotateGroupToken: vi.fn() },
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('RotateTokenDialog', () => {
    let root: Root
    let container: HTMLDivElement
    let client: QueryClient
    const onClose = vi.fn()
    const token = { id: 7, name: 'api-key', key: 'plain-key', key_preview: 'plain…-key', expired_at: 0 }
    const tokensQuery = ['tokens', 1]
    const groupTokensQuery = ['groupTokens', 'example-group']

    beforeEach(() => {
        vi.clearAllMocks()
        container = document.createElement('div')
        document.body.append(container)
        root = createRoot(container)
        client = new QueryClient()
        client.setQueryData(tokensQuery, { tokens: [] })
        client.setQueryData(groupTokensQuery, { tokens: [] })
    })
    afterEach(async () => {
        await act(async () => root.unmount())
        client.clear()
        container.remove()
    })

    async function render(props: { group?: string } = {}) {
        await act(async () => root.render(<QueryClientProvider client={client}>
            <RotateTokenDialog token={token} onClose={onClose} group={props.group} />
        </QueryClientProvider>))
    }
    async function rotate() {
        await act(async () => {
            document.querySelectorAll('button').forEach((button) => {
                if (button.textContent === 'token.rotateDialog.submit') button.click()
            })
        })
    }

    it('rotates with keep mode by default and shows the new key once', async () => {
        const newKey = 'x'.repeat(48)
        vi.mocked(tokenApi.rotateToken, { partial: true }).mockResolvedValueOnce({
            ...token, key: newKey, key_preview: 'xxxxx…xxxx',
        })
        await render()
        await rotate()

        expect(tokenApi.rotateToken).toHaveBeenCalledExactlyOnceWith(token.id, '', undefined)
        expect(client.getQueryState(tokensQuery)?.isInvalidated).toBe(true)
        expect(client.getQueryState(groupTokensQuery)?.isInvalidated).toBe(true)
        expect(document.body.textContent).toContain(newKey)
        expect(onClose).not.toHaveBeenCalled()
    })

    it('sends the picked expiry and can clear it with the never-expires switch', async () => {
        vi.mocked(tokenApi.rotateToken, { partial: true }).mockResolvedValue({ ...token, key: 'k' })
        await render()

        const input = document.querySelector<HTMLInputElement>('[type="datetime-local"]')!
        await act(async () => {
            Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, '2030-01-02T03:04')
            input.dispatchEvent(new Event('input', { bubbles: true }))
            input.dispatchEvent(new Event('change', { bubbles: true }))
        })
        await rotate()
        expect(tokenApi.rotateToken).toHaveBeenCalledExactlyOnceWith(
            token.id, '', new Date('2030-01-02T03:04').getTime(),
        )

        await act(async () => {
            document.querySelectorAll<HTMLElement>('[role="switch"]').forEach((toggle) => toggle.click())
        })
        await rotate()
        expect(tokenApi.rotateToken).toHaveBeenLastCalledWith(token.id, '', 0)
    })

    it('closes only after the one-time key dialog is acknowledged', async () => {
        vi.mocked(tokenApi.rotateToken, { partial: true }).mockResolvedValueOnce({
            ...token, key: 'new-key',
        })
        await render()
        await rotate()

        await act(async () => {
            document.querySelectorAll('button').forEach((button) => {
                if (button.textContent === 'token.oneTimeKey.acknowledge') button.click()
            })
        })
        expect(onClose).toHaveBeenCalledOnce()
    })

    it('uses the group endpoint when a group is given and stays open on failure', async () => {
        vi.mocked(tokenApi.rotateGroupToken, { partial: true }).mockRejectedValueOnce(
            new Error('boom'),
        )
        await render({ group: 'example-group' })
        await rotate()

        expect(tokenApi.rotateGroupToken).toHaveBeenCalledExactlyOnceWith('example-group', token.id, '', undefined)
        expect(onClose).not.toHaveBeenCalled()
    })
})
