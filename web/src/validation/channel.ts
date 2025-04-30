// src/validation/channel.ts
import { z } from 'zod'

export const channelCreateSchema = z.object({
    type: z.number().min(1, '厂商不能为空'),
    name: z.string().min(1, '名称不能为空'),
    key: z.string().min(1, '密钥不能为空'),
    base_url: z.string().min(1, '代理地址不能为空'),
    models: z.array(z.string()).min(1, '至少选择一个模型'),
    model_mapping: z.record(z.string(), z.string()).optional()
})

export type ChannelCreateForm = z.infer<typeof channelCreateSchema>