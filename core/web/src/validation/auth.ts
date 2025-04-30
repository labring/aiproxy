// src/validation/auth.ts
import { z } from 'zod'

export const loginSchema = z.object({
    token: z
        .string()
        .min(1, { message: '请输入Token' })
        .min(6, { message: 'Token长度不能少于6个字符' })
})

export type LoginFormValues = z.infer<typeof loginSchema>