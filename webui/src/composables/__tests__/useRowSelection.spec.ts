import { describe, it, expect, afterEach } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { useRowSelection } from '@/composables/useRowSelection'
import { useShortcutHelp } from '@/composables/useShortcutHelp'

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

// Esc-to-deselect is wired here, in the shared core, so every selection surface
// (the detail-view lists, the queue editor, the playlist reorder list) drops its
// selection the same way. The listener only exists while a host component is
// mounted, so these run the composable inside a throwaway component.
describe('useRowSelection Escape-to-deselect', () => {
    const wrappers: ReturnType<typeof mount>[] = []

    const mountSelection = (): {
        s: ReturnType<typeof useRowSelection>
        wrapper: ReturnType<typeof mount>
    } => {
        let s!: ReturnType<typeof useRowSelection>
        const Host = defineComponent({
            setup() {
                s = useRowSelection()
                return () => null
            }
        })
        const wrapper = mount(Host)
        wrappers.push(wrapper)
        return { s, wrapper }
    }

    const pressEscape = (init: KeyboardEventInit = {}): KeyboardEvent => {
        const event = new KeyboardEvent('keydown', { key: 'Escape', cancelable: true, ...init })
        document.dispatchEvent(event)
        return event
    }

    afterEach(() => {
        for (const w of wrappers.splice(0)) w.unmount()
        useShortcutHelp().close()
    })

    it('clears the selection when Escape is pressed', () => {
        const { s } = mountSelection()
        s.onRowClick(1, plain)
        pressEscape()
        expect(s.selectedIndices.value.size).toBe(0)
    })

    it('swallows the Escape it consumes so nothing else acts on it', () => {
        const { s } = mountSelection()
        s.onRowClick(1, plain)
        expect(pressEscape().defaultPrevented).toBe(true)
    })

    it('leaves Escape alone when nothing is selected', () => {
        mountSelection()
        expect(pressEscape().defaultPrevented).toBe(false)
    })

    it('leaves Escape to an open dialog or popover', () => {
        const { s } = mountSelection()
        s.onRowClick(1, plain)
        const overlay = document.createElement('div')
        overlay.className = 'p-popover'
        document.body.appendChild(overlay)
        pressEscape()
        expect(s.selectedIndices.value.size).toBe(1)
        overlay.remove()
    })

    it('leaves Escape to a focused text field', () => {
        const { s } = mountSelection()
        s.onRowClick(1, plain)
        const input = document.createElement('input')
        document.body.appendChild(input)
        input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
        expect(s.selectedIndices.value.size).toBe(1)
        input.remove()
    })

    it('leaves Escape to the shortcut help overlay while it is open', () => {
        const { s } = mountSelection()
        s.onRowClick(1, plain)
        useShortcutHelp().toggle()
        pressEscape()
        expect(s.selectedIndices.value.size).toBe(1)
    })

    it('stops listening once its host component is gone', () => {
        const { s, wrapper } = mountSelection()
        s.onRowClick(1, plain)
        wrapper.unmount()
        pressEscape()
        expect(s.selectedIndices.value.size).toBe(1)
    })
})
