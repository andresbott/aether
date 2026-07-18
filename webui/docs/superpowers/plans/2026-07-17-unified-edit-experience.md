# Unified Edit Experience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every editable detail view (Radio station, Playlist, Artist) the same header edit affordance: a pencil to enter edit mode, then icon-only Save/Cancel/Delete, with a confirm-first delete dialog.

**Architecture:** A new shared presentational component `EditActionBar` renders the header action set (pencil in read mode; Save/Cancel/Delete in edit mode) into `ContentScaffold`'s `#actions` slot, owning the delete confirmation. Each view binds `v-model:editing` + props and handles `@save/@cancel/@delete`. A single global CSS rule flips every ConfirmDialog footer to confirm | cancel.

**Tech Stack:** Vue 3 `<script setup>`, PrimeVue 4 (`Button`, `ConfirmDialog`, `useConfirm`), TanStack Query composables, Vitest + `@vue/test-utils`, SCSS.

## Global Constraints

- **No backwards compatibility.** Change shapes freely; no compat branches.
- **Music features stay on `/rest/`** — this is a pure webui change; no API changes.
- **Git (user rules):** single one-line commit message; **no** `Co-Authored-By` line; run `git add` as a **separate** step from `git commit`; commits require user approval (do not auto-approve).
- **Standard button classes** (test hooks, used app-wide): pencil `.edit-action-edit`, save `.edit-action-save`, cancel `.edit-action-cancel`, delete `.edit-action-delete`.
- **Icon set:** pencil `pi pi-pencil`, save `pi pi-check`, cancel `pi pi-times`, delete `pi pi-trash`; delete-confirm icon `pi pi-exclamation-triangle`, accept class `p-button-danger`.
- **Cover size cap:** `MAX_COVER_BYTES = 5 * 1024 * 1024`.
- Run all commands from `webui/`. Test runner: `npx vitest run <file>`; full suite (with type-check): `npm test`.

---

### Task 1: `EditActionBar` component

**Files:**
- Create: `webui/src/components/layout/EditActionBar.vue`
- Test: `webui/src/components/layout/__tests__/EditActionBar.spec.ts`

**Interfaces:**
- Consumes: nothing (leaf component). Requires `ConfirmationService` at runtime (registered globally in `main.ts`); in tests `primevue/useconfirm` is mocked.
- Produces (the contract every view relies on):
  - Props: `editing: boolean` (v-model), `saveDisabled?: boolean` (default `false`), `saving?: boolean` (default `false`), `canDelete?: boolean` (default `true`), `deleteHeader?: string` (default `'Delete?'`), `deleteMessage?: string` (default `'This cannot be undone.'`), `saveIcon?: string` (default `'pi pi-check'`), `saveTooltip?: string` (default `'Save'`).
  - Emits: `update:editing` (payload `boolean`), `save`, `cancel`, `delete`.
  - Slot: `#read-actions` (rendered only in read mode).
  - Button classes per Global Constraints.

- [ ] **Step 1: Write the failing test**

Create `webui/src/components/layout/__tests__/EditActionBar.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

const requireFn = vi.fn((opts: { accept: () => void }) => opts.accept())
vi.mock('primevue/useconfirm', () => ({ useConfirm: () => ({ require: requireFn }) }))

import EditActionBar from '@/components/layout/EditActionBar.vue'

const mountBar = (props: Record<string, unknown> = {}, slots: Record<string, string> = {}) =>
    mount(EditActionBar, {
        props: { editing: false, ...props },
        slots,
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

beforeEach(() => requireFn.mockClear())

describe('EditActionBar', () => {
    it('read mode: shows the pencil and read-actions, no edit buttons', () => {
        const w = mountBar({ editing: false }, { 'read-actions': '<button class="play">P</button>' })
        expect(w.find('.edit-action-edit').exists()).toBe(true)
        expect(w.find('.play').exists()).toBe(true)
        expect(w.find('.edit-action-save').exists()).toBe(false)
    })

    it('pencil emits update:editing true', async () => {
        const w = mountBar({ editing: false })
        await w.find('.edit-action-edit').trigger('click')
        expect(w.emitted('update:editing')?.[0]).toEqual([true])
    })

    it('edit mode: shows save/cancel/delete and hides read-actions and pencil', () => {
        const w = mountBar({ editing: true }, { 'read-actions': '<button class="play">P</button>' })
        expect(w.find('.edit-action-save').exists()).toBe(true)
        expect(w.find('.edit-action-cancel').exists()).toBe(true)
        expect(w.find('.edit-action-delete').exists()).toBe(true)
        expect(w.find('.play').exists()).toBe(false)
        expect(w.find('.edit-action-edit').exists()).toBe(false)
    })

    it('omits delete when canDelete is false', () => {
        const w = mountBar({ editing: true, canDelete: false })
        expect(w.find('.edit-action-delete').exists()).toBe(false)
    })

    it('save reflects saveDisabled and emits save', async () => {
        const w = mountBar({ editing: true, saveDisabled: true })
        expect(w.find('.edit-action-save').attributes('disabled')).toBeDefined()
        await w.setProps({ saveDisabled: false })
        await w.find('.edit-action-save').trigger('click')
        expect(w.emitted('save')).toHaveLength(1)
    })

    it('cancel emits cancel', async () => {
        const w = mountBar({ editing: true })
        await w.find('.edit-action-cancel').trigger('click')
        expect(w.emitted('cancel')).toHaveLength(1)
    })

    it('delete routes through confirm and emits delete on accept', async () => {
        const w = mountBar({ editing: true, deleteHeader: 'Delete X?', deleteMessage: 'Gone.' })
        await w.find('.edit-action-delete').trigger('click')
        expect(requireFn).toHaveBeenCalledWith(expect.objectContaining({ header: 'Delete X?', message: 'Gone.' }))
        expect(w.emitted('delete')).toHaveLength(1)
    })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npx vitest run src/components/layout/__tests__/EditActionBar.spec.ts`
