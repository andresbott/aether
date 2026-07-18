# Unified Play Experience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the read-mode Play / Add-to-queue / Star actions out of the `ContentScaffold` action bar and into the `HeroHeader` on every documented detail view, behind one shared `HeroActions` component.

**Architecture:** A new presentational `HeroActions.vue` renders the Play/Queue/Star button set (the read-mode analogue of `EditActionBar`). `HeroHeader` gains a read-mode-only `#actions` slot in its identity column. Each detail view composes `HeroActions` into that slot and drops the same buttons from its scaffold `#actions`. Artist play/queue gathers songs on demand via `getAlbum` because `getArtist` returns albums without songs.

**Tech Stack:** Vue 3 `<script setup>` + TypeScript, PrimeVue 4, Vitest + @vue/test-utils.

**Design spec:** `webui/docs/superpowers/specs/2026-07-18-unified-play-experience-design.md`

## Global Constraints

- No backwards-compatibility code — change shapes directly; no compat branches.
- Frontend indentation is **4 spaces** (match surrounding files).
- Git: **single one-line commit message**, **no `Co-Authored-By` line**, run `git add` as a separate step before `git commit`.
- Run a single spec file with `npx vitest run <path>`; the full gate is `npm test` (`vue-tsc --noEmit && vitest run`).
- After removing the last `<Button>` from a view's template, also remove its now-unused `import Button from 'primevue/button'` (vue-tsc flags unused imports).
- Star applies only to starrable entities (albums, artists); Add-to-queue only to track-list entities (albums, artists, playlists); radio = Play only.

---

### Task 1: `HeroActions` component

**Files:**
- Create: `webui/src/components/layout/HeroActions.vue`
- Test: `webui/src/components/layout/__tests__/HeroActions.spec.ts`

**Interfaces:**
- Consumes: nothing (leaf presentational component).
- Produces:
  - Props: `playLabel?: string = 'Play'`, `playDisabled?: boolean = false`, `busy?: boolean = false`, `canQueue?: boolean = false`, `canStar?: boolean = false`, `starred?: boolean = false`.
  - Emits: `play`, `queue`, `star`.
  - Stable classes for hosts/tests: `.hero-action-play`, `.hero-action-queue`, `.hero-action-star`.

- [ ] **Step 1: Write the failing test**

Create `webui/src/components/layout/__tests__/HeroActions.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import HeroActions from '@/components/layout/HeroActions.vue'

const mountActions = (props: Record<string, unknown> = {}) =>
    mount(HeroActions, {
        props,
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

describe('HeroActions', () => {
    it('always renders Play and emits play on click', async () => {
        const w = mountActions()
        const play = w.find('.hero-action-play')
        expect(play.exists()).toBe(true)
        await play.trigger('click')
        expect(w.emitted('play')).toHaveLength(1)
    })

    it('omits Add to queue and Star by default', () => {
        const w = mountActions()
        expect(w.find('.hero-action-queue').exists()).toBe(false)
        expect(w.find('.hero-action-star').exists()).toBe(false)
    })

    it('renders Add to queue when canQueue and emits queue', async () => {
        const w = mountActions({ canQueue: true })
        const q = w.find('.hero-action-queue')
        expect(q.exists()).toBe(true)
        await q.trigger('click')
        expect(w.emitted('queue')).toHaveLength(1)
    })

    it('renders Star when canStar, reflects starred, and emits star', async () => {
        const w = mountActions({ canStar: true, starred: true })
        const star = w.find('.hero-action-star')
        expect(star.exists()).toBe(true)
        expect(star.find('.pi-star-fill').exists()).toBe(true)
        await star.trigger('click')
        expect(w.emitted('star')).toHaveLength(1)
    })

    it('shows the outline star when not starred', () => {
        const w = mountActions({ canStar: true, starred: false })
        expect(w.find('.hero-action-star .pi-star').exists()).toBe(true)
        expect(w.find('.hero-action-star .pi-star-fill').exists()).toBe(false)
    })

    it('disables Play when playDisabled', () => {
        const w = mountActions({ playDisabled: true })
        expect(w.find('.hero-action-play').attributes('disabled')).toBeDefined()
    })

    it('puts Play in a loading state when busy', () => {
        const w = mountActions({ busy: true })
        expect(w.find('.hero-action-play').classes()).toContain('p-button-loading')
    })

    it('uses a custom play label', () => {
        const w = mountActions({ playLabel: 'Play all' })
        expect(w.find('.hero-action-play').text()).toContain('Play all')
    })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx vitest run src/components/layout/__tests__/HeroActions.spec.ts`
