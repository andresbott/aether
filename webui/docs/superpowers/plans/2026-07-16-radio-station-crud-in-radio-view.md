# Radio station CRUD in the /radio view — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move internet-radio-station create/edit/delete out of Settings and into the main `/radio` view — Discover/Add from the header, edit/delete via a playlist-style detail view.

**Architecture:** A new presentational `RadioStationForm` holds the fields + cover logic. A new routed `RadioStationDetailView` (`/radio/new` create mode, `/radio/:id` edit mode) wraps the form and owns the Create/Save/Delete/Play actions in the `ContentScaffold` header. `RadioView` gains Discover + Add header buttons; cards/rows become links into the detail view. The old Settings radio page and the orphaned `RadioStationDialog` are removed. All CRUD keeps using the existing Subsonic mutation hooks — no API changes.

**Tech Stack:** Vue 3 `<script setup>`, PrimeVue 4, TanStack Vue Query, vue-router 4, Vitest + @vue/test-utils.

## Global Constraints

- All station CRUD runs over the existing Subsonic (`/rest/`) hooks in `composables/useSubsonicQueries.ts` (`useRadioStations`, `useCreateRadioStation`, `useUpdateRadioStation`, `useDeleteRadioStation`). No new API endpoints, no `/api/v1` usage.
- Main content views follow the uniform layout: back-row + `ContentScaffold` (`title` + `summary` + `#actions`), self-scrolling body, `meta: { flush: true }` on the route. Reference: `PlaylistDetailView.vue`.
- Cover upload cap is `MAX_COVER_BYTES = 5 * 1024 * 1024` (5 MB); accept `image/png,image/jpeg`.
- Test command: `npm test` (runs `vue-tsc --noEmit && vitest run`). Single-test run: `npx vitest run <path>`.
- Git: single one-line commit message, no `Co-Authored-By`, `git add` as a separate step from `git commit`.

---

### Task 1: `RadioStationForm` presentational component

**Files:**
- Create: `src/components/library/RadioStationForm.vue`
- Test: `src/components/library/__tests__/RadioStationForm.spec.ts`

**Interfaces:**
- Consumes: `subsonicClient.getCoverArtUrl(id, size)` from `@/lib/api/subsonic`; `InternetRadioStation` from `@/types/subsonic`; `RadioStationPrefill` from `@/types/radiobrowser`.
- Produces:
  - Props: `{ station: InternetRadioStation | null; prefill?: RadioStationPrefill | null }`
  - Emits: `change` with `{ input: StationInput; valid: boolean; dirty: boolean }` where
    `StationInput = { name: string; streamUrl: string; homepageUrl?: string; coverFile?: File; coverClear?: boolean }`.
  - Field CSS hooks: `input.field-name`, `input.field-stream-url`, `input.field-homepage`; `FileUpload` (name `FileUpload`) emitting `select`; a `Remove cover` button.

- [ ] **Step 1: Write the failing test**

Create `src/components/library/__tests__/RadioStationForm.spec.ts`:

```ts
import { describe, it, expect, vi, beforeAll } from 'vitest'
import { mount } from '@vue/test-utils'
import type { InternetRadioStation } from '@/types/subsonic'
import type { RadioStationPrefill } from '@/types/radiobrowser'
import RadioStationForm from '@/components/library/RadioStationForm.vue'

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { getCoverArtUrl: (id: string) => `/cover/${id}` }
}))

beforeAll(() => {
    globalThis.URL.createObjectURL = vi.fn(() => 'blob:mock')
    globalThis.URL.revokeObjectURL = vi.fn()
})

const stubs = {
    InputText: {
        props: ['modelValue'],
        template:
            '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
    },
    Button: {
        props: ['label'],
        inheritAttrs: false,
        template: '<button @click="$emit(\'click\')">{{ label }}</button>'
    },
    FileUpload: { name: 'FileUpload', emits: ['select'], template: '<div class="file-upload" />' },
    Message: { template: '<div class="p-message"><slot /></div>' }
}

const station: InternetRadioStation = {
    id: 'r-1',
    name: 'BBC Radio 1',
    streamUrl: 'http://bbc/stream',
    homepageUrl: 'http://bbc/home',
    coverArt: 'r-1'
}

const mountForm = (props: Partial<{ station: InternetRadioStation | null; prefill: RadioStationPrefill | null }>) =>
    mount(RadioStationForm, { props: { station: null, ...props }, global: { stubs } })

// The last change payload the form emitted.
const last = (w: ReturnType<typeof mountForm>) => {
    const events = w.emitted('change') as Array<[{ input: any; valid: boolean; dirty: boolean }]>
    return events[events.length - 1][0]
}

describe('RadioStationForm', () => {
    it('starts invalid and not dirty when blank', () => {
        const w = mountForm({ station: null })
        expect(last(w).valid).toBe(false)
        expect(last(w).dirty).toBe(false)
    })

    it('becomes valid once name and stream URL are present, trimming them', async () => {
        const w = mountForm({ station: null })
        await w.find('input.field-name').setValue('  Jazz FM  ')
        await w.find('input.field-stream-url').setValue('  http://jazz  ')
        const p = last(w)
        expect(p.valid).toBe(true)
        expect(p.dirty).toBe(true)
        expect(p.input.name).toBe('Jazz FM')
        expect(p.input.streamUrl).toBe('http://jazz')
        expect(p.input.homepageUrl).toBeUndefined()
    })

    it('pre-fills fields from an existing station and stays clean until edited', async () => {
        const w = mountForm({ station })
        expect((w.find('input.field-name').element as HTMLInputElement).value).toBe('BBC Radio 1')
        expect(last(w).dirty).toBe(false)
        await w.find('input.field-name').setValue('BBC R1')
        expect(last(w).dirty).toBe(true)
        expect(last(w).input.name).toBe('BBC R1')
    })

    it('seeds fields and cover from a prefill', () => {
        const file = new File(['x'], 'favicon.png', { type: 'image/png' })
        const prefill: RadioStationPrefill = {
            name: 'Radio Paradise',
            streamUrl: 'http://rp/stream',
            homepageUrl: 'https://radioparadise.com',
            coverFile: file
        }
        const w = mountForm({ station: null, prefill })
        const p = last(w)
        expect(p.input.name).toBe('Radio Paradise')
        expect(p.input.coverFile).toBe(file)
    })

    it('rejects an oversized cover and stays invalid', async () => {
        const w = mountForm({ station: null })
        await w.find('input.field-name').setValue('Jazz')
        await w.find('input.field-stream-url').setValue('http://jazz')
        const big = new File([new Uint8Array(6 * 1024 * 1024)], 'big.png', { type: 'image/png' })
        w.findComponent({ name: 'FileUpload' }).vm.$emit('select', { files: [big] })
        await w.vm.$nextTick()
        expect(last(w).valid).toBe(false)
        expect(w.find('.p-message').text()).toContain('max is 5 MB')
    })

    it('includes a chosen cover file in the input', async () => {
        const w = mountForm({ station: null })
        await w.find('input.field-name').setValue('Jazz')
        await w.find('input.field-stream-url').setValue('http://jazz')
        const file = new File(['x'], 'cover.png', { type: 'image/png' })
        w.findComponent({ name: 'FileUpload' }).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()
        expect(last(w).input.coverFile).toBe(file)
    })

    it('stages a cover clear when the existing cover is removed', async () => {
        const w = mountForm({ station })
        const removeBtn = w.findAll('button').find((b) => b.text() === 'Remove cover')!
        await removeBtn.trigger('click')
        expect(last(w).input.coverClear).toBe(true)
        expect(last(w).dirty).toBe(true)
    })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `npx vitest run src/components/library/__tests__/RadioStationForm.spec.ts`
Expected: FAIL — cannot resolve `@/components/library/RadioStationForm.vue`.

- [ ] **Step 3: Create the component**

Create `src/components/library/RadioStationForm.vue`:

```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import InputText from 'primevue/inputtext'
import Button from 'primevue/button'
import FileUpload from 'primevue/fileupload'
import Message from 'primevue/message'
import type { InternetRadioStation } from '@/types/subsonic'
import type { RadioStationPrefill } from '@/types/radiobrowser'
import { subsonicClient } from '@/lib/api/subsonic'

