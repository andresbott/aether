import { ref } from 'vue'

export interface RowClickModifiers {
    additive: boolean
    range: boolean
}

export function useQueueEdit() {
    const editMode = ref(false)
    const selectedIndices = ref<Set<number>>(new Set())
    // The pivot a SHIFT range extends from, and the "committed" selection a
    // SHIFT range is unioned onto. Plain/ctrl clicks (and checkbox toggles)
    // commit a new base + anchor; SHIFT clicks leave both untouched so the
    // range can be re-dragged off the same pivot.
    const anchorIndex = ref<number | null>(null)
    let baseSelection = new Set<number>()

    const clearSelection = (): void => {
        selectedIndices.value = new Set()
        anchorIndex.value = null
        baseSelection = new Set()
    }

    const toggleEditMode = (): void => {
        editMode.value = !editMode.value
        if (!editMode.value) clearSelection()
    }

    const isSelected = (index: number): boolean => selectedIndices.value.has(index)

    const commit = (next: Set<number>, anchor: number): void => {
        selectedIndices.value = next
        anchorIndex.value = anchor
        baseSelection = new Set(next)
    }

    // The now-playing track sits between the history and upcoming lists and has
    // no selectable row, so it is left out of any range that straddles it.
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

    return {
        editMode,
        selectedIndices,
        anchorIndex,
        toggleEditMode,
        isSelected,
        onRowClick,
        selectionForDrag,
        clearSelection
    }
}
