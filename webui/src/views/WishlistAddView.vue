<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SelectButton from 'primevue/selectbutton'
import Button from 'primevue/button'
import Message from 'primevue/message'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import SearchField from '@/components/common/SearchField.vue'
import WishlistResultRow from '@/components/wishlist/WishlistResultRow.vue'
import WishlistManualForm from '@/components/wishlist/WishlistManualForm.vue'
import { MIN_WISHLIST_QUERY_LENGTH, useAddWishlistItems, useWishlistSearch } from '@/composables/useWishlist'
import { WISHLIST_KINDS, isWishlistKind } from '@/lib/wishlist'
import type { WishlistKind, WishlistSearchResult, WishlistSnapshot } from '@/types/wishlist'

/**
 * Add to the wishlist: the library search's layout (`SearchView`) pointed at
 * MusicBrainz — one search box, a scope row of filter buttons, results in
 * sections — except that it searches only when asked (Enter, the Search button,
 * or a filter click), never per keystroke: MusicBrainz allows one request per
 * second. Results are picked, not opened; a pick survives a new search, and the
 * selection pill under the filters adds them all at once.
 */

type Scope = 'all' | WishlistKind

const route = useRoute()
const router = useRouter()

const scopeOptions: { label: string; value: Scope; icon: string }[] = [
    { label: 'All', value: 'all', icon: 'ms-asterisk' },
    ...WISHLIST_KINDS.map((k) => ({ label: k.plural, value: k.value, icon: k.icon }))
]

// The scope is URL state (`?kind=album`, absent for All), so the wishlist's own
// filter can open this page on the kind it is showing.
const scope = computed<Scope>({
    get: () => (isWishlistKind(route.query.kind) ? route.query.kind : 'all'),
    set: (v) => {
        const query = { ...route.query }
        if (v === 'all') delete query.kind
        else query.kind = v
        router.replace({ query })
    }
})
const kindsFor = (s: Scope): WishlistKind[] => (s === 'all' ? WISHLIST_KINDS.map((k) => k.value) : [s])

const query = ref('')
const { term, results, loading, error, rateLimited, search, clear } = useWishlistSearch()
const addMutation = useAddWishlistItems()

const canSearch = computed(() => query.value.trim().length >= MIN_WISHLIST_QUERY_LENGTH)

function submit() {
    if (canSearch.value) search(query.value, kindsFor(scope.value))
}

// A filter click is a deliberate action, so once something has been searched it
// searches the scope it picks — from the cache when that kind is already there.
watch(scope, () => {
    if (term.value) submit()
})

// --- Picks ------------------------------------------------------------------
const selected = ref(new Map<string, WishlistSnapshot>())
const keyOf = (r: WishlistSnapshot) => `${r.kind}:${r.mbid}`
const isSelected = (r: WishlistSearchResult) => selected.value.has(keyOf(r))
const selectedCount = computed(() => selected.value.size)

function toggle(r: WishlistSearchResult) {
    if (r.wishlist_id) return
    const next = new Map(selected.value)
    if (next.has(keyOf(r))) next.delete(keyOf(r))
    else {
        const { kind, mbid, title, artist, album, date, type, disambiguation } = r
        next.set(keyOf(r), { kind, mbid, title, artist, album, date, type, disambiguation })
    }
    selected.value = next
}
function clearSelection() {
    selected.value = new Map()
}

function onAdd() {
    if (selectedCount.value === 0) return
    addMutation.mutate([...selected.value.values()], {
        onSuccess: () => {
            clearSelection()
            router.push({ name: 'wishlist' })
        }
    })
}

// An entry added by hand: no MusicBrainz id, the description as its note. Like
// adding picks, it takes the user back to the wishlist.
function onManualAdd(entry: { kind: WishlistKind; title: string; note: string }) {
    addMutation.mutate(
        [{ ...entry, mbid: '', artist: '', album: '', date: '', type: '', disambiguation: '' }],
        { onSuccess: () => router.push({ name: 'wishlist' }) }
    )
}

