import { describe, it, expect } from 'vitest'
import { computeInsertIndex, type QueueRowRect } from '@/utils/queueInsert'

// Three stacked rows, 20px tall each, mapped to queue indices 0,1,2.
const rows: QueueRowRect[] = [
    { queueIndex: 0, top: 0, bottom: 20 },
    { queueIndex: 1, top: 20, bottom: 40 },
    { queueIndex: 2, top: 40, bottom: 60 }
]

describe('computeInsertIndex', () => {
    it('inserts before the first row when the pointer is in its top half', () => {
        expect(computeInsertIndex(rows, 5, 3)).toBe(0)
    })

    it('inserts after a row when the pointer is in its bottom half', () => {
        // row 0 midpoint is 10; pointer at 15 → before row 1
        expect(computeInsertIndex(rows, 15, 3)).toBe(1)
    })

    it('treats the current-track boundary like any other row', () => {
        // pointer in row 1 top half → before index 1; bottom half → before index 2
        expect(computeInsertIndex(rows, 25, 3)).toBe(1)
        expect(computeInsertIndex(rows, 35, 3)).toBe(2)
    })

    it('appends when the pointer is below the last midpoint', () => {
        expect(computeInsertIndex(rows, 55, 3)).toBe(3)
    })

    it('returns the queue length for an empty row set', () => {
        expect(computeInsertIndex([], 0, 0)).toBe(0)
        expect(computeInsertIndex([], 100, 5)).toBe(5)
    })
})
