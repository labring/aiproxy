// @vitest-environment jsdom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import {
    getCoreRowModel,
    useReactTable,
    type ColumnDef,
} from '@tanstack/react-table'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DataTable } from './motion-data-table'

vi.mock('react-i18next', () => ({
    useTranslation: () => ({ t: (key: string) => key }),
}))

const reactTestEnvironment = globalThis as typeof globalThis & {
    IS_REACT_ACT_ENVIRONMENT: boolean
}
reactTestEnvironment.IS_REACT_ACT_ENVIRONMENT = true

type Item = { name: string; detail: string }
const data: Item[] = [{ name: 'example', detail: 'detail' }]
const columns: ColumnDef<Item>[] = [
    { accessorKey: 'name', header: 'Name' },
    {
        accessorKey: 'detail',
        header: 'Detail',
        cell: () => <button type="button">Details</button>,
    },
]

function Fixture({
    rows = data,
    loading = false,
    hideDetail = false,
    onRowClick,
}: {
    rows?: Item[]
    loading?: boolean
    hideDetail?: boolean
    onRowClick?: (row: Item) => void
}) {
    const table = useReactTable({
        data: rows,
        columns,
        getCoreRowModel: getCoreRowModel(),
        state: { columnVisibility: { detail: !hideDetail } },
    })
    return (
        <DataTable
            table={table}
            columns={columns}
            isLoading={loading}
            onRowClick={onRowClick}
        />
    )
}

describe('DataTable', () => {
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

    it('renders rows immediately and retains them during background loading', async () => {
        await act(async () => root.render(<Fixture />))
        expect(container.querySelector('tbody')?.textContent).toContain(
            'example',
        )
        expect(
            container.querySelector('tbody tr')?.getAttribute('style'),
        ).toBeNull()
        await act(async () => root.render(<Fixture loading />))
        expect(container.querySelector('tbody')?.textContent).toContain(
            'example',
        )
        expect(
            container.querySelector('[aria-busy]')?.getAttribute('aria-busy'),
        ).toBe('false')
    })

    it('uses only visible columns for loading and empty states', async () => {
        await act(async () =>
            root.render(<Fixture rows={[]} loading hideDetail />),
        )
        expect(
            container.querySelector('tbody td')?.getAttribute('colspan'),
        ).toBe('1')
        expect(
            container.querySelector('[aria-busy]')?.getAttribute('aria-busy'),
        ).toBe('true')
        await act(async () => root.render(<Fixture rows={[]} hideDetail />))
        expect(
            container.querySelector('tbody td')?.getAttribute('colspan'),
        ).toBe('1')
        expect(container.querySelector('tbody')?.textContent).toContain(
            'common.noResult',
        )
    })

    it('opens a focused row with Enter without intercepting nested controls', async () => {
        const onRowClick = vi.fn()
        await act(async () => root.render(<Fixture onRowClick={onRowClick} />))
        const row = container.querySelector('tbody tr')!
        expect(row.getAttribute('tabindex')).toBe('0')
        await act(async () =>
            row.dispatchEvent(
                new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }),
            ),
        )
        expect(onRowClick).toHaveBeenCalledExactlyOnceWith(data[0])
        await act(async () =>
            row
                .querySelector('button')!
                .dispatchEvent(
                    new KeyboardEvent('keydown', {
                        key: 'Enter',
                        bubbles: true,
                    }),
                ),
        )
        expect(onRowClick).toHaveBeenCalledTimes(1)
    })
})
