<script setup lang="ts">
import { computed } from 'vue'
import DataTable from 'primevue/datatable'
import type { DataTableRowClickEvent } from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import {
    getStatusSeverity,
    getStatusLabel,
    isActiveStatus,
    EXECUTION_STATUS
} from '@/composables/useTasks'
import type { ExecutionInfo } from '@/types/tasks'
import { useViewport } from '@/composables/useViewport'

const props = defineProps<{
    executions: ExecutionInfo[]
    isLoading: boolean
    canceling?: boolean
}>()

const emit = defineEmits<{
    cancel: [executionId: string]
    rowClick: [execution: ExecutionInfo]
}>()

const counts = computed(() => {
    const list = props.executions ?? []
    const by = (s: string) => list.filter((e) => e.status === s).length
    return {
        total: list.length,
        waiting: by(EXECUTION_STATUS.waiting),
        running: by(EXECUTION_STATUS.running),
        complete: by(EXECUTION_STATUS.complete),
        failed: by(EXECUTION_STATUS.failed)
    }
})

const formatDate = (s?: string): string => (s ? new Date(s).toLocaleString() : '-')

const duration = (e: ExecutionInfo): string => {
    if (!e.started_at || !e.ended_at) return '-'
    const ms = new Date(e.ended_at).getTime() - new Date(e.started_at).getTime()
    if (ms < 0) return '-'
    if (ms < 1000) return `${ms}ms`
    const sec = Math.floor(ms / 1000)
    if (sec < 60) return `${sec}s`
    const m = Math.floor(sec / 60)
    const s = sec % 60
    return s ? `${m}m ${s}s` : `${m}m`
}

const { tier } = useViewport()
// Spec §5: settings tables must not overflow a phone; these columns are the
// ones a phone admin can live without (the row's dialog still shows all).
const phoneCols = computed(() => tier.value === 'phone')
</script>

<template>
    <div>
        <p v-if="!isLoading" class="queue-summary">
            <strong>Total: {{ counts.total }}</strong>
            <span v-if="counts.total > 0">
                · Waiting: {{ counts.waiting }} · Running: {{ counts.running }}
                · Complete: {{ counts.complete }} · Failed: {{ counts.failed }}
            </span>
        </p>
        <div class="table-fit">
            <DataTable
                :value="executions"
                :loading="isLoading"
                dataKey="id"
                stripedRows
                class="queue-table"
                @rowClick="(e: DataTableRowClickEvent) => emit('rowClick', e.data as ExecutionInfo)"
            >
                <template #empty><div class="empty-table">No executions found</div></template>
                <Column field="task_name" header="Task" style="width: 140px" />
                <Column header="Queued" :hidden="phoneCols" style="width: 180px">
                    <template #body="{ data }">{{ formatDate(data.queued_at) }}</template>
                </Column>
                <Column header="Duration" :hidden="phoneCols" style="width: 100px">
                    <template #body="{ data }">{{ duration(data) }}</template>
                </Column>
                <Column header="Status" style="width: 110px">
                    <template #body="{ data }">
                        <Tag :value="getStatusLabel(data.status)" :severity="getStatusSeverity(data.status)" />
                    </template>
                </Column>
                <Column header="Actions" style="width: 120px">
                    <template #body="{ data }">
                        <Button
                            v-if="isActiveStatus(data.status)"
                            label="Cancel"
                            icon="pi pi-times"
                            size="small"
                            severity="secondary"
                            :disabled="canceling"
                            @click.stop="emit('cancel', data.id)"
                        />
                    </template>
                </Column>
            </DataTable>
        </div>
    </div>
</template>

<style scoped>
.queue-summary {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    margin: 0 0 0.5rem;
}
.queue-table :deep(.p-datatable-tbody > tr) {
    cursor: pointer;
}
.empty-table {
    text-align: center;
    padding: 2rem;
    color: var(--app-text-secondary);
}
.table-fit {
    overflow-x: auto;
}
</style>
