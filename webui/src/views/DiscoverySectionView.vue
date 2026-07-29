<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import SelectButton from 'primevue/selectbutton'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import AlbumCard from '@/components/library/AlbumCard.vue'
import AlbumRow from '@/components/library/AlbumRow.vue'
import PlaylistCard from '@/components/library/PlaylistCard.vue'
import {
    findSection,
    useDiscoverySection,
    SECTION_PAGE_ALBUM_COUNT,
    RANDOM_PAGE_ALBUM_COUNT
} from '@/composables/useDiscovery'

type Layout = 'grid' | 'list'

const props = defineProps<{ section: string }>()

const route = useRoute()
const router = useRouter()

const def = computed(() => findSection(props.section))
const isRandom = computed(() => props.section === 'random')

// Random reshuffles server-side per request, so it cannot be offset-paged; it
// fetches one larger batch and offers a refetch instead. Computed, not a
// constant: vue-router reuses this component across /discover/:section changes.
const albumSize = computed(() =>
    isRandom.value ? RANDOM_PAGE_ALBUM_COUNT : SECTION_PAGE_ALBUM_COUNT
)

const { albums, playlists, albumsLoading, albumsError, playlistsLoading, playlistsError, reshuffle } = useDiscoverySection(
    () => props.section,
    albumSize
)

const isLoading = computed(() => albumsLoading.value && playlistsLoading.value)
const isError = computed(() => albumsError.value && playlistsError.value)

// immediate so an unknown key redirects on first render, and re-checked because
// the param can change without remounting.
watch(
    def,
    (found) => {
        if (!found) router.replace({ name: 'discover' })
    },
    { immediate: true }
)

const layoutOptions = [
    { label: 'List', value: 'list', icon: 'pi pi-list' },
    { label: 'Grid', value: 'grid', icon: 'pi pi-th-large' }
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

const summary = computed(() => {
    const a = albums.value.length
    const p = playlists.value.length
    const parts: string[] = []
    if (a > 0) parts.push(`${a} ${a === 1 ? 'album' : 'albums'}`)
    if (p > 0) parts.push(`${p} ${p === 1 ? 'playlist' : 'playlists'}`)
    return parts.join(' • ')
})

</script>

<template>
    <ContentScaffold
        :title="def?.title ?? 'Discovery'"
        :summary="summary"
        showBack
        @back="router.back()"
    >
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
            <Button
                v-if="isRandom"
                class="section-shuffle"
                icon="pi pi-refresh"
                text
                rounded
                v-tooltip.bottom="'Shuffle'"
                aria-label="Shuffle"
                @click="reshuffle"
            />
        </template>

        <div class="section-scroll">
            <div v-if="isLoading" class="state content-col">
                <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
            </div>

            <div v-else-if="isError" class="state content-col">
                <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
                <p>Could not load this section</p>
            </div>

            <template v-else>
                <div v-if="albumsLoading" class="state content-col">
                    <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
                </div>
                <div v-else-if="albumsError" class="state content-col">
                    <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
                    <p>Could not load albums</p>
                </div>
                <div v-else-if="albums.length > 0" class="block content-col" :class="layout">
                    <template v-if="layout === 'grid'">
                        <AlbumCard v-for="al in albums" :key="al.id" :album="al" />
                    </template>
                    <template v-else>
                        <AlbumRow v-for="al in albums" :key="al.id" :album="al" />
                    </template>
                </div>

                <div v-if="playlistsLoading" class="state content-col">
                    <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
                </div>
                <div v-else-if="playlistsError" class="state content-col">
                    <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
                    <p>Could not load playlists</p>
                </div>
                <div v-else-if="playlists.length > 0" class="block content-col" :class="layout">
                    <PlaylistCard v-for="pl in playlists" :key="pl.id" :playlist="pl" />
                </div>

                <div
                    v-if="!albumsLoading && !albumsError && !playlistsLoading && !playlistsError && albums.length === 0 && playlists.length === 0"
                    class="state content-col"
                >
                    <i class="pi pi-compass" style="font-size: 2rem"></i>
                    <p>Nothing here yet</p>
                </div>
            </template>
        </div>
    </ContentScaffold>
</template>

<style scoped>
/* Recipe B, same as DiscoveryView. */
.section-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    padding-bottom: 1rem;
    box-sizing: border-box;
}

.block {
    padding-top: 1rem;
}

.block.grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 1.5rem;
}

.block.list {
    display: flex;
    flex-direction: column;
}

.state {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 4rem;
    gap: 1rem;
    color: var(--app-text-secondary);
}
</style>
