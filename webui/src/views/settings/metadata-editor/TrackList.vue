<script setup lang="ts">
import { computed } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import type { Track } from '@/types/metadata'

const props = defineProps<{
    tracks: Track[]
    isLoading: boolean
    selection: Track[]
}>()
const emit = defineEmits<{
    (e: 'update:selection', sel: Track[]): void
    (e: 'reload'): void
}>()

const rows = computed(() => props.tracks)

function selectionChanged(value: Track[]) {
    // PrimeVue emits the full selection on row checkbox toggles; strip error rows
    // (which we render but don't want treated as selectable).
    emit(
        'update:selection',
        value.filter((t) => !t.error)
    )
}
</script>

<template>
    <div class="track-list">
        <div class="track-list-header">
            <span class="count">{{ rows.length }} files</span>
            <Button
                icon="pi pi-refresh"
                text
                rounded
                aria-label="Reload"
                @click="emit('reload')"
            />
        </div>

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
        </div>
        <div v-else-if="rows.length === 0" class="empty">No audio files in this folder.</div>

        <div v-else class="table-wrapper">
        <DataTable
            :value="rows"
            :selection="selection"
            selectionMode="multiple"
            dataKey="path"
            @update:selection="selectionChanged"
            :rowClass="(t: Track) => (t.error ? 'row-error' : '')"
        >
            <Column selectionMode="multiple" style="width: 3rem" />
            <Column field="path" header="Path" />
            <Column header="" style="width: 10rem">
                <template #body="{ data }">
                    <span v-if="(data as Track).error" class="err" :title="(data as Track).error">
                        read error
                    </span>
                </template>
            </Column>
        </DataTable>
        </div>
    </div>
</template>

<style scoped>
.track-list {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
}

.table-wrapper {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
}
.track-list-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5rem;
}
.count {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}
.loading,
.empty {
    padding: 2rem;
    text-align: center;
    color: var(--app-text-secondary);
}
:deep(.row-error) {
    opacity: 0.5;
}
.err {
    color: var(--p-red-600, #dc2626);
    font-size: 0.8rem;
}
</style>
