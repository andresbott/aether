# Playlist UI Rework Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rework the playlist UI so track editing reuses the queue's edit component, the rename control is inline beside the title, and the playlists index mirrors the albums grid/list view.

**Architecture:** Extract the queue's edit-mode list into a reusable `TrackEditList.vue` that owns multi-select + drag reorder and emits `reorder`/`delete` intent (parents decide persistence). The playlist detail view runs a **batched** edit against a local working copy, persisting via the backend replace-all path (`createPlaylist` with `playlistId`). The playlists index gains a grid/list toggle with `PlaylistCard`/`PlaylistListView`, mirroring the albums view.

**Tech Stack:** Vue 3 (`<script setup>`), PrimeVue, TanStack Query, SortableJS, Vitest + @vue/test-utils.

## Global Constraints

- All music data flows through the `/rest/` Subsonic client (`subsonicClient`). No new `/api/v1` endpoints.
- Main content views use `ContentScaffold` (title + summary + `#actions`), a self-scrolling body centered on `--app-content-max-width`. See `docs/architecture/main-content-view-layout.md`.
- No backwards-compat/migration code.
- Commits: single one-line message, no `Co-Authored-By`, run `git add` as a separate step from `git commit`.
- Reorder/delete logic reuses the pure utils in `src/utils/queueReorder.ts` (`reorderQueue`, `computeDropTarget`).

---

### Task 1: `ContentScaffold` gains a `#title-actions` slot

**Files:**
- Modify: `src/components/layout/ContentScaffold.vue`
- Test: `src/components/layout/__tests__/ContentScaffold.spec.ts` (create)

**Interfaces:**
- Produces: `ContentScaffold` renders an optional `#title-actions` slot inside `.scaffold-title`, after the `<h1>`.

- [ ] **Step 1: Write the failing test**

Create `src/components/layout/__tests__/ContentScaffold.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'

describe('ContentScaffold', () => {
    it('renders the title and summary', () => {
        const w = mount(ContentScaffold, { props: { title: 'Playlists', summary: '3 playlists' } })
        expect(w.find('.scaffold-title h1').text()).toBe('Playlists')
        expect(w.find('.scaffold-summary').text()).toBe('3 playlists')
    })

    it('renders a #title-actions slot beside the title', () => {
        const w = mount(ContentScaffold, {
            props: { title: 'My Mix' },
            slots: { 'title-actions': '<button class="rename-probe">edit</button>' }
        })
        const title = w.find('.scaffold-title')
        expect(title.find('.rename-probe').exists()).toBe(true)
    })

    it('renders the #actions slot', () => {
        const w = mount(ContentScaffold, {
            props: { title: 'X' },
            slots: { actions: '<button class="act-probe">go</button>' }
        })
        expect(w.find('.scaffold-actions .act-probe').exists()).toBe(true)
    })
})
```

- [ ] **Step 2: Run the test to verify the title-actions case fails**

Run: `npx vitest run src/components/layout/__tests__/ContentScaffold.spec.ts`
Expected: FAIL — the `#title-actions` slot is not rendered (`.rename-probe` not found).

- [ ] **Step 3: Add the slot**

In `src/components/layout/ContentScaffold.vue`, change the `.scaffold-title` block to render the slot after the `<h1>`:

```vue
            <div class="scaffold-title">
                <h1>{{ title }}</h1>
                <slot name="title-actions" />
                <span v-if="summary" class="scaffold-summary">{{ summary }}</span>
            </div>
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npx vitest run src/components/layout/__tests__/ContentScaffold.spec.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add src/components/layout/ContentScaffold.vue src/components/layout/__tests__/ContentScaffold.spec.ts
git commit -m "feat(webui): add title-actions slot to ContentScaffold"
```

---

### Task 2: Extract `TrackEditList` and refactor `QueueView` onto it

**Files:**
- Create: `src/components/layout/TrackEditList.vue`
- Create: `src/components/layout/__tests__/TrackEditList.spec.ts`
- Modify: `src/components/layout/QueueRow.vue` (add `deleteLabel` prop)
- Modify: `src/components/layout/QueueView.vue` (use `TrackEditList` in the edit branch)

**Interfaces:**
- Consumes: `QueueRow` (`{ song, queueIndex, editing, selected, current, deleteLabel }`), `useRowSelection`, `reorderQueue`/`computeDropTarget` from `@/utils/queueReorder`, `buildMultiDragImage` from `@/utils/queueDragImage`, `Sortable` from `sortablejs`.
- Produces:
  - `TrackEditList` props: `songs: Song[]`, `currentIndex?: number` (default `-1`), `deleteLabel?: string` (default `'Remove'`), `group?: string` (default `'tracks'`).
  - `TrackEditList` emits: `reorder(indices: number[], target: number)`, `delete(indices: number[])`.
  - `TrackEditList` exposes (via `defineExpose`): `clearSelection(): void`.
  - Renders a `.queue-edit-list` listbox (`role="listbox"`, `tabindex="0"`) of `QueueRow` rows — markup/classes unchanged from the queue's current edit list.
  - `QueueRow` gains prop `deleteLabel?: string` (default `'Remove from queue'`) used for the edit-row delete button tooltip.

- [ ] **Step 1: Add the `deleteLabel` prop to `QueueRow`**

In `src/components/layout/QueueRow.vue`, extend the props and use the label on the delete button tooltip:

```ts
const props = defineProps<{
    song: Song
    queueIndex: number
    editing?: boolean
    selected?: boolean
    current?: boolean
    deleteLabel?: string
}>()
```

Change the delete `Button`'s tooltip from the hard-coded string to the prop with a fallback:

```vue
            <Button
                icon="pi pi-trash"
                text
                rounded
                size="small"
                severity="secondary"
                class="delete-button"
                v-tooltip.left="deleteLabel ?? 'Remove from queue'"
                @click.stop="emit('delete')"
            />
```

- [ ] **Step 2: Write the failing `TrackEditList` test**