const MAX_COVER_BYTES = 5 * 1024 * 1024

const props = defineProps<{
    station: InternetRadioStation | null
    prefill?: RadioStationPrefill | null
}>()

interface StationInput {
    name: string
    streamUrl: string
    homepageUrl?: string
    coverFile?: File
    coverClear?: boolean
}

const emit = defineEmits<{
    (e: 'change', payload: { input: StationInput; valid: boolean; dirty: boolean }): void
}>()

interface FormState {
    name: string
    streamUrl: string
    homepageUrl: string
}
function emptyForm(): FormState {
    return { name: '', streamUrl: '', homepageUrl: '' }
}

const form = ref<FormState>(emptyForm())
const baseline = ref<FormState>(emptyForm())
const selectedFile = ref<File | null>(null)
const previewUrl = ref<string | null>(null)
const coverClear = ref(false)
const sizeError = ref<string | null>(null)

const isEditMode = computed(() => props.station !== null)

function resetCoverState() {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    selectedFile.value = null
    coverClear.value = false
    sizeError.value = null
}

// Seed the form from the station (edit), the prefill (create-from-Discover), or
// blanks. Snapshot the baseline afterward so `dirty` reflects user edits only.
watch(
    () => [props.station, props.prefill],
    () => {
        resetCoverState()
        if (props.station) {
            form.value = {
                name: props.station.name,
                streamUrl: props.station.streamUrl,
                homepageUrl: props.station.homepageUrl ?? ''
            }
        } else if (props.prefill) {
            const init = props.prefill
            form.value = {
                name: init.name,
                streamUrl: init.streamUrl,
                homepageUrl: init.homepageUrl ?? ''
            }
            if (init.coverFile) {
                selectedFile.value = init.coverFile
                previewUrl.value = URL.createObjectURL(init.coverFile)
            }
        } else {
            form.value = emptyForm()
        }
        baseline.value = { ...form.value }
    },
    { immediate: true }
)

const hasExistingCover = computed(
    () => isEditMode.value && !!props.station?.coverArt && !coverClear.value
)
const displayedCoverUrl = computed(() => {
    if (previewUrl.value) return previewUrl.value
    if (hasExistingCover.value && props.station?.coverArt) {
        return subsonicClient.getCoverArtUrl(props.station.coverArt, 256)
    }
    return null
})

const valid = computed(
    () =>
        form.value.name.trim().length > 0 &&
        form.value.streamUrl.trim().length > 0 &&
        sizeError.value === null
)

const dirty = computed(
    () =>
        form.value.name !== baseline.value.name ||
        form.value.streamUrl !== baseline.value.streamUrl ||
        form.value.homepageUrl !== baseline.value.homepageUrl ||
        selectedFile.value !== null ||
        coverClear.value
)

const input = computed<StationInput>(() => {
    const homepage = form.value.homepageUrl.trim()
    return {
        name: form.value.name.trim(),
        streamUrl: form.value.streamUrl.trim(),
        homepageUrl: homepage === '' ? undefined : homepage,
        coverFile: selectedFile.value ?? undefined,
        coverClear: coverClear.value || undefined
    }
})

watch(
    [input, valid, dirty],
    () => emit('change', { input: input.value, valid: valid.value, dirty: dirty.value }),
    { immediate: true }
)

function onFileSelect(event: { files: File[] }) {
    const file = event.files?.[0]
    if (!file) return
    if (file.size > MAX_COVER_BYTES) {
        sizeError.value = `File is ${(file.size / 1024 / 1024).toFixed(1)} MB — max is 5 MB`
        return
    }
    sizeError.value = null
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    selectedFile.value = file
    previewUrl.value = URL.createObjectURL(file)
    coverClear.value = false
}

function onRemoveCover() {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    selectedFile.value = null
    coverClear.value = true
}
</script>

