<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SelectButton from 'primevue/selectbutton'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import AlbumListView from '@/components/library/AlbumListView.vue'
import AlbumGrid from '@/components/library/AlbumGrid.vue'
import ArtistListView from '@/components/library/ArtistListView.vue'
import ArtistGrid from '@/components/library/ArtistGrid.vue'
import DiscoveryFeed from '@/components/library/DiscoveryFeed.vue'
import { useMusicFolders } from '@/composables/useSubsonicQueries'
import { useAlbumIndex } from '@/composables/useAlbumIndex'
import { useArtistTable } from '@/composables/useArtistTable'
import { useDiscoveryFeed } from '@/composables/useDiscovery'

type ViewMode = 'discover' | 'albums' | 'artists'
type Layout = 'grid' | 'list'

const route = useRoute()
const router = useRouter()

const layoutOptions = [
    { label: 'List', value: 'list', icon: 'pi pi-list' },
    { label: 'Grid', value: 'grid', icon: 'pi pi-th-large' }
]

const folderId = computed<number | undefined>(() => {
    const raw = route.params.folderId
    const value = Array.isArray(raw) ? raw[0] : raw
    if (!value) return undefined
    const num = Number(value)
    return Number.isFinite(num) ? num : undefined
})

const { data: folders } = useMusicFolders()
const folder = computed(() => folders.value?.find((f) => f.id === folderId.value))
const folderName = computed(() =>
    folderId.value === undefined ? 'Library' : (folder.value?.name ?? 'Library')
)

// The Discovery ranking is cross-collection, so the tab exists only on the root
// route (no folderId). A per-library discovery feed is deliberately not a thing —
// see DiscoveryFeed.vue. This tab is also the ONLY way to reach the feed: there is
// no standalone /discover route.
const discoverTabVisible = computed(() => folderId.value === undefined)

const artistsTabVisible = computed(() => {
    if (folderId.value === undefined) return true
    return folder.value?.showArtists !== false
})

const viewOptions = computed(() => [
    ...(discoverTabVisible.value ? [{ label: 'Discover', value: 'discover' }] : []),
    { label: 'Albums', value: 'albums' },
    ...(artistsTabVisible.value ? [{ label: 'Artists', value: 'artists' }] : [])
])

// On the root route, Discover is the default; a folder keeps its configured
// default_view, which the server only ever reports as albums/artists.
const serverDefault = computed<ViewMode>(() => {
    if (discoverTabVisible.value) return 'discover'
    return folder.value?.defaultView ?? 'albums'
})

const hashView = computed<ViewMode | null>(() => {
    const h = route.hash.replace('#', '')
    return h === 'discover' || h === 'albums' || h === 'artists' ? h : null
})

const viewMode = computed<ViewMode>({
    get: () => {
        const wanted = hashView.value ?? serverDefault.value
        // A hash for a tab this route does not offer (a folder deep-linked to
        // #discover, or #artists on a library with showArtists=false) falls back
        // to albums rather than rendering a tab with no toggle to leave it.
        if (wanted === 'discover' && !discoverTabVisible.value) return 'albums'
        if (wanted === 'artists' && !artistsTabVisible.value) return 'albums'
        return wanted
    },
    set: (v) => {
        router.replace({ hash: `#${v}`, query: route.query })
    }
})

const layout = computed<Layout>({
    get: () => (route.query.view === 'list' ? 'list' : 'grid'),
    set: (v) => {
        const query = { ...route.query }
        if (v === 'list') query.view = 'list'
        else delete query.view
        router.replace({ hash: route.hash, query })
    }
})

// Header counts — only the active tab's index is fetched (dedups with the body view).
const { total: albumTotal } = useAlbumIndex(folderId, {
    enabled: computed(() => viewMode.value === 'albums')
})
const { total: artistTotal } = useArtistTable(folderId, {
    enabled: computed(() => viewMode.value === 'artists')
})
// Shares its query cache entry with the DiscoveryFeed in the body, so reading the
// count here costs no extra request.
const { items: discoveryItems } = useDiscoveryFeed()

const summary = computed(() => {
    if (viewMode.value === 'discover') {
        const n = discoveryItems.value.length
        return n > 0 ? `${n} item${n === 1 ? '' : 's'}` : ''
    }
    if (viewMode.value === 'albums') {
        return albumTotal.value > 0
            ? `${albumTotal.value} ${albumTotal.value === 1 ? 'album' : 'albums'}`
            : ''
    }
    return artistTotal.value > 0
        ? `${artistTotal.value} ${artistTotal.value === 1 ? 'artist' : 'artists'}`
        : ''
})
</script>

<template>
    <ContentScaffold :title="folderName" :summary="summary">
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
            <SelectButton
                v-if="viewOptions.length > 1"
                v-model="viewMode"
                :options="viewOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
            />
        </template>

        <DiscoveryFeed v-if="viewMode === 'discover'" :layout="layout" />
        <AlbumListView
            v-else-if="viewMode === 'albums' && layout === 'list'"
            :folderId="folderId"
        />
        <AlbumGrid v-else-if="viewMode === 'albums'" :folderId="folderId" />
        <ArtistListView v-else-if="layout === 'list'" :folderId="folderId" />
        <ArtistGrid v-else :folderId="folderId" />
    </ContentScaffold>
</template>