Create `src/components/layout/__tests__/TrackEditList.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '' }
}))

const sortableCreate = vi.fn(() => ({ destroy: vi.fn() }))
vi.mock('sortablejs', () => ({
    default: { create: (...args: unknown[]) => sortableCreate(...(args as [])) }
}))

import TrackEditList from '@/components/layout/TrackEditList.vue'

const song = (id: string) => ({ id, title: `Song ${id}`, artist: 'A', album: 'Al', duration: 60 })

const mountList = (props: Record<string, unknown> = {}) =>
    mount(TrackEditList, {
        props: { songs: [song('1'), song('2'), song('3')], ...props },
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

beforeEach(() => sortableCreate.mockClear())

describe('TrackEditList', () => {
    it('renders one editing row per song as a focusable listbox', () => {
        const w = mountList()
        const list = w.find('.queue-edit-list')
        expect(list.attributes('role')).toBe('listbox')
        expect(list.attributes('tabindex')).toBe('0')
        expect(w.findAll('.queue-edit-list .queue-row--editing')).toHaveLength(3)
    })

    it('marks the currentIndex row as current', () => {
        const w = mountList({ currentIndex: 1 })
        expect(w.find('[data-queue-index="1"]').classes()).toContain('queue-row--current')
    })

    it('emits delete with the single row index when it is not selected', async () => {
        const w = mountList()
        await w.find('[data-queue-index="2"] .delete-button').trigger('click')
        expect(w.emitted('delete')?.[0]).toEqual([[2]])
    })

    it('emits delete with every selected index when a selected row is deleted', async () => {
        const w = mountList()
        await w.find('[data-queue-index="0"]').trigger('click')
        await w.find('[data-queue-index="2"]').trigger('click', { ctrlKey: true })
        await w.find('[data-queue-index="2"] .delete-button').trigger('click')
        expect(w.emitted('delete')?.[0]).toEqual([[0, 2]])
    })

    it('Delete key emits delete with the whole selection', async () => {
        const w = mountList()
        await w.find('[data-queue-index="0"] .row-index--checkbox').trigger('click')
        await w.find('[data-queue-index="2"] .row-index--checkbox').trigger('click')
        await w.find('.queue-edit-list').trigger('keydown', { key: 'Delete' })
        expect(w.emitted('delete')?.[0]).toEqual([[0, 2]])
    })

    it('a Delete keypress with nothing selected emits nothing', async () => {
        const w = mountList()
        await w.find('.queue-edit-list').trigger('keydown', { key: 'Delete' })
        expect(w.emitted('delete')).toBeUndefined()
    })

    it('creates a Sortable with the given group and drag handle when mounted', async () => {
        mountList({ group: 'playlist' })
        await Promise.resolve()
        expect(sortableCreate).toHaveBeenCalledTimes(1)
        const opts = (sortableCreate.mock.calls[0] as unknown[])[1] as { handle: string; group: string }
        expect(opts.handle).toBe('.drag-handle')
        expect(opts.group).toBe('playlist')
    })

    it('uses the deleteLabel for the row delete tooltip', () => {
        const w = mountList({ deleteLabel: 'Remove from playlist' })
        expect(w.find('.delete-button').exists()).toBe(true)
    })
})
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `npx vitest run src/components/layout/__tests__/TrackEditList.spec.ts`
Expected: FAIL — `TrackEditList.vue` does not exist (import error).

- [ ] **Step 4: Create `TrackEditList.vue`**

Create `src/components/layout/TrackEditList.vue`. This lifts the queue's edit list, its SortableJS wiring, multi-select, and keyboard handling out of `QueueView`, generalized to emit intent instead of calling the player:

```vue
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
```

- [ ] **Step 5: Run the `TrackEditList` test to verify it passes**

Run: `npx vitest run src/components/layout/__tests__/TrackEditList.spec.ts`
Expected: PASS (8 tests).

- [ ] **Step 6: Refactor `QueueView` to use `TrackEditList`**

In `src/components/layout/QueueView.vue`:

1. Remove the now-relocated logic and imports: `Sortable` import; `useQueueDrop` stays; remove `useQueueEdit`'s selection usage but keep `editMode`/`toggleEditMode`. Remove the local `editListRef`, `sortables`, `hiddenRows`, `dragImageEl`, `handleSortStart`, `setDragData`, `cleanupMultiDrag`, `handleSortEnd`, `destroySortables`, `createSortables`, the `watch(editMode, ...)` for sortables, and `onUnmounted(destroySortables)`. Also remove `buildMultiDragImage` and `computeDropTarget` imports (they move to `TrackEditList`), and the `onSelectRow`/`onEditListKeydown`/`deleteSelected`/`isSelected`/`selectedIndices`/`selectionForDrag`/`RowClickModifiers` usages.

   Keep: `editRows`, `removeIndices`, `onDeleteRow` → replace with handlers below. Keep the album-drop wiring (`useQueueDrop`, `queueBodyRef`, drop indicator).

2. Update the composable destructure:

```ts
const { editMode, toggleEditMode } = useQueueEdit()
```

3. Add an `editListRef` for the exposed `clearSelection`, and reorder/delete handlers:

```ts
import TrackEditList from '@/components/layout/TrackEditList.vue'

const editListRef = ref<InstanceType<typeof TrackEditList> | null>(null)

const removeIndices = (indices: number[]): void => {
    if (indices.length === 0) return
    if (indices.length > 1) player.removeManyFromQueue(indices)
    else player.removeFromQueue(indices[0])
}

const onReorder = (indices: number[], target: number): void => {
    player.moveInQueue(indices, target)
}
```

   Update the drop composable to clear the child's selection on insert:

```ts
const {
    indicatorTop: dropIndicatorTop,
    indicatorCount: dropIndicatorCount,
    dragActive: dropActive,
    onDragOver: onQueueDragOver,
    onDragLeave: onQueueDragLeave,
    onDrop: onQueueDrop
} = useQueueDrop({ bodyRef: queueBodyRef, onInsert: () => editListRef.value?.clearSelection() })
```

4. Replace the entire edit-mode `<div v-if="editMode" ref="editListRef" class="queue-edit-list" ...> ... </div>` block with:

```vue
            <TrackEditList
                v-if="editMode"
                ref="editListRef"
                :songs="player.queue.value"
                :current-index="player.currentIndex.value"
                delete-label="Remove from queue"
                group="queue"
                @reorder="onReorder"
                @delete="removeIndices"
            />
