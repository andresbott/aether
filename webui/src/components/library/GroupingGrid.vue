<script setup lang="ts">
import { toRef } from 'vue'
import VirtualCardGrid from '@/components/library/VirtualCardGrid.vue'
import GroupingCard from '@/components/library/GroupingCard.vue'
import { useGroupingTable } from '@/composables/useGroupingTable'

const props = defineProps<{ folderId?: number }>()

const { total, letters, items, isLoading, error } = useGroupingTable(toRef(props, 'folderId'))
</script>

<template>
    <div class="grouping-grid-view">
        <div v-if="isLoading" class="loading">
            <i class="icon-spin ms-progress-activity" style="font-size: 2rem"></i>
        </div>
        <div v-else-if="error" class="empty-state">
            <i class="ms-warning" style="font-size: 3rem"></i>
            <p>Could not load collections</p>
        </div>
        <div v-else-if="total === 0" class="empty-state">
            <i class="ms-collections-bookmark" style="font-size: 3rem"></i>
            <p>No collections found</p>
        </div>
        <!-- lazyLoad not handled: the index arrives whole, with no paging. Keyed on the
             folder so switching library remeasures instead of keeping the old range. -->
        <VirtualCardGrid
            v-else
            :key="folderId ?? 'all'"
            :items="items"
            :letters="letters"
            :total="total"
        >
            <template #card="{ item }">
                <GroupingCard v-if="item" :grouping="item" />
            </template>
        </VirtualCardGrid>
    </div>
</template>

<style scoped>
.grouping-grid-view {
    height: 100%;
    min-height: 0;
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
