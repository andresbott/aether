import { describe, it, expect } from 'vitest'
import { formatDuration } from '@/utils/formatDuration'

describe('formatDuration', () => {
    it('formats under an hour as m:ss', () => {
        expect(formatDuration(0)).toBe('')
        expect(formatDuration(65)).toBe('1:05')
        expect(formatDuration(600)).toBe('10:00')
    })
    it('formats an hour or more as h:mm:ss', () => {
        expect(formatDuration(3661)).toBe('1:01:01')
        expect(formatDuration(7325)).toBe('2:02:05')
    })
    it('returns empty for missing or negative input', () => {
        expect(formatDuration(undefined)).toBe('')
        expect(formatDuration(-5)).toBe('')
    })
})