```

5. Delete the `.queue-edit-list:focus-visible` style block from `QueueView.vue` (it now lives in `TrackEditList`).

- [ ] **Step 7: Run the QueueView + queue-related suites**

Run: `npx vitest run src/components/layout/__tests__/QueueView.spec.ts`
Expected: PASS. The existing selectors (`.queue-edit-list`, `[data-queue-index]`, `.delete-button`, sortable group `queue`, `removeFromQueue`/`removeManyFromQueue`, album-drop-clears-selection) all still resolve because `QueueView` mounts `TrackEditList` as a real child.

If any test references removed internals rather than DOM/behavior, update the assertion to the DOM/emit-based equivalent (do not weaken coverage).

- [ ] **Step 8: Commit**

```bash
git add src/components/layout/TrackEditList.vue src/components/layout/__tests__/TrackEditList.spec.ts src/components/layout/QueueRow.vue src/components/layout/QueueView.vue
git commit -m "refactor(webui): extract reusable TrackEditList from QueueView"
```

---

### Task 3: Add the replace-all playlist API method and mutation hook

**Files:**
- Modify: `src/lib/api/subsonic.ts` (add `replacePlaylistTracks`)
- Modify: `src/composables/useSubsonicQueries.ts` (add `useReplacePlaylistTracks`)
- Test: `src/lib/api/__tests__/subsonic.spec.ts` (add a describe block)

**Interfaces:**
- Produces:
  - `subsonicClient.replacePlaylistTracks(playlistId: string, songIds: string[]): Promise<void>` — calls `createPlaylist.view?playlistId=…&songId=…` (the backend replace-all path).
  - `useReplacePlaylistTracks()` — TanStack mutation; `mutate({ playlistId, songIds })`; invalidates `queryKeys.playlists` and `['subsonic', 'playlist']`.

- [ ] **Step 1: Write the failing client test**

Append to `src/lib/api/__tests__/subsonic.spec.ts`:

```ts
describe('subsonicClient.replacePlaylistTracks', () => {
    beforeEach(() => subsonicClient.initWithDefaults())
    afterEach(() => vi.unstubAllGlobals())

    it('posts the full ordered song set to createPlaylist with the playlistId', async () => {
        const fetchMock = mockFetchOnce({})
        await subsonicClient.replacePlaylistTracks('pl-7', ['s1', 's2', 's3'])
        const url = fetchMock.mock.calls[0][0] as string
        expect(url).toContain('/rest/createPlaylist.view')
        expect(url).toContain('playlistId=pl-7')
        const params = new URL(url).searchParams
        expect(params.getAll('songId')).toEqual(['s1', 's2', 's3'])
    })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npx vitest run src/lib/api/__tests__/subsonic.spec.ts`
Expected: FAIL — `replacePlaylistTracks` is not a function.

- [ ] **Step 3: Add the client method**

In `src/lib/api/subsonic.ts`, add after `updatePlaylist` (mirrors `createPlaylist`'s URL building):

```ts
    async replacePlaylistTracks(playlistId: string, songIds: string[]): Promise<void> {
        if (!this.isConfigured()) return
        const url = new URL(this.buildUrl('createPlaylist.view', { playlistId }))
        songIds.forEach((id) => url.searchParams.append('songId', id))
        const response = await fetch(url.toString())
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`)
        const data = await response.json()
        if (data['subsonic-response'].status === 'failed') {
            throw new Error(data['subsonic-response'].error?.message || 'Unknown error')
        }
    }
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npx vitest run src/lib/api/__tests__/subsonic.spec.ts`
Expected: PASS.

- [ ] **Step 5: Add the mutation hook**

In `src/composables/useSubsonicQueries.ts`, add next to `useUpdatePlaylist` (match its `queryClient` + invalidation style):

```ts
export function useReplacePlaylistTracks() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (params: { playlistId: string; songIds: string[] }) =>
            subsonicClient.replacePlaylistTracks(params.playlistId, params.songIds),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: queryKeys.playlists })
            queryClient.invalidateQueries({ queryKey: ['subsonic', 'playlist'] })
        }
    })
}
```

- [ ] **Step 6: Verify the app type-checks**

Run: `npm run type-check`
Expected: PASS (no type errors).

- [ ] **Step 7: Commit**

```bash
git add src/lib/api/subsonic.ts src/composables/useSubsonicQueries.ts src/lib/api/__tests__/subsonic.spec.ts
git commit -m "feat(webui): add replacePlaylistTracks API and mutation hook"
```

---

### Task 4: Rework `PlaylistDetailView` — inline rename + batched track edit

**Files:**
- Modify: `src/views/PlaylistDetailView.vue`
- Test: `src/views/__tests__/PlaylistDetailView.spec.ts` (create)

**Interfaces:**
- Consumes: `usePlaylist`, `useUpdatePlaylist`, `useDeletePlaylist`, `useReplacePlaylistTracks` (Task 3), `TrackEditList` (Task 2), `ContentScaffold` `#title-actions` slot (Task 1), `usePlayer`, `reorderQueue` from `@/utils/queueReorder`.
- Produces: an edit-mode UI whose Save calls `useReplacePlaylistTracks` with the working order; inline rename via `useUpdatePlaylist`.

- [ ] **Step 1: Write the failing test**

Create `src/views/__tests__/PlaylistDetailView.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'

const playlist = ref<any>(null)
vi.mock('@/composables/useSubsonicQueries', () => ({
    usePlaylist: () => ({ data: playlist, isLoading: ref(false), error: ref(null) }),
    useUpdatePlaylist: () => ({ mutate: updateMutate, isPending: ref(false) }),
    useDeletePlaylist: () => ({ mutate: vi.fn() }),
    useReplacePlaylistTracks: () => ({ mutate: replaceMutate, isPending: ref(false) })
}))
const updateMutate = vi.fn()
const replaceMutate = vi.fn()

const playAlbum = vi.fn()
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum }) }))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '' }
}))
vi.mock('sortablejs', () => ({ default: { create: () => ({ destroy: vi.fn() }) } }))
vi.mock('vue-router', () => ({ useRouter: () => ({ back: vi.fn(), push: vi.fn() }) }))

import PlaylistDetailView from '@/views/PlaylistDetailView.vue'

const song = (id: string) => ({ id, title: `Song ${id}`, artist: 'A', album: 'Al', duration: 60 })

const mountView = () =>
    mount(PlaylistDetailView, {
        props: { id: 'pl1' },
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

beforeEach(() => {
    playlist.value = { id: 'pl1', name: 'My Mix', songCount: 3, entry: [song('1'), song('2'), song('3')] }
    updateMutate.mockReset()
    replaceMutate.mockReset()
    playAlbum.mockReset()
})

describe('PlaylistDetailView', () => {
    it('shows the playlist name and an inline rename control beside the title', () => {
        const w = mountView()
        expect(w.find('.scaffold-title h1').text()).toBe('My Mix')
        expect(w.find('.rename-toggle').exists()).toBe(true)
    })

    it('inline rename submits the new name', async () => {
        const w = mountView()
        await w.find('.rename-toggle').trigger('click')
        const input = w.find('.rename-input input')
        await input.setValue('Road Trip')
        await input.trigger('keyup.enter')
        expect(updateMutate).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl1', name: 'Road Trip' }),
            expect.anything()
        )
    })

    it('entering edit mode shows the TrackEditList and Save/Cancel', async () => {
        const w = mountView()
        expect(w.find('.queue-edit-list').exists()).toBe(false)
        await w.find('.edit-toggle').trigger('click')
        expect(w.find('.queue-edit-list').exists()).toBe(true)
        expect(w.find('.edit-save').exists()).toBe(true)
        expect(w.find('.edit-cancel').exists()).toBe(true)
    })

    it('deleting in edit mode is local until Save', async () => {
        const w = mountView()
        await w.find('.edit-toggle').trigger('click')
        await w.find('[data-queue-index="1"] .delete-button').trigger('click')
        expect(replaceMutate).not.toHaveBeenCalled()
        expect(w.findAll('.queue-edit-list .queue-row')).toHaveLength(2)
    })

    it('Save persists the working order via replacePlaylistTracks', async () => {
        const w = mountView()
        await w.find('.edit-toggle').trigger('click')
        await w.find('[data-queue-index="0"] .delete-button').trigger('click')
        await w.find('.edit-save').trigger('click')
        expect(replaceMutate).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl1', songIds: ['2', '3'] }),
            expect.anything()
        )
    })

    it('Cancel discards local edits and leaves edit mode', async () => {
        const w = mountView()
        await w.find('.edit-toggle').trigger('click')
        await w.find('[data-queue-index="0"] .delete-button').trigger('click')
        await w.find('.edit-cancel').trigger('click')
        expect(w.find('.queue-edit-list').exists()).toBe(false)
        expect(replaceMutate).not.toHaveBeenCalled()
    })

    it('Play queues all entries', async () => {
        const w = mountView()
        await w.find('.play-all').trigger('click')
        expect(playAlbum).toHaveBeenCalledWith(playlist.value.entry)
    })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npx vitest run src/views/__tests__/PlaylistDetailView.spec.ts`
Expected: FAIL — the view lacks `.rename-toggle`, `.edit-toggle`, `.edit-save`, etc.

- [ ] **Step 3: Rewrite `PlaylistDetailView.vue`**

Replace `src/views/PlaylistDetailView.vue` with:

```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import TrackEditList from '@/components/layout/TrackEditList.vue'
import {
    usePlaylist,
    useUpdatePlaylist,
    useDeletePlaylist,
    useReplacePlaylistTracks
} from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { reorderQueue } from '@/utils/queueReorder'
import type { Song } from '@/types/subsonic'

const props = defineProps<{ id: string }>()
const router = useRouter()
const player = usePlayer()

const { data: playlist, isLoading, error } = usePlaylist(props.id)
const updatePlaylist = useUpdatePlaylist()
const deletePlaylist = useDeletePlaylist()
const replaceTracks = useReplacePlaylistTracks()

// Inline rename state.
const renaming = ref(false)
const renameValue = ref('')

// Batched edit state: a local working copy of the entries.
const editMode = ref(false)
const working = ref<Song[]>([])

const summary = computed(() => {
    if (!playlist.value) return ''
    const parts: string[] = []
    const n = editMode.value
        ? working.value.length
        : (playlist.value.songCount ?? playlist.value.entry?.length ?? 0)
    if (n > 0) parts.push(`${n} ${n === 1 ? 'song' : 'songs'}`)
    if (!editMode.value && playlist.value.duration)
        parts.push(`${Math.floor(playlist.value.duration / 60)} min`)
    return parts.join(' • ')
})

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const playAll = (): void => {
    if (playlist.value?.entry) player.playAlbum(playlist.value.entry)
}

const playFromTrack = (index: number): void => {
    if (playlist.value?.entry) player.playAlbum(playlist.value.entry, index)
}

// --- Inline rename ---
const openRename = (): void => {
    renameValue.value = playlist.value?.name ?? ''
    renaming.value = true
}
const cancelRename = (): void => {
    renaming.value = false
}
const submitRename = (): void => {
    const name = renameValue.value.trim()
    if (!name) return
    updatePlaylist.mutate(
        { playlistId: props.id, name },
        { onSuccess: () => (renaming.value = false) }
    )
}

// --- Batched track edit ---
const enterEdit = (): void => {
    working.value = [...(playlist.value?.entry ?? [])]
    editMode.value = true
}
const cancelEdit = (): void => {
    editMode.value = false
    working.value = []
}
const onReorder = (indices: number[], target: number): void => {
    working.value = reorderQueue(working.value, indices, target)
}
const onDelete = (indices: number[]): void => {
    const drop = new Set(indices)
    working.value = working.value.filter((_, i) => !drop.has(i))
}
const saveEdit = (): void => {
    replaceTracks.mutate(
        { playlistId: props.id, songIds: working.value.map((s) => s.id) },
        { onSuccess: () => cancelEdit() }
    )
}

const handleDelete = (): void => {
    deletePlaylist.mutate(props.id, { onSuccess: () => router.push({ name: 'playlists' }) })
}

// Leaving edit mode / switching playlists drops any working copy.
watch(() => props.id, cancelEdit)
</script>

<template>
    <div class="playlist-detail-view">
        <div class="back-row">
            <Button icon="pi pi-arrow-left" text rounded @click="router.back()" />
        </div>

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="error" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>{{ error.message }}</p>
        </div>

        <ContentScaffold v-else-if="playlist" :title="renaming ? '' : playlist.name" :summary="summary">
            <template #title-actions>
                <span v-if="renaming" class="rename-input">
                    <InputText
                        v-model="renameValue"
                        autofocus
                        @keyup.enter="submitRename"
                        @keyup.esc="cancelRename"
                    />
                    <Button icon="pi pi-check" text rounded size="small" @click="submitRename" />
                    <Button icon="pi pi-times" text rounded size="small" @click="cancelRename" />
                </span>
                <Button
                    v-else
                    class="rename-toggle"
                    icon="pi pi-pencil"
                    text
                    rounded
                    size="small"
                    v-tooltip.bottom="'Rename playlist'"
                    @click="openRename"
                />
            </template>

            <template #actions>
                <template v-if="editMode">
                    <Button
                        class="edit-save"
                        label="Save"
                        icon="pi pi-check"
                        :loading="replaceTracks.isPending.value"
                        @click="saveEdit"
                    />
                    <Button class="edit-cancel" label="Cancel" text severity="secondary" @click="cancelEdit" />
                </template>
                <template v-else>
                    <Button class="play-all" label="Play" icon="pi pi-play" @click="playAll" />
                    <Button
                        class="edit-toggle"
                        icon="pi pi-list"
                        text
                        rounded
                        :disabled="!playlist.entry || playlist.entry.length === 0"
                        v-tooltip.bottom="'Edit tracks'"
                        @click="enterEdit"
                    />
                    <Button
                        icon="pi pi-trash"
                        text
                        rounded
                        severity="danger"
                        v-tooltip.bottom="'Delete playlist'"
                        @click="handleDelete"
                    />
                </template>
            </template>

            <div class="playlist-scroll">
                <div class="playlist-body">
                    <TrackEditList
                        v-if="editMode"
                        :songs="working"
                        delete-label="Remove from playlist"
                        group="playlist"
                        @reorder="onReorder"
                        @delete="onDelete"
                    />

                    <DataTable
                        v-else-if="playlist.entry && playlist.entry.length > 0"
                        :value="playlist.entry"
                        stripedRows
                        @row-click="(e: any) => playFromTrack(e.index)"
                        class="track-table"
                        :rowClass="() => 'clickable-row'"
                    >
                        <Column header="#" style="width: 60px">
                            <template #body="{ index }">{{ index + 1 }}</template>
                        </Column>
                        <Column field="title" header="Title" />
                        <Column field="artist" header="Artist" />
                        <Column field="album" header="Album" />
                        <Column header="Duration" style="width: 80px">
                            <template #body="{ data }">{{ formatDuration(data.duration) }}</template>
                        </Column>
                    </DataTable>

                    <div v-else class="empty-tracks">
                        <p>This playlist is empty</p>
                    </div>
                </div>
            </div>
        </ContentScaffold>
    </div>
</template>

<style scoped>
.playlist-detail-view { height: 100%; display: flex; flex-direction: column; min-height: 0; }
.back-row { flex-shrink: 0; padding: 0.5rem 2rem 0; }
.loading, .error { display: flex; flex-direction: column; align-items: center; padding: 3rem; gap: 1rem; color: var(--app-text-secondary); }
.error { color: #ef4444; }
.playlist-scroll { height: 100%; overflow-y: auto; scrollbar-gutter: stable; }
.playlist-body { max-width: var(--app-content-max-width); margin: 0 auto; padding: 1rem; }
.rename-input { display: inline-flex; align-items: center; gap: 0.25rem; }
.track-table :deep(.clickable-row) { cursor: pointer; }
.track-table :deep(.clickable-row:hover) { background-color: var(--app-hover) !important; }
.empty-tracks { padding: 3rem; text-align: center; color: var(--app-text-secondary); }
</style>
```

Note: the removed per-row remove button and rename `Dialog` from the old view are intentional — removal now happens in edit mode, rename is inline.

- [ ] **Step 4: Run the test to verify it passes**

Run: `npx vitest run src/views/__tests__/PlaylistDetailView.spec.ts`
Expected: PASS (7 tests).

- [ ] **Step 5: Type-check**

Run: `npm run type-check`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add src/views/PlaylistDetailView.vue src/views/__tests__/PlaylistDetailView.spec.ts
git commit -m "feat(webui): inline rename and batched track editing on playlist detail"
```

---

### Task 5: `PlaylistCard` grid card with play button

**Files:**
- Create: `src/components/library/PlaylistCard.vue`
- Test: `src/components/library/__tests__/PlaylistCard.spec.ts` (create)

**Interfaces:**
- Consumes: `subsonicClient` (`isConfigured`, `getCoverArtUrl`, `getPlaylist`), `usePlayer` (`playAlbum`), a `Playlist` type from `@/types/subsonic`.
- Produces: `PlaylistCard` prop `playlist: Playlist`; a `router-link` to `{ name: 'playlist-detail', params: { id } }`; a `.card-play` button that fetches entries and plays them.

- [ ] **Step 1: Write the failing test**

Create `src/components/library/__tests__/PlaylistCard.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const getPlaylist = vi.fn()
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '', getPlaylist }
}))
const playAlbum = vi.fn()
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum }) }))