Expected: FAIL — cannot resolve `@/components/layout/EditActionBar.vue`.

- [ ] **Step 3: Create the component**

Create `webui/src/components/layout/EditActionBar.vue`:

```vue
<script setup lang="ts">
import Button from 'primevue/button'
import { useConfirm } from 'primevue/useconfirm'

const props = withDefaults(
    defineProps<{
        editing: boolean
        saveDisabled?: boolean
        saving?: boolean
        canDelete?: boolean
        deleteHeader?: string
        deleteMessage?: string
        saveIcon?: string
        saveTooltip?: string
    }>(),
    {
        saveDisabled: false,
        saving: false,
        canDelete: true,
        deleteHeader: 'Delete?',
        deleteMessage: 'This cannot be undone.',
        saveIcon: 'pi pi-check',
        saveTooltip: 'Save'
    }
)

const emit = defineEmits<{
    (e: 'update:editing', value: boolean): void
    (e: 'save'): void
    (e: 'cancel'): void
    (e: 'delete'): void
}>()

const confirm = useConfirm()

function confirmDelete(): void {
    confirm.require({
        header: props.deleteHeader,
        message: props.deleteMessage,
        icon: 'pi pi-exclamation-triangle',
        acceptClass: 'p-button-danger',
        acceptLabel: 'Delete',
        rejectLabel: 'Cancel',
        accept: () => emit('delete')
    })
}
</script>

<template>
    <template v-if="!editing">
        <slot name="read-actions" />
        <Button
            class="edit-action-edit"
            icon="pi pi-pencil"
            text
            rounded
            v-tooltip.bottom="'Edit'"
            @click="emit('update:editing', true)"
        />
    </template>
    <template v-else>
        <Button
            class="edit-action-save"
            :icon="saveIcon"
            text
            rounded
            :disabled="saveDisabled"
            :loading="saving"
            v-tooltip.bottom="saveTooltip"
            @click="emit('save')"
        />
        <Button
            class="edit-action-cancel"
            icon="pi pi-times"
            text
            rounded
            v-tooltip.bottom="'Cancel'"
            @click="emit('cancel')"
        />
        <Button
            v-if="canDelete"
            class="edit-action-delete"
            icon="pi pi-trash"
            text
            rounded
            severity="danger"
            v-tooltip.bottom="'Delete'"
            @click="confirmDelete"
        />
    </template>
</template>
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npx vitest run src/components/layout/__tests__/EditActionBar.spec.ts`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit** (requires user approval)

```bash
git add src/components/layout/EditActionBar.vue src/components/layout/__tests__/EditActionBar.spec.ts
git commit -m "feat(webui): add EditActionBar for the unified edit affordance"
```

---

### Task 2: Global confirm-first dialog ordering

**Files:**
- Modify: `webui/src/assets/scss/_main.scss` (append at end, currently 67 lines)

**Interfaces:**
- Consumes: nothing. Produces: visual `confirm | cancel` order for every `ConfirmDialog` app-wide.

- [ ] **Step 1: Append the global rule**

