import { describe, it, expect } from 'vitest'
import { useQueueEdit } from '@/composables/useQueueEdit'

describe('useQueueEdit', () => {
    it('plain click selects only that row (replacing the selection)', () => {
        const e = useQueueEdit()
        e.onRowClick(1, false)
        e.onRowClick(3, false)
        expect([...e.selectedIndices.value]).toEqual([3])
    })

    it('ctrl/cmd click toggles rows in and out of the selection', () => {
        const e = useQueueEdit()
        e.onRowClick(1, false)
        e.onRowClick(3, true)
        expect([...e.selectedIndices.value].sort()).toEqual([1, 3])
        e.onRowClick(1, true)
        expect([...e.selectedIndices.value]).toEqual([3])
    })

    it('checkbox toggles a single row independently', () => {
        const e = useQueueEdit()
        e.toggleCheckbox(2)
        expect(e.isSelected(2)).toBe(true)
        e.toggleCheckbox(2)
        expect(e.isSelected(2)).toBe(false)
    })

    it('selectionForDrag returns all selected when the dragged row is in a multi-selection', () => {
        const e = useQueueEdit()
        e.toggleCheckbox(0)
        e.toggleCheckbox(3)
        expect(e.selectionForDrag(3)).toEqual([0, 3])
    })

    it('selectionForDrag returns just the dragged row otherwise', () => {
        const e = useQueueEdit()
        e.toggleCheckbox(0)
        expect(e.selectionForDrag(5)).toEqual([5]) // dragged row not selected
        expect(e.selectionForDrag(0)).toEqual([0]) // selection size 1
    })

    it('toggleEditMode clears the selection when turning off', () => {
        const e = useQueueEdit()
        e.toggleEditMode() // on
        e.toggleCheckbox(1)
        e.toggleEditMode() // off
        expect(e.editMode.value).toBe(false)
        expect(e.selectedIndices.value.size).toBe(0)
    })
})
