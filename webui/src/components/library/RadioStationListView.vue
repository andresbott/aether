<script setup lang="ts">
import VirtualScroller from 'primevue/virtualscroller'
import RadioStationRow from '@/components/library/RadioStationRow.vue'
import { useRadioTable } from '@/composables/useRadioTable'

const { total, items, isLoading, error } = useRadioTable()
</script>

<template>
    <div class="radio-list-view">
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
        <div v-else class="list-body">
            <div class="list-header">
                <div class="header-row">
                    <div class="col-avatar"></div>
                    <div class="col-name">Station</div>
                    <div class="col-homepage">Homepage</div>
                </div>
            </div>
            <VirtualScroller :items="items" :itemSize="56" class="list-scroller">
                <template #item="{ item }">
                    <RadioStationRow :station="item" />
                </template>
            </VirtualScroller>
        </div>
    </div>
</template>

<style scoped>
.radio-list-view {
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
    padding-left: var(--app-content-gutter);
    padding-right: calc(var(--app-rail-clearance) + 2 * var(--sb-w, 0px) + var(--app-content-gutter));
}

.header-row {
    display: grid;
    grid-template-columns: 48px 1fr minmax(0, 1fr);
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

.list-scroller {
    flex: 1;
    min-height: 0;
    width: 100%;
    scrollbar-gutter: stable;
}

/* Center the rows in the shared content column to match the album/artist views;
   the scroller stays full width so its scroll bar stays flush right. */
.list-body :deep(.radio-row) {
    max-width: var(--app-content-max-width);
    margin-left: auto;
    margin-right: auto;
}

/* Uniform rail clearance + gutter, matching the album/artist lists, so the
   column doesn't shift when navigating between views. */
.list-body :deep(.p-virtualscroller-content) {
    box-sizing: border-box;
    padding-left: var(--app-content-gutter);
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px) + var(--app-content-gutter));
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
