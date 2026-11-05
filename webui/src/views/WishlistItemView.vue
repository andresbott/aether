<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Textarea from 'primevue/textarea'
import InputText from 'primevue/inputtext'
import SelectButton from 'primevue/selectbutton'
import ConfirmDialog from 'primevue/confirmdialog'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import HeroHeader from '@/components/layout/HeroHeader.vue'
import EditActionBar from '@/components/layout/EditActionBar.vue'
import { useDeleteWishlistItem, useUpdateWishlistItem, useWishlistItem } from '@/composables/useWishlist'
import { subsonicClient } from '@/lib/api/subsonic'
import {
    isManualEntry,
    musicBrainzUrl,
    wishlistDetails,
    WISHLIST_KINDS,
    wishlistKind,
    wishlistLibraryId,
    wishlistLibraryLabel,
    wishlistLibraryRoute,
    wishlistSubtitle
} from '@/lib/wishlist'
import type { WishlistKind } from '@/types/wishlist'

// Mirror the server's limits (handlers/wishlist).
const NOTE_MAX = 2000
const TITLE_MAX = 500

const props = defineProps<{ id: string }>()
const router = useRouter()

const numericId = computed(() => Number(props.id))
const { data: item, isLoading, isError } = useWishlistItem(numericId)
const updateMutation = useUpdateWishlistItem()
const deleteMutation = useDeleteWishlistItem()

const notFound = computed(() => !isLoading.value && !item.value)

const kindLabel = computed(() => (item.value ? wishlistKind(item.value.kind).label : ''))
const eyebrow = computed(() => (kindLabel.value ? `Wishlist · ${kindLabel.value}` : 'Wishlist'))
const placeholderIcon = computed(() => (item.value ? wishlistKind(item.value.kind).icon : 'ms-bookmark-heart'))
const subtitle = computed(() => (item.value ? wishlistSubtitle(item.value) : ''))
const details = computed(() => (item.value ? wishlistDetails(item.value) : []))

const libraryRoute = computed(() => (item.value ? wishlistLibraryRoute(item.value) : null))
const libraryLabel = computed(() => (item.value ? wishlistLibraryLabel(item.value.kind) : ''))
const mbUrl = computed(() => (item.value ? musicBrainzUrl(item.value.kind, item.value.mbid) : null))
const manual = computed(() => (item.value ? isManualEntry(item.value) : false))

// A found entry wears the library's cover of what it matched.
const coverUrl = computed(() => {
    const id = item.value ? wishlistLibraryId(item.value) : null
    return id && subsonicClient.isConfigured() ? subsonicClient.getCoverArtUrl(id, 500) : null
})

const foundSince = computed(() => {
    const at = item.value?.found_at
    if (!at) return ''
    const d = new Date(at)
    return Number.isNaN(d.getTime()) ? '' : d.toLocaleDateString()
})

// --- Editing ---------------------------------------------------------------
// An entry added by hand owns every field (type, title, description); a
// MusicBrainz pick only its description — the rest is its MusicBrainz snapshot.
interface FormState {
    kind: WishlistKind
    title: string
    note: string
}
const kindOptions = WISHLIST_KINDS.map((k) => ({ label: k.label, value: k.value, icon: k.icon }))
const editing = ref(false)
const form = ref<FormState>({ kind: 'album', title: '', note: '' })
const baseline = ref<FormState>({ kind: 'album', title: '', note: '' })
function seedForm() {
    const it = item.value
    form.value = { kind: it?.kind ?? 'album', title: it?.title ?? '', note: it?.note ?? '' }
    baseline.value = { ...form.value }
}
watch(
    () => [item.value?.id, item.value?.kind, item.value?.title, item.value?.note] as const,
    () => {
        if (!editing.value) seedForm()
    },
    { immediate: true }
)
const dirty = computed(
    () =>
        form.value.note !== baseline.value.note ||
        (manual.value && (form.value.title !== baseline.value.title || form.value.kind !== baseline.value.kind))
)
const valid = computed(
    () =>
        form.value.note.length <= NOTE_MAX &&
        (!manual.value || (form.value.title.trim().length > 0 && form.value.title.length <= TITLE_MAX))
)

const submittedClean = ref(false)
watch(dirty, (d) => {
    if (d) submittedClean.value = false
})