// Reset puts the page back to how it loads: empty box, no results, All. The
// picks are kept — they are what the visit is collecting.
const isPristine = computed(() => query.value === '' && !term.value && scope.value === 'all')
function resetSearch() {
    query.value = ''
    clear()
    scope.value = 'all'
}

// --- Results ----------------------------------------------------------------
const sections = computed(() =>
    WISHLIST_KINDS.filter((k) => kindsFor(scope.value).includes(k.value))
        .map((k) => ({ kind: k.value, label: k.plural, items: results.value[k.value] ?? [] }))
        .filter((s) => s.items.length > 0)
)
const showSectionLabels = computed(() => scope.value === 'all')
const hasResults = computed(() => sections.value.length > 0)

const summary = computed(() =>
    sections.value
        .map((s) => `${s.items.length} ${s.items.length === 1 ? s.label.slice(0, -1).toLowerCase() : s.label.toLowerCase()}`)
        .join(' • ')
)

const emptyMessage = computed(() =>
    scope.value === 'all'
        ? 'No matches on MusicBrainz'
        : `No ${scopeOptions.find((o) => o.value === scope.value)?.label.toLowerCase()} found on MusicBrainz`
)

function onBack() {
    router.push({ name: 'wishlist' })
}
</script>

<template>
    <ContentScaffold title="Add to wishlist" :summary="summary" show-back @back="onBack">
        <div class="search-page">
            <div class="search-hero">
                <form class="search-row" role="search" @submit.prevent="submit">
                    <SearchField
                        v-model="query"
                        placeholder="Search MusicBrainz for artists, albums, songs..."
                        :resettable="!isPristine"
                        autofocus
                        @reset="resetSearch"
                        @submit="submit"
                    />
                    <Button
                        type="submit"
                        class="wishlist-search-submit"
                        label="Search"
                        icon="ms-search"
                        rounded
                        :disabled="!canSearch"
                        :loading="loading"
                    />
                </form>
                <SelectButton
                    v-model="scope"
                    :options="scopeOptions"
                    optionLabel="label"
                    optionValue="value"
                    :allowEmpty="false"
                    dataKey="value"
                    class="search-filters as-button-group"
                    aria-label="Search for"
                >
                    <template #option="slotProps">
                        <i :class="slotProps.option.icon"></i>
                        <span>{{ slotProps.option.label }}</span>
                    </template>
                </SelectButton>

                <!-- The picks, frozen above the scrolling results like Search's
                     selection pill; they survive a new search. -->
                <div v-if="selectedCount > 0" class="search-selection-pill">
                    <span class="selection-count">{{ selectedCount }} selected</span>
                    <Button
                        class="wishlist-add-selected"
                        label="Add to wishlist"
                        icon="ms-add"
                        :loading="addMutation.isPending.value"
                        @click="onAdd"
                    />
                    <Button
                        class="wishlist-clear-selection"
                        icon="ms-close"
                        text
                        rounded
                        severity="secondary"
                        v-tooltip.bottom="'Clear selection'"
                        aria-label="Clear selection"
                        @click="clearSelection"
                    />
                </div>
            </div>

            <div class="search-scroll">
                <div v-if="!term" class="state-message">
                    <i class="ms-bookmark-heart" style="font-size: 3rem"></i>
                    <p>Search MusicBrainz for music you want, then pick the results to add.</p>
                    <p class="state-hint">A title, a name, or both — "in utero nirvana".</p>
                </div>

                <div v-else-if="!hasResults && loading" class="state-message">
                    <i class="icon-spin ms-progress-activity" style="font-size: 2rem"></i>
                </div>

                <div v-else-if="!hasResults && error" class="state-message">
                    <Message :severity="rateLimited ? 'warn' : 'error'" :closable="false">{{ error }}</Message>
                </div>

                <div v-else-if="!hasResults" class="state-message">
                    <i class="ms-search" style="font-size: 3rem"></i>
                    <p>{{ emptyMessage }}</p>
                </div>

                <div v-else class="search-content content-col">
                    <Message v-if="error" :severity="rateLimited ? 'warn' : 'error'" :closable="false">
                        {{ error }}
                    </Message>
                    <section v-for="s in sections" :key="s.kind" class="result-section" :class="`section-${s.kind}`">
                        <h2 v-if="showSectionLabels" class="section-label">{{ s.label }}</h2>
                        <div class="row-list">
                            <WishlistResultRow
                                v-for="r in s.items"
                                :key="r.mbid"
                                :result="r"
                                :selected="isSelected(r)"
                                @toggle="toggle(r)"
                            />
                        </div>
                    </section>
                    <div v-if="loading" class="more-loading">
                        <i class="icon-spin ms-progress-activity"></i>
                    </div>
                </div>

                <!-- Below the search, whatever it showed: the way in for music
                     MusicBrainz does not list. -->
                <section class="manual-section content-col">
                    <WishlistManualForm :saving="addMutation.isPending.value" @add="onManualAdd" />
                </section>
            </div>
        </div>
    </ContentScaffold>
