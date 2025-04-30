// src/validation/model.ts
import { z } from 'zod'

export const modelCreateSchema = z.object({
    model: z.string().min(1, 'Model name is required'),
    type: z.number().min(0, 'Type is required'),
})

export type ModelCreateForm = z.infer<typeof modelCreateSchema>