<template>
    <div class="radio-station-form">
        <div class="field-row">
            <label>Name</label>
            <InputText class="field-name" v-model="form.name" placeholder="e.g. BBC Radio 1" />
        </div>

        <div class="field-row">
            <label>Stream URL</label>
            <InputText
                class="field-stream-url"
                v-model="form.streamUrl"
                placeholder="http://example.com/stream"
            />
        </div>

        <div class="field-row">
            <label>Homepage URL</label>
            <InputText class="field-homepage" v-model="form.homepageUrl" placeholder="optional" />
        </div>

        <div class="field-block">
            <label>Cover</label>
            <div class="cover-row">
                <div class="cover-preview">
                    <img v-if="displayedCoverUrl" :src="displayedCoverUrl" alt="cover" />
                    <div v-else class="cover-placeholder">
                        <i class="pi pi-image" style="font-size: 1.5rem"></i>
                    </div>
                </div>
                <div class="cover-actions">
                    <FileUpload
                        mode="basic"
                        accept="image/png,image/jpeg"
                        :maxFileSize="MAX_COVER_BYTES"
                        :auto="false"
                        chooseLabel="Choose image"
                        @select="onFileSelect"
                    />
                    <Button
                        v-if="hasExistingCover || selectedFile"
                        text
                        severity="secondary"
                        label="Remove cover"
                        @click="onRemoveCover"
                    />
                    <Message v-if="sizeError" severity="error" :closable="false">
                        {{ sizeError }}
                    </Message>
                    <small v-if="coverClear" class="cleared-note">
                        Cover will be cleared on save.
                    </small>
                </div>
            </div>
        </div>
    </div>
</template>

<style scoped>
.radio-station-form {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
}
.field-row {
    display: grid;
    grid-template-columns: 8rem 1fr;
    align-items: center;
    gap: 0.5rem;
}
.field-block {
    display: grid;
    grid-template-columns: 8rem 1fr;
    align-items: start;
    gap: 0.5rem;
}
.field-row label,
.field-block label {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}
.field-row :deep(.p-inputtext) {
    width: 100%;
}
.cover-row {
    display: flex;
    gap: 1rem;
    align-items: flex-start;
}
.cover-preview {
    width: 6rem;
    height: 6rem;
    border-radius: 8px;
    overflow: hidden;
    background: var(--app-bg-subtle, #f3f4f6);
    flex-shrink: 0;
}
.cover-preview img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}
.cover-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--app-text-secondary);
}
.cover-actions {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    align-items: flex-start;
    min-width: 0;
}
.cleared-note {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
}
</style>
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npx vitest run src/components/library/__tests__/RadioStationForm.spec.ts`
Expected: PASS (all cases).

- [ ] **Step 5: Commit**

```bash
git add src/components/library/RadioStationForm.vue src/components/library/__tests__/RadioStationForm.spec.ts
```
```bash
git commit -m "feat(webui): add RadioStationForm presentational component"
```

---

### Task 2: `RadioStationDetailView` + routes

**Files:**
- Create: `src/views/RadioStationDetailView.vue`
- Modify: `src/router/index.ts` (add two routes after the `/radio` route, lines ~70-75)
- Test: `src/views/__tests__/RadioStationDetailView.spec.ts`
- Test: `src/router/__tests__/radio-routes.spec.ts`

**Interfaces:**
- Consumes: `RadioStationForm` (Task 1) — its `change` payload `{ input, valid, dirty }`; the radio hooks from `@/composables/useSubsonicQueries`; `usePlayer().playNow`; `stationToSong` from `@/utils/radioSong`; `fetchRadioFavicon` from `@/lib/api/RadioBrowser`.
- Produces: named routes `radio-station-new` (`/radio/new`, `props: { create: true }`) and `radio-station-detail` (`/radio/:id`, `props: true`). Header action buttons carry classes `.create-station`, `.save-station`, `.delete-station`, `.play-station`.

- [ ] **Step 1: Write the routes test (failing)**

Create `src/router/__tests__/radio-routes.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import router from '@/router'

describe('radio routes', () => {
    it('resolves /radio/new to the create route with the create prop', () => {
        const r = router.resolve('/radio/new')
        expect(r.name).toBe('radio-station-new')
        expect(r.meta.flush).toBe(true)
    })

    it('resolves /radio/:id to the detail route with the id param', () => {
        const r = router.resolve('/radio/abc123')
        expect(r.name).toBe('radio-station-detail')
        expect(r.params.id).toBe('abc123')
    })

    it('prefers /radio/new over the :id param', () => {
        expect(router.resolve('/radio/new').name).toBe('radio-station-new')
    })
})
```

- [ ] **Step 2: Run it to verify it fails**

Run: `npx vitest run src/router/__tests__/radio-routes.spec.ts`
Expected: FAIL — resolves to no name / `radio-station-detail` catches `/radio/new`.

- [ ] **Step 3: Add the routes**

In `src/router/index.ts`, immediately after the `/radio` route object (the one with `name: 'radio'`, ending `meta: { flush: true }`), insert:

```ts
    {
        path: '/radio/new',
        name: 'radio-station-new',
        component: () => import('@/views/RadioStationDetailView.vue'),
        props: { create: true },
        meta: { flush: true }
    },
    {
        path: '/radio/:id',
        name: 'radio-station-detail',
        component: () => import('@/views/RadioStationDetailView.vue'),
        props: true,
        meta: { flush: true }
    },
```

`/radio/new` MUST come before `/radio/:id`.

- [ ] **Step 4: Write the detail-view test (failing)**

Create `src/views/__tests__/RadioStationDetailView.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import type { InternetRadioStation } from '@/types/subsonic'

const stations = ref<InternetRadioStation[]>([])
const isLoading = ref(false)
const createMutate = vi.fn((_i: unknown, o?: { onSuccess?: () => void }) => o?.onSuccess?.())
const updateMutate = vi.fn((_i: unknown, o?: { onSuccess?: () => void }) => o?.onSuccess?.())
const deleteMutate = vi.fn((_i: unknown, o?: { onSuccess?: () => void }) => o?.onSuccess?.())
vi.mock('@/composables/useSubsonicQueries', () => ({
    useRadioStations: () => ({ data: stations, isLoading }),
    useCreateRadioStation: () => ({ mutate: createMutate, isPending: ref(false) }),
    useUpdateRadioStation: () => ({ mutate: updateMutate, isPending: ref(false) }),
    useDeleteRadioStation: () => ({ mutate: deleteMutate, isPending: ref(false) })
}))