</template>

<style scoped>
/* The frame, hero and scroller follow SearchView's, so the two searches read
   as one design. */
.search-page {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
}

.search-hero {
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.9rem;
    width: 100%;
    box-sizing: border-box;
    padding: 2rem var(--app-content-gutter) 1.25rem;
    /* Recipe A: center on the content column, left of the results' scrollbar. */
    padding-right: calc(var(--sb-w, 0px) + var(--app-content-gutter));
}

.search-row {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    width: 100%;
    max-width: 760px;
}
.search-row :deep(.search-input-wrapper) {
    max-width: none;
    flex: 1;
    min-width: 0;
}
.wishlist-search-submit {
    flex-shrink: 0;
    height: 3.1rem;
    padding-inline: 1.25rem;
}

.search-filters {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
}

/* Search's dark frosted chip, holding the add action instead of play/queue. */
.search-selection-pill {
    align-self: center;
    max-width: 100%;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.4rem 0.4rem 0.4rem 0.9rem;
    background: rgba(17, 23, 31, 0.92);
    border: 1px solid rgba(255, 255, 255, 0.14);
    border-radius: 12px;
    backdrop-filter: blur(8px);
}
.selection-count {
    color: #e2edf4;
    font-size: 0.875rem;
    font-weight: 600;
    white-space: nowrap;
}
.search-selection-pill :deep(.p-button) {
    height: 2.125rem;
    padding-block: 0;
    padding-inline: 0.7rem;
    font-size: 0.875rem;
}
.search-selection-pill :deep(.p-button-icon-only) {
    width: 2.125rem;
    padding: 0;
}
.search-selection-pill :deep(.p-button.p-button-secondary.p-button-text) {
    color: #e2edf4;
}
.search-selection-pill :deep(.p-button.p-button-secondary.p-button-text:hover) {
    background: rgba(255, 255, 255, 0.14);
    color: #ffffff;
}

.search-scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    scrollbar-gutter: stable;
    box-sizing: border-box;
}

.state-message {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 2.5rem 1rem;
    gap: 1rem;
    color: var(--app-text-secondary);
    text-align: center;
}
.state-message p {
    margin: 0;
}
.state-hint {
    font-size: 0.875rem;
}

.search-content {
    padding-top: 1rem;
    padding-bottom: 1rem;
    display: flex;
    flex-direction: column;
    gap: 2rem;
}

.section-label {
    margin: 0 0 0.75rem;
    font-size: 0.85rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
}

.row-list {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
}

.manual-section {
    padding-top: 1.5rem;
    padding-bottom: 2.5rem;
}

.more-loading {
    display: flex;
    justify-content: center;
    color: var(--app-text-secondary);
}

@media (max-width: 767.98px) {
    /* On a phone the button keeps only its icon beside the box. */
    .wishlist-search-submit :deep(.p-button-label) {
        display: none;
    }
    .wishlist-search-submit {
        padding-inline: 0;
        width: 3.1rem;
    }
}
</style>
