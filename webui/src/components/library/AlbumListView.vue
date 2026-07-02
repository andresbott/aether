<script setup lang="ts">
import { ref, toRef } from 'vue'
import VirtualScroller from 'primevue/virtualscroller'
import type { VirtualScrollerLazyEvent } from 'primevue/virtualscroller'
import AlphabetRail from '@/components/library/AlphabetRail.vue'
import AlbumRow from '@/components/library/AlbumRow.vue'
import { useAlbumTable } from '@/composables/useAlbumTable'

const props = defineProps<{ folderId?: number }>()

const { total, letters, items, isLoading, error, ensureRange } = useAlbumTable(
    toRef(props, 'folderId')
)

const scroller = ref<InstanceType<typeof VirtualScroller> | null>(null)

function onLazyLoad(event: VirtualScrollerLazyEvent): void {
    void ensureRange(event.first, event.last)
}

function onSelectLetter(offset: number): void {
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
            <i class="pi pi-music" style="font-size: 3rem"></i>
            <p>No albums found</p>
        </div>
        <div v-else class="list-body">
            <div class="table-region">
                <div class="album-row header-row">
                    <div class="col-cover"></div>
                    <div class="col-title">Title</div>
                    <div class="col-artist">Artist</div>
                    <div class="col-songs">Songs</div>
                    <div class="col-duration">Duration</div>
                </div>
                <VirtualScroller
                    ref="scroller"
                    :items="items"
                    :itemSize="56"
                    lazy
                    :numToleratedItems="10"
                    class="album-scroller"
                    @lazy-load="onLazyLoad"
                >
                    <template #item="{ item }">
                        <AlbumRow :album="item" />
                    </template>
                </VirtualScroller>
            </div>
            <AlphabetRail :letters="letters" @select="onSelectLetter" />
        </div>
    </div>
</template>

<style scoped>
.list-body {
    display: flex;
    gap: 0.5rem;
    align-items: flex-start;
}

.table-region {
    flex: 1;
    min-width: 0;
}

.album-scroller {
    height: calc(100vh - 260px);
    width: 100%;
}

.header-row {
    display: grid;
    grid-template-columns: 48px 2fr 1.5fr 4rem 5rem;
    align-items: center;
    gap: 1rem;
    height: 40px;
    padding: 0 0.5rem;
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--app-text-secondary);
    border-bottom: 2px solid var(--p-content-border-color);
}

.header-row .col-songs,
.header-row .col-duration {
    text-align: right;
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
