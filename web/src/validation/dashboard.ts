import { z } from 'zod'

export const dashboardFiltersSchema = z.object({
    key: z.string().optional(),
    model: z.string().optional(),
    dateRange: z.object({
        from: z.date().optional(),
        to: z.date().optional(),
    }).optional(),
    timespan: z.enum(['day', 'hour']).default('day'),
})

export type DashboardFiltersForm = z.infer<typeof dashboardFiltersSchema> 