const playNow = vi.fn()
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playNow }) }))
vi.mock('@/utils/radioSong', () => ({ stationToSong: (s: InternetRadioStation) => ({ id: s.id }) }))

const fetchRadioFavicon = vi.fn(async () => null)
vi.mock('@/lib/api/RadioBrowser', () => ({ fetchRadioFavicon }))

// Auto-accept the delete confirmation.
vi.mock('primevue/useconfirm', () => ({
    useConfirm: () => ({ require: (opts: { accept: () => void }) => opts.accept() })
}))

const push = vi.fn()
const back = vi.fn()
const route = { query: {} as Record<string, string> }
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ push, back }),
    onBeforeRouteLeave: vi.fn()
}))

// Stub the form so the test drives `change` directly; stub the scaffold to expose
// title/summary/actions; stub ConfirmDialog and Button.
const FormStub = {
    name: 'RadioStationForm',
    props: ['station', 'prefill'],
    template: '<div class="form-stub" />'
}
const ScaffoldStub = {
    name: 'ContentScaffold',
    props: ['title', 'summary'],
    template: '<div><slot name="actions" /><slot /></div>'
}
const stubs = {
    RadioStationForm: FormStub,
    ContentScaffold: ScaffoldStub,
    ConfirmDialog: { template: '<div />' },
    Button: {
        props: ['label', 'disabled'],
        inheritAttrs: false,
        template:
            '<button :class="$attrs.class" :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>'
    }
}

import RadioStationDetailView from '@/views/RadioStationDetailView.vue'

const station: InternetRadioStation = {
    id: 's1',
    name: 'Jazz FM',
    streamUrl: 'http://stream/jazz',
    homepageUrl: 'http://jazzfm.example',
    coverArt: 'ca1'
}

const emitChange = (w: ReturnType<typeof mount>, valid: boolean, dirty = true) =>
    w.findComponent(FormStub).vm.$emit('change', {
        input: { name: 'Jazz FM', streamUrl: 'http://stream/jazz' },
        valid,
        dirty
    })

const mountView = (props: Record<string, unknown>) =>
    mount(RadioStationDetailView, { props, global: { stubs, directives: { tooltip: {} } } })

beforeEach(() => {
    stations.value = [station]
    isLoading.value = false
    route.query = {}
    createMutate.mockClear()
    updateMutate.mockClear()
    deleteMutate.mockClear()
    playNow.mockClear()
    push.mockClear()
    fetchRadioFavicon.mockClear()
})

describe('RadioStationDetailView', () => {
    it('create mode: titles "Add station" and has a disabled Create until valid', async () => {
        const w = mountView({ create: true })
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('Add station')
        expect(w.find('.create-station').attributes('disabled')).toBeDefined()
        emitChange(w, true)
        await w.vm.$nextTick()
        expect(w.find('.create-station').attributes('disabled')).toBeUndefined()
    })

    it('create mode: Create calls the create mutation and returns to /radio', async () => {
        const w = mountView({ create: true })
        emitChange(w, true)
        await w.vm.$nextTick()
        await w.find('.create-station').trigger('click')
        expect(createMutate).toHaveBeenCalledWith(
            expect.objectContaining({ name: 'Jazz FM', streamUrl: 'http://stream/jazz' }),
            expect.anything()
        )
        expect(push).toHaveBeenCalledWith({ name: 'radio' })
    })

    it('create mode: seeds the form prefill from query params', () => {
        route.query = { name: 'RP', streamUrl: 'http://rp', homepage: 'http://rp.com' }
        const w = mountView({ create: true })
        expect(w.findComponent(FormStub).props('prefill')).toMatchObject({
            name: 'RP',
            streamUrl: 'http://rp',
            homepageUrl: 'http://rp.com'
        })
    })

    it('edit mode: shows the station name and resolves it from the list', () => {
        const w = mountView({ id: 's1' })
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('Jazz FM')
        expect(w.findComponent(FormStub).props('station')).toMatchObject({ id: 's1' })
    })

    it('edit mode: Save calls the update mutation with the id', async () => {
        const w = mountView({ id: 's1' })
        emitChange(w, true)
        await w.vm.$nextTick()
        await w.find('.save-station').trigger('click')
        expect(updateMutate).toHaveBeenCalledWith(
            expect.objectContaining({ id: 's1', name: 'Jazz FM' }),
            expect.anything()
        )
    })

    it('edit mode: Delete (confirmed) removes the station and returns to /radio', async () => {
        const w = mountView({ id: 's1' })
        await w.find('.delete-station').trigger('click')
        expect(deleteMutate).toHaveBeenCalledWith('s1', expect.anything())
        expect(push).toHaveBeenCalledWith({ name: 'radio' })
    })

    it('edit mode: Play enqueues the station', async () => {
        const w = mountView({ id: 's1' })
        await w.find('.play-station').trigger('click')
        expect(playNow).toHaveBeenCalledWith({ id: 's1' })
    })

    it('edit mode: shows not-found when the id is absent after loading', () => {
        stations.value = []
        isLoading.value = false
        const w = mountView({ id: 'missing' })
        expect(w.text()).toContain('not found')
    })
})
```

- [ ] **Step 5: Run both tests to verify they fail**

Run: `npx vitest run src/router/__tests__/radio-routes.spec.ts src/views/__tests__/RadioStationDetailView.spec.ts`
Expected: routes test now PASSES (Step 3 added them); detail-view test FAILS — cannot resolve `RadioStationDetailView.vue`.

- [ ] **Step 6: Create the detail view**

Create `src/views/RadioStationDetailView.vue`:

```vue
<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter, onBeforeRouteLeave } from 'vue-router'
import Button from 'primevue/button'
import ConfirmDialog from 'primevue/confirmdialog'
import { useConfirm } from 'primevue/useconfirm'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import RadioStationForm from '@/components/library/RadioStationForm.vue'
import {
    useRadioStations,
    useCreateRadioStation,
    useUpdateRadioStation,
    useDeleteRadioStation
} from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { stationToSong } from '@/utils/radioSong'
import { fetchRadioFavicon } from '@/lib/api/RadioBrowser'
import type { InternetRadioStation } from '@/types/subsonic'
import type { RadioStationPrefill } from '@/types/radiobrowser'

