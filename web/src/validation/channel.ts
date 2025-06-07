// src/validation/channel.ts
import { z } from 'zod'

export const channelCreateSchema = z.object({
    type: z.number().min(1, '厂商不能为空'),
    name: z.string().min(1, '名称不能为空'),
    key: z.string().min(1, '密钥不能为空'),
    base_url: z.string().optional(),
    models: z.array(z.string()).min(1, '至少选择一个模型'),
    model_mapping: z.record(z.string(), z.string()).optional(),
    sets: z.array(z.string()).optional()
})

export type ChannelCreateForm = z.infer<typeof channelCreateSchema>