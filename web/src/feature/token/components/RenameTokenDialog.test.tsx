// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { tokenApi } from '@/api/token'
import { RenameTokenDialog } from './RenameTokenDialog'

vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/token', () => ({ tokenApi: { updateToken: vi.fn() } }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('RenameTokenDialog', () => {
    let root: Root
    let container: HTMLDivElement
    let client: QueryClient
    const onClose = vi.fn()
    const token = { id: 7, name: '原名称' }
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
            {open && <RenameTokenDialog onClose={onClose} token={token} />}
        </QueryClientProvider>))
    }
    async function changeName(name: string) {
        const input = document.querySelector<HTMLInputElement>('[role="dialog"] input')!
        await act(async () => {
            Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, name)
            input.dispatchEvent(new Event('input', { bubbles: true }))
        })
    }
    async function submit() {
        await act(async () => {
            document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
        })
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    }

    it('submits only the trimmed name and refreshes both token lists', async () => {
        vi.mocked(tokenApi.updateToken, { partial: true }).mockResolvedValueOnce({ id: token.id, name: '新名称-测试' })
        await render()
        await changeName(' 新名称-测试 ')
        await submit()
        expect(tokenApi.updateToken).toHaveBeenCalledExactlyOnceWith(7, { name: '新名称-测试' })
        expect(onClose).toHaveBeenCalledOnce()
        expect(client.getQueryState(tokensQuery)?.isInvalidated).toBe(true)
        expect(client.getQueryState(groupTokensQuery)?.isInvalidated).toBe(true)
    })

    it('validates blank names and the backend UTF-8 byte limit', async () => {
        await render()
        await changeName('   ')
        expect(document.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(true)
        await submit()
        await changeName('中'.repeat(10) + 'abc')
        expect(document.querySelector('[role="alert"]')?.textContent).toBe('token.renameDialog.nameTooLong')
        await submit()
        expect(tokenApi.updateToken).not.toHaveBeenCalled()
        await changeName('中'.repeat(10) + 'ab')
        expect(document.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(false)
    })

    it('keeps the dialog open and displays a server error for duplicate names', async () => {
        vi.mocked(tokenApi.updateToken).mockRejectedValue(new Error('token name already exists in this group'))
        await render()
        await changeName('duplicate')
        await submit()
        expect(document.querySelector('[role="alert"]')?.textContent).toContain('already exists')
        expect(onClose).not.toHaveBeenCalled()
    })

    it('resets an unsaved name when the dialog is reopened', async () => {
        await render()
        await changeName('unsaved')
        await render(false)
        await render()
        expect(document.querySelector<HTMLInputElement>('[role="dialog"] input')?.value).toBe(token.name)
    })
})
