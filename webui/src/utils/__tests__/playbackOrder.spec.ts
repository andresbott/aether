import { describe, it, expect } from 'vitest'
import { nextQueueIndex } from '@/utils/playbackOrder'

describe('nextQueueIndex', () => {
    it('returns the following index within the queue', () => {
        expect(nextQueueIndex(0, 3, 'none')).toBe(1)
        expect(nextQueueIndex(1, 3, 'none')).toBe(2)
    })

    it('returns null past the last track when repeat is none', () => {
        expect(nextQueueIndex(2, 3, 'none')).toBeNull()
    })

    it('wraps to the first track past the end when repeat is all', () => {
        expect(nextQueueIndex(2, 3, 'all')).toBe(0)
    })

    it('returns null for an empty queue', () => {
        expect(nextQueueIndex(0, 0, 'none')).toBeNull()
        expect(nextQueueIndex(0, 0, 'all')).toBeNull()
    })
})
