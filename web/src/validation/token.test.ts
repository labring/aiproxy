import { describe, expect, it } from 'vitest'
import { parseSubnets } from './token'

describe('parseSubnets', () => {
    it('splits comma, space and mixed separators', () => {
        expect(parseSubnets('10.0.0.0/8, 192.168.1.0/24')).toEqual({
            subnets: ['10.0.0.0/8', '192.168.1.0/24'],
        })
        expect(parseSubnets('10.0.0.0/8\n192.168.1.0/24')).toEqual({
            subnets: ['10.0.0.0/8', '192.168.1.0/24'],
        })
    })

    it('treats blank input as no restriction', () => {
        expect(parseSubnets('')).toEqual({ subnets: [] })
        expect(parseSubnets('  ,  ')).toEqual({ subnets: [] })
    })

    it('accepts IPv4 and IPv6 CIDRs', () => {
        expect(parseSubnets('0.0.0.0/0')).toEqual({ subnets: ['0.0.0.0/0'] })
        expect(parseSubnets('2001:db8::/32')).toEqual({ subnets: ['2001:db8::/32'] })
        expect(parseSubnets('fd00::/8')).toEqual({ subnets: ['fd00::/8'] })
    })

    it('rejects invalid entries', () => {
        expect(parseSubnets('10.0.0.0/8 1.2.3.4')).toEqual({ subnets: [], invalid: '1.2.3.4' })
        expect(parseSubnets('10.0.0.0/33')).toEqual({ subnets: [], invalid: '10.0.0.0/33' })
        expect(parseSubnets('999.1.2.3/24')).toEqual({ subnets: [], invalid: '999.1.2.3/24' })
        expect(parseSubnets('2001:db8::/129')).toEqual({ subnets: [], invalid: '2001:db8::/129' })
    })
})