const props = defineProps<{ id?: string; create?: boolean }>()
const route = useRoute()
const router = useRouter()
const player = usePlayer()
const confirm = useConfirm()

const { data: stations, isLoading } = useRadioStations()
const createMutation = useCreateRadioStation()
const updateMutation = useUpdateRadioStation()
const deleteMutation = useDeleteRadioStation()

interface StationInput {
    name: string
    streamUrl: string
    homepageUrl?: string
    coverFile?: File
    coverClear?: boolean
}
const latest = ref<{ input: StationInput; valid: boolean; dirty: boolean }>({
    input: { name: '', streamUrl: '' },
    valid: false,
    dirty: false
})
// After a successful create/save/delete, suppress the unsaved-changes guard.
const submittedClean = ref(false)
function onFormChange(payload: { input: StationInput; valid: boolean; dirty: boolean }) {
    latest.value = payload
    if (payload.dirty) submittedClean.value = false
}

// Edit mode resolves the station from the cached list (Subsonic has no single GET).
const station = computed<InternetRadioStation | null>(() =>
    props.create ? null : stations.value?.find((s) => s.id === props.id) ?? null
)
const notFound = computed(() => !props.create && !isLoading.value && !station.value)

// Create-mode prefill from Discover query params; fetch the favicon lazily.
const prefill = ref<RadioStationPrefill | null>(null)
onMounted(async () => {
    if (!props.create) return
    const q = route.query
    const name = typeof q.name === 'string' ? q.name : ''
    const streamUrl = typeof q.streamUrl === 'string' ? q.streamUrl : ''
    const homepage = typeof q.homepage === 'string' ? q.homepage : undefined
    if (!name && !streamUrl) return
    const base: RadioStationPrefill = { name, streamUrl, homepageUrl: homepage }
    prefill.value = base
    const favicon = typeof q.favicon === 'string' ? q.favicon : ''
    if (favicon) {
        const cover = await fetchRadioFavicon(favicon)
        if (cover && prefill.value?.streamUrl === streamUrl) {
            prefill.value = { ...base, coverFile: cover }
        }
    }
})

const submitting = computed(
    () => createMutation.isPending.value || updateMutation.isPending.value
)
const title = computed(() => (props.create ? 'Add station' : station.value?.name ?? ''))
const summary = computed(() => (props.create ? '' : station.value?.streamUrl ?? ''))

function onCreate() {
    if (!latest.value.valid) return
    createMutation.mutate(latest.value.input, {
        onSuccess: () => {
            submittedClean.value = true
            router.push({ name: 'radio' })
        }
    })
}
function onSave() {
    if (!latest.value.valid || !station.value) return
    updateMutation.mutate(
        { id: station.value.id, ...latest.value.input },
        { onSuccess: () => (submittedClean.value = true) }
    )
}
function onDelete() {
    const s = station.value
    if (!s) return
    confirm.require({
        message: `Delete station "${s.name}"? This cannot be undone.`,
        header: 'Delete station?',
        icon: 'pi pi-exclamation-triangle',
        rejectLabel: 'Cancel',
        acceptLabel: 'Delete',
        acceptClass: 'p-button-danger',
        accept: () =>
            deleteMutation.mutate(s.id, {
                onSuccess: () => {
                    submittedClean.value = true
                    router.push({ name: 'radio' })
                }
            })
    })
}
function onPlay() {
    if (station.value) player.playNow(stationToSong(station.value))
}

onBeforeRouteLeave(() => {
    if (latest.value.dirty && !submittedClean.value) {
        return window.confirm('You have unsaved changes. Leave without saving?')
    }
})
const onBeforeUnload = (e: BeforeUnloadEvent): void => {
    if (!latest.value.dirty || submittedClean.value) return
    e.preventDefault()
    e.returnValue = ''
}
onMounted(() => window.addEventListener('beforeunload', onBeforeUnload))
onUnmounted(() => window.removeEventListener('beforeunload', onBeforeUnload))
</script>

<template>
    <div class="radio-station-detail-view">
        <div class="back-row">
            <Button icon="pi pi-arrow-left" text rounded @click="router.back()" />
        </div>

        <div v-if="isLoading && !create" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="notFound" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>Station not found</p>
        </div>

        <ContentScaffold v-else :title="title" :summary="summary">
            <template #actions>
                <Button
                    v-if="!create"
                    class="play-station"
                    label="Play"
                    icon="pi pi-play"
                    @click="onPlay"
                />
                <Button
                    v-if="create"
                    class="create-station"
                    label="Create"
                    icon="pi pi-plus"
                    :disabled="!latest.valid"
                    :loading="submitting"
                    @click="onCreate"
                />
                <Button
                    v-else
                    class="save-station"
                    label="Save"
                    icon="pi pi-check"
                    :disabled="!latest.valid"
                    :loading="submitting"
                    @click="onSave"
                />
                <Button
                    v-if="!create"
                    class="delete-station"
                    icon="pi pi-trash"
                    text
                    rounded
                    severity="danger"
                    v-tooltip.bottom="'Delete station'"
                    @click="onDelete"
                />
            </template>

            <div class="detail-scroll">
                <div class="detail-body">
                    <RadioStationForm
                        :station="station"
                        :prefill="prefill"
                        @change="onFormChange"
                    />
                </div>
            </div>
        </ContentScaffold>

        <ConfirmDialog />
    </div>
</template>