Add to the end of `webui/src/assets/scss/_main.scss`:

```scss
/* Confirmation dialogs read confirm | cancel (confirm on the left). PrimeVue
   renders reject (Cancel) then accept, so reverse the footer visually. This is
   visual-only; keyboard focus order still reaches Cancel first — the safer
   default focus for a destructive action. Single source of truth for the order;
   see docs/architecture/unified-edit-experience.md. */
.p-confirmdialog .p-dialog-footer {
    display: flex;
    flex-direction: row-reverse;
}
```

- [ ] **Step 2: Verify the build compiles**

Run: `npm run type-check` (or `npm run build` if type-check is unavailable)
Expected: no SCSS/compile errors. (Visual order is verified manually in Task 6's smoke check; there is no unit test for global CSS.)

- [ ] **Step 3: Commit** (requires user approval)

```bash
git add src/assets/scss/_main.scss
git commit -m "style(webui): order confirm dialogs confirm | cancel globally"
```

---

### Task 3: PlaylistDetailView adopts EditActionBar

**Files:**
- Modify: `webui/src/views/PlaylistDetailView.vue` (imports; `#actions` slot at lines ~264-303; `handleDelete` at lines ~211-224)
- Test: `webui/src/views/__tests__/PlaylistDetailView.spec.ts`

**Interfaces:**
- Consumes: `EditActionBar` (Task 1). Reuses existing `saveEdit`, `cancelEdit`, `savePending`, `dirty`, `deletePlaylist`, `push`.
- Produces: no new exports.

- [ ] **Step 1: Update the tests first (red)**

In `webui/src/views/__tests__/PlaylistDetailView.spec.ts`:

Change the `enterEdit` helper (line ~63):

```ts
const enterEdit = async (w: ReturnType<typeof mountView>) => {
    await w.find('.edit-action-edit').trigger('click')
}
```

Replace the view-mode/edit-mode assertions test (lines ~98-109) with:

```ts
    it('view mode shows Play + pencil and no Save; edit mode shows Save/Cancel and hides Play', async () => {
        const w = mountView()
        expect(w.find('.play-all').exists()).toBe(true)
        expect(w.find('.edit-action-edit').exists()).toBe(true)
        expect(w.find('.edit-action-save').exists()).toBe(false)

        await enterEdit(w)
        expect(w.find('.hero-header').classes()).toContain('editing')
        expect(w.find('.edit-action-save').exists()).toBe(true)
        expect(w.find('.edit-action-cancel').exists()).toBe(true)
        expect(w.find('.edit-action-delete').exists()).toBe(true)
        expect(w.find('.play-all').exists()).toBe(false)
    })
```

Replace every remaining selector in this file: `.edit-save` → `.edit-action-save`, `.edit-cancel` → `.edit-action-cancel`, `.delete-playlist` → `.edit-action-delete`. (Occurs in the name-save, cancel, track-order-save, save-pending, cover-save, cover-clear-save, delete, and reseed tests.) Leave `.play-all`, `.edit-only input`, `.queue-edit-list`, `.cover-remove`, `.cleared-note`, `.flip-front` untouched.

- [ ] **Step 2: Run tests to verify they fail**

Run: `npx vitest run src/views/__tests__/PlaylistDetailView.spec.ts`
Expected: FAIL — `.edit-action-*` not found (view still renders old markup).

- [ ] **Step 3: Update the view**

In `webui/src/views/PlaylistDetailView.vue`:

Add the import beside the other `@/components/layout` imports (after the `HeroHeader` import, line ~10):

```ts
import EditActionBar from '@/components/layout/EditActionBar.vue'
```

Replace the entire `<template #actions>…</template>` block (lines ~265-303) with:

```vue
            <template #actions>
                <EditActionBar
                    v-model:editing="editing"
                    :save-disabled="savePending"
                    :saving="savePending"
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
            </template>
```

Replace `handleDelete` (lines ~211-224) — the confirmation now lives in `EditActionBar`, so this just runs the mutation:

```ts
const handleDelete = (): void => {
    deletePlaylist.mutate(props.id, {
        onSuccess: () => router.push({ name: 'playlists' })
    })
}
```

Remove the now-unused confirm wiring: delete the `import ConfirmDialog from 'primevue/confirmdialog'` and `import { useConfirm } from 'primevue/useconfirm'` lines (~7-8) **only if** `<ConfirmDialog />` is also removed. **Do not remove** `<ConfirmDialog />` from the template (line ~358) — `EditActionBar`'s `useConfirm` needs a mounted dialog. Therefore keep `import ConfirmDialog`, and **remove only** `import { useConfirm }` and the `const confirm = useConfirm()` line (~29).

- [ ] **Step 4: Run tests to verify they pass**

Run: `npx vitest run src/views/__tests__/PlaylistDetailView.spec.ts`
Expected: PASS (all tests).

- [ ] **Step 5: Commit** (requires user approval)

```bash
git add src/views/PlaylistDetailView.vue src/views/__tests__/PlaylistDetailView.spec.ts
git commit -m "refactor(webui): playlist detail uses EditActionBar for edit controls"
```

---

### Task 4: RadioStationDetailView adopts EditActionBar

**Files:**
- Modify: `webui/src/views/RadioStationDetailView.vue` (imports; `#actions` slot lines ~304-350; `onDelete` lines ~252-270; add `onSubmit`/`onCancel`)
- Test: `webui/src/views/__tests__/RadioStationDetailView.spec.ts`

**Interfaces:**
- Consumes: `EditActionBar` (Task 1). Reuses `onCreate`, `onSave`, `valid`, `submitting`, `station`, `deleteMutation`, `router`, `baseline`, `form`, `resetCoverState`.
- Produces: new local handlers `onSubmit` (create→`onCreate`, else `onSave`) and `onCancel` (create→navigate to radio, else reseed + leave edit).

- [ ] **Step 1: Update the tests first (red)**

In `webui/src/views/__tests__/RadioStationDetailView.spec.ts`:

The create-mode Save button is now `.edit-action-save`; existing stations open read-only, so tests must enter edit mode before Save/Delete. Replace the create/edit tests (lines ~91-144) with:

```ts
    it('create mode: has a disabled Save until valid', async () => {
        const w = mountView({ create: true })
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('Add station')
        expect(w.find('.edit-action-save').attributes('disabled')).toBeDefined()

        const inputs = editInputs(w)
        await inputs[0].setValue('Jazz FM')
        await inputs[1].setValue('http://stream/jazz')
        expect(w.find('.edit-action-save').attributes('disabled')).toBeUndefined()
    })

    it('create mode: Save calls the create mutation and returns to /radio', async () => {
        const w = mountView({ create: true })
        const inputs = editInputs(w)
        await inputs[0].setValue('Jazz FM')
        await inputs[1].setValue('http://stream/jazz')
        await w.find('.edit-action-save').trigger('click')
        expect(createMutate).toHaveBeenCalledWith(
            expect.objectContaining({ name: 'Jazz FM', streamUrl: 'http://stream/jazz' }),
            expect.anything()
        )
        expect(push).toHaveBeenCalledWith({ name: 'radio' })
    })

    it('create mode: has no Delete button', () => {
        const w = mountView({ create: true })
        expect(w.find('.edit-action-delete').exists()).toBe(false)
    })

    it('create mode: seeds the form from query params', () => {
        route.query = { name: 'RP', streamUrl: 'http://rp', homepage: 'http://rp.com' }
        const w = mountView({ create: true })
        const inputs = editInputs(w)
        expect((inputs[0].element as HTMLInputElement).value).toBe('RP')
        expect((inputs[1].element as HTMLInputElement).value).toBe('http://rp')
        expect((inputs[2].element as HTMLInputElement).value).toBe('http://rp.com')
    })

    it('existing station opens read-only with Play + pencil', () => {
        const w = mountView({ id: 's1' })
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('')
        expect(w.find('.hero-name').text()).toBe('Jazz FM')
        expect(w.find('.play-station').exists()).toBe(true)
        expect(w.find('.edit-action-edit').exists()).toBe(true)
        expect(w.find('.edit-action-save').exists()).toBe(false)
    })

    it('edit mode: Save calls the update mutation with the id', async () => {
        const w = mountView({ id: 's1' })
        await w.find('.edit-action-edit').trigger('click')
        await w.find('.edit-action-save').trigger('click')
        expect(updateMutate).toHaveBeenCalledWith(
            expect.objectContaining({ id: 's1', name: 'Jazz FM' }),
            expect.anything()
        )
    })

    it('edit mode: Delete (confirmed) removes the station and returns to /radio', async () => {
        const w = mountView({ id: 's1' })
        await w.find('.edit-action-edit').trigger('click')
        await w.find('.edit-action-delete').trigger('click')
        expect(deleteMutate).toHaveBeenCalledWith('s1', expect.anything())
        expect(push).toHaveBeenCalledWith({ name: 'radio' })
    })

    it('read mode: Play enqueues the station', async () => {
        const w = mountView({ id: 's1' })
        await w.find('.play-station').trigger('click')
        expect(playNow).toHaveBeenCalledWith({ id: 's1' })
    })
```

Keep the final not-found test unchanged.

- [ ] **Step 2: Run tests to verify they fail**

Run: `npx vitest run src/views/__tests__/RadioStationDetailView.spec.ts`
Expected: FAIL — `.edit-action-*` not found.

- [ ] **Step 3: Update the view**

In `webui/src/views/RadioStationDetailView.vue`:

Add the import after the `HeroHeader` import (line ~9):

```ts
import EditActionBar from '@/components/layout/EditActionBar.vue'
```

Remove the confirm wiring: delete `import { useConfirm } from 'primevue/useconfirm'` (line ~7) and `const confirm = useConfirm()` (line ~29). **Keep** `import ConfirmDialog` (line ~6) and the `<ConfirmDialog />` in the template — EditActionBar's confirm renders through it.

Add two handlers next to `onSave`/`onDelete` (after `onSave`, ~line 251):

```ts
function onSubmit() {
    if (props.create) onCreate()
    else onSave()
}
function onCancel() {
    if (props.create) {
        router.push({ name: 'radio' })
        return
    }
    // Reseed the form from the station and drop cover staging, then leave edit mode.
    if (station.value) {
        form.value = {
            name: station.value.name,
            streamUrl: station.value.streamUrl,
            homepageUrl: station.value.homepageUrl ?? ''
        }
        baseline.value = { ...form.value }
    }
    resetCoverState()
    editing.value = false
}
```

Replace `onDelete` (lines ~252-270) — confirmation now lives in `EditActionBar`:

```ts
function onDelete() {
    const s = station.value
    if (!s) return
    deleteMutation.mutate(s.id, {
        onSuccess: () => {
            submittedClean.value = true
            router.push({ name: 'radio' })
        }
    })
}
```

Replace the entire `<template #actions>…</template>` block (lines ~304-350) with:

```vue
            <template #actions>
                <EditActionBar
                    v-model:editing="editing"
                    :can-delete="!create"
                    :save-disabled="!valid"
                    :saving="submitting"
                    :save-tooltip="create ? 'Create' : 'Save'"
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
            </template>
```

Note: create mode has `editing` starting `true` (line ~83, unchanged), so it renders Save/Cancel with no pencil and, via `:can-delete="!create"`, no Delete.

- [ ] **Step 4: Run tests to verify they pass**

Run: `npx vitest run src/views/__tests__/RadioStationDetailView.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit** (requires user approval)

```bash
git add src/views/RadioStationDetailView.vue src/views/__tests__/RadioStationDetailView.spec.ts
git commit -m "refactor(webui): radio station detail uses EditActionBar for edit controls"
```

---

### Task 5: ArtistView adopts EditActionBar with staged cover editing

**Files:**
- Modify: `webui/src/views/ArtistView.vue` (whole `<script setup>` edit block + `#actions` slot)
- Test: `webui/src/views/__tests__/ArtistView.spec.ts`

**Interfaces:**
- Consumes: `EditActionBar` (Task 1); existing `useUpdateArtistCover` (`updateCover.mutate`, `updateCover.isPending`), `subsonicClient`, `useToggleStar`.
- Produces: staged-cover behavior — `onCoverSelect`/`onRemoveCover` stage locally; `saveEdit` persists on Save; `cancelEdit` discards; `dirty` gates Save and the unsaved-change guards. `canDelete=false` (artists cannot be deleted).

- [ ] **Step 1: Update the tests first (red)**

Rewrite `webui/src/views/__tests__/ArtistView.spec.ts`. Change the router mock to include `onBeforeRouteLeave`, add a `useConfirm` mock (EditActionBar calls it at setup), replace the `editButton` helper, and change the cover tests from "immediate" to "staged":

Replace the router mock (lines ~31-33) with:

```ts
vi.mock('vue-router', () => ({
    useRouter: () => ({ back: vi.fn() }),
    onBeforeRouteLeave: vi.fn()
}))

vi.mock('primevue/useconfirm', () => ({
    useConfirm: () => ({ require: (opts: { accept: () => void }) => opts.accept() })
}))
```

Add `directives: { tooltip: {} }` to the mount `global` (line ~48) and add `ConfirmDialog: true` to a `stubs` entry alongside the existing `ContentScaffold` stub:

```ts
const mountView = () =>
    mount(ArtistView, {
        props: { id: 'ar-1' },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { ContentScaffold: ScaffoldStub, ConfirmDialog: true }
        }
    })

const enterEdit = async (w: ReturnType<typeof mountView>) => {
    await w.find('.edit-action-edit').trigger('click')
}
```

Delete the old `editButton` helper (lines ~52-53). Replace the edit-toggle test (lines ~109-115) with:

```ts
    it('the pencil toggles the hero into edit mode (flips the cover)', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        expect(w.find('.hero-cover').classes()).not.toContain('flipped')
        await enterEdit(w)
        expect(w.find('.hero-cover').classes()).toContain('flipped')
    })

    it('has no Delete button (artists cannot be deleted)', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        await enterEdit(w)
        expect(w.find('.edit-action-delete').exists()).toBe(false)
    })
```

Replace the "uploads immediately" test (lines ~117-127) with staged + Save:

```ts
    it('selecting a file stages a preview and Save uploads it via updateArtistCover', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        const file = new File(['x'], 'a.png', { type: 'image/png' })
        w.findComponent(FileUpload).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()
        expect(w.find('.flip-front img').attributes('src')).toContain('blob:')
        expect(coverMutate).not.toHaveBeenCalled()

        await enterEdit(w)
        await w.find('.edit-action-save').trigger('click')
        expect(coverMutate).toHaveBeenCalledWith(
            expect.objectContaining({ artistId: 'ar-1', coverFile: file }),
            expect.anything()
        )
    })
```

Replace the "Remove sends a cover clear" test (lines ~140-148) with staged clear + Save:

```ts
    it('Remove stages a cover clear that Save commits', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        await w.find('.cover-remove').trigger('click')
        expect(coverMutate).not.toHaveBeenCalled()

        await enterEdit(w)
        await w.find('.edit-action-save').trigger('click')
        expect(coverMutate).toHaveBeenCalledWith(
            expect.objectContaining({ artistId: 'ar-1', coverClear: true }),
            expect.anything()
        )
    })
```

Add a stub for object URLs in `beforeEach` (after line ~62, inside the existing block):

```ts
    global.URL.createObjectURL = vi.fn(() => 'blob:mock')
    global.URL.revokeObjectURL = vi.fn()
```

Leave the loading, error, title, meta-row, star, and oversize-file tests as-is (the oversize test still asserts `coverMutate` not called + a `Message` shown, which staging preserves).

- [ ] **Step 2: Run tests to verify they fail**

Run: `npx vitest run src/views/__tests__/ArtistView.spec.ts`
Expected: FAIL — staged behavior not implemented; `.edit-action-*` not found.

- [ ] **Step 3: Update the view script**

In `webui/src/views/ArtistView.vue`, add imports:

```ts
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRouter, onBeforeRouteLeave } from 'vue-router'
import EditActionBar from '@/components/layout/EditActionBar.vue'
```

(Merge with the existing `computed, ref` and `useRouter` imports; add `onMounted, onUnmounted`, `onBeforeRouteLeave`, and `EditActionBar`.)

Replace the edit block (current lines ~20-54: `editing`/`cacheBust`/`coverSizeError` decls, `onCoverSelect`, `onRemoveCover`, `coverUrl`) with the staged version:

```ts
const editing = ref(false)
const cacheBust = ref(0)
const selectedFile = ref<File | null>(null)
const previewUrl = ref<string | null>(null)
const coverClear = ref(false)
const coverSizeError = ref<string | null>(null)

const dirty = computed(() => selectedFile.value !== null || coverClear.value)

const handleStar = () => {
    if (!artist.value) return
    toggleStar.mutate({ id: artist.value.id, starred: !!artist.value.starred })
}

function resetCoverStaging(): void {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    selectedFile.value = null
    coverClear.value = false
    coverSizeError.value = null
}

// Artist edit = cover only. Changes are staged locally and applied on Save.
const onCoverSelect = (file: File): void => {
    if (file.size > MAX_COVER_BYTES) {
        coverSizeError.value = `File is ${(file.size / 1024 / 1024).toFixed(1)} MB — max is 5 MB`
        return
    }
    coverSizeError.value = null
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    selectedFile.value = file
    previewUrl.value = URL.createObjectURL(file)
    coverClear.value = false
}

const onRemoveCover = (): void => {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    selectedFile.value = null
    coverClear.value = true
    coverSizeError.value = null
}

const saveEdit = (): void => {
    if (!dirty.value) {
        editing.value = false
        return
    }
    updateCover.mutate(
        {
            artistId: props.id,
            coverFile: selectedFile.value ?? undefined,
            coverClear: coverClear.value || undefined
        },
        {
            onSuccess: () => {
                resetCoverStaging()
                cacheBust.value++
                editing.value = false
            }
        }
    )
}

const cancelEdit = (): void => {
    resetCoverStaging()
    editing.value = false
}

const coverUrl = computed(() => {
    if (previewUrl.value) return previewUrl.value
    if (coverClear.value) return null
    if (!artist.value?.coverArt || !subsonicClient.isConfigured()) return null
    const base = subsonicClient.getCoverArtUrl(artist.value.coverArt, 250)
    return cacheBust.value > 0 ? `${base}&_cb=${cacheBust.value}` : base
})

// Unsaved-changes guards (mirror Playlist/Radio detail views).
onBeforeRouteLeave(() => {
    if (dirty.value) {
        return window.confirm('You have unsaved changes. Leave without saving?')
    }
})
const onBeforeUnload = (e: BeforeUnloadEvent): void => {
    if (!dirty.value) return
    e.preventDefault()
    e.returnValue = ''
}
onMounted(() => window.addEventListener('beforeunload', onBeforeUnload))
onUnmounted(() => {
    window.removeEventListener('beforeunload', onBeforeUnload)
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
})
```

(Delete the old standalone `handleStar` const at lines ~24-27 to avoid a duplicate — it is re-declared in the block above. Keep the existing `heroMeta` and `sortedAlbums` computeds unchanged.)

Update the `HeroHeader` binding to pass the staged error (it already binds `:cover-url="coverUrl"`); ensure it also binds `:cover-size-error="coverSizeError"` (already present, unchanged).

- [ ] **Step 4: Update the view template `#actions`**

Replace the `<template #actions>…</template>` block (current lines ~93-108) with:

```vue
            <template #actions>
                <EditActionBar
                    v-model:editing="editing"
                    :can-delete="false"
                    :save-disabled="!dirty"
                    :saving="updateCover.isPending"
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
            </template>
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `npx vitest run src/views/__tests__/ArtistView.spec.ts`
Expected: PASS.

- [ ] **Step 6: Commit** (requires user approval)

```bash
git add src/views/ArtistView.vue src/views/__tests__/ArtistView.spec.ts
git commit -m "refactor(webui): artist detail uses EditActionBar with staged cover editing"
```

---

### Task 6: Architecture registry doc + cross-link + full verify

**Files:**
- Create: `docs/architecture/unified-edit-experience.md` (repo root `docs/`, sibling of `main-content-view-layout.md`)
- Modify: `docs/architecture/main-content-view-layout.md` (add a cross-link line)

**Interfaces:** Documentation only.

- [ ] **Step 1: Write the registry doc**

Create `docs/architecture/unified-edit-experience.md`:

```markdown
# Unified Edit Experience — Canonical Behavior

Every editable **detail view** (a single-item main content view) shares one edit
affordance. This is the registry of that behavior; the implementation lives in
`webui/src/components/layout/EditActionBar.vue` and one global CSS rule.

## The affordance

- **Read mode:** the view's own header actions (Play, Star, Add to Queue, …) plus a
  **pencil** icon button (`pi pi-pencil`, no text, tooltip "Edit"). Clicking it enters
  edit mode.
- **Edit mode:** three **icon-only** buttons with tooltips, and the read-mode actions are
  hidden:
  - **Save** — `pi pi-check`, tooltip "Save" (or "Create" when creating a new item).
  - **Cancel** — `pi pi-times`, tooltip "Cancel".
  - **Delete** — `pi pi-trash`, `severity="danger"`, tooltip "Delete".
- **Delete** opens a confirmation dialog ordered **confirm | cancel** (confirm on the
  left, styled danger). Confirming performs the delete; cancelling closes the dialog.

## Single sources of truth

- **`EditActionBar`** (`webui/src/components/layout/EditActionBar.vue`) renders the whole
  set into `ContentScaffold`'s `#actions` slot and owns the delete confirmation.
  - Props: `editing` (v-model), `saveDisabled`, `saving`, `canDelete` (default `true`),
    `deleteHeader`, `deleteMessage`, `saveIcon`, `saveTooltip`.
  - Emits: `update:editing`, `save`, `cancel`, `delete` (emitted only after the user
    confirms). The **view** owns state and decides when to leave edit mode (a save may be
    async or fail), so the bar never flips `editing` itself except via the pencil.
  - Slot `#read-actions` holds view-specific read-mode buttons; hidden in edit mode.
  - Stable test/hook classes: `.edit-action-edit`, `.edit-action-save`,
    `.edit-action-cancel`, `.edit-action-delete`.
- **Confirm-first ordering** is a single global rule in
  `webui/src/assets/scss/_main.scss` (`.p-confirmdialog .p-dialog-footer { flex-direction:
  row-reverse }`). It is visual-only — keyboard focus still reaches Cancel first, the safer
  default for a destructive action — and applies to every confirm dialog app-wide.

## How views adapt

A view shows only the buttons that apply:

| View | Route | Editable | Delete? | Notes |
|------|-------|----------|---------|-------|
| Radio station | `/radio/:id`, `/radio/new` | name, stream, homepage, cover | yes | Create mode (`/radio/new`) starts in edit mode, `canDelete=false`, Save reads "Create", Cancel returns to `/radio`. |
| Playlist | `/playlist/:id` | name, description, cover, tracks | yes | Track reordering is always-on; identity/cover/track edits persist together on Save. |
| Artist | `/artist/:id` | cover only | no | `canDelete=false`. Cover edits are **staged** and applied on Save. |
| Album | `/album/:id` | — | — | **Not editable** (album data derives from file metadata); no pencil. |

Rules for a new editable detail view:

1. Import `EditActionBar` and place it in `ContentScaffold`'s `#actions`, read-mode extras
   in `#read-actions`.
2. Stage edits locally; gate Save with a `dirty`/`valid` computed via `save-disabled`, and
   pass the in-flight mutation state via `saving`.
3. `@save` persists then leaves edit mode; `@cancel` discards staged edits and leaves edit
   mode; `@delete` runs the delete mutation (the bar already confirmed).
4. Set `canDelete=false` when the entity cannot be deleted.
5. Keep a `<ConfirmDialog />` mounted in the view (the bar's `useConfirm` renders through it).
6. Add unsaved-changes guards (`onBeforeRouteLeave` + `beforeunload`) when edits are staged.
```

- [ ] **Step 2: Cross-link from the layout doc**

In `docs/architecture/main-content-view-layout.md`, add this line to the "Reference
implementations (copy these)" bullet list (after the `QueueView` bullet, ~line 18):

```markdown
- `webui/src/components/layout/EditActionBar.vue` — the uniform edit affordance for
  editable detail views; see [`unified-edit-experience.md`](unified-edit-experience.md).
```

- [ ] **Step 3: Run the full webui test suite + type-check**

Run: `npm test`
Expected: PASS (whole suite, including the three refactored views + `EditActionBar`).

Run: `npm run type-check`
Expected: no type errors.

- [ ] **Step 4: Manual smoke check (confirm-first order)**

Run the app (`npm run dev` or the project `/run` flow), open a playlist, enter edit mode,
click Delete, and verify the dialog shows **Delete** left of **Cancel**. Repeat on a radio
station.

- [ ] **Step 5: Commit** (requires user approval)

```bash
git add docs/architecture/unified-edit-experience.md docs/architecture/main-content-view-layout.md
git commit -m "docs(architecture): register the unified edit experience"
```

---

## Self-Review

**Spec coverage:**
- Pencil-enters-edit, icon-only Save/Cancel/Delete, hide read actions → Task 1 (component) + Tasks 3-5 (per view). ✅
- Confirm-first delete dialog, global → Task 2. ✅
- Artist adapts (staged cover, no delete) → Task 5. ✅
- Radio create mode (Save=Create, Cancel→/radio, no delete) → Task 4. ✅
- Album out of scope → documented in Task 6 registry. ✅
- Registry doc in architecture + cross-link → Task 6. ✅
- Icon-only + tooltips (brainstorm answer) → Task 1 markup. ✅
- Global confirm order (brainstorm answer) → Task 2. ✅

**Placeholder scan:** none — every step carries concrete code/commands.

**Type consistency:** `EditActionBar` prop/emit names (`editing`, `saveDisabled`, `saving`,
`canDelete`, `deleteHeader`, `deleteMessage`, `saveIcon`, `saveTooltip`; `update:editing`,
`save`, `cancel`, `delete`) are used identically in Tasks 3-5. Standard classes
(`.edit-action-edit/-save/-cancel/-delete`) match across component, views, and tests. View
handlers (`saveEdit`/`cancelEdit`/`handleDelete` for Playlist; `onSubmit`/`onCancel`/
`onDelete` for Radio; `saveEdit`/`cancelEdit` for Artist) are defined in their own tasks.
```
