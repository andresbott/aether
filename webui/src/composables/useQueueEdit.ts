import { ref } from 'vue'
import { useRowSelection, type RowClickModifiers } from '@/composables/useRowSelection'

export type { RowClickModifiers }

/**
 * Queue edit mode: wraps the shared row-selection core (plain/ctrl/SHIFT clicks,
 * `selectionForDrag`) with an on/off `editMode` toggle that clears the selection
 * on exit. Every track is selectable in edit mode, including the now-playing one
 * (which carries no playback control there).
 */
export function useQueueEdit() {
    const editMode = ref(false)
    const selection = useRowSelection()

    const toggleEditMode = (): void => {
        editMode.value = !editMode.value
        if (!editMode.value) selection.clearSelection()
    }

    return {
        editMode,
        toggleEditMode,
        ...selection
    }
}
