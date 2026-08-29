<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import SongRow from '@/components/library/SongRow.vue'
import { useSongList } from '@/composables/useSongList'

const props = defineProps<{
    folderId?: number
    favoritesOnly?: boolean
}>()

const { items, isLoading, isError, hasNextPage, isFetchingNextPage, fetchNextPage } = useSongList(
    () => props.folderId,
    () => props.favoritesOnly ?? false
)

const isEmpty = computed(() => !isLoading.value && !isError.value && items.value.length === 0)

// Intersection observer over a sentinel at the end of the list. Attached only
// while more pages remain, so a fully-loaded list holds no observer.
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
    <div class="song-list-scroll">
        <div v-if="isLoading" class="song-list-state content-col">
            <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
        </div>

        <div v-else-if="isError" class="song-list-state content-col">
            <i class="pi pi-exclamation-triangle"></i>
            <span>Could not load songs</span>
        </div>

        <div v-else-if="isEmpty" class="song-list-state content-col">
            <span>No songs found</span>
        </div>

        <template v-else>
            <div class="song-list content-col">
                <!-- Sticky header row, styled like ArtistListView's .list-header.
                     Grid mirrors SongRow's template. -->
                <div class="list-header">
                    <div class="header-row">
                        <span class="col-cover"></span>
                        <span class="col-title">Title</span>
                        <span class="col-album">Album</span>
                        <span class="col-duration">Duration</span>
                        <!-- Blank: the favorite column is hover-revealed per row. -->
                        <span class="col-star"></span>
                    </div>
                </div>
                <!-- Key on song id. -->
                <SongRow
                    v-for="(song, idx) in items"
                    :key="song.id"
                    :song="song"
                    :index="idx"
                />
            </div>

            <div v-if="hasNextPage" ref="sentinel" class="song-list-sentinel content-col">
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
.song-list-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    padding-bottom: 1rem;
    box-sizing: border-box;
}

.song-list {
    display: flex;
    flex-direction: column;
    padding-top: 0;
}

/* Sticky to the top of the scrolling list, with an opaque background so rows
   pass behind it rather than through it. The gap above the labels is
   `padding-top` on the sticky element ITSELF, not on the container — container
   padding scrolls away, which would collapse the gap the moment the list moved.
   That padding is part of the opaque box, so it also masks the rows sliding
   underneath. Same reason the border sits on the inner `.header-row` rather
   than here: the underline has to hug the labels, not the bottom of the padded
   box. */
.song-list .list-header {
    position: sticky;
    top: 0;
    z-index: 2;
    padding-top: var(--app-list-header-top);
    background: var(--app-background);
}

/* Mirrors SongRow's grid template, favorite column included, so the header
   lines up with all rows. */
.song-list .header-row {
    display: grid;
    grid-template-columns: 48px 1fr 1fr 5rem 2rem;
    align-items: center;
    gap: 1rem;
    height: 36px;
    padding: 0 0.5rem;
    box-sizing: border-box;
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
    border-bottom: 1px solid var(--p-content-border-color);
}

.song-list .header-row .col-duration {
    text-align: right;
}

.song-list-state {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding-block: 1.5rem;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
}

.song-list-sentinel {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 3rem;
    color: var(--app-text-secondary);
}
</style>
