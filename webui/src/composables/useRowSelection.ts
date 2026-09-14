import { computed, getCurrentInstance, onBeforeUnmount, onMounted, ref } from 'vue'
import { isTypingTarget } from '@/utils/shortcuts'
import { useShortcutHelp } from '@/composables/useShortcutHelp'

export interface RowClickModifiers {
    additive: boolean
    range: boolean
}

/**
 * Index-based row selection shared by the queue editor and the album track list:
 * plain click replaces the selection, ctrl/⌘ toggles a row, SHIFT extends a range
 * from the anchor. `selectionForDrag` resolves which rows a drag should carry.
 * Callers that have a non-selectable row in the middle (the now-playing track)
 * pass its index as `currentIndex` so a SHIFT range straddling it skips it; the
 * album list has no such gap and relies on the `-1` default.
 */
export function useRowSelection() {
    const selectedIndices = ref<Set<number>>(new Set())
    // The pivot a SHIFT range extends from, and the "committed" selection a
    // SHIFT range is unioned onto. Plain/ctrl clicks commit a new base + anchor;
    // SHIFT clicks leave both untouched so the range can be re-dragged off the
    // same pivot.
    const anchorIndex = ref<number | null>(null)
    let baseSelection = new Set<number>()

    const clearSelection = (): void => {
        selectedIndices.value = new Set()
        anchorIndex.value = null
        baseSelection = new Set()
    }

    const isSelected = (index: number): boolean => selectedIndices.value.has(index)

    // Reactive size for the hosts' selection-aware UI (the in-hero action row and
    // the rows' "reveal every checkbox while selecting" state). `.value.size` read
    // straight in a template would not track Set mutations.
    const selectedCount = computed(() => selectedIndices.value.size)

    const commit = (next: Set<number>, anchor: number): void => {
        selectedIndices.value = next
        anchorIndex.value = anchor
        baseSelection = new Set(next)
    }

    const rangeBetween = (a: number, b: number, currentIndex: number): number[] => {
        const lo = Math.min(a, b)
        const hi = Math.max(a, b)
        const out: number[] = []
        for (let i = lo; i <= hi; i++) {
            if (i !== currentIndex) out.push(i)
        }
        return out
    }

    const onRowClick = (index: number, modifiers: RowClickModifiers, currentIndex = -1): void => {
        if (modifiers.range && anchorIndex.value !== null) {
            const next = new Set(baseSelection)
            for (const i of rangeBetween(anchorIndex.value, index, currentIndex)) next.add(i)
            selectedIndices.value = next
            return
        }

        if (modifiers.additive) {
            const next = new Set(selectedIndices.value)
            if (next.has(index)) next.delete(index)
            else next.add(index)
            commit(next, index)
            return
        }

        commit(new Set([index]), index)
    }

    const selectionForDrag = (draggedIndex: number): number[] => {
        const sel = selectedIndices.value
        if (sel.has(draggedIndex) && sel.size > 1) {
            return [...sel].sort((a, b) => a - b)
        }
        return [draggedIndex]
    }

    // Esc drops the whole selection — the same as every surface's Clear button — so
    // the detail-view lists, the queue editor and the playlist reorder list all
    // deselect the same way. It yields Escape first to anything that owns it: a
    // focused text field, an open PrimeVue dialog/popover (which closes itself), and
    // the keyboard-shortcut help overlay (closed by useKeyboardShortcuts). Wired only
    // when the composable runs in a component's setup; the pure unit tests call it
    // bare and want no global listener.
    const onEscape = (event: KeyboardEvent): void => {
        if (event.key !== 'Escape') return
        if (selectedIndices.value.size === 0) return
        if (isTypingTarget(event.target)) return
        if (useShortcutHelp().open.value) return
        if (document.querySelector('.p-dialog, .p-popover')) return
        clearSelection()
        event.preventDefault()
    }
    if (getCurrentInstance()) {
        onMounted(() => document.addEventListener('keydown', onEscape))
        onBeforeUnmount(() => document.removeEventListener('keydown', onEscape))
    }

    return {
        selectedIndices,
        selectedCount,
        anchorIndex,
        isSelected,
        onRowClick,
        selectionForDrag,
        clearSelection
    }
}
