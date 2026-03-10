// src/validation/token.ts
import { z } from 'zod'

export const tokenCreateSchema = z.object({
    name: z.string()
        .min(1, '名称不能为空')
        .regex(/^[a-zA-Z0-9_]+$/, '名称只能包含字母、数字和下划线')
        .max(20, '名称长度不能超过20个字符'),
    quota: z.number().min(0).optional(),
    period_quota: z.number().min(0).optional(),
    period_type: z.string().optional().nullable(),
})

export const tokenUpdateSchema = z.object({
    name: z.string()
        .min(1, '名称不能为空')
        .regex(/^[a-zA-Z0-9_]+$/, '名称只能包含字母、数字和下划线')
        .max(20, '名称长度不能超过20个字符')
        .optional(),
    quota: z.number().min(0).optional(),
    period_quota: z.number().min(0).optional(),
    period_type: z.string().optional().nullable(),
})

export type TokenCreateForm = z.infer<typeof tokenCreateSchema>
export type TokenUpdateForm = z.infer<typeof tokenUpdateSchema>