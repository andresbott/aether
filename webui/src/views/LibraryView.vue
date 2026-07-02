<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SelectButton from 'primevue/selectbutton'
import LibraryScaffold from '@/components/library/LibraryScaffold.vue'
import AlbumListView from '@/components/library/AlbumListView.vue'
import AlbumGrid from '@/components/library/AlbumGrid.vue'
import ArtistListView from '@/components/library/ArtistListView.vue'
import ArtistGrid from '@/components/library/ArtistGrid.vue'
import { useMusicFolders } from '@/composables/useSubsonicQueries'
import { useAlbumIndex } from '@/composables/useAlbumIndex'
import { useArtistTable } from '@/composables/useArtistTable'

type ViewMode = 'albums' | 'artists'
type Layout = 'grid' | 'list'

const route = useRoute()
const router = useRouter()

const viewOptions = [
    { label: 'Albums', value: 'albums' },
    { label: 'Artists', value: 'artists' }
]
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
const serverDefault = computed<ViewMode>(() => folder.value?.defaultView ?? 'albums')

const hashView = computed<ViewMode | null>(() => {
    const h = route.hash.replace('#', '')
    return h === 'albums' || h === 'artists' ? h : null
})

const viewMode = computed<ViewMode>({
    get: () => hashView.value ?? serverDefault.value,
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

const summary = computed(() => {
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
    <LibraryScaffold :title="folderName" :summary="summary">
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
                v-model="viewMode"
                :options="viewOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
            />
        </template>

        <AlbumListView v-if="viewMode === 'albums' && layout === 'list'" :folderId="folderId" />
        <AlbumGrid v-else-if="viewMode === 'albums'" :folderId="folderId" />
        <ArtistListView v-else-if="layout === 'list'" :folderId="folderId" />
        <ArtistGrid v-else :folderId="folderId" />
    </LibraryScaffold>
</template>
