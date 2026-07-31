<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import CardGrid from '@/components/library/CardGrid.vue'
import DiscoveryFeedItem from '@/components/library/DiscoveryFeedItem.vue'
import { useDiscoveryFeed } from '@/composables/useDiscovery'

/**
 * The Discovery feed BODY — states, grid/list rendering, and the infinite-scroll
 * sentinel — with no header of its own, so its consumer frames it: the Discover tab
 * in `LibraryView` (`/library`, no folder), inside that view's `ContentScaffold`.
 *
 * There is deliberately no standalone `/discover` route: the feed is Library's
 * default tab and the sidebar's `Library` entry is the single door to it. The
 * component stays header-free anyway — that separation is what let the tab adopt it
 * unchanged, and what would let a second consumer do the same.
 *
 * It owns the query rather than taking items as a prop. The query key is shared,
 * so a consumer that also needs the count (for a header summary) calls
 * `useDiscoveryFeed` itself and reads the same cache entry — the same pattern
 * `LibraryView` already uses for `useAlbumIndex` / `useArtistTable`.
 *
 * The feed is deliberately NOT library-scoped: the ranking is cross-collection, so
 * there is no `musicFolderId` prop and the tab exists only on the root `/library`.
 */
const props = defineProps<{ layout: 'grid' | 'list' }>()

const { items, isLoading, isError, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useDiscoveryFeed()

const isEmpty = computed(() => !isLoading.value && !isError.value && items.value.length === 0)

// CardGrid keys its cells by `id`, so wrap each entry with the entity id it already
// dedupes on. Ids are prefixed per type (`al-`/`pl-`) and so cannot collide.
const gridItems = computed(() =>
    items.value.map((entry) => ({
        id: entry.type === 'album' ? entry.album.id : entry.playlist.id,
        entry
    }))
)

// Intersection observer over a sentinel at the end of the feed. Attached only
// while more pages remain, so a fully-loaded feed holds no observer.
const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

const stopObserving = (): void => {
    observer?.disconnect()
    observer = null
}

watch(sentinel, (el) => {
    stopObserving()
    if (!el || typeof IntersectionObserver === 'undefined') return
    observer = new IntersectionObserver((entries) => {
        if (entries.some((e) => e.isIntersecting)) fetchNextPage()
    })
    observer.observe(el)
})

onBeforeUnmount(stopObserving)
</script>

<template>
    <div class="discovery-scroll">
        <div v-if="isLoading" class="discovery-state content-col">
            <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
        </div>

        <div v-else-if="isError" class="discovery-state content-col">
            <i class="pi pi-exclamation-triangle"></i>
            <span>Could not load the discovery feed</span>
        </div>

        <div v-else-if="isEmpty" class="discovery-state content-col">
            <span>Nothing here yet</span>
        </div>

        <template v-else>
            <!-- Grid uses the app's shared CardGrid layout — the same component
                 /library's VirtualCardGrid builds its rows from — so the column
                 math and the min-width:0 constraint that keeps every cell the
                 same width live in one place. List is a plain column, matching
                 the other *ListView components. -->
            <CardGrid
                v-if="props.layout === 'grid'"
                class="discovery-feed content-col"
                :items="gridItems"
            >
                <template #card="{ item }">
                    <DiscoveryFeedItem v-if="item" :entry="item.entry" layout="grid" />
                </template>
            </CardGrid>

            <div v-else class="discovery-feed discovery-feed-list content-col">
                <!-- Key on entity id, not rank — rank is a position, so a rank shift would
                     destroy and recreate rather than move. Id is what flattenDiscoveryPages
                     dedupes on and is prefixed per type, so cannot collide. -->
                <DiscoveryFeedItem
                    v-for="entry in items"
                    :key="entry.type === 'album' ? entry.album.id : entry.playlist.id"
                    :entry="entry"
                    layout="list"
                />
            </div>

            <div v-if="hasNextPage" ref="sentinel" class="discovery-sentinel content-col">
                <i
                    v-if="isFetchingNextPage"
                    class="pi pi-spin pi-spinner"
                    style="font-size: 1.25rem"
                ></i>
            </div>
        </template>
    </div>
</template>

<style scoped>
/* Recipe B (see docs/architecture/main-content-view-layout.md): plain scrolling
   body, one scrollbar's worth of rail clearance on the right. --sb-w comes from
   PlayerLayout; never re-measure it here. */
.discovery-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    padding-bottom: 1rem;
    box-sizing: border-box;
}

/* No grid-template-columns here by design: CardGrid owns the column math for the
   whole app. This only adds the feed's own spacing. */
.discovery-feed {
    padding-top: 1rem;
}

.discovery-feed-list {
    display: flex;
    flex-direction: column;
}

.discovery-state {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding-block: 1.5rem;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
}

.discovery-sentinel {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 3rem;
    color: var(--app-text-secondary);
}
</style>
