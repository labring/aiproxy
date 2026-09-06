// @vitest-environment jsdom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ProxyConfig from './ProxyConfig'
import OpenAPIConfig from './OpenAPIConfig'
import EmbedMCP from '../EmbedMCP'
import { getEmbedMCPs, saveEmbedMCP } from '@/api/mcp'

vi.mock('react-i18next', () => ({
    useTranslation: () => ({
        t: (key: string) => key,
        i18n: { language: 'en' },
    }),
}))
vi.mock('@/components/ui/use-toast', () => ({
    useToast: () => ({ toast: vi.fn() }),
}))
vi.mock('@/api/mcp', () => ({
    getEmbedMCPs: vi.fn(),
    saveEmbedMCP: vi.fn().mockResolvedValue(undefined),
}))

const testEnvironment = globalThis as typeof globalThis & {
    IS_REACT_ACT_ENVIRONMENT: boolean
}
testEnvironment.IS_REACT_ACT_ENVIRONMENT = true

describe('MCP configuration state', () => {
    let container: HTMLDivElement
    let root: Root

    beforeEach(() => {
        vi.clearAllMocks()
        container = document.createElement('div')
        document.body.appendChild(container)
        root = createRoot(container)
    })

    afterEach(async () => {
        await act(async () => root.unmount())
        container.remove()
    })

    it('restores proxy values when the parent resets the form', async () => {
        const initial = {
            url: 'https://example.com',
            headers: { existing: 'value' },
            querys: {},
            reusing: {},
        }
        await act(async () =>
            root.render(<ProxyConfig config={initial} onChange={vi.fn()} />),
        )
        expect(container.querySelector<HTMLInputElement>('#url')?.value).toBe(
            initial.url,
        )
        await act(async () =>
            root.render(<ProxyConfig config={undefined} onChange={vi.fn()} />),
        )
        expect(container.querySelector<HTMLInputElement>('#url')?.value).toBe(
            '',
        )
        expect(container.textContent).not.toContain('existing')
    })

    it('restores OpenAPI values when the parent resets the form', async () => {
        await act(async () =>
            root.render(
                <OpenAPIConfig
                    config={{
                        openapi_spec: 'https://example.com/schema.json',
                        v2: true,
                    }}
                    onChange={vi.fn()}
                />,
            ),
        )
        expect(
            container.querySelector<HTMLInputElement>('#openapi_spec')?.value,
        ).toContain('schema.json')
        await act(async () =>
            root.render(
                <OpenAPIConfig config={undefined} onChange={vi.fn()} />,
            ),
        )
        expect(
            container.querySelector<HTMLInputElement>('#openapi_spec')?.value,
        ).toBe('')
        expect(
            container.querySelector('#v2')?.getAttribute('aria-checked'),
        ).toBe('false')
    })

    it('preserves saved empty and extra embedded configuration values when saving', async () => {
        vi.mocked(getEmbedMCPs).mockResolvedValue([
            {
                id: 'fixture',
                name: 'Fixture',
                enabled: false,
                readme: '',
                tags: [],
                config_templates: {
                    timeout: {
                        name: 'Timeout',
                        required: false,
                        example: '30',
                        description: '',
                    },
                },
                embed_config: {
                    init: { timeout: '', extra: 'retained' },
                    reusing: {},
                },
            },
        ])
        await act(async () => root.render(<EmbedMCP />))
        const save = Array.from(container.querySelectorAll('button')).find(
            (button) => button.textContent?.includes('mcp.config.submit'),
        )!
        await act(async () => save.click())
        expect(saveEmbedMCP).toHaveBeenCalledExactlyOnceWith({
            id: 'fixture',
            enabled: false,
            init_config: { timeout: '', extra: 'retained' },
        })
    })
})
