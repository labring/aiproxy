// @vitest-environment jsdom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ChannelLabel } from './ChannelLabel'

vi.mock('react-i18next', () => ({
    useTranslation: () => ({ t: (key: string) => key }),
}))

const reactTestEnvironment = globalThis as typeof globalThis & {
    IS_REACT_ACT_ENVIRONMENT: boolean
}
reactTestEnvironment.IS_REACT_ACT_ENVIRONMENT = true

const channel = { name: 'Example channel', remark: 'Example remark', type: 1 }
const disabledSelector = '[aria-label="channel.currentlyDisabled"]'

describe.each([false, true])('ChannelLabel (compact: %s)', compact => {
    let container: HTMLDivElement
    let root: Root

    beforeEach(() => {
        container = document.createElement('div')
        document.body.appendChild(container)
        root = createRoot(container)
    })

    afterEach(async () => {
        await act(async () => root.unmount())
        container.remove()
    })

    it('shows disabled and backup-only states together while preserving identity', async () => {
        await act(async () => root.render(
            <ChannelLabel id={7} info={{ ...channel, status: 2, backup_only: true }} compact={compact} />,
        ))
        expect(container.textContent).toContain(channel.name)
        expect(container.textContent).toContain('#7')
        expect(container.querySelector('[title]')?.getAttribute('title')).toContain(channel.remark)
        expect(container.querySelector(disabledSelector)).not.toBeNull()
        if (compact) {
            expect(container.querySelector('[aria-label="channel.dialog.backupOnly"]')).not.toBeNull()
        } else {
            expect(container.textContent).toContain('channel.disabled')
            expect(container.textContent).toContain('channel.dialog.backupOnly')
        }

        await act(async () => root.render(
            <ChannelLabel id={7} info={{ ...channel, status: 1, backup_only: true }} compact={compact} />,
        ))
        expect(container.querySelector(disabledSelector)).toBeNull()
    })

    it.each([0, 1, undefined])('does not mark status %s as disabled', async status => {
        await act(async () => root.render(
            <ChannelLabel id={7} info={{ ...channel, status }} compact={compact} />,
        ))
        expect(container.querySelector(disabledSelector)).toBeNull()
    })

    it('retains the ID fallback without guessing an unavailable status', async () => {
        await act(async () => root.render(<ChannelLabel id={7} compact={compact} />))
        expect(container.textContent).toBe('#7')
        expect(container.querySelector(disabledSelector)).toBeNull()
    })
})