<style scoped>
.radio-station-detail-view {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
}
.back-row {
    flex-shrink: 0;
    padding: 0.5rem 2rem 0;
}
.loading,
.error {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 3rem;
    gap: 1rem;
    color: var(--app-text-secondary);
}
.detail-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
}
.detail-body {
    max-width: var(--app-content-max-width);
    margin: 0 auto;
    padding: 1rem;
}
</style>
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `npx vitest run src/router/__tests__/radio-routes.spec.ts src/views/__tests__/RadioStationDetailView.spec.ts`
Expected: PASS (both files).

- [ ] **Step 8: Commit**

```bash
git add src/views/RadioStationDetailView.vue src/views/__tests__/RadioStationDetailView.spec.ts src/router/index.ts src/router/__tests__/radio-routes.spec.ts
```
```bash
git commit -m "feat(webui): add routed radio station detail view for create/edit/delete"
```

---

### Task 3: Card and row navigate to the detail view

**Files:**
- Modify: `src/components/library/RadioStationCard.vue`
- Modify: `src/components/library/RadioStationRow.vue`
- Test: `src/components/library/__tests__/RadioStationCard.spec.ts`
- Test: `src/components/library/__tests__/RadioStationRow.spec.ts`

**Interfaces:**
- Consumes: the `radio-station-detail` route (Task 2).
- Produces: the card/row root is a `RouterLink` to `{ name: 'radio-station-detail', params: { id: station.id } }`; drag + hover play preserved.

- [ ] **Step 1: Update the card test (failing)**

In `src/components/library/__tests__/RadioStationCard.spec.ts`, add a player mock after the existing mocks and replace the "does not play the station on click" test with navigation assertions. Add near the top (after the `subsonic` mock):

```ts
const playNow = vi.fn()
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playNow }) }))
vi.mock('@/utils/radioSong', () => ({ stationToSong: (s: any) => ({ id: s.id, title: s.name, streamUrl: s.streamUrl }) }))
```

Change `mountCard` to register the RouterLink stub:

```ts
import { RouterLinkStub } from '@vue/test-utils'

const mountCard = (s?: InternetRadioStation) =>
    mount(RadioStationCard, {
        props: { station: s },
        global: { stubs: { RouterLink: RouterLinkStub } }
    })
```

Replace the `does not play the station on click` test with:

```ts
    it('links to the station detail route', () => {
        const w = mountCard(station)
        expect(w.findComponent(RouterLinkStub).props('to')).toEqual({
            name: 'radio-station-detail',
            params: { id: 's1' }
        })
    })

    it('the play button plays the station without navigating', async () => {
        const w = mountCard(station)
        await w.find('.card-play').trigger('click')
        expect(playNow).toHaveBeenCalledTimes(1)
    })
```

- [ ] **Step 2: Run it to verify it fails**

Run: `npx vitest run src/components/library/__tests__/RadioStationCard.spec.ts`
Expected: FAIL — no `RouterLink`, `.card-play` play not wired to navigate-safe handler.

- [ ] **Step 3: Update `RadioStationCard.vue`**

Change the rendered station root from a `<div class="radio-card">` to a `RouterLink`, keeping drag and the play button. Replace the second template block (the `v-else` div) with:

```vue
    <RouterLink
        v-else
        class="radio-card"
        :to="{ name: 'radio-station-detail', params: { id: station.id } }"
        draggable="true"
        @dragstart="onCardDragStart"
        @dragend="songsDrag.end"
    >
        <div class="card-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="station.name" draggable="false" />
            <div v-else class="cover-placeholder">
                <i class="pi pi-wifi" style="font-size: 2rem"></i>
            </div>
        </div>
        <div class="card-info">
            <div class="card-text">
                <div class="card-title">{{ station.name }}</div>
                <div class="card-subtitle">
                    <template v-if="station.homepageUrl">{{ station.homepageUrl }}</template>
                    <template v-else>&nbsp;</template>
                </div>
            </div>
            <button class="card-play" type="button" aria-label="Play station" @click="onPlay">
                <i class="pi pi-play"></i>
            </button>
        </div>
    </RouterLink>
```

Update `onPlay` to also prevent default navigation:

```ts
const onPlay = (event: Event): void => {
    event.preventDefault()
    event.stopPropagation()
    if (props.station) player.playNow(stationToSong(props.station))
}
```

(No `import` changes needed — `RouterLink` is globally registered by vue-router. `.radio-card` styles already cover a flex column; `text-decoration: none; color: inherit` are present.)

- [ ] **Step 4: Update the row test (failing first, then implement)**

In `src/components/library/__tests__/RadioStationRow.spec.ts`, register the RouterLink stub and replace the "does not play the station on click" test:

```ts
import { RouterLinkStub } from '@vue/test-utils'

const mountRow = (s?: InternetRadioStation) =>
    mount(RadioStationRow, {
        props: { station: s },
        global: { stubs: { RouterLink: RouterLinkStub } }
    })
```

Replace the `does not play the station on click` test with:

```ts
    it('links to the station detail route', () => {
        const w = mountRow(station)
        expect(w.findComponent(RouterLinkStub).props('to')).toEqual({
            name: 'radio-station-detail',
            params: { id: 's1' }
        })
    })
```

- [ ] **Step 5: Update `RadioStationRow.vue`**

Change the rendered `v-else` `<div class="radio-row">` to a `RouterLink`:

```vue
    <RouterLink
        v-else
        class="radio-row"
        :to="{ name: 'radio-station-detail', params: { id: station.id } }"
        draggable="true"
        @dragstart="onRowDragStart"
        @dragend="songsDrag.end"
    >
        <div class="col-avatar">
            <img v-if="coverUrl" :src="coverUrl" :alt="station.name" draggable="false" />
            <div v-else class="avatar-placeholder"><i class="pi pi-wifi"></i></div>
        </div>
        <div class="col-name">{{ station.name }}</div>
        <div class="col-homepage">{{ station.homepageUrl }}</div>
    </RouterLink>
```

(`.radio-row` already sets `text-decoration: none; color: inherit`. The `:deep(.radio-row)` centering rule in `RadioStationListView.vue` still applies since the class is unchanged.)

- [ ] **Step 6: Run both tests to verify they pass**

