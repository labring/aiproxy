// @vitest-environment jsdom

import { act, useState, type PropsWithChildren } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { ChannelConfigEditor } from './ChannelConfigEditor'
import type { ChannelTypeMeta } from '@/types/channel'

vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
vi.mock('@/components/ui/select', () => ({
    Select: ({ value, onValueChange, children }: PropsWithChildren<{ value: string; onValueChange: (value: string) => void }>) => (
        <select value={value} onChange={event => onValueChange(event.target.value)}>{children}</select>
    ),
    SelectTrigger: () => null,
    SelectValue: () => null,
    SelectContent: ({ children }: PropsWithChildren) => <>{children}</>,
    SelectItem: ({ value, children }: PropsWithChildren<{ value: string }>) => <option value={value}>{children}</option>,
}))

const testEnvironment = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }
testEnvironment.IS_REACT_ACT_ENVIRONMENT = true

it('adds, changes and removes model protocol overrides while retaining unrelated configuration', async () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    let saved = ''
    const meta: ChannelTypeMeta = {
        name: 'Aliyun', keyHelp: '', defaultBaseUrl: '',
        configSchema: { type: 'object', properties: {
            stt_model_protocols: { type: 'object', title: 'STT overrides', additionalProperties: {
                type: 'string', enum: ['auto', 'dashscope_realtime', 'openai_chat_audio'],
            } },
        } },
    }
    function Harness() {
        const [value, setValue] = useState(JSON.stringify({
            stt_model_protocols: { existing: 'dashscope_realtime' }, custom: { retain: true },
        }))
        return <ChannelConfigEditor value={value} meta={meta} onChange={next => { saved = next; setValue(next) }} />
    }
    try {
        await act(async () => root.render(<Harness />))
        const input = container.querySelector<HTMLInputElement>('[aria-label="channel.dialog.models"]')!
        const setName = async (name: string) => act(async () => {
            Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, name)
            input.dispatchEvent(new Event('input', { bubbles: true }))
        })
        const add = () => Array.from(container.querySelectorAll('button')).find(button => button.textContent === 'channel.defaultModels.add')!
        await setName('existing')
        expect(add().disabled).toBe(true)
        await setName('qwen-alias')
        await act(async () => {
            const select = Array.from(container.querySelectorAll('select')).at(-1)!
            select.value = 'openai_chat_audio'
            select.dispatchEvent(new Event('change', { bubbles: true }))
        })
        await act(async () => add().click())
        expect(JSON.parse(saved)).toEqual({
            custom: { retain: true }, stt_model_protocols: { existing: 'dashscope_realtime', 'qwen-alias': 'openai_chat_audio' },
        })
        await act(async () => {
            const select = container.querySelector('select')!
            select.value = 'auto'
            select.dispatchEvent(new Event('change', { bubbles: true }))
        })
        expect(JSON.parse(saved).stt_model_protocols.existing).toBe('auto')
        await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="common.delete existing"]')!.click())
        expect(JSON.parse(saved)).toEqual({ custom: { retain: true }, stt_model_protocols: { 'qwen-alias': 'openai_chat_audio' } })
    } finally {
        await act(async () => root.unmount())
        container.remove()
    }
})
