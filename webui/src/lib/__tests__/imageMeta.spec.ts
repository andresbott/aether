import { describe, it, expect } from 'vitest'
import { formatBytes, formatImageMeta } from '@/lib/imageMeta'

describe('formatBytes', () => {
    it('shows raw bytes under 1 KB', () => {
        expect(formatBytes(512)).toBe('512 B')
    })
    it('shows whole KB without a trailing decimal', () => {
        expect(formatBytes(245 * 1024)).toBe('245 KB')
    })
    it('shows one decimal for fractional KB', () => {
        expect(formatBytes(1536)).toBe('1.5 KB')
    })
    it('rolls over to MB', () => {
        expect(formatBytes(3 * 1024 * 1024)).toBe('3 MB')
    })
})

describe('formatImageMeta', () => {
    it('joins dimensions, format and size', () => {
        expect(
            formatImageMeta({ width: 1000, height: 1000, format: 'jpeg', bytes: 245 * 1024 })
        ).toBe('1000 × 1000 · JPEG · 245 KB')
    })
    it('omits dimensions and format when unknown, keeping the size', () => {
        expect(formatImageMeta({ width: 0, height: 0, format: '', bytes: 512 })).toBe('512 B')
    })
    it('uppercases the format', () => {
        expect(formatImageMeta({ width: 800, height: 600, format: 'png', bytes: 1536 })).toBe(
            '800 × 600 · PNG · 1.5 KB'
        )
    })
})
