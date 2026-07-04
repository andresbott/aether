import { describe, it, expect } from 'vitest'
import {
    computeGridColumns,
    computeColumnWidth,
    chunkRows,
    offsetToRow,
    rowRangeToItemRange
} from '@/utils/cardGrid'

describe('computeGridColumns', () => {
    it('mirrors auto-fill minmax(min, 1fr) with gap', () => {
        // (932 + 32) / (200 + 32) = 4.15 -> 4
        expect(computeGridColumns(932, 200, 32)).toBe(4)
        // exactly enough for 5: 5*200 + 4*32 = 1128 -> (1128+32)/232 = 5
        expect(computeGridColumns(1128, 200, 32)).toBe(5)
    })

    it('never returns less than 1, even at zero/negative width', () => {
        expect(computeGridColumns(0, 200, 32)).toBe(1)
        expect(computeGridColumns(-50, 200, 32)).toBe(1)
        expect(computeGridColumns(150, 200, 32)).toBe(1)
    })
})

describe('computeColumnWidth', () => {
    it('subtracts inter-column gaps', () => {
        // 4 cols in 932 with 32 gap: (932 - 3*32) / 4 = 209
        expect(computeColumnWidth(932, 4, 32)).toBe(209)
    })

    it('returns 0 for non-positive columns', () => {
        expect(computeColumnWidth(932, 0, 32)).toBe(0)
    })
})

describe('chunkRows', () => {
    it('splits a dense array into rows of `columns`', () => {
        expect(chunkRows([1, 2, 3, 4, 5], 2)).toEqual([[1, 2], [3, 4], [5]])
    })

    it('preserves undefined (unloaded) slots', () => {
        const rows = chunkRows<number>([undefined, undefined, 3], 2)
        expect(rows).toHaveLength(2)
        expect(rows[0][0]).toBeUndefined()
        expect(rows[1][0]).toBe(3)
    })

    it('returns [] for zero columns or empty input', () => {
        expect(chunkRows([1, 2], 0)).toEqual([])
        expect(chunkRows([], 3)).toEqual([])
    })
})

describe('offsetToRow', () => {
    it('maps an item offset to its row index', () => {
        expect(offsetToRow(0, 4)).toBe(0)
        expect(offsetToRow(3, 4)).toBe(0)
        expect(offsetToRow(4, 4)).toBe(1)
        expect(offsetToRow(9, 4)).toBe(2)
    })
})

describe('rowRangeToItemRange', () => {
    it('covers the entire span of visible rows (last row inclusive)', () => {
        // rows 0..2 at 4 cols -> items 0..11
        expect(rowRangeToItemRange(0, 2, 4, 100)).toEqual({ first: 0, last: 11 })
    })

    it('clamps the last item to total - 1', () => {
        // rows 0..2 at 4 cols would reach 11, but only 6 items exist
        expect(rowRangeToItemRange(0, 2, 4, 6)).toEqual({ first: 0, last: 5 })
    })

    it('is safe when total is 0', () => {
        expect(rowRangeToItemRange(0, 2, 4, 0)).toEqual({ first: 0, last: 0 })
    })

    it('over-covers rather than under-covers a partial last row', () => {
        // even if the caller passed an exclusive lastRow of 3, items 0..15 (clamped) still
        // fully cover what rows 0..2 (items 0..11) need — never a gap.
        const r = rowRangeToItemRange(0, 3, 4, 100)
        expect(r.first).toBe(0)
        expect(r.last).toBeGreaterThanOrEqual(11)
    })
})