Expected: FAIL — cannot resolve `@/components/layout/HeroActions.vue`.

- [ ] **Step 3: Write minimal implementation**

Create `webui/src/components/layout/HeroActions.vue`:

```vue
<script setup lang="ts">
import Button from 'primevue/button'

withDefaults(
    defineProps<{
        playLabel?: string
        playDisabled?: boolean
        busy?: boolean
        canQueue?: boolean
        canStar?: boolean
        starred?: boolean
    }>(),
    {
        playLabel: 'Play',
        playDisabled: false,
        busy: false,
        canQueue: false,
        canStar: false,
        starred: false
    }
)

const emit = defineEmits<{
    (e: 'play'): void
    (e: 'queue'): void
    (e: 'star'): void
}>()
</script>

<template>
    <div class="hero-actions">
        <Button
            class="hero-action-play"
            :label="playLabel"
            icon="pi pi-play"
            :disabled="playDisabled"
            :loading="busy"
            @click="emit('play')"
        />
        <Button
            v-if="canQueue"
            class="hero-action-queue"
            label="Add to queue"
            icon="pi pi-plus"
            severity="secondary"
            text
            @click="emit('queue')"
        />
        <Button
            v-if="canStar"
            class="hero-action-star"
            :icon="starred ? 'pi pi-star-fill' : 'pi pi-star'"
            text
            rounded
            v-tooltip.bottom="'Toggle star'"
            @click="emit('star')"
        />
    </div>
</template>

<style scoped>
.hero-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.4rem;
}
</style>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx vitest run src/components/layout/__tests__/HeroActions.spec.ts`
Expected: PASS (8 tests).

- [ ] **Step 5: Commit**

```bash
git add src/components/layout/HeroActions.vue src/components/layout/__tests__/HeroActions.spec.ts
git commit -m "feat(webui): add HeroActions component for hero play/queue/star"
```

---

### Task 2: `HeroHeader` read-mode `#actions` slot

**Files:**
- Modify: `webui/src/components/layout/HeroHeader.vue`
- Test: `webui/src/components/layout/__tests__/HeroHeader.spec.ts`

**Interfaces:**
- Consumes: existing `editing` prop.
- Produces: a new `#actions` slot rendered inside `.hero-info`, present **only when `!editing`**.

- [ ] **Step 1: Write the failing test**

Add these two tests inside the `describe('HeroHeader', …)` block in `webui/src/components/layout/__tests__/HeroHeader.spec.ts` (after the existing `renders both read and edit slot content` test):

```ts
    it('renders the #actions slot in read mode', () => {
        const w = mountHero(
            { editing: false },
            { actions: '<button class="hero-act-probe">go</button>' }
        )
        expect(w.find('.hero-act-probe').exists()).toBe(true)
    })

    it('hides the #actions slot in edit mode', () => {
        const w = mountHero(
            { editing: true },
            { actions: '<button class="hero-act-probe">go</button>' }
        )
        expect(w.find('.hero-act-probe').exists()).toBe(false)
    })
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx vitest run src/components/layout/__tests__/HeroHeader.spec.ts`
Expected: FAIL — `hero-act-probe` not found in read mode (slot not yet rendered).

- [ ] **Step 3: Write minimal implementation**

In `webui/src/components/layout/HeroHeader.vue`, in the `.hero-info` block, add the actions slot after the `edit-only` div. Change:

```vue
        <div class="hero-info">
            <span class="eyebrow">{{ eyebrow }}</span>
            <div class="read-only"><slot name="read" /></div>
            <div class="edit-only"><slot name="edit" /></div>
        </div>
```

to:

```vue
        <div class="hero-info">
            <span class="eyebrow">{{ eyebrow }}</span>
            <div class="read-only"><slot name="read" /></div>
            <div class="edit-only"><slot name="edit" /></div>
            <div v-if="!editing" class="hero-actions-slot"><slot name="actions" /></div>
        </div>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx vitest run src/components/layout/__tests__/HeroHeader.spec.ts`
Expected: PASS (all tests, including the two new ones).

- [ ] **Step 5: Commit**

```bash
git add src/components/layout/HeroHeader.vue src/components/layout/__tests__/HeroHeader.spec.ts
git commit -m "feat(webui): add read-mode actions slot to HeroHeader"
```

---

### Task 3: AlbumView — relocate Play/Queue/Star into the hero

**Files:**
- Modify: `webui/src/views/AlbumView.vue`
- Test: `webui/src/views/__tests__/AlbumView.spec.ts`

