import { describe, it, expect } from 'vitest'
import { reorderQueue, computeDropTarget } from '@/utils/queueReorder'

const q = ['A', 'B', 'C', 'D', 'E', 'F', 'G']

describe('reorderQueue', () => {
    it('moves a single item before the target index', () => {
        // move E (index 4) to before index 1 (B)
        expect(reorderQueue(q, [4], 1)).toEqual(['A', 'E', 'B', 'C', 'D', 'F', 'G'])
    })

    it('moves a contiguous selection together', () => {
        // move [B,C] (1,2) to before index 0 (A)
        expect(reorderQueue(q, [1, 2], 0)).toEqual(['B', 'C', 'A', 'D', 'E', 'F', 'G'])
    })

    it('moves a non-contiguous selection together as one block', () => {
        // move [A,D] (0,3) to before index 5 (F)
        expect(reorderQueue(q, [0, 3], 5)).toEqual(['B', 'C', 'E', 'A', 'D', 'F', 'G'])
    })

    it('appends the block when the target is at or past the end', () => {
        expect(reorderQueue(q, [0, 3], q.length)).toEqual(['B', 'C', 'E', 'F', 'G', 'A', 'D'])
    })

    it('treats a target pointing at a moved item as the next non-moved slot', () => {
        // move [2] to before index 2 (itself) → lands in its original slot (no-op-ish)
        expect(reorderQueue(q, [2], 2)).toEqual(['A', 'B', 'C', 'D', 'E', 'F', 'G'])
    })

    it('ignores duplicate indices and returns a new array', () => {
        const out = reorderQueue(q, [4, 4], 0)
        expect(out).toEqual(['E', 'A', 'B', 'C', 'D', 'F', 'G'])
        expect(out).not.toBe(q)
    })

    it('returns a copy when nothing is moved', () => {
        const out = reorderQueue(q, [], 3)
        expect(out).toEqual(q)
        expect(out).not.toBe(q)
    })
})

describe('computeDropTarget', () => {
    it('uses the anchor index when a following row exists', () => {
        expect(computeDropTarget(3, 10)).toBe(3)
    })
    it('falls back to the queue length when dropped at the end', () => {
        expect(computeDropTarget(undefined, 10)).toBe(10)
    })
})
