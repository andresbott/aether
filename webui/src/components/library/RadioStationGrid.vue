<script setup lang="ts">
import VirtualCardGrid from '@/components/library/VirtualCardGrid.vue'
import RadioStationCard from '@/components/library/RadioStationCard.vue'
import { useRadioTable } from '@/composables/useRadioTable'

const { total, items, isLoading, error } = useRadioTable()
</script>

<template>
    <div class="radio-grid-view">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>
        <div v-else-if="error" class="empty-state">
            <i class="pi pi-exclamation-triangle" style="font-size: 3rem"></i>
            <p>Could not load stations</p>
        </div>
        <div v-else-if="total === 0" class="empty-state">
            <i class="pi pi-wifi" style="font-size: 3rem"></i>
            <p>No radio stations</p>
        </div>
        <VirtualCardGrid v-else :items="items" :letters="[]" :total="total" :showRail="false">
            <template #card="{ item }">
                <RadioStationCard v-if="item" :station="item" />
            </template>
        </VirtualCardGrid>
    </div>
</template>

<style scoped>
.radio-grid-view {
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
