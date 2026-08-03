<script setup lang="ts">
import { computed, ref, toRef } from 'vue'
import VirtualScroller from 'primevue/virtualscroller'
import type { VirtualScrollerLazyEvent } from 'primevue/virtualscroller'
import AlphabetRail from '@/components/library/AlphabetRail.vue'
import AlbumRow from '@/components/library/AlbumRow.vue'
import { ALBUM_PAGE_SIZE } from '@/composables/useAlbumTable'
import { useAlbumSource } from '@/composables/useLibrarySource'

const props = defineProps<{ folderId?: number; favoritesOnly?: boolean }>()

const { total, letters, items, isLoading, error, ensureRange } = useAlbumSource(
    toRef(props, 'folderId'),
    computed(() => props.favoritesOnly === true)
)
const scroller = ref<InstanceType<typeof VirtualScroller> | null>(null)

function onLazyLoad(event: VirtualScrollerLazyEvent): void {
    void ensureRange(event.first, event.last)
}

function onSelectLetter(offset: number): void {
    void ensureRange(offset, offset + ALBUM_PAGE_SIZE - 1)
    scroller.value?.scrollToIndex(offset)
}
</script>

<template>
    <div class="album-list-view">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>
        <div v-else-if="error" class="empty-state">
            <i class="pi pi-exclamation-triangle" style="font-size: 3rem"></i>
            <p>Could not load albums</p>
        </div>
        <div v-else-if="total === 0" class="empty-state">
            <i :class="favoritesOnly ? 'pi pi-heart' : 'pi pi-music'" style="font-size: 3rem"></i>
            <p v-if="favoritesOnly">No favorite albums yet</p>
            <p v-else>No albums found</p>
        </div>
        <div v-else class="list-body">
            <div class="list-header">
                <div class="header-row">
                    <div class="col-cover"></div>
                    <div class="col-title">Album</div>
                    <div class="col-artist">Artist</div>
                    <!-- The favorite column is hover-revealed per row, so its header
                         stays blank rather than labelling a usually-invisible
                         control. -->
                    <div class="col-star"></div>
                    <div class="col-songs">Songs</div>
                    <div class="col-duration">Duration</div>
                </div>
            </div>
            <!-- Keyed on folder + source: each is a different dataset, and a
                 retained scroll offset from the previous one lands nowhere. -->
            <VirtualScroller
                ref="scroller"
                :key="`${folderId ?? 'all'}-${favoritesOnly ? 'fav' : 'all'}`"
                :items="items"
                :itemSize="56"
                lazy
                :numToleratedItems="10"
                class="list-scroller"
                @lazy-load="onLazyLoad"
            >
                <template #item="{ item }">
                    <AlbumRow :album="item" />
                </template>
            </VirtualScroller>
            <AlphabetRail :letters="letters" @select="onSelectLetter" />
        </div>
    </div>
</template>

<style scoped>
.album-list-view {
    height: 100%;
    min-height: 0;
}

.list-body {
    position: relative;
    height: 100%;
    display: flex;
    flex-direction: column;
}

/* The top padding is the shared list-header gap — see --app-list-header-top. */
.list-header {
    flex-shrink: 0;
    box-sizing: border-box;
    padding-top: var(--app-list-header-top);
    padding-left: var(--app-content-gutter);
    padding-right: calc(var(--app-rail-clearance) + 2 * var(--sb-w, 0px) + var(--app-content-gutter));
}

/* Mirrors AlbumRow's grid template, favorite column included. */
.header-row {
    display: grid;
    grid-template-columns: 48px 2fr 1.5fr 2rem 4rem 5rem;
    align-items: center;
    gap: 1rem;
    height: 36px;
    padding: 0 0.5rem;
    max-width: var(--app-content-max-width);
    margin-left: auto;
    margin-right: auto;
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
    border-bottom: 1px solid var(--p-content-border-color);
}

.header-row .col-songs,
.header-row .col-duration {
    text-align: right;
}

.list-scroller {
    flex: 1;
    min-height: 0;
    width: 100%;
    scrollbar-gutter: stable;
}

/* Rail hugs the LEFT of the flush-right native scrollbar (offset by its width). */
.list-body :deep(.alphabet-rail) {
    position: absolute;
    top: 0;
    bottom: 0;
    right: var(--sb-w, 0px);
    width: 1.75rem;
    background: var(--app-bg, transparent);
}

/* Recipe C: rail clearance + shared gutter on the scroll content so centered
   rows never slide under the rail and keep a gutter at narrow widths. */
.list-body :deep(.p-virtualscroller-content) {
    box-sizing: border-box;
    padding-left: var(--app-content-gutter);
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px) + var(--app-content-gutter));
}

/* Center the rows in the shared content column to match the album/artist grid;
   the scroller stays full width so its scroll bar + the rail stay flush right. */
.list-body :deep(.album-row) {
    max-width: var(--app-content-max-width);
    margin-left: auto;
    margin-right: auto;
}

.loading {
    display: flex;
    justify-content: center;
    padding: 3rem;
    color: var(--app-text-secondary);
}

.empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 4rem;
    gap: 1rem;
    color: var(--app-text-secondary);
}
</style>