**Interfaces:**
- Consumes: `HeroActions` (Task 1), `HeroHeader` `#actions` slot (Task 2).
- Produces: no new exports; existing `playAlbum`, `addToQueue`, `handleStar` handlers now wired to `HeroActions`.

- [ ] **Step 1: Write the failing test**

In `webui/src/views/__tests__/AlbumView.spec.ts`, replace the player and query mocks so the queue/star handlers are assertable.

Replace:

```ts
vi.mock('@/composables/useSubsonicQueries', () => ({
    useAlbum: () => ({ data: albumData, isLoading: ref(false), error: ref(null) }),
    useToggleStar: () => ({ mutate: vi.fn() })
}))
```

with:

```ts
const toggleStarMutate = vi.fn()
vi.mock('@/composables/useSubsonicQueries', () => ({
    useAlbum: () => ({ data: albumData, isLoading: ref(false), error: ref(null) }),
    useToggleStar: () => ({ mutate: toggleStarMutate })
}))
```

Replace:

```ts
const playAlbum = vi.fn()
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ playAlbum, addMultipleToQueue: vi.fn(), currentTrack: ref(null) })
}))
```

with:

```ts
const playAlbum = vi.fn()
const addMultipleToQueue = vi.fn()
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ playAlbum, addMultipleToQueue, currentTrack: ref(null) })
}))
```

In `beforeEach`, after `playAlbum.mockClear()`, add:

```ts
    addMultipleToQueue.mockClear()
    toggleStarMutate.mockClear()
```

Append a new describe block at the end of the file:

```ts
describe('AlbumView hero actions', () => {
    it('Play in the hero plays the album', async () => {
        const w = mountView()
        await w.find('.hero-action-play').trigger('click')
        expect(playAlbum).toHaveBeenCalledWith(album.song)
    })

    it('Add to queue in the hero enqueues the album songs', async () => {
        const w = mountView()
        await w.find('.hero-action-queue').trigger('click')
        expect(addMultipleToQueue).toHaveBeenCalledWith(album.song)
    })

    it('Star in the hero toggles the album star', async () => {
        const w = mountView()
        await w.find('.hero-action-star').trigger('click')
        expect(toggleStarMutate).toHaveBeenCalledWith({ id: 'al1', starred: false })
    })

    it('keeps only the drag handle in the scaffold actions', () => {
        const w = mountView()
        expect(w.find('.album-drag-handle').exists()).toBe(true)
        // Hero owns play/queue/star now; they render inside the HeroHeader.
        expect(w.find('.hero-header .hero-action-play').exists()).toBe(true)
    })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx vitest run src/views/__tests__/AlbumView.spec.ts`
Expected: FAIL — `.hero-action-play` / `.hero-action-queue` / `.hero-action-star` not found (still in scaffold as plain Buttons).

- [ ] **Step 3: Write minimal implementation**

In `webui/src/views/AlbumView.vue`:

1. Remove `import Button from 'primevue/button'` and add the HeroActions import next to the HeroHeader import:

```ts
import HeroActions from '@/components/layout/HeroActions.vue'
```

2. Replace the scaffold `#actions` block:

```vue
            <template #actions>
                <Button label="Play" icon="pi pi-play" @click="playAlbum" />
                <Button
                    label="Add to Queue"
                    icon="pi pi-plus"
                    severity="secondary"
                    text
                    @click="addToQueue"
                />
                <Button
                    :icon="album?.starred ? 'pi pi-star-fill' : 'pi pi-star'"
                    text
                    rounded
                    @click="handleStar"
                />
                <span
                    class="album-drag-handle"
                    draggable="true"
                    v-tooltip.bottom="'Drag album to queue'"
                    @dragstart="onAlbumDragStart"
                    @dragend="albumDrag.end"
                >
                    <i class="pi pi-bars"></i>
                </span>
            </template>
```

with (drag handle stays; the three buttons move to the hero):

```vue
            <template #actions>
                <span
                    class="album-drag-handle"
                    draggable="true"
                    v-tooltip.bottom="'Drag album to queue'"
                    @dragstart="onAlbumDragStart"
                    @dragend="albumDrag.end"
                >
                    <i class="pi pi-bars"></i>
                </span>
            </template>
```

3. Add an `#actions` slot to the `<HeroHeader>` (after the closing `</template>` of the `#read` slot, before `</HeroHeader>`):

```vue
                        <template #actions>
                            <HeroActions
                                :play-disabled="!album.song?.length"
                                can-queue
                                can-star
                                :starred="!!album.starred"
                                @play="playAlbum"
                                @queue="addToQueue"
                                @star="handleStar"
                            />
                        </template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx vitest run src/views/__tests__/AlbumView.spec.ts`
