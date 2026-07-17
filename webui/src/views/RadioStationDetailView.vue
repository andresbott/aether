<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter, onBeforeRouteLeave } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import ConfirmDialog from 'primevue/confirmdialog'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import HeroHeader from '@/components/layout/HeroHeader.vue'
import EditActionBar from '@/components/layout/EditActionBar.vue'
import {
    useRadioStations,
    useCreateRadioStation,
    useUpdateRadioStation,
    useDeleteRadioStation
} from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { stationToSong } from '@/utils/radioSong'
import { fetchRadioFavicon } from '@/lib/api/RadioBrowser'
import { subsonicClient } from '@/lib/api/subsonic'
import type { InternetRadioStation } from '@/types/subsonic'
import type { RadioStationPrefill } from '@/types/radiobrowser'

const MAX_COVER_BYTES = 5 * 1024 * 1024

const props = defineProps<{ id?: string; create?: boolean }>()
const route = useRoute()
const router = useRouter()
const player = usePlayer()

const { data: stations, isLoading } = useRadioStations()
const createMutation = useCreateRadioStation()
const updateMutation = useUpdateRadioStation()
const deleteMutation = useDeleteRadioStation()

// Edit mode resolves the station from the cached list (Subsonic has no single GET).
const station = computed<InternetRadioStation | null>(() =>
    props.create ? null : (stations.value?.find((s) => s.id === props.id) ?? null)
)
const notFound = computed(() => !props.create && !isLoading.value && !station.value)

// Create-mode prefill from Discover query params; fetch the favicon lazily.
const prefill = ref<RadioStationPrefill | null>(null)
if (props.create) {
    const q = route.query
    const name = typeof q.name === 'string' ? q.name : ''
    const streamUrl = typeof q.streamUrl === 'string' ? q.streamUrl : ''
    const homepage = typeof q.homepage === 'string' ? q.homepage : undefined
    if (name || streamUrl) {
        prefill.value = { name, streamUrl, homepageUrl: homepage }
    }
}
onMounted(async () => {
    if (!props.create || !prefill.value) return
    const q = route.query
    const favicon = typeof q.favicon === 'string' ? q.favicon : ''
    if (!favicon) return
    const base = prefill.value
    const cover = await fetchRadioFavicon(favicon)
    if (cover && prefill.value?.streamUrl === base.streamUrl) {
        prefill.value = { ...base, coverFile: cover }
    }
})

// --- Form state (lifted from the former RadioStationForm) ---
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

// Create starts already in edit mode; existing stations open read-only.
const editing = ref(!!props.create)

function resetCoverState() {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    selectedFile.value = null
    coverClear.value = false
    sizeError.value = null
}

// Seed the text fields from the station (edit) or the prefill (create-from-Discover),
// keyed on the TEXT identity rather than the prefill object reference. Note this MUST be
// a multi-source array of individual getters — Vue only compares each source elementwise
// with `hasChanged` in that form; a single getter returning a fresh array every time
// would always compare as "changed" and re-fire on every prefill reassignment. This way a
// later reassignment of `prefill` that only folds in a favicon cover does not re-seed and
// clobber in-progress user edits. Snapshot the baseline afterward so `dirty` reflects user
// edits only.
watch(
    [
        () => station.value?.id ?? null,
        () => prefill.value?.name,
        () => prefill.value?.streamUrl,
        () => prefill.value?.homepageUrl
    ],
    () => {
        if (station.value) {
            form.value = {
                name: station.value.name,
                streamUrl: station.value.streamUrl,
                homepageUrl: station.value.homepageUrl ?? ''
            }
        } else if (prefill.value) {
            form.value = {
                name: prefill.value.name,
                streamUrl: prefill.value.streamUrl,
                homepageUrl: prefill.value.homepageUrl ?? ''
            }
        } else {
            form.value = emptyForm()
        }
        baseline.value = { ...form.value }
    },
    { immediate: true }
)

// Reset the cover picker whenever the station/prefill IDENTITY changes, keyed the same way
// as the text-seed watcher — NOT on the prefill object reference or its `coverFile`. This
// way folding a fetched favicon into the *same* prefill does not wipe cover edits.
watch(
    [
        () => station.value?.id ?? null,
        () => prefill.value?.name,
        () => prefill.value?.streamUrl,
        () => prefill.value?.homepageUrl
    ],
    () => {
        resetCoverState()
        if (!station.value && prefill.value?.coverFile) {
            selectedFile.value = prefill.value.coverFile
            previewUrl.value = URL.createObjectURL(prefill.value.coverFile)
        }
    },
    { immediate: true }
)

