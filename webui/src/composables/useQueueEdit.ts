import { ref } from 'vue'

export function useQueueEdit() {
    const editMode = ref(false)
    const selectedIndices = ref<Set<number>>(new Set())

    const clearSelection = (): void => {
        selectedIndices.value = new Set()
    }

    const toggleEditMode = (): void => {
        editMode.value = !editMode.value
        if (!editMode.value) clearSelection()
    }

    const isSelected = (index: number): boolean => selectedIndices.value.has(index)

    const onRowClick = (index: number, additive: boolean): void => {
        const next = new Set(additive ? selectedIndices.value : [])
        if (additive && next.has(index)) {
            next.delete(index)
        } else {
            next.add(index)
        }
        selectedIndices.value = next
    }

    const toggleCheckbox = (index: number): void => {
        const next = new Set(selectedIndices.value)
        if (next.has(index)) next.delete(index)
        else next.add(index)
        selectedIndices.value = next
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
        toggleEditMode,
        isSelected,
        onRowClick,
        toggleCheckbox,
        selectionForDrag,
        clearSelection
    }
}