Expected: PASS (existing drag/disc/selection tests plus the 4 new hero-action tests).

- [ ] **Step 5: Commit**

```bash
git add src/views/AlbumView.vue src/views/__tests__/AlbumView.spec.ts
git commit -m "feat(webui): move AlbumView play/queue/star into the hero"
```

---

### Task 4: PlaylistDetailView — Play + Add-to-queue in the hero

**Files:**
- Modify: `webui/src/views/PlaylistDetailView.vue`
- Test: `webui/src/views/__tests__/PlaylistDetailView.spec.ts`

**Interfaces:**
- Consumes: `HeroActions`, `HeroHeader` `#actions` slot.
- Produces: new `queueAll()` handler (`player.addMultipleToQueue(working.value)`); existing `playAll()` re-wired to the hero.

- [ ] **Step 1: Write the failing test**

In `webui/src/views/__tests__/PlaylistDetailView.spec.ts`:

Replace the player mock:

```ts
const playAlbum = vi.fn()
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum }) }))
```

with:

```ts
const playAlbum = vi.fn()
const addMultipleToQueue = vi.fn()
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum, addMultipleToQueue }) }))
```

In `beforeEach`, after `playAlbum.mockReset()`, add:

```ts
    addMultipleToQueue.mockClear()
```

Update the selectors that referenced `.play-all` (the Play button now lives in the hero as `.hero-action-play`). Change the test `view mode shows Play + pencil and no Save; edit mode shows Save/Cancel and hides Play` to:

```ts
    it('view mode shows Play + pencil and no Save; edit mode shows Save/Cancel and hides Play', async () => {
        const w = mountView()
        expect(w.find('.hero-action-play').exists()).toBe(true)
        expect(w.find('.edit-action-edit').exists()).toBe(true)
        expect(w.find('.edit-action-save').exists()).toBe(false)

        await enterEdit(w)
        expect(w.find('.hero-header').classes()).toContain('editing')
        expect(w.find('.edit-action-save').exists()).toBe(true)
        expect(w.find('.edit-action-cancel').exists()).toBe(true)
        expect(w.find('.edit-action-delete').exists()).toBe(true)
        expect(w.find('.hero-action-play').exists()).toBe(false)
    })
```

In `editing the name and saving persists it and exits edit mode`, change the final assertion `expect(w.find('.play-all').exists()).toBe(true)` to:

```ts
        expect(w.find('.hero-action-play').exists()).toBe(true)
```

Replace the two Play tests:

```ts
    it('Play queues the current on-screen list', async () => {
        const w = mountView()
        await w.find('.play-all').trigger('click')
        expect(playAlbum).toHaveBeenCalledWith([song('1'), song('2'), song('3')])
    })

    it('Play reflects local edits before saving', async () => {
        const w = mountView()
        await w.find('[data-queue-index="0"] .delete-button').trigger('click')
        await w.find('.play-all').trigger('click')
        expect(playAlbum).toHaveBeenCalledWith([song('2'), song('3')])
    })
```

with:

```ts
    it('Play queues the current on-screen list', async () => {
        const w = mountView()
        await w.find('.hero-action-play').trigger('click')
        expect(playAlbum).toHaveBeenCalledWith([song('1'), song('2'), song('3')])
    })

    it('Play reflects local edits before saving', async () => {
        const w = mountView()
        await w.find('[data-queue-index="0"] .delete-button').trigger('click')
        await w.find('.hero-action-play').trigger('click')
        expect(playAlbum).toHaveBeenCalledWith([song('2'), song('3')])
    })

    it('Add to queue enqueues the current on-screen list', async () => {
        const w = mountView()
        await w.find('.hero-action-queue').trigger('click')
        expect(addMultipleToQueue).toHaveBeenCalledWith([song('1'), song('2'), song('3')])
    })
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx vitest run src/views/__tests__/PlaylistDetailView.spec.ts`
Expected: FAIL — `.hero-action-play` / `.hero-action-queue` not found.

- [ ] **Step 3: Write minimal implementation**

In `webui/src/views/PlaylistDetailView.vue`:

1. Remove `import Button from 'primevue/button'` and add:

```ts
import HeroActions from '@/components/layout/HeroActions.vue'
```

2. Add a `queueAll` handler next to `playAll`:

```ts
const queueAll = (): void => {
    if (working.value.length) player.addMultipleToQueue(working.value)
}
```

