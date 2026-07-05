<script setup lang="ts">
import { ref, toRef } from 'vue'
import VirtualScroller from 'primevue/virtualscroller'
import AlphabetRail from '@/components/library/AlphabetRail.vue'
import ArtistRow from '@/components/library/ArtistRow.vue'
import { useArtistTable } from '@/composables/useArtistTable'
import { useScrollbarWidth } from '@/composables/useScrollbarWidth'

const props = defineProps<{ folderId?: number }>()

const { total, letters, items, isLoading, error } = useArtistTable(toRef(props, 'folderId'))
const scrollbarWidth = useScrollbarWidth()
const scroller = ref<InstanceType<typeof VirtualScroller> | null>(null)

function onSelectLetter(offset: number): void {
    scroller.value?.scrollToIndex(offset)
}
</script>

<template>
    <div class="artist-list-view" :style="{ '--sb-w': scrollbarWidth + 'px' }">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>
        <div v-else-if="error" class="empty-state">
            <i class="pi pi-exclamation-triangle" style="font-size: 3rem"></i>
            <p>Could not load artists</p>
        </div>
        <div v-else-if="total === 0" class="empty-state">
            <i class="pi pi-users" style="font-size: 3rem"></i>
            <p>No artists found</p>
        </div>
        <div v-else class="list-body">
            <div class="list-header">
                <div class="header-row">
                    <div class="col-avatar"></div>
                    <div class="col-name">Artist</div>
                    <div class="col-count">Albums</div>
                </div>
            </div>
            <VirtualScroller
                ref="scroller"
                :items="items"
                :itemSize="56"
                class="list-scroller"
            >
                <template #item="{ item }">
                    <ArtistRow :artist="item" />
                </template>
            </VirtualScroller>
            <AlphabetRail :letters="letters" @select="onSelectLetter" />
        </div>
    </div>
</template>

<style scoped>
.artist-list-view {
    height: 100%;
    min-height: 0;
}

.list-body {
    position: relative;
    height: 100%;
    display: flex;
    flex-direction: column;
}

.list-header {
    flex-shrink: 0;
    box-sizing: border-box;
    padding-right: calc(2.75rem + var(--sb-w, 0px));
}

.header-row {
    display: grid;
    grid-template-columns: 48px 1fr 7rem;
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

.header-row .col-count {
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

/* Reserve rail clearance (rail 1.75rem + 1rem gap + scrollbar) on the scroll
   content so centered rows never slide under the rail. border-box keeps the
   min-width:100% content wrapper from overflowing once padded. */
.list-body :deep(.p-virtualscroller-content) {
    box-sizing: border-box;
    padding-right: calc(2.75rem + var(--sb-w, 0px));
}

/* Center the rows in the shared content column to match the album/artist grid;
   the scroller stays full width so its scroll bar + the rail stay flush right. */
.list-body :deep(.artist-row) {
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
