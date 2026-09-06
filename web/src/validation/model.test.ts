import { describe, expect, it } from 'vitest'

import { priceSchema, retryBudgetSchema } from './model'

describe('retry budget validation', () => {
    it.each([0, 1, 60, 180])('accepts %s seconds', value => {
        expect(retryBudgetSchema.safeParse(value).success).toBe(true)
    })
    it.each([-1, 181, 1.5, Infinity, NaN])('rejects %s seconds', value => {
        expect(retryBudgetSchema.safeParse(value).success).toBe(false)
    })
})

describe('daily conditional pricing validation', () => {
    it('accepts multiple non-overlapping daily peak ranges', () => {
        const result = priceSchema.safeParse({
            conditional_prices: [
                {
                    condition: {
                        daily_start_time: '09:00',
                        daily_end_time: '12:00',
                        timezone: 'Asia/Shanghai',
                    },
                    price: { input_price: 2 },
                },
                {
                    condition: {
                        daily_start_time: '14:00',
                        daily_end_time: '18:00',
                        timezone: 'Asia/Shanghai',
                    },
                    price: { input_price: 2 },
                },
            ],
        })

        expect(result.success).toBe(true)
    })

    it('accepts a range that crosses midnight', () => {
        const result = priceSchema.safeParse({
            conditional_prices: [{
                condition: {
                    daily_start_time: '22:00',
                    daily_end_time: '06:00',
                    timezone: 'Asia/Shanghai',
                },
                price: { input_price: 2 },
            }],
        })

        expect(result.success).toBe(true)
    })

    it('accepts combined absolute and daily time ranges', () => {
        const result = priceSchema.safeParse({
            conditional_prices: [{
                condition: {
                    start_time: 1782835200,
                    end_time: 1785513599,
                    daily_start_time: '09:00',
                    daily_end_time: '12:00',
                    timezone: 'Asia/Shanghai',
                },
                price: { input_price: 2 },
            }],
        })

        expect(result.success).toBe(true)
    })

    it.each([
        {
            daily_start_time: '09:00',
            timezone: 'Asia/Shanghai',
        },
        {
            daily_start_time: '09:00',
            daily_end_time: '09:00',
            timezone: 'Asia/Shanghai',
        },
        {
            daily_start_time: '9:00',
            daily_end_time: '12:00',
            timezone: 'Asia/Shanghai',
        },
        {
            daily_start_time: '09:00',
            daily_end_time: '12:00',
            timezone: 'Mars/Olympus',
        },
    ])('rejects an invalid daily range: %j', (condition) => {
        const result = priceSchema.safeParse({
            conditional_prices: [{ condition, price: { input_price: 2 } }],
        })

        expect(result.success).toBe(false)
    })

    it('rejects an invalid absolute time range', () => {
        const result = priceSchema.safeParse({
            conditional_prices: [{
                condition: { start_time: 200, end_time: 100 },
                price: { input_price: 2 },
            }],
        })

        expect(result.success).toBe(false)
    })
})