// Fold in a favicon cover that resolves asynchronously after the initial seed, without
// touching `form`/`baseline`. Skip if the user already picked their own file or staged a
// clear during the fetch window.
watch(
    () => prefill.value?.coverFile,
    (coverFile) => {
        if (!coverFile) return
        if (selectedFile.value !== null || coverClear.value) return
        if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
        selectedFile.value = coverFile
        previewUrl.value = URL.createObjectURL(coverFile)
    }
)

const hasExistingCover = computed(
    () => !props.create && !!station.value?.coverArt && !coverClear.value
)
const displayedCoverUrl = computed(() => {
    if (previewUrl.value) return previewUrl.value
    if (hasExistingCover.value && station.value?.coverArt) {
        return subsonicClient.getCoverArtUrl(station.value.coverArt, 250)
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

const input = computed(() => {
    const homepage = form.value.homepageUrl.trim()
    return {
        name: form.value.name.trim(),
        streamUrl: form.value.streamUrl.trim(),
        homepageUrl: homepage === '' ? undefined : homepage,
        coverFile: selectedFile.value ?? undefined,
        coverClear: coverClear.value || undefined
    }
})

function onCoverSelect(file: File) {
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

// After a successful create/save/delete, suppress the unsaved-changes guard.
const submittedClean = ref(false)
watch(dirty, (d) => {
    if (d) submittedClean.value = false
})

const submitting = computed(() => createMutation.isPending.value || updateMutation.isPending.value)
const title = computed(() => (props.create ? 'Add station' : ''))

function onCreate() {
    if (!valid.value) return
    createMutation.mutate(input.value, {
        onSuccess: () => {
            submittedClean.value = true
            router.push({ name: 'radio' })
        }
    })
}
function onSave() {
    if (!valid.value || !station.value) return
    updateMutation.mutate(
        { id: station.value.id, ...input.value },
        {
            onSuccess: () => {
                submittedClean.value = true
                baseline.value = { ...form.value }
                resetCoverState()
                editing.value = false
            }
        }
    )
}
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
function onPlay() {
    if (station.value) player.playNow(stationToSong(station.value))
}

onBeforeRouteLeave(() => {
    if (dirty.value && !submittedClean.value) {
        return window.confirm('You have unsaved changes. Leave without saving?')
    }
})
const onBeforeUnload = (e: BeforeUnloadEvent): void => {
    if (!dirty.value || submittedClean.value) return
    e.preventDefault()
    e.returnValue = ''
}
onMounted(() => window.addEventListener('beforeunload', onBeforeUnload))
onUnmounted(() => {
    window.removeEventListener('beforeunload', onBeforeUnload)
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
})
</script>

<template>
    <div class="radio-station-detail-view">
        <div v-if="isLoading && !create" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="notFound" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>Station not found</p>
        </div>

        <ContentScaffold v-else :title="title" show-back @back="router.back()">
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

            <div class="detail-scroll">
                <div class="detail-body">
                    <HeroHeader
                        eyebrow="Radio Station"
                        cover-back-label="Station cover"
                        :cover-url="displayedCoverUrl"
                        :cover-size-error="sizeError"
                        v-model:editing="editing"
                        @cover-select="onCoverSelect"
                        @cover-remove="onRemoveCover"
                    >
                        <template #read>
                            <h2 class="hero-name">{{ station?.name }}</h2>
                            <div class="meta-row">
                                <span v-if="station?.streamUrl">{{ station.streamUrl }}</span>
                                <span
                                    v-if="station?.homepageUrl"
                                    :class="{ dot: !!station?.streamUrl }"
                                >
                                    {{ station.homepageUrl }}
                                </span>
                            </div>
                        </template>
                        <template #edit>
                            <label class="form-field">
                                <span class="field-label">Name</span>
                                <InputText v-model="form.name" placeholder="e.g. BBC Radio 1" />
                            </label>
                            <label class="form-field">
                                <span class="field-label">Stream URL</span>
                                <InputText
                                    v-model="form.streamUrl"
                                    placeholder="http://example.com/stream"
                                />
                            </label>
                            <label class="form-field">
                                <span class="field-label">Homepage URL</span>
                                <InputText v-model="form.homepageUrl" placeholder="optional" />
                            </label>
                        </template>
                    </HeroHeader>
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
.form-field {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
}
.form-field :deep(.p-inputtext) {
    width: 100%;
}
.field-label {
    font-size: 0.72rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--app-text-secondary);
}
</style>
