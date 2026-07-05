import { describe, it, expect } from 'vitest'
import { useQueueEdit } from '@/composables/useQueueEdit'

const plain = { additive: false, range: false }
const ctrl = { additive: true, range: false }
const shift = { additive: false, range: true }

const sorted = (e: ReturnType<typeof useQueueEdit>): number[] =>
    [...e.selectedIndices.value].sort((a, b) => a - b)

describe('useQueueEdit', () => {
    it('plain click selects only that row (replacing the selection)', () => {
        const e = useQueueEdit()
        e.onRowClick(1, plain)
        e.onRowClick(3, plain)
        expect([...e.selectedIndices.value]).toEqual([3])
    })

    it('ctrl/cmd click toggles rows in and out of the selection', () => {
        const e = useQueueEdit()
        e.onRowClick(1, plain)
        e.onRowClick(3, ctrl)
        expect(sorted(e)).toEqual([1, 3])
        e.onRowClick(1, ctrl)
        expect([...e.selectedIndices.value]).toEqual([3])
    })

    it('shift click selects the inclusive range from the anchor', () => {
        const e = useQueueEdit()
        e.onRowClick(2, plain)
        e.onRowClick(5, shift)
        expect(sorted(e)).toEqual([2, 3, 4, 5])
    })

    it('shift click unions the range onto the ctrl-committed selection', () => {
        const e = useQueueEdit()
        e.onRowClick(2, plain) // {2} anchor 2
        e.onRowClick(5, shift) // {2,3,4,5}
        e.onRowClick(8, ctrl) // {2,3,4,5,8} anchor 8
        e.onRowClick(6, shift) // range 6..8 ∪ committed
        expect(sorted(e)).toEqual([2, 3, 4, 5, 6, 7, 8])
    })

    it('repeated shift clicks re-extend from the same anchor', () => {
        const e = useQueueEdit()
        e.onRowClick(2, plain)
        e.onRowClick(6, shift) // {2,3,4,5,6}
        e.onRowClick(4, shift) // shrinks back to {2,3,4}
        expect(sorted(e)).toEqual([2, 3, 4])
    })

    it('shift range skips the now-playing index', () => {
        const e = useQueueEdit()
        e.onRowClick(1, plain)
        e.onRowClick(5, shift, 3) // current track at index 3
        expect(sorted(e)).toEqual([1, 2, 4, 5])
    })

    it('shift click with no anchor behaves as a plain click', () => {
        const e = useQueueEdit()
        e.onRowClick(4, shift)
        expect([...e.selectedIndices.value]).toEqual([4])
        expect(e.anchorIndex.value).toBe(4)
    })

    it('clearSelection resets the anchor so a later shift click starts fresh', () => {
        const e = useQueueEdit()
        e.onRowClick(2, plain)
        e.clearSelection()
        e.onRowClick(5, shift)
        expect([...e.selectedIndices.value]).toEqual([5])
    })

    it('selectionForDrag returns all selected when the dragged row is in a multi-selection', () => {
        const e = useQueueEdit()
        e.onRowClick(0, plain)
        e.onRowClick(3, ctrl)
        expect(e.selectionForDrag(3)).toEqual([0, 3])
    })

    it('selectionForDrag returns just the dragged row otherwise', () => {
        const e = useQueueEdit()
        e.onRowClick(0, plain)
        expect(e.selectionForDrag(5)).toEqual([5]) // dragged row not selected
        expect(e.selectionForDrag(0)).toEqual([0]) // selection size 1
    })

    it('toggleEditMode clears the selection when turning off', () => {
        const e = useQueueEdit()
        e.toggleEditMode() // on
        e.onRowClick(1, plain)
        e.toggleEditMode() // off
        expect(e.editMode.value).toBe(false)
        expect(e.selectedIndices.value.size).toBe(0)
    })
})