import PlaylistCard from '@/components/library/PlaylistCard.vue'

const stubs = { RouterLink: { template: '<a><slot /></a>' } }

beforeEach(() => {
    getPlaylist.mockReset()
    playAlbum.mockReset()
})

describe('PlaylistCard', () => {
    const playlist = { id: 'pl1', name: 'My Mix', songCount: 12 }

    it('renders the name and song count', () => {
        const w = mount(PlaylistCard, { props: { playlist }, global: { stubs } })
        expect(w.text()).toContain('My Mix')
        expect(w.text()).toContain('12 songs')
    })

    it('play fetches the playlist entries and plays them', async () => {
        getPlaylist.mockResolvedValue({ id: 'pl1', entry: [{ id: 's1' }, { id: 's2' }] })
        const w = mount(PlaylistCard, { props: { playlist }, global: { stubs } })
        await w.find('.card-play').trigger('click')
        await flushPromises()
        expect(getPlaylist).toHaveBeenCalledWith('pl1')
        expect(playAlbum).toHaveBeenCalledWith([{ id: 's1' }, { id: 's2' }])
    })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npx vitest run src/components/library/__tests__/PlaylistCard.spec.ts`
Expected: FAIL — `PlaylistCard.vue` does not exist.

- [ ] **Step 3: Create `PlaylistCard.vue`** (mirrors `AlbumCard.vue`)

```vue
<script setup lang="ts">
import { computed } from 'vue'
import type { Playlist } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { usePlayer } from '@/composables/usePlayer'

const props = defineProps<{ playlist: Playlist }>()
const player = usePlayer()

const coverUrl = computed(() => {
    const art = props.playlist.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 200)
})

const onPlay = async (event: Event): Promise<void> => {
    event.preventDefault()
    event.stopPropagation()
    const full = await subsonicClient.getPlaylist(props.playlist.id)
    if (full?.entry?.length) player.playAlbum(full.entry)
}
</script>

<template>
    <router-link
        :to="{ name: 'playlist-detail', params: { id: playlist.id } }"
        class="playlist-card"
    >
        <div class="card-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="playlist.name" draggable="false" />
            <div v-else class="cover-placeholder">
                <i class="pi pi-list" style="font-size: 2rem"></i>
            </div>
        </div>
        <div class="card-info">
            <div class="card-text">
                <div class="card-title">{{ playlist.name }}</div>
                <div class="card-subtitle">{{ playlist.songCount }} songs</div>
            </div>
            <button class="card-play" type="button" aria-label="Play playlist" @click="onPlay">
                <i class="pi pi-play"></i>
            </button>
        </div>
    </router-link>
</template>

<style scoped>
.playlist-card { position: relative; display: flex; flex-direction: column; text-decoration: none; color: inherit; border: 1px solid transparent; border-radius: 10px; padding: 0.5rem; transition: border-color 0.2s, background 0.2s, box-shadow 0.2s; cursor: pointer; }
.playlist-card:hover { border-color: var(--app-accent); background: var(--app-accent-soft); box-shadow: 0 6px 18px rgba(0, 0, 0, 0.12); }
.card-cover { width: 100%; aspect-ratio: 1; border-radius: 8px; overflow: hidden; background: var(--app-bg-subtle); }
.card-cover img { width: 100%; height: 100%; object-fit: cover; }
.cover-placeholder { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: rgba(255, 255, 255, 0.8); }
.card-info { display: flex; align-items: stretch; gap: 0.5rem; padding: 0.5rem 0.15rem 0.1rem; }
.card-text { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.card-title { font-size: 0.9rem; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.card-subtitle { font-size: 0.8rem; color: var(--app-text-secondary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.card-play { flex-shrink: 0; display: flex; align-items: center; justify-content: center; border: none; background: none; padding: 0 0.15rem; line-height: 1; color: var(--app-text-secondary); font-size: 2rem; cursor: pointer; opacity: 0; transition: opacity 0.15s, color 0.15s; }
.playlist-card:hover .card-play { opacity: 1; }
.card-play:hover { color: var(--app-accent); }
</style>
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npx vitest run src/components/library/__tests__/PlaylistCard.spec.ts`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add src/components/library/PlaylistCard.vue src/components/library/__tests__/PlaylistCard.spec.ts
git commit -m "feat(webui): add PlaylistCard with hover play button"
```

---

### Task 6: `PlaylistListView` list rows

**Files:**
- Create: `src/components/library/PlaylistListView.vue`
- Test: `src/components/library/__tests__/PlaylistListView.spec.ts` (create)

**Interfaces:**
- Consumes: `subsonicClient` (`isConfigured`, `getCoverArtUrl`, `getPlaylist`), `usePlayer` (`playAlbum`), `Playlist` type.
- Produces: `PlaylistListView` prop `playlists: Playlist[]`; each row is a `router-link` to `playlist-detail`; each row has a `.row-play` button that fetches + plays.

- [ ] **Step 1: Write the failing test**

Create `src/components/library/__tests__/PlaylistListView.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const getPlaylist = vi.fn()
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '', getPlaylist }
}))
const playAlbum = vi.fn()
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum }) }))

import PlaylistListView from '@/components/library/PlaylistListView.vue'

const stubs = { RouterLink: { template: '<a><slot /></a>' } }
const playlists = [
    { id: 'pl1', name: 'Mix One', songCount: 3 },
    { id: 'pl2', name: 'Mix Two', songCount: 5 }
]

beforeEach(() => {
    getPlaylist.mockReset()
    playAlbum.mockReset()
})

describe('PlaylistListView', () => {
    it('renders a row per playlist', () => {
        const w = mount(PlaylistListView, { props: { playlists }, global: { stubs } })
        expect(w.findAll('.playlist-row')).toHaveLength(2)
        expect(w.text()).toContain('Mix One')
        expect(w.text()).toContain('Mix Two')
    })

    it('the row play button fetches and plays that playlist', async () => {
        getPlaylist.mockResolvedValue({ id: 'pl2', entry: [{ id: 's9' }] })
        const w = mount(PlaylistListView, { props: { playlists }, global: { stubs } })
        await w.findAll('.playlist-row')[1].find('.row-play').trigger('click')
        await flushPromises()
        expect(getPlaylist).toHaveBeenCalledWith('pl2')
        expect(playAlbum).toHaveBeenCalledWith([{ id: 's9' }])
    })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npx vitest run src/components/library/__tests__/PlaylistListView.spec.ts`
Expected: FAIL — component does not exist.

- [ ] **Step 3: Create `PlaylistListView.vue`**

```vue
<script setup lang="ts">
import type { Playlist } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { usePlayer } from '@/composables/usePlayer'

defineProps<{ playlists: Playlist[] }>()
const player = usePlayer()

const coverUrl = (art?: string): string | null => {
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 48)
}

const play = async (event: Event, id: string): Promise<void> => {
    event.preventDefault()
    event.stopPropagation()
    const full = await subsonicClient.getPlaylist(id)
    if (full?.entry?.length) player.playAlbum(full.entry)
}
</script>

<template>
    <div class="playlist-list">
        <div class="list-header">
            <span class="col-cover"></span>
            <span class="col-name">Playlist</span>
            <span class="col-count">Songs</span>
            <span class="col-play"></span>
        </div>
        <router-link
            v-for="pl in playlists"
            :key="pl.id"
            :to="{ name: 'playlist-detail', params: { id: pl.id } }"
            class="playlist-row"
        >
            <span class="col-cover">
                <img v-if="coverUrl(pl.coverArt)" :src="coverUrl(pl.coverArt)!" alt="" />
                <i v-else class="pi pi-list"></i>
            </span>
            <span class="col-name">{{ pl.name }}</span>
            <span class="col-count">{{ pl.songCount }}</span>
            <span class="col-play">
                <button class="row-play" type="button" aria-label="Play playlist" @click="play($event, pl.id)">
                    <i class="pi pi-play"></i>
                </button>
            </span>
        </router-link>
    </div>
</template>

<style scoped>
.playlist-list { max-width: var(--app-content-max-width); margin: 0 auto; padding: 0 0.5rem; }
.list-header, .playlist-row { display: grid; grid-template-columns: 48px 1fr 4rem 3rem; align-items: center; gap: 1rem; padding: 0 0.5rem; }
.list-header { height: 36px; font-size: 0.75rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: var(--app-text-secondary); border-bottom: 1px solid var(--p-content-border-color); }
.list-header .col-count { text-align: right; }
.playlist-row { height: 56px; text-decoration: none; color: inherit; border-radius: 6px; }
.playlist-row:hover { background: var(--app-hover); }
.col-cover { width: 40px; height: 40px; border-radius: 4px; overflow: hidden; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: rgba(255, 255, 255, 0.85); }
.col-cover img { width: 100%; height: 100%; object-fit: cover; }
.col-name { min-width: 0; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.col-count { text-align: right; color: var(--app-text-secondary); font-size: 0.85rem; }
.col-play { display: flex; justify-content: center; }
.row-play { border: none; background: none; color: var(--app-text-secondary); font-size: 1.1rem; cursor: pointer; opacity: 0; transition: opacity 0.15s, color 0.15s; }
.playlist-row:hover .row-play { opacity: 1; }
.row-play:hover { color: var(--app-accent); }
</style>
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npx vitest run src/components/library/__tests__/PlaylistListView.spec.ts`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add src/components/library/PlaylistListView.vue src/components/library/__tests__/PlaylistListView.spec.ts
git commit -m "feat(webui): add PlaylistListView list layout"
```

---

### Task 7: `PlaylistsView` grid/list toggle

**Files:**
- Modify: `src/views/PlaylistsView.vue`
- Test: `src/views/__tests__/PlaylistsView.spec.ts` (create)

**Interfaces:**
- Consumes: `usePlaylists`, `useCreatePlaylist`, `PlaylistCard` (Task 5), `PlaylistListView` (Task 6), PrimeVue `SelectButton`, `ContentScaffold`.
- Produces: a layout toggle (grid default / list) persisted to the route query (`?view=list`), mirroring `LibraryView`.

- [ ] **Step 1: Write the failing test**

Create `src/views/__tests__/PlaylistsView.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'

const playlists = ref<any[]>([])
vi.mock('@/composables/useSubsonicQueries', () => ({
    usePlaylists: () => ({ data: playlists, isLoading: ref(false) }),
    useCreatePlaylist: () => ({ mutate: vi.fn(), isPending: ref(false) })
}))

const replaceSpy = vi.fn()
const route = { query: {} as Record<string, unknown> }
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace: replaceSpy })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '', getPlaylist: vi.fn() }
}))
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum: vi.fn() }) }))