function onSave() {
    if (!item.value || !valid.value) return
    const patch = manual.value
        ? { kind: form.value.kind, title: form.value.title.trim(), note: form.value.note }
        : { note: form.value.note }
    updateMutation.mutate(
        { id: item.value.id, patch },
        {
            onSuccess: () => {
                submittedClean.value = true
                editing.value = false
                // The watcher reseeds when the saved entry differs; reseed now
                // too, in case it came back unchanged.
                seedForm()
            }
        }
    )
}
function onCancel() {
    form.value = { ...baseline.value }
    editing.value = false
}
function onDelete() {
    if (!item.value) return
    deleteMutation.mutate(item.value.id, {
        onSuccess: () => {
            submittedClean.value = true
            router.push({ name: 'wishlist' })
        }
    })
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
onUnmounted(() => window.removeEventListener('beforeunload', onBeforeUnload))
</script>

<template>
    <div class="wishlist-item-view">
        <div v-if="isLoading" class="loading">
            <i class="icon-spin ms-progress-activity" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="notFound" class="error">
            <i class="ms-warning" style="font-size: 2rem"></i>
            <p>{{ isError ? 'Could not load this wishlist entry' : 'Wishlist entry not found' }}</p>
        </div>

        <ContentScaffold v-else title="" show-back @back="router.push({ name: 'wishlist' })">
            <template #actions>
                <EditActionBar
                    v-model:editing="editing"
                    can-delete
                    :save-disabled="!valid"
                    :saving="updateMutation.isPending.value"
                    save-tooltip="Save"
                    :dirty="dirty"
                    delete-header="Remove from wishlist?"
                    :delete-message="`Remove &quot;${item?.title}&quot; from the wishlist?`"
                    @save="onSave"
                    @cancel="onCancel"
                    @delete="onDelete"
                />
            </template>
            <div class="detail-scroll">
                <HeroHeader
                    class="detail-hero"
                    :eyebrow="eyebrow"
                    :cover-url="coverUrl"
                    :cover-placeholder-icon="placeholderIcon"
                    :cover-editable="false"
                    :round-cover="item?.kind === 'artist'"
                    v-model:editing="editing"
                >
                    <template #read>
                        <h2 class="hero-name">{{ item?.title }}</h2>
                        <p v-if="subtitle" class="hero-desc">{{ subtitle }}</p>
                        <div v-if="details.length" class="meta-row">
                            <span v-for="(d, i) in details" :key="i" :class="{ dot: i > 0 }">{{ d }}</span>
                        </div>
                        <p class="status" :class="{ found: item?.in_library }">
                            <template v-if="item?.in_library">
                                <i class="ms-check-circle"></i>
                                <span>In your library<template v-if="foundSince"> since {{ foundSince }}</template></span>
                            </template>
                            <template v-else-if="manual">
                                <i class="ms-bookmark-heart"></i>
                                <span>Added by hand — with no MusicBrainz id, a scan cannot mark it found.</span>
                            </template>
                            <template v-else>
                                <i class="ms-bookmark-heart"></i>
                                <span>Not in your library yet — a scan marks it found once it is.</span>
                            </template>
                        </p>
                        <p v-if="item?.note" class="hero-desc note">{{ item.note }}</p>
                    </template>
                    <template #edit>
                        <template v-if="manual">
                            <div class="form-field">
                                <span id="wishlist-kind-label" class="field-label">Type</span>
                                <SelectButton
                                    v-model="form.kind"
                                    :options="kindOptions"
                                    optionLabel="label"
                                    optionValue="value"
                                    :allowEmpty="false"
                                    dataKey="value"
                                    class="as-button-group wishlist-kind"
                                    aria-labelledby="wishlist-kind-label"
                                >
                                    <template #option="slotProps">
                                        <i :class="slotProps.option.icon"></i>
                                        <span>{{ slotProps.option.label }}</span>
                                    </template>
                                </SelectButton>
                            </div>
                            <label class="form-field">
                                <span class="field-label">Title</span>
                                <InputText v-model="form.title" class="wishlist-title" :maxlength="TITLE_MAX" />
                            </label>
                        </template>
                        <label class="form-field">
                            <span class="field-label">Description</span>
                            <Textarea
                                v-model="form.note"
                                class="wishlist-note"
                                rows="4"
                                auto-resize
                                :maxlength="NOTE_MAX"
                                placeholder="Which edition, where to buy it…"
                            />
                        </label>
                    </template>
                    <template v-if="libraryRoute || mbUrl" #actions>
                        <div class="wishlist-actions">
                            <router-link v-if="libraryRoute" :to="libraryRoute" custom v-slot="{ navigate }">
                                <Button
                                    class="open-in-library"
                                    :label="libraryLabel"
                                    icon="ms-library-music"
                                    @click="navigate"
                                />
                            </router-link>
                            <a
                                v-if="mbUrl"
                                class="p-button p-button-outlined open-musicbrainz"
                                :href="mbUrl ?? undefined"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                <i class="ms-open-in-new"></i>
                                <span>MusicBrainz</span>
                            </a>
                        </div>
                    </template>
                </HeroHeader>
            </div>
        </ContentScaffold>

        <ConfirmDialog />
    </div>
</template>

<style scoped>
.wishlist-item-view {
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
    box-sizing: border-box;
}
/* Light inks: the identity sits on the hero's dark band in both themes. */
.status {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin: 0.5rem 0 0;
    color: #e2edf4;
}
.status.found i {
    color: #8fecff;
}
.note {
    margin-top: 0.5rem;
    white-space: pre-wrap;
    font-style: italic;
}
.wishlist-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
}
.open-musicbrainz {
    gap: 0.4rem;
    text-decoration: none;
}
.form-field {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
}
.form-field :deep(.p-textarea),
.form-field :deep(.p-inputtext) {
    width: 100%;
}
.wishlist-kind {
    align-self: flex-start;
}
/* The edit form sits on the dark duotone band, so its label takes a light ink. */
.field-label {
    font-size: 0.72rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: #cdd7df;
}
</style>
