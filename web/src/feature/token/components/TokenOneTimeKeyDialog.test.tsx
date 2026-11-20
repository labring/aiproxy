// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TokenOneTimeKeyDialog } from './TokenOneTimeKeyDialog'

vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
const writeTextMock = vi.hoisted(() => vi.fn())
vi.mock('@/lib/clipboard', () => ({ writeTextToClipboard: writeTextMock }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('TokenOneTimeKeyDialog', () => {
    let root: Root
    let container: HTMLDivElement
    const onOpenChange = vi.fn()
    const tokenKey = 'a'.repeat(48)

    beforeEach(() => {
        vi.clearAllMocks()
        container = document.createElement('div')
        document.body.append(container)
        root = createRoot(container)
    })
    afterEach(async () => {
        await act(async () => root.unmount())
        container.remove()
    })

    async function render() {
        await act(async () => root.render(
            <TokenOneTimeKeyDialog open onOpenChange={onOpenChange} tokenKey={tokenKey} />,
        ))
    }

    it('shows the raw key and closes on acknowledge', async () => {
        await render()
        expect(document.body.textContent).toContain(tokenKey)

        await act(async () => {
            document.querySelectorAll('button').forEach((button) => {
                if (button.textContent === 'token.oneTimeKey.acknowledge') button.click()
            })
        })
        expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false)
    })

    it('copies the key on demand', async () => {
        writeTextMock.mockResolvedValueOnce(undefined)
        await render()
        await act(async () => {
            document.querySelectorAll('button').forEach((button) => {
                if (button.textContent === 'token.copyKey') button.click()
            })
        })
        expect(writeTextMock).toHaveBeenCalledExactlyOnceWith(tokenKey)
        expect(onOpenChange).not.toHaveBeenCalled()
    })
})
