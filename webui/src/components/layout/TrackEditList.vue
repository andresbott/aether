<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import Sortable from 'sortablejs'
import QueueRow from '@/components/layout/QueueRow.vue'
import { useRowSelection, type RowClickModifiers } from '@/composables/useRowSelection'
import { computeDropTarget } from '@/utils/queueReorder'
import { buildMultiDragImage } from '@/utils/queueDragImage'
import type { Song } from '@/types/subsonic'

const props = withDefaults(
    defineProps<{
        songs: Song[]
        currentIndex?: number
        deleteLabel?: string
        group?: string
    }>(),
    { currentIndex: -1, deleteLabel: 'Remove', group: 'tracks' }
)

const emit = defineEmits<{
    reorder: [indices: number[], target: number]
    delete: [indices: number[]]
}>()

const { isSelected, selectedIndices, onRowClick, selectionForDrag, clearSelection } =
    useRowSelection()

const listRef = ref<HTMLElement | null>(null)
let sortable: Sortable | null = null
let hiddenRows: HTMLElement[] = []
let dragImageEl: HTMLElement | null = null

const rows = computed(() => props.songs.map((song, i) => ({ ...song, index: i })))

const onSelectRow = (index: number, payload: RowClickModifiers): void => {
    onRowClick(index, payload, props.currentIndex)
    listRef.value?.focus()
}

const onDeleteRow = (index: number): void => {
    const ids = selectionForDrag(index)
    if (ids.length === 0) return
    emit('delete', ids)
    clearSelection()
}

const onListKeydown = (e: KeyboardEvent): void => {
    if (e.key !== 'Delete' && e.key !== 'Backspace') return
    if (selectedIndices.value.size === 0) return
    e.preventDefault()
    emit('delete', [...selectedIndices.value].sort((a, b) => a - b))
    clearSelection()
}

const handleSortStart = (evt: Sortable.SortableEvent): void => {
    const item = evt.item as HTMLElement
    const ids = selectionForDrag(Number(item.dataset.queueIndex))
    if (ids.length <= 1) return
    const selected = new Set(ids)
    const list = listRef.value
    if (!list) return
    for (const child of Array.from(list.children)) {
        const el = child as HTMLElement
        if (el !== item && selected.has(Number(el.dataset.queueIndex))) {
            el.style.display = 'none'
            hiddenRows.push(el)
        }
    }
}

const setDragData = (dataTransfer: DataTransfer | null, dragEl: HTMLElement): void => {
    if (!dataTransfer) return
    const ids = selectionForDrag(Number(dragEl.dataset.queueIndex))
    if (ids.length <= 1) return
    const img = buildMultiDragImage(dragEl, ids.length)
    document.body.appendChild(img)
    dragImageEl = img
    dataTransfer.setDragImage(img, 24, 24)
}

const cleanupMultiDrag = (): void => {
    hiddenRows.forEach((el) => {
        el.style.display = ''
    })
    hiddenRows = []
    if (dragImageEl) {
        dragImageEl.remove()
        dragImageEl = null
    }
}

const handleSortEnd = (evt: Sortable.SortableEvent): void => {
    cleanupMultiDrag()
    const item = evt.item as HTMLElement
    const draggedIndex = Number(item.dataset.queueIndex)
    const toList = evt.to as HTMLElement
    const after = toList.children[(evt.newIndex ?? 0) + 1] as HTMLElement | undefined
    const anchorRaw = after?.dataset.queueIndex
    const anchorIndex = anchorRaw !== undefined ? Number(anchorRaw) : undefined
    const targetIndex = computeDropTarget(anchorIndex, props.songs.length)

    // Revert SortableJS's DOM mutation so Vue re-renders cleanly from state.
    const fromList = evt.from as HTMLElement
    const reference = fromList.children[evt.oldIndex ?? 0] ?? null
    fromList.insertBefore(item, reference)

    if (Number.isNaN(draggedIndex)) return
    emit('reorder', selectionForDrag(draggedIndex), targetIndex)
    clearSelection()
}

onMounted(() => {
    if (!listRef.value) return
    sortable = Sortable.create(listRef.value, {
        group: props.group,
        handle: '.drag-handle',
        animation: 150,
        onStart: handleSortStart,
        setData: setDragData,
        onEnd: handleSortEnd
    })
})

onUnmounted(() => {
    cleanupMultiDrag()
    sortable?.destroy()
    sortable = null
})

defineExpose({ clearSelection })
</script>

<template>
    <div
        ref="listRef"
        class="queue-edit-list"
        role="listbox"
        aria-multiselectable="true"
        tabindex="0"
        @keydown="onListKeydown"
    >
        <QueueRow
            v-for="row in rows"
            :key="row.id + ':' + row.index"
            :song="row"
            :queue-index="row.index"
            editing
            :selected="isSelected(row.index)"
            :current="row.index === currentIndex"
            :delete-label="deleteLabel"
            @select="(p) => onSelectRow(row.index, p)"
            @delete="onDeleteRow(row.index)"
        />
    </div>
</template>

<style scoped>
.queue-edit-list:focus-visible {
    outline: 2px solid var(--app-accent);
    outline-offset: -2px;
    border-radius: 6px;
}
</style>