Run: `npx vitest run src/components/library/__tests__/RadioStationCard.spec.ts src/components/library/__tests__/RadioStationRow.spec.ts`
Expected: PASS (both files).

- [ ] **Step 7: Commit**

```bash
git add src/components/library/RadioStationCard.vue src/components/library/RadioStationRow.vue src/components/library/__tests__/RadioStationCard.spec.ts src/components/library/__tests__/RadioStationRow.spec.ts
```
```bash
git commit -m "feat(webui): navigate to station detail from radio cards and rows"
```

---

### Task 4: Relocate `StationSearchDialog` to `components/library`

**Files:**
- Move: `src/views/settings/radio-stations/StationSearchDialog.vue` → `src/components/library/StationSearchDialog.vue`
- Move: `src/views/settings/radio-stations/__tests__/StationSearchDialog.spec.ts` → `src/components/library/__tests__/StationSearchDialog.spec.ts`
- Modify: `src/views/settings/RadioStationsView.vue` (update the import path — temporary; this view is deleted in Task 6)

**Interfaces:**
- Produces: `StationSearchDialog` at `@/components/library/StationSearchDialog.vue`, unchanged props (`visible`) and emits (`update:visible`, `select` with `RadioBrowserStation`).

- [ ] **Step 1: Move the component and its test with git**

```bash
git mv src/views/settings/radio-stations/StationSearchDialog.vue src/components/library/StationSearchDialog.vue
```
```bash
git mv src/views/settings/radio-stations/__tests__/StationSearchDialog.spec.ts src/components/library/__tests__/StationSearchDialog.spec.ts
```

- [ ] **Step 2: Fix the moved test's import path**

In `src/components/library/__tests__/StationSearchDialog.spec.ts`, change the component import to:

```ts
import StationSearchDialog from '@/components/library/StationSearchDialog.vue'
```

