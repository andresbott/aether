import { describe, it, expect } from 'vitest'
import { formatRelativeDate } from '@/utils/formatRelativeDate'

// Noon on both sides, so the calendar-day difference holds in every time zone.
const now = new Date('2026-10-18T12:00:00Z')
const daysBefore = (days: number) => new Date(now.getTime() - days * 86_400_000).toISOString()

describe('formatRelativeDate', () => {
    it('names today and yesterday', () => {
        expect(formatRelativeDate(daysBefore(0), now)).toBe('today')
        expect(formatRelativeDate(daysBefore(1), now)).toBe('yesterday')
    })

    it('counts days for two weeks, then weeks, months and years', () => {
        expect(formatRelativeDate(daysBefore(3), now)).toBe('3 days ago')
        expect(formatRelativeDate(daysBefore(13), now)).toBe('13 days ago')
        expect(formatRelativeDate(daysBefore(14), now)).toBe('2 weeks ago')
        expect(formatRelativeDate(daysBefore(60), now)).toBe('8 weeks ago')
        expect(formatRelativeDate(daysBefore(61), now)).toBe('2 months ago')
        expect(formatRelativeDate(daysBefore(364), now)).toBe('11 months ago')
        expect(formatRelativeDate(daysBefore(365), now)).toBe('a year ago')
        expect(formatRelativeDate(daysBefore(800), now)).toBe('2 years ago')
    })

    it('returns empty for a missing or unparseable date, and today for a future one', () => {
        expect(formatRelativeDate(undefined, now)).toBe('')
        expect(formatRelativeDate('not a date', now)).toBe('')
        expect(formatRelativeDate(daysBefore(-2), now)).toBe('today')
    })
})
