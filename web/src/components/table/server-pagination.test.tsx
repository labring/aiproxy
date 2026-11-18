// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { ServerPagination } from './server-pagination'

vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

it('corrects the current page only when the server total makes it out of range', async () => {
    const container = document.createElement('div')
    const root = createRoot(container)
    const onPageChange = vi.fn()
    const onPageSizeChange = vi.fn()

    async function render(total: number | undefined, page = 3) {
        await act(async () => root.render(
            <ServerPagination
                total={total}
                page={page}
                pageSize={20}
                onPageChange={onPageChange}
                onPageSizeChange={onPageSizeChange}
            />,
        ))
    }

    try {
        await render(undefined)
        await render(45)
        expect(onPageChange).not.toHaveBeenCalled()

        await render(21)
        expect(onPageChange).toHaveBeenCalledExactlyOnceWith(2)

        onPageChange.mockClear()
        await render(0, 2)
        expect(onPageChange).toHaveBeenCalledExactlyOnceWith(1)

        await render(0, 1)
        expect(onPageChange).toHaveBeenCalledTimes(1)
        expect(container.querySelector<HTMLButtonElement>('[aria-label="ui.nextPage"]')?.disabled).toBe(true)
        expect(container.querySelector<HTMLButtonElement>('[aria-label="ui.previousPage"]')?.disabled).toBe(true)
    } finally {
        await act(async () => root.unmount())
    }
})
