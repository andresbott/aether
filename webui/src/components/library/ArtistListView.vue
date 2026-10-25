<script setup lang="ts">
import { computed, ref, toRef } from 'vue'
import VirtualScroller from 'primevue/virtualscroller'
import AlphabetRail from '@/components/library/AlphabetRail.vue'
import ArtistRow from '@/components/library/ArtistRow.vue'
import { useArtistSource } from '@/composables/useLibrarySource'
import { useListCardMetrics } from '@/composables/useListCardMetrics'

const props = defineProps<{ folderId?: number; favoritesOnly?: boolean }>()

const { total, letters, items, isLoading, error } = useArtistSource(
    toRef(props, 'folderId'),
    computed(() => props.favoritesOnly === true)
)
const { itemSize } = useListCardMetrics()
const scroller = ref<InstanceType<typeof VirtualScroller> | null>(null)

function onSelectLetter(offset: number): void {
    scroller.value?.scrollToIndex(offset)
}
</script>

<template>
    <div class="artist-list-view">
        <div v-if="isLoading" class="loading">
            <i class="icon-spin ms-progress-activity" style="font-size: 2rem"></i>
        </div>
        <div v-else-if="error" class="empty-state">
            <i class="ms-warning" style="font-size: 3rem"></i>
            <p>Could not load artists</p>
        </div>
        <div v-else-if="total === 0" class="empty-state">
            <i :class="favoritesOnly ? 'mso-favorite' : 'ms-artist'" style="font-size: 3rem"></i>
            <p v-if="favoritesOnly">No favorite artists yet</p>
            <p v-else>No artists found</p>
        </div>
        <div v-else class="list-body">
            <!-- Keyed on folder + source: each is a different dataset, and a
                 retained scroll offset from the previous one lands nowhere. -->
            <VirtualScroller
                ref="scroller"
                :key="`${folderId ?? 'all'}-${favoritesOnly ? 'fav' : 'all'}`"
                :items="items"
                :itemSize="itemSize"
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
    width: 1.75rem;
    background: var(--app-bg, transparent);
}

/* Recipe C, plus the gap under the header — see AlbumListView. */
.list-body :deep(.p-virtualscroller-content) {
    box-sizing: border-box;
    padding-top: 1rem;
    padding-left: var(--app-content-gutter);
    padding-right: calc(var(--app-rail-clearance) + var(--app-content-gutter));
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
