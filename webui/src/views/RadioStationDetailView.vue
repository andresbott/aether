<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { useRouter, onBeforeRouteLeave } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import ConfirmDialog from 'primevue/confirmdialog'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import HeroHeader from '@/components/layout/HeroHeader.vue'
import HeroActions from '@/components/layout/HeroActions.vue'
import EditActionBar from '@/components/layout/EditActionBar.vue'
import StationSearchDialog from '@/components/library/StationSearchDialog.vue'
import {
    useRadioStations,
    useCreateRadioStation,
    useUpdateRadioStation,
    useDeleteRadioStation
} from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { useAuth } from '@/composables/useAuth'
import { stationToSong } from '@/utils/radioSong'
import { fetchRadioFavicon } from '@/lib/api/RadioBrowser'
import { subsonicClient } from '@/lib/api/subsonic'
import { bumpCoverVersion, versionedCoverUrl } from '@/composables/useCoverVersion'
import type { InternetRadioStation } from '@/types/subsonic'
import type { RadioBrowserStation } from '@/types/radiobrowser'

const MAX_COVER_BYTES = 5 * 1024 * 1024

const props = defineProps<{ id?: string; create?: boolean }>()
const router = useRouter()
const player = usePlayer()

// Discover proxies through the admin-only /api/v1/radiobrowser endpoints, so
// non-admins don't get a button that can only 403. (Station CRUD itself rides
// on /rest and stays open until the planned role gate there — TODO.md.)
const { isAdmin } = useAuth()

const { data: stations, isLoading } = useRadioStations()
const createMutation = useCreateRadioStation()
const updateMutation = useUpdateRadioStation()
const deleteMutation = useDeleteRadioStation()

// Edit mode resolves the station from the cached list (Subsonic has no single GET).
const station = computed<InternetRadioStation | null>(() =>
    props.create ? null : (stations.value?.find((s) => s.id === props.id) ?? null)
)
const notFound = computed(() => !props.create && !isLoading.value && !station.value)

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

// Seed the form from the station when its identity changes (edit mode); create mode
// starts empty. Snapshot the baseline afterward so `dirty` reflects user edits only.
watch(
    () => station.value?.id ?? null,
    () => {
        if (station.value) {
            form.value = {
                name: station.value.name,
                streamUrl: station.value.streamUrl,
                homepageUrl: station.value.homepageUrl ?? ''
            }
        } else {
            form.value = emptyForm()
        }
        baseline.value = { ...form.value }
        resetCoverState()
    },
    { immediate: true }
)

const hasExistingCover = computed(
    () => !props.create && !!station.value?.coverArt && !coverClear.value
)
const displayedCoverUrl = computed(() => {
    if (previewUrl.value) return previewUrl.value
    if (hasExistingCover.value && station.value?.coverArt) {
        const base = subsonicClient.getCoverArtUrl(station.value.coverArt, 250)
        return versionedCoverUrl(base, station.value.coverArt)
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

// Discover: search radio-browser.info and fill the form from the picked station.
// Picking is an explicit choice, so it overwrites the whole form and any staged
// cover. The baseline stays at the empty form — a discovered fill counts as
// unsaved changes, same as typing the values by hand.
const searchVisible = ref(false)
let unmounted = false

function onDiscoverSelect(s: RadioBrowserStation) {
    form.value = {
        name: s.name,
        streamUrl: s.streamUrl,
        homepageUrl: s.homepage ?? ''
    }
    resetCoverState()
    if (!s.favicon) return
    const seededStreamUrl = s.streamUrl
    fetchRadioFavicon(s.favicon).then((cover) => {
        // Skip when superseded during the fetch: re-discovered / stream URL
        // edited / user staged their own cover or a clear / unmounted.
        if (!cover) return
        if (unmounted) return
        if (form.value.streamUrl !== seededStreamUrl) return
        if (selectedFile.value !== null || coverClear.value) return
        selectedFile.value = cover
        previewUrl.value = URL.createObjectURL(cover)
    })
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
                // The cover URL is unchanged, so bust it by version — otherwise
                // the browser's in-memory cache keeps the old image, here and
                // after navigating away and back.
                if (station.value?.coverArt) bumpCoverVersion(station.value.coverArt)
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
        // Cancelling create leaves the page. Suppress the route-leave guard's
        // unsaved-changes prompt: cancelling is already an explicit discard (and Esc
        // has verified via EditActionBar), so a second confirm would be redundant.
        submittedClean.value = true
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
    unmounted = true
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
                <Button
                    v-if="create && isAdmin"
                    class="discover-station"
                    icon="pi pi-globe"
                    text
                    rounded
                    v-tooltip.bottom="'Discover'"
                    aria-label="Discover"
                    @click="searchVisible = true"
                />
                <EditActionBar
                    v-if="isAdmin"
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
            </template>

            <div class="detail-scroll">
                <div class="detail-body content-col">
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
                        <template #actions>
                            <HeroActions @play="onPlay" />
                        </template>
                    </HeroHeader>
                </div>
            </div>
        </ContentScaffold>

        <ConfirmDialog />
        <StationSearchDialog
            v-if="create && isAdmin"
            v-model:visible="searchVisible"
            @select="onDiscoverSelect"
        />
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
    /* Recipe B: uniform rail clearance so the column matches the list views. */
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    box-sizing: border-box;
}
.detail-body {
    padding-top: 1rem;
    padding-bottom: 1rem;
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
