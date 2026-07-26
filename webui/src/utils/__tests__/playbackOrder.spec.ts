import { describe, it, expect } from 'vitest'
import {
    nextQueueIndex,
    shuffleItems,
    buildShuffleOrder,
    resyncShuffleOrder,
    stepOrderPosition
} from '@/utils/playbackOrder'

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

// Always drawing slot 0 makes Fisher-Yates rotate the list — a deterministic
// order that still proves a reorder happened.
const first = () => 0

describe('shuffleItems', () => {
    it('keeps every element exactly once', () => {
        const out = shuffleItems(['a', 'b', 'c', 'd'], first)
        expect([...out].sort()).toEqual(['a', 'b', 'c', 'd'])
    })

    it('reorders the list', () => {
        expect(shuffleItems(['a', 'b', 'c', 'd'], first)).toEqual(['b', 'c', 'd', 'a'])
    })

    it('does not mutate the input', () => {
        const input = ['a', 'b', 'c']
        shuffleItems(input, first)
        expect(input).toEqual(['a', 'b', 'c'])
    })

    it('handles empty and single-item lists', () => {
        expect(shuffleItems([], first)).toEqual([])
        expect(shuffleItems(['a'], first)).toEqual(['a'])
    })
})

describe('buildShuffleOrder', () => {
    it('puts the starting track at the head', () => {
        expect(buildShuffleOrder(['a', 'b', 'c', 'd'], 'c', first)[0]).toBe('c')
    })

    it('covers the whole queue exactly once', () => {
        const order = buildShuffleOrder(['a', 'b', 'c', 'd'], 'c', first)
        expect([...order].sort()).toEqual(['a', 'b', 'c', 'd'])
    })

    it('shuffles everything when there is no starting track', () => {
        expect(buildShuffleOrder(['a', 'b', 'c'], null, first)).toEqual(['b', 'c', 'a'])
    })

    it('shuffles everything when the starting track is not queued', () => {
        const order = buildShuffleOrder(['a', 'b', 'c'], 'z', first)
        expect([...order].sort()).toEqual(['a', 'b', 'c'])
    })
})

describe('resyncShuffleOrder', () => {
    it('drops entries that left the queue', () => {
        expect(resyncShuffleOrder(['c', 'a', 'b'], ['a', 'c'], 'c', first)).toEqual(['c', 'a'])
    })

    it('keeps the relative order of surviving entries', () => {
        expect(resyncShuffleOrder(['d', 'b', 'a', 'c'], ['a', 'b', 'c', 'd'], 'd', first)).toEqual([
            'd',
            'b',
            'a',
            'c'
        ])
    })

    it('splices new tracks in after the current one', () => {
        const out = resyncShuffleOrder(['b', 'a', 'c'], ['a', 'b', 'c', 'x', 'y'], 'b', first)
        expect(out[0]).toBe('b')
        expect([...out].sort()).toEqual(['a', 'b', 'c', 'x', 'y'])
        // Additions never land before the playing track.
        expect(out.indexOf('x')).toBeGreaterThan(0)
        expect(out.indexOf('y')).toBeGreaterThan(0)
    })

    it('appends new tracks when the current track is unknown', () => {
        const out = resyncShuffleOrder([], ['a', 'b'], null, first)
        expect([...out].sort()).toEqual(['a', 'b'])
    })

    it('returns an empty order for an empty queue', () => {
        expect(resyncShuffleOrder(['a', 'b'], [], 'a', first)).toEqual([])
    })
})

describe('stepOrderPosition', () => {
    it('steps forward and backward', () => {
        expect(stepOrderPosition(1, 4, 1, 'none')).toBe(2)
        expect(stepOrderPosition(1, 4, -1, 'none')).toBe(0)
    })

    it('returns null at the ends when repeat is none', () => {
        expect(stepOrderPosition(3, 4, 1, 'none')).toBeNull()
        expect(stepOrderPosition(0, 4, -1, 'none')).toBeNull()
    })

    it('wraps around both ends when repeat is all', () => {
        expect(stepOrderPosition(3, 4, 1, 'all')).toBe(0)
        expect(stepOrderPosition(0, 4, -1, 'all')).toBe(3)
    })

    it('enters the order from either end when the position is unknown', () => {
        expect(stepOrderPosition(-1, 4, 1, 'none')).toBe(0)
        expect(stepOrderPosition(-1, 4, -1, 'none')).toBe(3)
    })

    it('returns null for an empty order', () => {
        expect(stepOrderPosition(0, 0, 1, 'all')).toBeNull()
        expect(stepOrderPosition(-1, 0, -1, 'all')).toBeNull()
    })
})
