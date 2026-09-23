<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import SelectButton from 'primevue/selectbutton'
import ToggleButton from 'primevue/togglebutton'
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
    { label: 'List', value: 'list', icon: 'ms-view-list' },
    { label: 'Grid', value: 'grid', icon: 'ms-grid-view' }
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

// Favorites filter, URL state (?favorites=1) like the layout so it survives a
// reload and is linkable — the same contract as LibraryView's.
//
// Unlike the library's, this one is a plain client-side predicate rather than a
// second data source: `getPlaylists` already returns every playlist WITH its
// `starred` timestamp (the playlistStar extension) and is unpaginated, so
// `getStarred2`'s playlist array would be the same rows over another request.
const favoritesOnly = computed<boolean>({
    get: () => route.query.favorites === '1',
    set: (v) => {
        const query = { ...route.query }
        if (v) query.favorites = '1'
        else delete query.favorites
        router.replace({ query })
    }
})

// Public filter, URL state (?public=1) like favorites. Another client-side
// predicate — getPlaylists already carries each playlist's `public` flag.
const publicOnly = computed<boolean>({
    get: () => route.query.public === '1',
    set: (v) => {
        const query = { ...route.query }
        if (v) query.public = '1'
        else delete query.public
        router.replace({ query })
    }
})

const visiblePlaylists = computed(() => {
    let list = playlists.value ?? []
    if (favoritesOnly.value) list = list.filter((pl) => !!pl.starred)
    if (publicOnly.value) list = list.filter((pl) => !!pl.public)
    return list
})

const summary = computed(() => {
    const count = visiblePlaylists.value.length
    if (count === 0) return ''
    // Filtered, the count is of the active filter — a bare "3 playlists" would
    // read as the whole set. Matches LibraryView's wording.
    if (favoritesOnly.value) return `${count} favorite${count === 1 ? '' : 's'}`
    if (publicOnly.value) return `${count} public`
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
            <!-- Favorites filter, first in the bar because it changes WHAT is
                 listed while the layout toggle only changes how. Same heart pair
                 and wording as LibraryView's — see unified-play-experience.md. -->
            <ToggleButton
                v-model="favoritesOnly"
                class="playlists-favorites-filter as-button-group"
                onIcon="msf-favorite"
                offIcon="ms-favorite"
                onLabel=""
                offLabel=""
                :aria-label="favoritesOnly ? 'Show all' : 'Show favorites only'"
                v-tooltip.bottom="favoritesOnly ? 'Show all' : 'Show favorites only'"
            />
            <!-- Public filter — like favorites, changes WHAT is listed. Globe
                 matches the playlist editor's Public toggle; the active state
                 reads from the .as-button-group accent. -->
            <ToggleButton
                v-model="publicOnly"
                class="playlists-public-filter as-button-group"
                onIcon="ms-public"
                offIcon="ms-public"
                onLabel=""
                offLabel=""
                :aria-label="publicOnly ? 'Show all' : 'Show public only'"
                v-tooltip.bottom="publicOnly ? 'Show all' : 'Show public only'"
            />
            <SelectButton
                v-model="layout"
                :options="layoutOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
                dataKey="value"
                aria-label="Layout"
                class="as-button-group"
            >
                <template #option="slotProps">
                    <i :class="slotProps.option.icon"></i>
                </template>
            </SelectButton>
            <Button
                icon="ms-add"
                text
                rounded
                v-tooltip.bottom="'Create playlist'"
                aria-label="Create playlist"
                @click="showCreateDialog = true"
            />
        </template>

        <div class="playlists-scroll">
            <div v-if="isLoading" class="loading">
                <i class="icon-spin ms-progress-activity" style="font-size: 2rem"></i>
            </div>

            <template v-else-if="visiblePlaylists.length > 0">
                <PlaylistListView v-if="layout === 'list'" :playlists="visiblePlaylists" />
                <div v-else class="playlist-grid content-col">
                    <PlaylistCard v-for="pl in visiblePlaylists" :key="pl.id" :playlist="pl" />
                </div>
            </template>

            <!-- Name the active filter rather than "No playlists": with a filter
                 on, the latter would claim the user has none at all. -->
            <div v-else class="empty-state">
                <i
                    :class="favoritesOnly ? 'ms-favorite' : publicOnly ? 'ms-public' : 'ms-queue-music'"
                    style="font-size: 3rem"
                ></i>
                <p v-if="favoritesOnly">No favorite playlists yet</p>
                <p v-else-if="publicOnly">No public playlists</p>
                <p v-else>No playlists</p>
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
.playlists-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    box-sizing: border-box;
}
.loading { display: flex; justify-content: center; padding: 3rem; color: var(--app-text-secondary); }
.playlist-grid { padding-top: 1rem; padding-bottom: 1rem; display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 2rem; }
.empty-state { display: flex; flex-direction: column; align-items: center; padding: 4rem; gap: 1rem; color: var(--app-text-secondary); }
.create-form { padding: 1rem 0; }

/* The icon-only filter toggles (favorites + public). The button-group look —
   bordered, transparent, accent when active — comes from the global
   .as-button-group treatment in _main.scss; locally they are just icon-only
   squares matching the add button. The favorites one mirrors LibraryView's
   .library-favorites-filter (change one, change both). */
.playlists-favorites-filter,
.playlists-public-filter {
    width: 2.125rem;
    height: 2.125rem;
    min-width: 0;
}

.playlists-favorites-filter :deep(.p-togglebutton-content),
.playlists-public-filter :deep(.p-togglebutton-content) {
    padding: 0;
    gap: 0;
}

.playlists-favorites-filter :deep(.p-togglebutton-label),
.playlists-public-filter :deep(.p-togglebutton-label) {
    display: none;
}
</style>