import PlaylistsView from '@/views/PlaylistsView.vue'

const stubs = { RouterLink: { template: '<a><slot /></a>' } }

const mountView = () =>
    mount(PlaylistsView, { global: { plugins: [PrimeVue], directives: { tooltip: {} }, stubs } })

beforeEach(() => {
    playlists.value = [
        { id: 'pl1', name: 'Mix One', songCount: 3 },
        { id: 'pl2', name: 'Mix Two', songCount: 5 }
    ]
    replaceSpy.mockReset()
    route.query = {}
})

describe('PlaylistsView', () => {
    it('defaults to the grid layout with a card per playlist', () => {
        const w = mountView()
        expect(w.findAll('.playlist-card')).toHaveLength(2)
        expect(w.find('.playlist-list').exists()).toBe(false)
    })

    it('renders the list layout when the route query says so', () => {
        route.query = { view: 'list' }
        const w = mountView()
        expect(w.find('.playlist-list').exists()).toBe(true)
        expect(w.find('.playlist-card').exists()).toBe(false)
    })

    it('shows the count summary', () => {
        const w = mountView()
        expect(w.text()).toContain('2 playlists')
    })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npx vitest run src/views/__tests__/PlaylistsView.spec.ts`
Expected: FAIL — no `.playlist-card` (current view uses `.playlist-grid`/inline cards), no layout toggle.

- [ ] **Step 3: Rewrite `PlaylistsView.vue`**

Replace `src/views/PlaylistsView.vue` with:

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import SelectButton from 'primevue/selectbutton'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import PlaylistCard from '@/components/library/PlaylistCard.vue'
import PlaylistListView from '@/components/library/PlaylistListView.vue'
import { usePlaylists, useCreatePlaylist } from '@/composables/useSubsonicQueries'

type Layout = 'grid' | 'list'

const route = useRoute()
const router = useRouter()
const { data: playlists, isLoading } = usePlaylists()
const createPlaylist = useCreatePlaylist()

const showCreateDialog = ref(false)
const newPlaylistName = ref('')

const layoutOptions = [
    { label: 'List', value: 'list', icon: 'pi pi-list' },
    { label: 'Grid', value: 'grid', icon: 'pi pi-th-large' }
]

const layout = computed<Layout>({
    get: () => (route.query.view === 'list' ? 'list' : 'grid'),
    set: (v) => {
        const query = { ...route.query }
        if (v === 'list') query.view = 'list'
        else delete query.view
        router.replace({ query })
    }
})

const summary = computed(() => {
    const count = playlists.value?.length ?? 0
    if (count === 0) return ''
    return `${count} ${count === 1 ? 'playlist' : 'playlists'}`
})

const handleCreate = () => {
    if (!newPlaylistName.value.trim()) return
    createPlaylist.mutate(
        { name: newPlaylistName.value.trim() },
        {
            onSuccess: () => {
                showCreateDialog.value = false
                newPlaylistName.value = ''
            }
        }
    )
}
</script>

<template>
    <ContentScaffold title="Playlists" :summary="summary">
        <template #actions>
            <SelectButton
                v-model="layout"
                :options="layoutOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
                dataKey="value"
                aria-label="Layout"
            >
                <template #option="slotProps">
                    <i :class="slotProps.option.icon"></i>
                </template>
            </SelectButton>
            <Button label="Create Playlist" icon="pi pi-plus" @click="showCreateDialog = true" />
        </template>

        <div class="playlists-scroll">
            <div v-if="isLoading" class="loading">
                <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
            </div>

            <template v-else-if="playlists && playlists.length > 0">
                <PlaylistListView v-if="layout === 'list'" :playlists="playlists" />
                <div v-else class="playlist-grid">
                    <PlaylistCard v-for="pl in playlists" :key="pl.id" :playlist="pl" />
                </div>
            </template>

            <div v-else class="empty-state">
                <i class="pi pi-list" style="font-size: 3rem"></i>
                <p>No playlists</p>
            </div>
        </div>

        <Dialog
            v-model:visible="showCreateDialog"
            header="Create Playlist"
            :modal="true"
            :style="{ width: '400px' }"
        >
            <div class="create-form">
                <InputText
                    v-model="newPlaylistName"
                    placeholder="Playlist name"
                    class="w-full"
                    @keyup.enter="handleCreate"
                />
            </div>
            <template #footer>
                <Button label="Cancel" text @click="showCreateDialog = false" />
                <Button label="Create" :loading="createPlaylist.isPending.value" @click="handleCreate" />
            </template>
        </Dialog>
    </ContentScaffold>
</template>

<style scoped>
.playlists-scroll { height: 100%; overflow-y: auto; scrollbar-gutter: stable; }
.loading { display: flex; justify-content: center; padding: 3rem; color: var(--app-text-secondary); }
.playlist-grid { max-width: var(--app-content-max-width); margin: 0 auto; padding: 1rem; display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 2rem; }
.empty-state { display: flex; flex-direction: column; align-items: center; padding: 4rem; gap: 1rem; color: var(--app-text-secondary); }
.create-form { padding: 1rem 0; }
</style>
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npx vitest run src/views/__tests__/PlaylistsView.spec.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Type-check**

Run: `npm run type-check`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add src/views/PlaylistsView.vue src/views/__tests__/PlaylistsView.spec.ts
git commit -m "feat(webui): add grid/list toggle to playlists view"
```

---

### Task 8: Full verification pass

**Files:** none (verification only)

- [ ] **Step 1: Run the full unit test suite**

Run: `npm test`
Expected: PASS — all suites green, including the refactored `QueueView.spec.ts` and the new playlist suites.

- [ ] **Step 2: Type-check and lint**

Run: `npm run type-check`
Expected: PASS (no type errors). (This project has no separate lint script; `npm test` also runs vue-tsc.) Fix any issues surfaced.

- [ ] **Step 3: Manual smoke check (optional but recommended)**

Run the app (`npm run dev` or the project's `/run` flow) and verify:
- Playlists index toggles grid ↔ list; card/row play buttons start playback; clicking opens the detail.
- Detail: inline rename beside the title (Enter saves, Esc cancels); Edit tracks → reorder via drag + multi-select delete → Save persists; Cancel discards.
- Queue Now Playing edit mode still reorders/deletes and accepts album drops.

- [ ] **Step 4: Final commit if any fixes were made**

```bash
git add -A
git commit -m "chore(webui): fixes from verification pass"
```

---

## Notes / open item

- **Two pencil icons on the detail view:** the inline rename pencil (`pi pi-pencil` beside the title) and the edit-tracks toggle. This plan uses `pi pi-list` for the edit-tracks toggle to disambiguate them. If review prefers the queue's `pi pi-pencil` for edit-tracks instead, swap the `edit-toggle` icon in Task 4, Step 3.
