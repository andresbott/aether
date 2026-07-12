import { describe, it, expect } from 'vitest'
import { useRowSelection } from '@/composables/useRowSelection'

const plain = { additive: false, range: false }
const ctrl = { additive: true, range: false }
const shift = { additive: false, range: true }

const sorted = (s: ReturnType<typeof useRowSelection>): number[] =>
    [...s.selectedIndices.value].sort((a, b) => a - b)

describe('useRowSelection', () => {
    it('plain click selects only that row (replacing the selection)', () => {
        const s = useRowSelection()
        s.onRowClick(1, plain)
        s.onRowClick(3, plain)
        expect([...s.selectedIndices.value]).toEqual([3])
    })

    it('ctrl/cmd click toggles rows in and out of the selection', () => {
        const s = useRowSelection()
        s.onRowClick(1, plain)
        s.onRowClick(3, ctrl)
        expect(sorted(s)).toEqual([1, 3])
        s.onRowClick(1, ctrl)
        expect([...s.selectedIndices.value]).toEqual([3])
    })

    it('shift click selects the inclusive range from the anchor', () => {
        const s = useRowSelection()
        s.onRowClick(2, plain)
        s.onRowClick(5, shift)
        expect(sorted(s)).toEqual([2, 3, 4, 5])
    })

    it('shift range skips the passed currentIndex gap', () => {
        const s = useRowSelection()
        s.onRowClick(1, plain)
        s.onRowClick(5, shift, 3)
        expect(sorted(s)).toEqual([1, 2, 4, 5])
    })

    it('defaults currentIndex to -1 so no row is skipped (album list has no gap)', () => {
        const s = useRowSelection()
        s.onRowClick(0, plain)
        s.onRowClick(3, shift)
        expect(sorted(s)).toEqual([0, 1, 2, 3])
    })

    it('isSelected reflects membership', () => {
        const s = useRowSelection()
        s.onRowClick(2, plain)
        expect(s.isSelected(2)).toBe(true)
        expect(s.isSelected(3)).toBe(false)
    })

    it('selectionForDrag returns all selected only for a dragged row in a multi-selection', () => {
        const s = useRowSelection()
        s.onRowClick(0, plain)
        s.onRowClick(3, ctrl)
        expect(s.selectionForDrag(3)).toEqual([0, 3])
        expect(s.selectionForDrag(5)).toEqual([5]) // dragged row not selected
    })

    it('clearSelection resets the anchor so a later shift click starts fresh', () => {
        const s = useRowSelection()
        s.onRowClick(2, plain)
        s.clearSelection()
        s.onRowClick(5, shift)
        expect([...s.selectedIndices.value]).toEqual([5])
    })
})
