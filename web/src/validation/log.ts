import { z } from 'zod'

// 日志过滤器验证schema
export const logFilterSchema = z.object({
    keyName: z.string().optional(),
    model: z.string().optional(),
    dateRange: z.object({
        from: z.date().optional(),
        to: z.date().optional()
    }).optional(),
    code_type: z.enum(['all', 'success', 'error']).default('all'),
    page: z.number().min(1).default(1),
    per_page: z.number().min(1).max(100).default(10)
})

export type LogFilterForm = z.infer<typeof logFilterSchema> 