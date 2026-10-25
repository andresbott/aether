import { describe, it, expect } from 'vitest'
import { formatDuration, formatTotalDuration } from '@/utils/formatDuration'

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

describe('formatTotalDuration', () => {
    it('reads a length in minutes, dropping the seconds', () => {
        expect(formatTotalDuration(125)).toBe('2 min')
        expect(formatTotalDuration(2615)).toBe('43 min')
    })
    it('adds the hours from an hour on, and drops a zero minute count', () => {
        expect(formatTotalDuration(7807)).toBe('2 hr 10 min')
        expect(formatTotalDuration(3600)).toBe('1 hr')
    })
    it('reads under a minute in seconds rather than as zero minutes', () => {
        expect(formatTotalDuration(32)).toBe('32 sec')
    })
    it('returns empty for missing, zero or negative input', () => {
        expect(formatTotalDuration(undefined)).toBe('')
        expect(formatTotalDuration(0)).toBe('')
        expect(formatTotalDuration(-5)).toBe('')
    })
})
