<script setup lang="ts">
import { ref, computed } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import LogViewer from '@/components/admin/LogViewer.vue'
import { useCancelExecution } from '@/composables/useTasks'
import type { ExecutionInfo } from '@/types/tasks'

const props = defineProps<{
    executions: ExecutionInfo[]
    isLoading: boolean
}>()

const cancelMutation = useCancelExecution()

const logDialogVisible = ref(false)
const selectedExecution = ref<ExecutionInfo | null>(null)

const openLog = (execution: ExecutionInfo) => {
    selectedExecution.value = execution
    logDialogVisible.value = true
}

const cancelExec = (execution: ExecutionInfo) => {
    cancelMutation.mutate({
        taskName: execution.task_name,
        executionId: execution.id
    })
}

const statusSeverity = (
    status: string
): 'info' | 'warn' | 'success' | 'danger' | 'secondary' => {
    switch (status) {
        case 'Queued':
            return 'info'
        case 'Running':
            return 'warn'
        case 'Completed':
            return 'success'
        case 'Failed':
            return 'danger'
        case 'Cancelled':
            return 'secondary'
        default:
            return 'secondary'
    }
}

const formatDate = (dateStr?: string): string => {
    if (!dateStr) return '-'
    const d = new Date(dateStr)
    return d.toLocaleString()
}

const computeDuration = (execution: ExecutionInfo): string => {
    if (!execution.started_at || !execution.ended_at) return '-'
    const start = new Date(execution.started_at).getTime()
    const end = new Date(execution.ended_at).getTime()
    const diffMs = end - start
    if (diffMs < 1000) return `${diffMs}ms`
    const secs = Math.floor(diffMs / 1000)
    if (secs < 60) return `${secs}s`
    const mins = Math.floor(secs / 60)
    const remSecs = secs % 60
    return `${mins}m ${remSecs}s`
}

const isActive = (status: string): boolean => {
    return status === 'Queued' || status === 'Running'
}

const selectedTaskName = computed(() => selectedExecution.value?.task_name ?? '')
const selectedExecutionId = computed(() => selectedExecution.value?.id ?? '')
</script>

<template>
    <DataTable :value="executions" :loading="isLoading" stripedRows>
        <template #empty>
            <div class="empty-table">No executions found</div>
        </template>
        <Column field="task_name" header="Task" style="width: 120px" />
        <Column field="status" header="Status" style="width: 110px">
            <template #body="{ data }">
                <Tag :value="data.status" :severity="statusSeverity(data.status)" />
            </template>
        </Column>
        <Column header="Queued" style="width: 180px">
            <template #body="{ data }">{{ formatDate(data.queued_at) }}</template>
        </Column>
        <Column header="Started" style="width: 180px">
            <template #body="{ data }">{{ formatDate(data.started_at) }}</template>
        </Column>
        <Column header="Duration" style="width: 100px">
            <template #body="{ data }">{{ computeDuration(data) }}</template>
        </Column>
        <Column header="Actions" style="width: 140px">
            <template #body="{ data }">
                <div class="action-buttons">
                    <Button
                        icon="pi pi-file"
                        text
                        rounded
                        size="small"
                        v-tooltip="'View Log'"
                        @click="openLog(data)"
                    />
                    <Button
                        v-if="isActive(data.status)"
                        icon="pi pi-times"
                        text
                        rounded
                        size="small"
                        severity="danger"
                        v-tooltip="'Cancel'"
                        @click="cancelExec(data)"
                    />
                </div>
            </template>
        </Column>
    </DataTable>

    <LogViewer
        v-if="selectedExecution"
        v-model:visible="logDialogVisible"
        :taskName="selectedTaskName"
        :executionId="selectedExecutionId"
    />
</template>

<style scoped>
.empty-table {
    text-align: center;
    padding: 2rem;
    color: var(--app-text-secondary);
}

.action-buttons {
    display: flex;
    gap: 0.25rem;
}
</style>
