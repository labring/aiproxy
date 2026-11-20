// src/validation/token.ts
import { z } from 'zod'

const IPV4_CIDR = /^((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)\.){3}(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)\/(3[0-2]|[12]?\d)$/
const IPV6_CIDR = /^[0-9a-fA-F:]+\/(12[0-8]|1[01]\d|[1-9]?\d)$/

const isCIDR = (value: string): boolean => IPV4_CIDR.test(value) || IPV6_CIDR.test(value)

// Splits comma/space separated CIDRs; empty input means no restriction.
export const parseSubnets = (input: string): { subnets: string[]; invalid?: string } => {
    const subnets = input.split(/[\s,]+/).map(s => s.trim()).filter(Boolean)
    const invalid = subnets.find(s => !isCIDR(s))
    if (invalid !== undefined) {
        return { subnets: [], invalid }
    }
    return { subnets }
}

export const tokenCreateSchema = z.object({
    name: z.string()
        .min(1, '名称不能为空')
        .regex(/^[a-zA-Z0-9_]+$/, '名称只能包含字母、数字和下划线')
        .max(20, '名称长度不能超过20个字符'),
    hashed_key: z.boolean().optional(),
    subnets: z.string()
        .refine(v => parseSubnets(v).invalid === undefined, '无效的 CIDR 格式')
        .optional(),
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