3. Replace the `EditActionBar` block (drop the `#read-actions` Play slot):

```vue
                <EditActionBar
                    v-model:editing="editing"
                    :save-disabled="savePending || !valid"
                    :saving="savePending"
                    :dirty="dirty"
                    delete-header="Delete playlist?"
                    :delete-message="`Delete playlist &quot;${playlist.name}&quot;? This cannot be undone.`"
                    @save="saveEdit"
                    @cancel="cancelEdit"
                    @delete="handleDelete"
                >
                    <template #read-actions>
                        <Button
                            class="play-all"
                            label="Play"
                            icon="pi pi-play"
                            :disabled="working.length === 0"
                            @click="playAll"
                        />
                    </template>
                </EditActionBar>
```

with:

```vue
                <EditActionBar
                    v-model:editing="editing"
                    :save-disabled="savePending || !valid"
                    :saving="savePending"
                    :dirty="dirty"
                    delete-header="Delete playlist?"
                    :delete-message="`Delete playlist &quot;${playlist.name}&quot;? This cannot be undone.`"
                    @save="saveEdit"
                    @cancel="cancelEdit"
                    @delete="handleDelete"
                />
```

4. Add an `#actions` slot to `<HeroHeader>` (after the `#edit` slot's closing `</template>`, before `</HeroHeader>`):

```vue
                        <template #actions>
                            <HeroActions
                                :play-disabled="working.length === 0"
                                can-queue
                                @play="playAll"
                                @queue="queueAll"
                            />
                        </template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx vitest run src/views/__tests__/PlaylistDetailView.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/views/PlaylistDetailView.vue src/views/__tests__/PlaylistDetailView.spec.ts
git commit -m "feat(webui): move PlaylistDetailView play into the hero and add queue"
```

---

### Task 5: RadioStationDetailView — Play in the hero

**Files:**
- Modify: `webui/src/views/RadioStationDetailView.vue`
- Test: `webui/src/views/__tests__/RadioStationDetailView.spec.ts`

**Interfaces:**
- Consumes: `HeroActions`, `HeroHeader` `#actions` slot.
- Produces: existing `onPlay()` re-wired to the hero; radio hero is Play-only (`canQueue`/`canStar` left default false).

- [ ] **Step 1: Write the failing test**

In `webui/src/views/__tests__/RadioStationDetailView.spec.ts`, update the two selectors that used `.play-station`:

In `existing station opens read-only with Play + pencil`:

```ts
    it('existing station opens read-only with Play + pencil', () => {
        const w = mountView({ id: 's1' })
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('')
        expect(w.find('.hero-name').text()).toBe('Jazz FM')
        expect(w.find('.hero-action-play').exists()).toBe(true)
        expect(w.find('.edit-action-edit').exists()).toBe(true)
        expect(w.find('.edit-action-save').exists()).toBe(false)
    })
```

In `read mode: Play enqueues the station`:

```ts
    it('read mode: Play enqueues the station', async () => {
        const w = mountView({ id: 's1' })
        await w.find('.hero-action-play').trigger('click')
        expect(playNow).toHaveBeenCalledWith({ id: 's1' })
    })
```

Add a test asserting create mode has no hero actions (append inside the describe block):

```ts
    it('create mode: has no hero play action', () => {
        const w = mountView({ create: true })
        expect(w.find('.hero-action-play').exists()).toBe(false)
    })
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx vitest run src/views/__tests__/RadioStationDetailView.spec.ts`
Expected: FAIL — `.hero-action-play` not found (Play still `.play-station` in the scaffold).

- [ ] **Step 3: Write minimal implementation**

In `webui/src/views/RadioStationDetailView.vue`:

1. Remove `import Button from 'primevue/button'` and add:

```ts
import HeroActions from '@/components/layout/HeroActions.vue'
```

2. Replace the `EditActionBar` block (drop the `#read-actions` Play slot):

```vue
                <EditActionBar
                    v-model:editing="editing"
                    :can-delete="!create"
                    :save-disabled="!valid"
                    :saving="submitting"
                    :save-tooltip="create ? 'Create' : 'Save'"
                    :dirty="dirty"
                    delete-header="Delete station?"
                    :delete-message="`Delete station &quot;${station?.name}&quot;? This cannot be undone.`"
                    @save="onSubmit"
                    @cancel="onCancel"
                    @delete="onDelete"
                >
                    <template #read-actions>
                        <Button
                            class="play-station"
                            label="Play"
                            icon="pi pi-play"
                            @click="onPlay"
                        />
                    </template>
                </EditActionBar>
```

with:

```vue
                <EditActionBar
                    v-model:editing="editing"
                    :can-delete="!create"
                    :save-disabled="!valid"
                    :saving="submitting"
                    :save-tooltip="create ? 'Create' : 'Save'"
                    :dirty="dirty"
                    delete-header="Delete station?"
                    :delete-message="`Delete station &quot;${station?.name}&quot;? This cannot be undone.`"
                    @save="onSubmit"
                    @cancel="onCancel"
                    @delete="onDelete"
                />
```

3. Add an `#actions` slot to `<HeroHeader>` (after the `#edit` slot's closing `</template>`, before `</HeroHeader>`):

```vue
                        <template #actions>
                            <HeroActions @play="onPlay" />
                        </template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx vitest run src/views/__tests__/RadioStationDetailView.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/views/RadioStationDetailView.vue src/views/__tests__/RadioStationDetailView.spec.ts
git commit -m "feat(webui): move RadioStationDetailView play into the hero"
```

---

### Task 6: ArtistView — Play/Queue (gather songs) + Star in the hero

**Files:**
- Modify: `webui/src/views/ArtistView.vue`
- Test: `webui/src/views/__tests__/ArtistView.spec.ts`

**Interfaces:**
- Consumes: `HeroActions`, `HeroHeader` `#actions` slot, `usePlayer` (`playAlbum`, `addMultipleToQueue`), `subsonicClient.getAlbum(id)` (returns `AlbumWithSongs | null`).
- Produces: `gathering: Ref<boolean>` driving `HeroActions` `busy`; `gatherSongs()`, `onPlay()`, `onQueue()` handlers.

- [ ] **Step 1: Write the failing test**

In `webui/src/views/__tests__/ArtistView.spec.ts`:

Change the test-utils import to include `flushPromises`:

```ts
import { mount, flushPromises } from '@vue/test-utils'
```

Extend the subsonic mock to serve album songs. Replace:

```ts
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size?: number) => `/cover/${id}?size=${size}`
    }
}))
```

with:

```ts
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size?: number) => `/cover/${id}?size=${size}`,
        getAlbum: (id: string) =>
            Promise.resolve(
                (
                    {
                        a1: { id: 'a1', song: [{ id: 's1' }, { id: 's2' }] },
                        a2: { id: 'a2', song: [{ id: 's3' }] }
                    } as Record<string, unknown>
                )[id] ?? null
            )
    }
}))
```

Add a player mock (ArtistView now uses `usePlayer`). Add after the subsonic mock:

```ts
const playAlbum = vi.fn()
const addMultipleToQueue = vi.fn()
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ playAlbum, addMultipleToQueue })
}))
```

In `beforeEach`, after `coverMutate.mockClear()`, add:

```ts
    playAlbum.mockClear()
    addMultipleToQueue.mockClear()
```

Replace the star test:

```ts
    it('toggles star on click', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, starred: undefined }
        const w = mountView()
        const starBtn = w.findAll('button').find((b) => b.attributes('title') === 'Toggle star')
        await starBtn!.trigger('click')
        expect(toggleStarMutate).toHaveBeenCalledWith({ id: 'ar-1', starred: false })
    })
```

with:

```ts
    it('toggles star on click', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, starred: undefined }
        const w = mountView()
        await w.find('.hero-action-star').trigger('click')
        expect(toggleStarMutate).toHaveBeenCalledWith({ id: 'ar-1', starred: false })
    })

    it('Play gathers each album\'s songs then plays them in order', async () => {
        artist.value = {
            id: 'ar-1',
            name: 'Nirvana',
            album: [
                { id: 'a1', name: 'A1' },
                { id: 'a2', name: 'A2' }
            ]
        }
        const w = mountView()
        await w.find('.hero-action-play').trigger('click')
        await flushPromises()
        expect(playAlbum).toHaveBeenCalledWith([{ id: 's1' }, { id: 's2' }, { id: 's3' }])
    })

    it('Add to queue gathers songs then enqueues them', async () => {
        artist.value = {
            id: 'ar-1',
            name: 'Nirvana',
            album: [
                { id: 'a1', name: 'A1' },
                { id: 'a2', name: 'A2' }
            ]
        }
        const w = mountView()
        await w.find('.hero-action-queue').trigger('click')
        await flushPromises()
        expect(addMultipleToQueue).toHaveBeenCalledWith([{ id: 's1' }, { id: 's2' }, { id: 's3' }])
    })

    it('hides the hero actions in edit mode', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        expect(w.find('.hero-action-star').exists()).toBe(true)
        await enterEdit(w)
        expect(w.find('.hero-action-star').exists()).toBe(false)
    })
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx vitest run src/views/__tests__/ArtistView.spec.ts`
Expected: FAIL — `.hero-action-star` / `.hero-action-play` / `.hero-action-queue` not found (star still in `#read-actions`, no play/queue).

- [ ] **Step 3: Write minimal implementation**

In `webui/src/views/ArtistView.vue`:

1. Remove `import Button from 'primevue/button'`. Add these imports:

```ts
import HeroActions from '@/components/layout/HeroActions.vue'
import { usePlayer } from '@/composables/usePlayer'
import type { Song } from '@/types/subsonic'
```

2. Add the player + gather logic in `<script setup>` (after `const updateCover = useUpdateArtistCover()`):

```ts
const player = usePlayer()
const gathering = ref(false)

// getArtist returns albums without their songs, so gather each album's songs on
// demand before playing/queuing the whole discography.
async function gatherSongs(): Promise<Song[]> {
    const results = await Promise.all(sortedAlbums.value.map((a) => subsonicClient.getAlbum(a.id)))
    return results.flatMap((al) => al?.song ?? [])
}

const onPlay = async (): Promise<void> => {
    if (gathering.value) return
    gathering.value = true
    try {
        const songs = await gatherSongs()
        if (songs.length) player.playAlbum(songs)
    } finally {
        gathering.value = false
    }
}

const onQueue = async (): Promise<void> => {
    if (gathering.value) return
    gathering.value = true
    try {
        const songs = await gatherSongs()
        if (songs.length) player.addMultipleToQueue(songs)
    } finally {
        gathering.value = false
    }
}
```

Note: `sortedAlbums` is declared later in the current file but this is fine — it is a `computed` referenced only inside the async handlers at call time, not at setup evaluation. If your linter prefers declaration order, move the `gatherSongs`/`onPlay`/`onQueue` block to just after the existing `sortedAlbums` computed instead.

3. Replace the `EditActionBar` block (drop the `#read-actions` Star slot):

```vue
                <EditActionBar
                    v-model:editing="editing"
                    :can-delete="false"
                    :save-disabled="!dirty"
                    :saving="updateCover.isPending.value"
                    :dirty="dirty"
                    @save="saveEdit"
                    @cancel="cancelEdit"
                >
                    <template #read-actions>
                        <Button
                            :icon="artist?.starred ? 'pi pi-star-fill' : 'pi pi-star'"
                            text
                            rounded
                            title="Toggle star"
                            @click="handleStar"
                        />
                    </template>
                </EditActionBar>
```

with:

```vue
                <EditActionBar
                    v-model:editing="editing"
                    :can-delete="false"
                    :save-disabled="!dirty"
                    :saving="updateCover.isPending.value"
                    :dirty="dirty"
                    @save="saveEdit"
                    @cancel="cancelEdit"
                />
```

4. Add an `#actions` slot to `<HeroHeader>` (after the `#read` slot's closing `</template>`, before `</HeroHeader>`):

```vue
                        <template #actions>
                            <HeroActions
                                can-queue
                                can-star
                                :starred="!!artist.starred"
                                :busy="gathering"
                                @play="onPlay"
                                @queue="onQueue"
                                @star="handleStar"
                            />
                        </template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx vitest run src/views/__tests__/ArtistView.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/views/ArtistView.vue src/views/__tests__/ArtistView.spec.ts
git commit -m "feat(webui): add hero play/queue/star to ArtistView"
```

---

### Task 7: Registry doc + cross-link

**Files:**
- Create: `docs/architecture/unified-play-experience.md` (repo root `docs/`, i.e. `/aether/docs/architecture/`)
- Modify: `docs/architecture/main-content-view-layout.md`

**Interfaces:**
- Consumes: nothing (documentation).
- Produces: canonical behavior registry + cross-links.

- [ ] **Step 1: Create the registry doc**

Create `docs/architecture/unified-play-experience.md`:

```markdown
# Unified Play Experience — Registry

Canonical behavior for the **read-mode playback actions** on detail views. Read this before
adding or changing a Play / Add-to-queue / Star affordance. This is the play-side counterpart
to [`unified-edit-experience.md`](unified-edit-experience.md): edit chrome lives in the
`ContentScaffold` action bar; **playback lives in the hero**.

## The rule

- **Play, Add to queue, and Star live in the `HeroHeader`**, below the identity, rendered by
  the shared **`HeroActions`** component (`webui/src/components/layout/HeroActions.vue`).
- They are **read-mode only**. `HeroHeader` renders its `#actions` slot behind `v-if="!editing"`,
  so entering edit mode (the scaffold pencil) removes them; Save/Cancel/Delete then own the bar.
- The `ContentScaffold` `#actions` bar carries **only** edit chrome (`EditActionBar`) and, on
  AlbumView, the drag-to-queue handle. No Play/Queue/Star buttons in the scaffold bar.

## Single sources of truth

- **`HeroActions.vue`** — the button set, icons, styling, and emits. Props: `playLabel`,
  `playDisabled`, `busy`, `canQueue`, `canStar`, `starred`. Emits: `play`, `queue`, `star`.
  Stable classes: `.hero-action-play`, `.hero-action-queue`, `.hero-action-star`.
- **`HeroHeader` `#actions` slot** — the placement + the read-mode `v-if` gate.

## Applicability per view

| View | Play | Add to queue | Star | Notes |
|------|------|--------------|------|-------|
| `AlbumView` | ✅ | ✅ | ✅ | Songs already loaded; drag handle stays in the scaffold bar |
| `ArtistView` | ✅ | ✅ | ✅ | `getArtist` has no songs — Play/Queue gather via `getAlbum` per album (`busy` while gathering) |
| `PlaylistDetailView` | ✅ | ✅ | — | Uses the live `working` track list; Subsonic does not star playlists |
| `RadioStationDetailView` | ✅ | — | — | Single live stream: queue/star not applicable |

**Applicability rules:** Star only where the entity is starrable (albums, artists). Add-to-queue
only where the entity expands to a track list (albums, artists, playlists).

## Deliberately out of scope

- **Drag-to-queue** (the album `pi pi-bars` handle) stays in the scaffold bar for now — not
  moved into the hero.
- Non-detail main content views (Library, Search, Radio/Playlists lists, Genres, Home, Now
  Playing) have no hero and are unaffected. Radio **create** mode (`/radio/new`) has no read
  mode, so no hero actions render there.
```

- [ ] **Step 2: Cross-link from the layout doc**

In `docs/architecture/main-content-view-layout.md`, in the **Reference implementations** list, add a bullet after the `EditActionBar` bullet (currently ending `see [unified-edit-experience.md]…`):

```markdown
- `webui/src/components/layout/HeroActions.vue` — the uniform read-mode play/queue/star
  affordance rendered in the hero; see [`unified-play-experience.md`](unified-play-experience.md).
```

- [ ] **Step 3: Verify the docs render / links resolve**

Run: `ls docs/architecture/unified-play-experience.md docs/architecture/main-content-view-layout.md docs/architecture/unified-edit-experience.md`
Expected: all three paths exist (the two cross-linked files are siblings, so the relative links resolve).

- [ ] **Step 4: Commit**

```bash
git add docs/architecture/unified-play-experience.md docs/architecture/main-content-view-layout.md
git commit -m "docs: record the unified play experience registry"
```

---

### Task 8: Full gate

- [ ] **Step 1: Run the complete type-check + test suite**

Run (from `webui/`): `npm test`
Expected: `vue-tsc --noEmit` passes (no unused-import or type errors) and all Vitest suites pass.

- [ ] **Step 2: Commit any fixups**

If step 1 surfaced fixes, stage and commit them:

```bash
git add -A
git commit -m "fix(webui): resolve type-check and test issues for hero actions"
```

---

## Self-Review

**Spec coverage:**
- `HeroActions` component (spec §"Component: HeroActions.vue") → Task 1. ✅
- `HeroHeader` `#actions` read-mode slot (spec §"HeroHeader.vue change") → Task 2. ✅
- AlbumView / PlaylistDetailView / RadioStationDetailView / ArtistView per-view changes (spec §"Per-view changes") → Tasks 3–6. ✅
- Artist gather-then-play via `getAlbum` (spec §ArtistView) → Task 6. ✅
- Registry doc + cross-link (spec §"Registry doc") → Task 7. ✅
- Testing (spec §Testing) → covered in each task's TDD steps + Task 8 gate. ✅

**Placeholder scan:** No TBD/TODO; every code step shows full code. ✅

**Type consistency:** `HeroActions` props/emits/classes (`hero-action-play|queue|star`) are used identically across Tasks 1, 3, 4, 5, 6. `gatherSongs(): Promise<Song[]>` returns `Song[]` from `AlbumWithSongs.song`; `player.playAlbum(songs)` / `player.addMultipleToQueue(songs)` match `usePlayer` signatures. `HeroHeader` `#actions` gated by `v-if="!editing"` matches every "hidden in edit mode" assertion. ✅
