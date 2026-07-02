<script setup lang="ts">
import { ref, toRef } from 'vue'
import VirtualScroller from 'primevue/virtualscroller'
import type { VirtualScrollerLazyEvent } from 'primevue/virtualscroller'
import AlphabetRail from '@/components/library/AlphabetRail.vue'
import AlbumRow from '@/components/library/AlbumRow.vue'
import { useAlbumTable, ALBUM_PAGE_SIZE } from '@/composables/useAlbumTable'
import { useScrollbarWidth } from '@/composables/useScrollbarWidth'

const props = defineProps<{ folderId?: number }>()

const { total, letters, items, isLoading, error, ensureRange } = useAlbumTable(
    toRef(props, 'folderId')
)
const scrollbarWidth = useScrollbarWidth()
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
    <div class="album-list-view" :style="{ '--sb-w': scrollbarWidth + 'px' }">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>
        <div v-else-if="error" class="empty-state">
            <i class="pi pi-exclamation-triangle" style="font-size: 3rem"></i>
            <p>Could not load albums</p>
        </div>
        <div v-else-if="total === 0" class="empty-state">
            <i class="pi pi-music" style="font-size: 3rem"></i>
            <p>No albums found</p>
        </div>
        <div v-else class="list-body">
            <VirtualScroller
                ref="scroller"
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
}

.list-scroller {
    height: 100%;
    width: 100%;
    scrollbar-gutter: stable;
}

/* Rail hugs the LEFT of the flush-right native scrollbar (offset by its width). */
.list-body :deep(.alphabet-rail) {
    position: absolute;
    top: 0;
    bottom: 0;
    right: var(--sb-w, 0px);
    width: 1.25rem;
    background: var(--app-bg, transparent);
}

.list-body :deep(.album-row) {
    padding-right: calc(1.25rem + var(--sb-w, 0px));
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