(Other imports use `@/…` aliases and are unaffected. The moved `.vue` file's own imports are all `@/…` aliases, so they need no changes.)

- [ ] **Step 3: Update the settings view import (temporary)**

In `src/views/settings/RadioStationsView.vue`, change:

```ts
import StationSearchDialog from './radio-stations/StationSearchDialog.vue'
```
to:
```ts
import StationSearchDialog from '@/components/library/StationSearchDialog.vue'
```

- [ ] **Step 4: Run the affected tests + type-check**

Run: `npx vitest run src/components/library/__tests__/StationSearchDialog.spec.ts`
Expected: PASS.
Run: `npm run type-check`
Expected: no errors (the settings view still compiles).

- [ ] **Step 5: Commit**

Stage only this task's files (the working tree has unrelated WIP — never `git add -A`). The `git mv` in Step 1 already staged the renames; add the edited files:

```bash
git add src/components/library/StationSearchDialog.vue src/components/library/__tests__/StationSearchDialog.spec.ts src/views/settings/RadioStationsView.vue
```
```bash
git commit -m "refactor(webui): move StationSearchDialog into components/library"
```

---

### Task 5: `RadioView` header — Discover + Add Station

**Files:**
- Modify: `src/views/RadioView.vue`
- Test: `src/views/__tests__/RadioView.spec.ts`

**Interfaces:**
- Consumes: `StationSearchDialog` (Task 4); the `radio-station-new` route (Task 2).
- Produces: header buttons `.discover-station` and `.add-station`; on `StationSearchDialog`'s `select`, `router.push({ name: 'radio-station-new', query })` with defined `name`/`streamUrl`/`homepage`/`favicon` only.

- [ ] **Step 1: Update the RadioView test (failing)**

In `src/views/__tests__/RadioView.spec.ts`:

Change the router mock to expose `push`:

```ts
const replace = vi.fn()
const push = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace, push })
}))
```

Add a stub for the dialog to the `stubs` and to the `mountView` stubs list:

```ts
const SearchDialogStub = {
    name: 'StationSearchDialog',
    props: ['visible'],
    emits: ['update:visible', 'select'],
    template: '<div class="search-dialog-stub" />'
}
```
and in `mountView`'s `stubs`: `StationSearchDialog: SearchDialogStub`.

Reset `push` in `beforeEach`: `push.mockReset()`.

Add these tests inside `describe('RadioView', …)`:

```ts
    it('renders Discover and Add Station buttons in the header', () => {
        const w = mountView()
        expect(w.find('.discover-station').exists()).toBe(true)
        expect(w.find('.add-station').exists()).toBe(true)
    })

    it('Add Station navigates to the create route', async () => {
        const w = mountView()
        await w.find('.add-station').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'radio-station-new' })
    })

    it('picking a station from Discover navigates to create with query prefill', async () => {
        const w = mountView()
        w.findComponent(SearchDialogStub).vm.$emit('select', {
            name: 'Radio Paradise',
            streamUrl: 'http://rp/stream',
            homepage: 'http://rp.com',
            favicon: 'http://rp.com/fav.png'
        })
        await w.vm.$nextTick()
        expect(push).toHaveBeenCalledWith({
            name: 'radio-station-new',
            query: {
                name: 'Radio Paradise',
                streamUrl: 'http://rp/stream',
                homepage: 'http://rp.com',
                favicon: 'http://rp.com/fav.png'
            }
        })
    })
```

The Button stub must expose its class. Add to `mountView`'s `stubs` (if not already a real PrimeVue Button):

```ts
Button: {
    props: ['label'],
    inheritAttrs: false,
    template: '<button :class="$attrs.class" @click="$emit(\'click\')">{{ label }}</button>'
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `npx vitest run src/views/__tests__/RadioView.spec.ts`
Expected: FAIL — buttons and dialog not present.

- [ ] **Step 3: Update `RadioView.vue`**

Add imports and the Discover state/handlers to `<script setup>`:

```ts
import Button from 'primevue/button'
import StationSearchDialog from '@/components/library/StationSearchDialog.vue'
import type { RadioBrowserStation } from '@/types/radiobrowser'
```

Add near the other refs (after `const layout = …`):

```ts
const searchVisible = ref(false)

function openAdd() {
    router.push({ name: 'radio-station-new' })
}

function onDiscoverSelect(station: RadioBrowserStation) {
    const query: Record<string, string> = {
        name: station.name,
        streamUrl: station.streamUrl
    }
    if (station.homepage) query.homepage = station.homepage
    if (station.favicon) query.favicon = station.favicon
    router.push({ name: 'radio-station-new', query })
}
```

Ensure `ref` is imported (add to the existing `import { computed } from 'vue'` → `import { computed, ref } from 'vue'`).

In the template `#actions` slot, after the `SelectButton`, add:

```vue
            <Button
                class="discover-station"
                label="Discover"
                icon="pi pi-globe"
                outlined
                @click="searchVisible = true"
            />
            <Button
                class="add-station"
                label="Add Station"
                icon="pi pi-plus"
                @click="openAdd"
            />
```

After the `RadioStationListView`/`RadioStationGrid` block (still inside `ContentScaffold`), add the dialog:

```vue
        <StationSearchDialog v-model:visible="searchVisible" @select="onDiscoverSelect" />
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `npx vitest run src/views/__tests__/RadioView.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/views/RadioView.vue src/views/__tests__/RadioView.spec.ts
```
```bash
git commit -m "feat(webui): add Discover and Add Station buttons to the radio view header"
```

---

### Task 6: Remove the Settings radio page and orphaned dialog

**Files:**
- Delete: `src/views/settings/RadioStationsView.vue`
- Delete: `src/views/settings/radio-stations/StationList.vue` (+ `__tests__/StationList.spec.ts`)
- Delete: `src/views/settings/radio-stations/StationEditPanel.vue` (+ `__tests__/StationEditPanel.spec.ts`)
- Delete: `src/views/settings/radio-stations/__tests__/RadioStationsView.spec.ts`
- Delete: `src/components/library/RadioStationDialog.vue` (+ `__tests__/RadioStationDialog.spec.ts`)
- Modify: `src/router/index.ts` (remove the `settings-radio` child route)
- Modify: `src/layouts/SettingsLayout.vue` (remove the "Radio Stations" nav entry, line ~40)
- Modify: `src/router/__tests__/settings-routes.spec.ts` (remove the `settings-radio` assertion)

**Interfaces:**
- Consumes: nothing new. This task only removes code; `RadioStationsView` was the sole consumer of `StationList`/`StationEditPanel`, and `RadioStationDialog` was already unused.

- [ ] **Step 1: Update the settings-routes test first**

In `src/router/__tests__/settings-routes.spec.ts`, remove the line:

```ts
        expect(router.resolve('/settings/radio').name).toBe('settings-radio')
```

- [ ] **Step 2: Remove the route and nav entry**

In `src/router/index.ts`, delete the `settings-radio` child route object:

```ts
            {
                path: 'radio',
                name: 'settings-radio',
                component: () => import('@/views/settings/RadioStationsView.vue')
            }
```
(and remove the trailing comma left dangling on the previous entry if needed).

In `src/layouts/SettingsLayout.vue`, delete the nav item:

```ts
            { label: 'Radio Stations', icon: 'pi pi-wifi', route: '/settings/radio' }
```
(fix the preceding item's trailing comma as needed).

- [ ] **Step 3: Delete the dead files**

```bash
git rm src/views/settings/RadioStationsView.vue \
       src/views/settings/radio-stations/StationList.vue \
       src/views/settings/radio-stations/StationEditPanel.vue \
       src/views/settings/radio-stations/__tests__/StationList.spec.ts \
       src/views/settings/radio-stations/__tests__/StationEditPanel.spec.ts \
       src/views/settings/radio-stations/__tests__/RadioStationsView.spec.ts \
       src/components/library/RadioStationDialog.vue \
       src/components/library/__tests__/RadioStationDialog.spec.ts
```

- [ ] **Step 4: Confirm no dangling references**

Run: `grep -rn "RadioStationsView\|StationEditPanel\|StationList\|RadioStationDialog\|settings-radio\|settings/radio" src`
Expected: no matches (an empty result). If `radio-stations/` is now an empty directory, remove it: `rmdir src/views/settings/radio-stations/__tests__ src/views/settings/radio-stations 2>/dev/null || true`.

- [ ] **Step 5: Full suite + type-check**

Run: `npm test`
Expected: PASS — `vue-tsc --noEmit` clean and all Vitest suites green.

- [ ] **Step 6: Commit**

The `git rm` in Step 3 staged the deletions; stage only the files this task modified (never `git add -A` — the tree has unrelated WIP):

```bash
git add src/router/index.ts src/layouts/SettingsLayout.vue src/router/__tests__/settings-routes.spec.ts
```
```bash
git commit -m "refactor(webui): remove settings radio page now that CRUD lives in /radio"
```

---

## Self-Review

**Spec coverage:**
- Routes (`/radio/new`, `/radio/:id`, remove `settings-radio`) → Task 2, Task 6. ✓
- RadioView header Discover + Add → Task 5. ✓
- Discover prefill via query + favicon refetch → Task 2 (detail view `onMounted`) + Task 5 (navigation). ✓
- `RadioStationDetailView` create/edit modes, Play/Save/Delete, not-found → Task 2. ✓
- `RadioStationForm` extraction (fields, cover, 5 MB guard, clear) → Task 1. ✓
- Card/row whole-element navigation, drag + play preserved → Task 3. ✓
- Deletions (settings view, StationList, StationEditPanel, RadioStationDialog, route, nav link, specs) → Task 6. ✓
- `StationSearchDialog` moved not deleted → Task 4. ✓
- Tests: form, detail view, RadioView, card, row, router, moved search dialog → Tasks 1-6. ✓

**Placeholder scan:** No TBD/TODO; every code step shows full code. ✓

**Type consistency:** `StationInput` shape `{ name, streamUrl, homepageUrl?, coverFile?, coverClear? }` is identical in Task 1 (form emit), Task 2 (detail view `latest`), and the mutation calls. `change` payload `{ input, valid, dirty }` matches between Task 1's emit and Task 2's `onFormChange`. Route names `radio-station-new` / `radio-station-detail` are consistent across Tasks 2, 3, 5. ✓
